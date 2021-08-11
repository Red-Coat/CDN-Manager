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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "git.redcoat.dev/cdn/pkg/api/v1alpha1"
	"git.redcoat.dev/cdn/pkg/util"
	networking "k8s.io/api/networking/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AnnotationDistributionClass        = "cdn.redcoat.dev/distribution-class"
	AnnotationClusterDistributionClass = "cdn.redcoat.dev/cluster-distribution-class"
)

// +kubebuilder:rbac:groups=cdn.redcoat.dev,resources=distributions,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=networking,resources=ingresses,verbs=get;watch
type IngressReconciler struct {
	client.Client

	// The current scheme we are working with
	Scheme *runtime.Scheme
}

// Creates a new IngressController
func NewIngressController(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networking.Ingress{}).
		Owns(&api.Distribution{}).
		Complete(&IngressReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		})
}

// The main reconciliation loop
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("Ingress Reconciliation")

	var ingress networking.Ingress
	r.Get(ctx, req.NamespacedName, &ingress)

	class := getDistributionClass(ingress)

	if class == nil {
		log.V(1).Info("Ignoring ingress without annotations")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("class", class)
	log.Info("Starting Reconciliation")

	var distros api.DistributionList
	r.List(ctx, &distros,
		client.InNamespace(req.Namespace),
		client.MatchingLabels(ingress.GetLabels()),
	)

	if len(distros.Items) == 0 {
		desired := getDesiredDistribution(ingress, *class)
		addDistributionMeta(ingress, &desired)

		err := r.Create(ctx, &desired)
		if err != nil {
			log.V(-3).Error(err, "Couldn't create distribution")
		}
	} else if len(distros.Items) == 1 {
		distro := distros.Items[0]
		desired := getDesiredDistribution(ingress, *class)
		if !reflect.DeepEqual(desired.Spec, distro.Spec) {
			log.V(1).Info("Distribution is out of sync!")

			distros.Items[0].Spec = desired.Spec
			err := r.Update(ctx, &distros.Items[0])
			if err != nil {
				log.V(-3).Error(err, "Couldn't update distribution")
			}
		}
	} else {
		log.V(-3).Info("Unexpected number of distributions matching ingress")
	}

	return ctrl.Result{}, nil
}

// Looks at the annotations on the Ingress object and tries to determine
// the (Cluster)DistributionClass that is desired.
func getDistributionClass(ingress networking.Ingress) *api.ObjectReference {
	annotations := ingress.GetAnnotations()
	if class := annotations[AnnotationDistributionClass]; class != "" {
		return &api.ObjectReference{
			Kind: "DistributionClass",
			Name: class,
		}
	} else if class := annotations[AnnotationClusterDistributionClass]; class != "" {
		return &api.ObjectReference{
			Kind: "ClusterDistributionClass",
			Name: class,
		}
	}

	// No matching annotation found
	return nil
}

// Returns a Distribution with the desired Spec for this Ingress
func getDesiredDistribution(ingress networking.Ingress, class api.ObjectReference) api.Distribution {
	desired := api.Distribution{
		Spec: api.DistributionSpec{
			DistributionClassRef: class,
			Origin: api.Origin{
				Host:      util.GetIngressHost(ingress.Status.LoadBalancer.Ingress),
				HTTPPort:  &api.ServicePort{Number: 80},
				HTTPSPort: &api.ServicePort{Number: 443},
			},
			SupportedMethods: []string{"GET", "HEAD"},
		},
	}

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

// Adds the default metadata to the given Distribuition
//
// This is only called at creation, and not on update so that third
// party annotations, finalizers, etc, are not cleared.
func addDistributionMeta(ingress networking.Ingress, distro *api.Distribution) {
	distro.SetName(ingress.GetName())
	distro.SetLabels(ingress.GetLabels())
	distro.SetNamespace(ingress.GetNamespace())

	// Controller needs to be a *bool so we'll define the value here and
	// then pass the reference
	truth := true
	kind := ingress.GetObjectKind().GroupVersionKind()
	distro.SetOwnerReferences([]meta.OwnerReference{meta.OwnerReference{
		APIVersion: kind.Group + "/" + kind.Version,
		Kind:       kind.Kind,
		Name:       ingress.GetName(),
		UID:        ingress.GetUID(),
		Controller: &truth,
	}})
}
