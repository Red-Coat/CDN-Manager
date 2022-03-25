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

package indexer

import (
	ctrl "sigs.k8s.io/controller-runtime"

	api "gitlab.com/redcoat/k8s-cdn-controller/pkg/api/v1alpha1"
)

// Sets up the field indexes for Distribution resources
//
// These are:
// - DistributionClasses referenced in DistributionClassRef
// - ClusterDistributionClasses referenced in DistributionClassRef
// - Secrets referenced in TLS.SecretRef
func SetUpDistributionIndexers(mgr ctrl.Manager) {
	NewIndexer(mgr, "Secret", GetSecretRef)
	NewObjectReferenceIndexer(mgr, "DistributionClass", GetDistributionClassRef)
	NewObjectReferenceIndexer(mgr, "ClusterDistributionClass", GetDistributionClassRef)
}

// Returns the DistributionClassRef for the given Distribution
func GetDistributionClassRef(distro api.Distribution) api.ObjectReference {
	return distro.Spec.DistributionClassRef
}

// Returns the secret name for the given Distribution
//
// If TLS is not specified, an empty string is returned.
func GetSecretRef(distro api.Distribution) string {
	if tlsSpec := distro.Spec.TLS; tlsSpec != nil {
		return tlsSpec.SecretRef
	} else {
		return ""
	}
}
