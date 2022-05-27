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

	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "gitlab.com/redcoat/cdn-manager/pkg/api/v1alpha1"
	"gitlab.com/redcoat/cdn-manager/pkg/resolver"
	"gitlab.com/redcoat/cdn-manager/pkg/util"
)

// +kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;watch;list

type IngressReconciler struct {
	client.Client

	// The current scheme we are working with
	Scheme *runtime.Scheme

	IngressService *client.ObjectKey
}

// Creates a new IngressController
func NewIngressController(mgr ctrl.Manager, ingressService string) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networking.Ingress{}).
		Owns(&api.Distribution{}).
		Complete(&IngressReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			IngressService: util.ObjectKeyFromString(ingressService),
		})
}

// The main reconciliation loop
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("Ingress Reconciliation")

	var ingress networking.Ingress
	r.Get(ctx, req.NamespacedName, &ingress)

	class := resolver.GetDistributionClass(&ingress)

	if class == nil {
		log.V(1).Info("Ignoring ingress without annotations")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("class", class)
	log.Info("Starting Reconciliation")

	var distro api.Distribution
	err := r.Get(ctx, req.NamespacedName, &distro)

	desired := r.getDesiredDistribution(ingress, *class)
	if desired.Spec.Origin.Host == "" {
		log.V(-1).Info("Unable to determine origin for ingress. Skipping")
		return ctrl.Result{}, nil
	}

	if err != nil {
		resolver.AddDistributionMeta(&ingress, &desired)

		err := r.Create(ctx, &desired)
		if err != nil {
			log.V(-3).Error(err, "Couldn't create distribution")
		}
	} else {
		if !reflect.DeepEqual(desired.Spec, distro.Spec) {
			log.V(1).Info("Distribution is out of sync!")

			distro.Spec = desired.Spec
			err := r.Update(ctx, &distro)
			if err != nil {
				log.V(-3).Error(err, "Couldn't update distribution")
			}
		}
	}

	return ctrl.Result{}, nil
}

// Returns a Distribution with the desired Spec for this Ingress
func (r *IngressReconciler) getDesiredDistribution(
	ingress networking.Ingress,
	class api.ObjectReference,
) api.Distribution {
	var ingressLB []corev1.LoadBalancerIngress
	if r.IngressService == nil {
		ingressLB = ingress.Status.LoadBalancer.Ingress
	} else {
		var svc corev1.Service
		r.Get(context.TODO(), *r.IngressService, &svc)
		ingressLB = svc.Status.LoadBalancer.Ingress
	}

	desired := resolver.DistributionFromIngress(class, ingressLB)

	// Currently only one TLS certificate is supported and hosts are only
	// added if TLS is enabled.
	// TODO: Add hosts from ingress rules so TLS is not required
	// TODO: Decide how to handle multiple IngressTLS entries
	if len(ingress.Spec.TLS) > 0 {
		desired.Spec.Hosts = ingress.Spec.TLS[0].Hosts
		desired.Spec.TLS = &api.TLSSpec{
			SecretRef: ingress.Spec.TLS[0].SecretName,
			// A sensible default for the moment
			Mode: "redirect",
		}
	}

	return desired
}
