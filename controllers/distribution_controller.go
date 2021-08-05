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

package controllers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdnv1alpha1 "redcoat.dev/cdn/api/v1alpha1"
	"redcoat.dev/cdn/provider/cloudfront"
	"redcoat.dev/cdn/provider/kubernetes"
)

// DistributionReconciler reconciles a Distribution object
type DistributionReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	OriginResolver *kubernetes.OriginResolver
}

//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions/finalizers,verbs=update
//+kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributionclasses,verbs=get;watch

func (r *DistributionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var distro cdnv1alpha1.Distribution
	var class cdnv1alpha1.DistributionClassSpec

	r.Get(ctx, req.NamespacedName, &distro)

	if distro.Spec.DistributionClassRef.Kind == "ClusterDistributionClass" {
		var parent cdnv1alpha1.ClusterDistributionClass
		r.Get(ctx, client.ObjectKey{
			Name: distro.Spec.DistributionClassRef.Name,
		}, &parent)
		class = parent.Spec
	} else {
		var parent cdnv1alpha1.DistributionClass
		r.Get(ctx, client.ObjectKey{
			Namespace: distro.Namespace,
			Name:      distro.Spec.DistributionClassRef.Name,
		}, &parent)
		class = parent.Spec
	}

	resolvedOrigin, err := r.OriginResolver.Resolve(distro)

	if err != nil {
		log.Info("Bad Origin Configuration", "error", err, "resolved", resolvedOrigin)
		return ctrl.Result{}, nil
	}

	if class.Providers.CloudFront == nil {
		log.Info("CloudFront not requested. Ignoring Resource")
		// TODO Check status and cleanup distro if needed

		return ctrl.Result{}, nil
	}

	log.Info("CloudFront requested, processing...")

	provider, _ := cloudfront.NewProvider(&distro, *resolvedOrigin)

	if provider.GetStatus() != nil {
		log.Info("Status found, performing checks")
		err := provider.Check()

		if err != nil {
			log.Info("Error", "error", err)
		}
	} else {
		log.Info("No status found, creating resource")
		err := provider.Create()

		if err != nil {
			log.Info("Error", "error", err)
		}
	}

	if provider.IsDirty() {
		distro.Status.Ready = provider.IsReady()
		distro.Status.Endpoints = provider.Endpoints
		r.Status().Update(ctx, &distro)
	}

	var result ctrl.Result

	if provider.NeedsRecheck() {
		result.RequeueAfter, _ = time.ParseDuration("1m")
	}

	return result, nil
}

func (r *DistributionReconciler) Watch(
	builder *builder.Builder,
	mgr ctrl.Manager,
	kind client.Object,
	cacheKey string,
	cache func(*cdnv1alpha1.Distribution) string,
	namespaced bool,
) {
	mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&cdnv1alpha1.Distribution{},
		cacheKey,
		func(object client.Object) []string {
			if key := cache(object.(*cdnv1alpha1.Distribution)); key != "" {
				return []string{key}
			} else {
				return []string{}
			}
		},
	)

	builder.Watches(
		&source.Kind{Type: kind},
		handler.EnqueueRequestsFromMapFunc(
			func(object client.Object) []ctrl.Request {
				ctx := context.Background()

				var distroList cdnv1alpha1.DistributionList
				predicate := client.MatchingFields{cacheKey: object.GetName()}

				if namespaced {
					r.List(ctx, &distroList, predicate,
						client.InNamespace(object.GetNamespace()),
					)
				} else {
					r.List(ctx, &distroList, predicate)
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

func DistributionClassRefChecker(Kind string) func(*cdnv1alpha1.Distribution) string {
	return func(distro *cdnv1alpha1.Distribution) string {
		if distro.Spec.DistributionClassRef.Kind == Kind {
			return distro.Spec.DistributionClassRef.Name
		} else {
			return ""
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DistributionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).For(&cdnv1alpha1.Distribution{})

	r.Watch(
		builder,
		mgr,
		&cdnv1alpha1.DistributionClass{},
		"distributionClass",
		DistributionClassRefChecker("DistributionClass"),
		true,
	)
	r.Watch(
		builder,
		mgr,
		&cdnv1alpha1.ClusterDistributionClass{},
		"clusterDistributionClass",
		DistributionClassRefChecker("ClusterDistributionClass"),
		false,
	)

	return builder.Complete(r)
}
