/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "gitlab.com/redcoat/cdn-manager/pkg/api/v1alpha1"
	"gitlab.com/redcoat/cdn-manager/pkg/handler"
	"gitlab.com/redcoat/cdn-manager/pkg/provider"
	"gitlab.com/redcoat/cdn-manager/pkg/provider/cloudfront"
	"gitlab.com/redcoat/cdn-manager/pkg/resolver"
)

// The name of the finalizer used by this controller to manage the
// cleanup of distributions on resource deletion
const finalizer = "cdn.redcoat.dev/finalizer"

// The DistributionReconciler contains all of the top level logic for
// reconciling Distribution resources
//
// Although it performs the generic actions required during a
// reconciliation, most of the specific actions are performed by the
// relavent cloud-specific providers.
//
// +kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions/finalizers,verbs=update
// +kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributionclasses,verbs=get;watch;list
// +kubebuilder:rbac:groups=cdn.redcoat.dev,resources=clusterdistributionclasses,verbs=get;watch;list
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;watch;list

type DistributionReconciler struct {
	resolver.DistributionClassReader

	// Used to load the required certificate for the distribution's TLS
	// settings
	CertificateResolver resolver.CertificateResolver

	// List of providers supported
	Providers []provider.CDNProvider

	// The generic Logger interface for the reconciller
	Logger logr.Logger

	// The specific Logger in use during the current run (this has values
	// added for the current Distribution & DistributionClass)
	log logr.Logger
}

// SetupWithManager sets up the controller with the Manager.
func NewDistributionController(mgr ctrl.Manager, logger logr.Logger) error {
	client := mgr.GetClient()

	return ctrl.NewControllerManagedBy(mgr).For(&api.Distribution{}).
		Watches(handler.BuildIndexedReferenceWatcher(client, &api.DistributionClass{})).
		Watches(handler.BuildIndexedReferenceWatcher(client, &api.ClusterDistributionClass{})).
		Watches(handler.BuildIndexedReferenceWatcher(client, &corev1.Secret{})).
		Complete(&DistributionReconciler{
			DistributionClassReader: resolver.DistributionClassReader{Client: client},
			Logger:                  logger.WithName("ctrl"),
			CertificateResolver:     resolver.CertificateResolver{Client: client},
			Providers: []provider.CDNProvider{
				cloudfront.CloudFrontProvider{},
			},
		})
}

// Main function called when a reconciliation is required
//
// This method's primary job is loading up the resources in question
// (the Distribution and associated DistributionClass or
// ClusterDistributionClass), and then kicking off either a
// reconciliation job or a deletion job.
func (r *DistributionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = r.Logger.WithValues("distribution", req.Namespace+"/"+req.Name)
	r.log.Info("Reconcilliation")

	var distro api.Distribution
	if err := r.Get(ctx, req.NamespacedName, &distro); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	class, err := r.GetDistributionClassSpec(ctx, distro.Spec.DistributionClassRef, &distro)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if distro.ObjectMeta.DeletionTimestamp.IsZero() {
		r.log.V(1).Info("Starting Reconcilliation Loop")
		if !controllerutil.ContainsFinalizer(&distro, finalizer) {
			controllerutil.AddFinalizer(&distro, finalizer)
			r.Update(ctx, &distro)
		}

		return r.reconcileProviders(ctx, *class, distro), nil
	} else if controllerutil.ContainsFinalizer(&distro, finalizer) {
		r.log.V(1).Info("Starting Deletion Loop")
		allDeleted, result := r.deleteProviders(ctx, *class, distro)

		if allDeleted {
			r.log.Info("Deletion Complete. Removing Fianlizer")
			controllerutil.RemoveFinalizer(&distro, finalizer)
			r.Update(ctx, &distro)
		}

		return result, nil
	}

	return ctrl.Result{}, nil
}

// Loops over the Providers and asks each one to reconicle if it has
// configuration for the distribution class
func (r *DistributionReconciler) reconcileProviders(
	ctx context.Context,
	class api.DistributionClassSpec,
	distro api.Distribution,
) ctrl.Result {
	var err error
	var cert *resolver.Certificate
	if tls := distro.Spec.TLS; tls != nil {
		r.log.V(1).Info("Distro has TLS. Running CertificateResolver")
		cert, err = r.CertificateResolver.Resolve(client.ObjectKey{
			Namespace: distro.Namespace,
			Name:      tls.SecretRef,
		})
		if err != nil {
			r.log.Error(err, "Unable to load certificate")
			r.updateStatus(ctx, api.DistributionStatus{Ready: false}, distro)
			return ctrl.Result{}
		}
	}

	newStatus := distro.Status.DeepCopy()
	newStatus.Ready = true

	var result ctrl.Result

	for _, provider := range r.Providers {
		if !provider.Wants(class) {
			continue
		}

		err := provider.Reconcile(class, distro, cert, newStatus)

		if err != nil {
			// In the event of an error we'll requeue immediately
			result.Requeue = true
			newStatus.Ready = false
			r.log.Error(err, "Unable to run provider")
		}
	}

	// If there hasn't been an error requiring immediate requeue, but we
	// aren't ready yet, we'll requeue in a minute
	r.requeueIfNotReady(&result, newStatus.Ready)

	r.updateStatus(ctx, *newStatus, distro)

	return result
}

// Loops over the controllers and asks each one to delete
func (r *DistributionReconciler) deleteProviders(
	ctx context.Context,
	class api.DistributionClassSpec,
	distro api.Distribution,
) (bool, ctrl.Result) {
	log := log.FromContext(ctx)

	var result ctrl.Result
	newStatus := distro.Status.DeepCopy()
	newStatus.Ready = false
	allDeleted := true

	for _, provider := range r.Providers {
		if !provider.Has(distro.Status) {
			continue
		}

		err := provider.Delete(class, distro, newStatus)

		if err != nil {
			result.Requeue = true
			log.Info("Error", "error", err)
		}

		allDeleted = allDeleted && !provider.Has(*newStatus)
	}

	r.requeueIfNotReady(&result, allDeleted)
	r.updateStatus(ctx, *newStatus, distro)

	return allDeleted, result
}

// Checks to see if the status has been updated during the
// reconciliation and updates it with the api-server if it has done
func (r *DistributionReconciler) updateStatus(
	ctx context.Context,
	newStatus api.DistributionStatus,
	distro api.Distribution,
) {
	if !reflect.DeepEqual(newStatus, distro.Status) {
		r.log.V(1).Info("Status change detected. Updating with api-server")
		distro.Status = newStatus
		r.Status().Update(ctx, &distro)
	}
}

// Checks to see if the given condition is not met and adds a requeue in
// one minute request to the given result
//
// The "condition" depends on the caller - for the reconciliation code,
// this is normally "Status.Ready", for the deletion code, this is if
// all resources have been deleted.
//
// NB: This method checks to see if the Requeue flag has already been
// set on the result. If it has, it does not add a 1m RequeueAfter as
// the Requeue flag is assumed to mean "requeue immediately" (this is
// normally set in the event of failure).
func (r *DistributionReconciler) requeueIfNotReady(result *ctrl.Result, condition bool) {
	if !result.Requeue && !condition {
		r.log.Info("Resource is not in desired state. Scheduling recheck in 1m")
		result.RequeueAfter, _ = time.ParseDuration("1m")
	}
}
