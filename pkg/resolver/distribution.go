/*
Copyright 2021 Red Coat Development Ltd.

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

package resolver

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "gitlab.com/redcoat/cdn-manager/pkg/api/v1alpha1"
)

const (
	AnnotationDistributionClass        = "cdn.redcoat.dev/distribution-class"
	AnnotationClusterDistributionClass = "cdn.redcoat.dev/cluster-distribution-class"
)

// Looks at the annotations on the given object and tries to determine
// the (Cluster)DistributionClass that is desired.
func GetDistributionClass(object client.Object) *api.ObjectReference {
	annotations := object.GetAnnotations()
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

// Adds the default metadata to the given Distribuition
//
// This is only called at creation, and not on update so that third
// party annotations, finalizers, etc, are not cleared.
func AddDistributionMeta(object client.Object, distro *api.Distribution) {
	distro.SetName(object.GetName())
	distro.SetLabels(object.GetLabels())
	distro.SetNamespace(object.GetNamespace())

	// Controller needs to be a *bool so we'll define the value here and
	// then pass the reference
	truth := true
	kind := object.GetObjectKind().GroupVersionKind()
	distro.SetOwnerReferences([]meta.OwnerReference{meta.OwnerReference{
		APIVersion: kind.Group + "/" + kind.Version,
		Kind:       kind.Kind,
		Name:       object.GetName(),
		UID:        object.GetUID(),
		Controller: &truth,
	}})
}
