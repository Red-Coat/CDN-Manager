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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "git.redcoat.dev/cdn/pkg/api/v1alpha1"
	"git.redcoat.dev/cdn/pkg/provider"
	"git.redcoat.dev/cdn/pkg/provider/cloudfront"
	"git.redcoat.dev/cdn/pkg/provider/kubernetes"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
)

// DistributionReconciler reconciles a Distribution object
type DistributionReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	OriginResolver *kubernetes.OriginResolver
	Providers      []provider.CDNProvider
}

//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions/finalizers,verbs=update
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributionclasses,verbs=get;watch
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=clusterdistributionclasses,verbs=get;watch
//+kubebuilder:rbac:groups=v1,resources=services,verbs=get;watch
//+kubebuilder:rbac:groups=networking,resources=ingresses,verbs=get;watch

func (r *DistributionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	distro, class := r.Load(ctx, req)
	result, newStatus := r.ReconcileProviders(ctx, class, distro)

	if !reflect.DeepEqual(newStatus, distro.Status) {
		distro.Status = &newStatus
		r.Status().Update(ctx, &distro)
	}

	// If there hasn't been an error requiring immediate requeue, but we
	// aren't ready yet, we'll requeue in a minute
	if !result.Requeue && !distro.Status.Ready {
		result.RequeueAfter, _ = time.ParseDuration("1m")
	}

	return result, nil
}

// Loads a k8s Distribution and the DistributionClassSpec for its
// referenced distribution class
//
// The reason this gives the _spec_ for the distribution class is that
// it'll load either the ClusterDistributionClass or the namespaced
// DistributionClass, depending on which is referenced.
func (r *DistributionReconciler) Load(
	ctx context.Context,
	req ctrl.Request,
) (api.Distribution, api.DistributionClassSpec) {
	var distro api.Distribution
	r.Get(ctx, req.NamespacedName, &distro)

	if distro.Spec.DistributionClassRef.Kind == "ClusterDistributionClass" {
		var parent api.ClusterDistributionClass
		r.Get(ctx, client.ObjectKey{
			Name: distro.Spec.DistributionClassRef.Name,
		}, &parent)
		return distro, parent.Spec
	} else {
		var parent api.DistributionClass
		r.Get(ctx, client.ObjectKey{
			Namespace: distro.Namespace,
			Name:      distro.Spec.DistributionClassRef.Name,
		}, &parent)
		return distro, parent.Spec
	}
}

// Loops over the Providers and asks each one to reconicle if it has
// configuration for the distribution class
func (r *DistributionReconciler) ReconcileProviders(
	ctx context.Context,
	class api.DistributionClassSpec,
	distro api.Distribution,
) (ctrl.Result, api.DistributionStatus) {
	log := log.FromContext(ctx)
	resolvedOrigin, err := r.OriginResolver.Resolve(distro)

	if err != nil {
		log.Error(err, "resolved", resolvedOrigin)
		return ctrl.Result{}, api.DistributionStatus{Ready: false}
	}

	newStatus := api.DistributionStatus{Ready: true}

	var result ctrl.Result

	for _, provider := range r.Providers {
		if !provider.Wants(class) {
			continue
		}

		err := provider.Reconcile(class, distro, resolvedOrigin, &newStatus)

		if err != nil {
			// In the event of an error we'll requeue immediately
			result.Requeue = true
			newStatus.Ready = false
			log.Info("Error", "error", err)
		}
	}

	return result, newStatus
}

func Watch(
	builder *builder.Builder,
	mgr ctrl.Manager,
	kind client.Object,
	cache func(*api.Distribution) *api.ObjectReference,
	namespaced bool,
) {
	kindName := kind.GetObjectKind().GroupVersionKind().Kind
	ctx := context.Background()

	mgr.GetFieldIndexer().IndexField(ctx, &api.Distribution{}, kindName,
		func(object client.Object) []string {
			ref := cache(object.(*api.Distribution))
			if ref != nil && ref.Kind == kindName {
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
					requests[i] = ctrl.Request{
						NamespacedName: client.ObjectKeyFromObject(&distro),
					}
				}

				return requests
			},
		),
	)
}

func GetDistributionClassRef(distro *api.Distribution) *api.ObjectReference {
	return &distro.Spec.DistributionClassRef
}

func GetOriginTargetRef(distro *api.Distribution) *api.ObjectReference {
	return distro.Spec.Origin.Target
}

// SetupWithManager sets up the controller with the Manager.
func (r *DistributionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).For(&api.Distribution{})

	Watch(builder, mgr, &api.DistributionClass{}, GetDistributionClassRef, true)
	Watch(builder, mgr, &api.ClusterDistributionClass{}, GetDistributionClassRef, false)
	Watch(builder, mgr, &corev1.Service{}, GetOriginTargetRef, true)
	Watch(builder, mgr, &networking.Ingress{}, GetOriginTargetRef, true)

	r.Providers = []provider.CDNProvider{
		cloudfront.CloudFrontProvider{},
	}

	return builder.Complete(r)
}
