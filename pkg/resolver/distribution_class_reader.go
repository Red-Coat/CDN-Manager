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
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	api "gitlab.com/redcoat/k8s-cdn-controller/pkg/api/v1alpha1"
)

// A DistributionClassReader is a kind of client.Reader which is
// tailored to loading DistributionClass resources from a given Object
// Reference
type DistributionClassReader struct {
	client.Client
}

// Gets a DistributionClassSpec for the given Object Reference and
// calling Object
//
// If it is not a cluster-scoped type, the Namespace of the given
// calling Object will be used in the query.
func (r *DistributionClassReader) GetDistributionClassSpec(
	ctx context.Context,
	ref api.ObjectReference,
	obj client.Object,
) (*api.DistributionClassSpec, error) {
	switch ref.Kind {
	case "ClusterDistributionClass":
		var parent api.ClusterDistributionClass
		err := r.Get(ctx, client.ObjectKey{Name: ref.Name}, &parent)
		return &parent.Spec, err
	case "DistributionClass":
		var parent api.DistributionClass
		err := r.Get(ctx, client.ObjectKey{
			Name:      ref.Name,
			Namespace: obj.GetNamespace(),
		}, &parent)
		return &parent.Spec, err
	default:
		return nil, fmt.Errorf("Passed ObjectReference was not acceptable")
	}
}
