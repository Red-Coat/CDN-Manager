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

	api "redcoat.dev/cdn/api/v1alpha1"
	"redcoat.dev/cdn/provider"
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

	var distro api.Distribution
	var class api.DistributionClassSpec

	r.Get(ctx, req.NamespacedName, &distro)

	if distro.Spec.DistributionClassRef.Kind == "ClusterDistributionClass" {
		var parent api.ClusterDistributionClass
		r.Get(ctx, client.ObjectKey{
			Name: distro.Spec.DistributionClassRef.Name,
		}, &parent)
		class = parent.Spec
	} else {
		var parent api.DistributionClass
		r.Get(ctx, client.ObjectKey{
			Namespace: distro.Namespace,
			Name:      distro.Spec.DistributionClassRef.Name,
		}, &parent)
		class = parent.Spec
	}

	resolvedOrigin, err := r.OriginResolver.Resolve(distro)

	if err != nil {
		log.Error(err, "resolved", resolvedOrigin)
		return ctrl.Result{}, nil
	}

	dirty := false
	recheck := false
	distro.Status.Ready = true
	distro.Status.Endpoints = []api.Endpoint{}
	providers := []provider.Provider{
		cloudfront.NewProvider(&distro, *resolvedOrigin),
	}

	for _, provider := range providers {
		if !provider.Wants(class) {
			continue
		}

		var err error
		if provider.Has() {
			err = provider.Check()
		} else {
			err = provider.Create()
		}

		if err != nil {
			log.Info("Error", "error", err)
		}

		dirty = provider.IsDirty() || dirty
		recheck = recheck || provider.NeedsRecheck()
		distro.Status.Ready = distro.Status.Ready && provider.IsReady()
		distro.Status.Endpoints = append(distro.Status.Endpoints, provider.GetEndpoints()...)
	}

	if dirty {
		r.Status().Update(ctx, &distro)
	}

	var result ctrl.Result

	if recheck {
		result.RequeueAfter, _ = time.ParseDuration("1m")
	}

	return result, nil
}

func (r *DistributionReconciler) Watch(
	builder *builder.Builder,
	mgr ctrl.Manager,
	kind client.Object,
	cacheKey string,
	cache func(*api.Distribution) string,
	namespaced bool,
) {
	mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&api.Distribution{},
		cacheKey,
		func(object client.Object) []string {
			if key := cache(object.(*api.Distribution)); key != "" {
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

				var distroList api.DistributionList
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

func DistributionClassRefChecker(Kind string) func(*api.Distribution) string {
	return func(distro *api.Distribution) string {
		if distro.Spec.DistributionClassRef.Kind == Kind {
			return distro.Spec.DistributionClassRef.Name
		} else {
			return ""
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DistributionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).For(&api.Distribution{})

	r.Watch(
		builder,
		mgr,
		&api.DistributionClass{},
		"distributionClass",
		DistributionClassRefChecker("DistributionClass"),
		true,
	)
	r.Watch(
		builder,
		mgr,
		&api.ClusterDistributionClass{},
		"clusterDistributionClass",
		DistributionClassRefChecker("ClusterDistributionClass"),
		false,
	)

	return builder.Complete(r)
}
