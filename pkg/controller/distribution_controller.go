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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "git.redcoat.dev/cdn/pkg/api/v1alpha1"
	"git.redcoat.dev/cdn/pkg/provider"
	"git.redcoat.dev/cdn/pkg/provider/cloudfront"
	"git.redcoat.dev/cdn/pkg/resolver"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
)

// DistributionReconciler reconciles a Distribution object
type DistributionReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	OriginResolver      *resolver.OriginResolver
	CertificateResolver *resolver.CertificateResolver
	Providers           []provider.CDNProvider
	Logger              logr.Logger
	log                 logr.Logger
}

const FINALIZER = "cdn.redcoat.dev/finalizer"

//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions/finalizers,verbs=update
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributionclasses,verbs=get;watch
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=clusterdistributionclasses,verbs=get;watch
//+kubebuilder:rbac:groups=v1,resources=services,verbs=get;watch
//+kubebuilder:rbac:groups=networking,resources=ingresses,verbs=get;watch

func (r *DistributionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = r.Logger.WithValues("distribution", req.Namespace+"/"+req.Name)
	r.log.Info("Reconcilliation")

	distro, class := r.load(ctx, req)

	if distro == nil {
		return ctrl.Result{}, nil
	}

	if distro.ObjectMeta.DeletionTimestamp.IsZero() {
		r.log.V(1).Info("Starting Reconcilliation Loop")
		if !controllerutil.ContainsFinalizer(distro, FINALIZER) {
			controllerutil.AddFinalizer(distro, FINALIZER)
			r.Update(ctx, distro)
		}

		return r.reconcileProviders(ctx, *class, *distro), nil
	} else if controllerutil.ContainsFinalizer(distro, FINALIZER) {
		r.log.V(1).Info("Starting Deletion Loop")
		allDeleted, result := r.deleteProviders(ctx, *class, *distro)

		if allDeleted {
			r.log.Info("Deletion Complete. Removing Fianlizer")
			controllerutil.RemoveFinalizer(distro, FINALIZER)
			r.Update(ctx, distro)
		}

		return result, nil
	}

	return ctrl.Result{}, nil
}

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

// Loads a k8s Distribution and the DistributionClassSpec for its
// referenced distribution class
//
// The reason this gives the _spec_ for the distribution class is that
// it'll load either the ClusterDistributionClass or the namespaced
// DistributionClass, depending on which is referenced.
func (r *DistributionReconciler) load(
	ctx context.Context,
	req ctrl.Request,
) (*api.Distribution, *api.DistributionClassSpec) {
	var distro api.Distribution
	var class api.DistributionClassSpec
	err := r.Get(ctx, req.NamespacedName, &distro)

	if err != nil {
		r.log.V(-3).Error(err, "Could not load distribution")
		return nil, nil
	}

	r.log = r.log.WithValues("class", distro.Spec.DistributionClassRef.Name)
	r.log.V(1).Info(distro.Spec.DistributionClassRef.Kind)

	if distro.Spec.DistributionClassRef.Kind == "ClusterDistributionClass" {
		var parent api.ClusterDistributionClass
		err = r.Get(ctx, client.ObjectKey{
			Name: distro.Spec.DistributionClassRef.Name,
		}, &parent)
		class = parent.Spec
	} else {
		var parent api.DistributionClass
		err = r.Get(ctx, client.ObjectKey{
			Namespace: distro.Namespace,
			Name:      distro.Spec.DistributionClassRef.Name,
		}, &parent)
		class = parent.Spec
	}

	if err != nil {
		r.log.V(-3).Error(err, "Could not load distribution class")
		return nil, nil
	}

	return &distro, &class
}

func (r *DistributionReconciler) requeueIfNotReady(result *ctrl.Result, condition bool) {
	if !result.Requeue && !condition {
		r.log.Info("Resource is not in desired state. Scheduling recheck in 1m")
		result.RequeueAfter, _ = time.ParseDuration("1m")
	}
}

// Loops over the Providers and asks each one to reconicle if it has
// configuration for the distribution class
func (r *DistributionReconciler) reconcileProviders(
	ctx context.Context,
	class api.DistributionClassSpec,
	distro api.Distribution,
) ctrl.Result {
	resolvedOrigin, err := r.OriginResolver.Resolve(distro)
	if err != nil {
		r.log.Error(err, "Unable to resolve origin")
		r.updateStatus(ctx, api.DistributionStatus{Ready: false}, distro)
		return ctrl.Result{}
	}

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

		err := provider.Reconcile(class, distro, resolvedOrigin, cert, newStatus)

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

func watch(
	log logr.Logger,
	builder *builder.Builder,
	mgr ctrl.Manager,
	kind client.Object,
	cache func(*api.Distribution) *api.ObjectReference,
	namespaced bool,
) {
	kindName := reflect.TypeOf(kind).Elem().Name()
	ctx := context.Background()
	log = log.WithValues("kind", kindName)

	mgr.GetFieldIndexer().IndexField(ctx, &api.Distribution{}, kindName,
		func(object client.Object) []string {
			ref := cache(object.(*api.Distribution))
			if ref != nil && ref.Kind == kindName {
				log.V(2).Info(
					"Indexing Distribution",
					"ref", ref.Name,
					"distribution", client.ObjectKeyFromObject(object).String(),
				)
				return []string{ref.Name}
			} else {
				return []string{}
			}
		},
	)

	builder.Watches(
		&source.Kind{Type: kind},
		handler.EnqueueRequestsFromMapFunc(
			func(object client.Object) []ctrl.Request {
				log := log.WithValues("resource", client.ObjectKeyFromObject(object))
				log.V(2).Info("Detected change")

				var distroList api.DistributionList
				predicate := client.MatchingFields{kindName: object.GetName()}

				if namespaced {
					mgr.GetClient().List(ctx, &distroList, predicate,
						client.InNamespace(object.GetNamespace()),
					)
				} else {
					mgr.GetClient().List(ctx, &distroList, predicate)
				}

				requests := make([]ctrl.Request, len(distroList.Items))
				for i, distro := range distroList.Items {
					key := client.ObjectKeyFromObject(&distro)
					log.V(1).Info("Queuing Reconcilliation", "distribution", key.String())
					requests[i] = ctrl.Request{NamespacedName: key}
				}

				return requests
			},
		),
	)
}

func getDistributionClassRef(distro *api.Distribution) *api.ObjectReference {
	return &distro.Spec.DistributionClassRef
}

func getOriginTargetRef(distro *api.Distribution) *api.ObjectReference {
	return distro.Spec.Origin.Target
}

func getSecretRef(distro *api.Distribution) *api.ObjectReference {
	if tlsSpec := distro.Spec.TLS; tlsSpec != nil {
		if secret := tlsSpec.SecretRef; secret != "" {
			return &api.ObjectReference{
				Kind: "Secret",
				Name: secret,
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DistributionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).For(&api.Distribution{})

	r.Logger = r.Logger.WithName("ctrl")

	log := r.Logger.WithName("watch")
	watch(log, builder, mgr, &api.DistributionClass{}, getDistributionClassRef, true)
	watch(log, builder, mgr, &api.ClusterDistributionClass{}, getDistributionClassRef, false)
	watch(log, builder, mgr, &corev1.Secret{}, getSecretRef, true)
	watch(log, builder, mgr, &corev1.Service{}, getOriginTargetRef, true)
	watch(log, builder, mgr, &networking.Ingress{}, getOriginTargetRef, true)

	r.Providers = []provider.CDNProvider{
		cloudfront.CloudFrontProvider{},
	}

	r.OriginResolver = &resolver.OriginResolver{Client: mgr.GetClient()}
	r.CertificateResolver = &resolver.CertificateResolver{Client: mgr.GetClient()}

	return builder.Complete(r)
}
