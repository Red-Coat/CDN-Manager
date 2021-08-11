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
package provider

import (
	api "git.redcoat.dev/cdn/pkg/api/v1alpha1"
	"git.redcoat.dev/cdn/pkg/resolver"
)

// A CDNProvider is the top level logic holder for a CDN integration (eg
// CloudFront)
type CDNProvider interface {
	// Checks if the given DistributionClassSpec includes details for this
	// provider
	//
	// Normally this would be a simple check:
	//   spec.Providers.[ProviderName] != nil
	Wants(api.DistributionClassSpec) bool

	// Checks if the given DistributionStatus already has a status for
	// this provider
	//
	// Normally this would be a simple check:
	//   status.[ProviderName] != nil
	Has(api.DistributionStatus) bool

	// Creates a specific DistributionProvider for the given Distribution,
	// ResolvedOrigin, and DistribitionClassSpec
	//
	// This is typically called by the DistributionController after it has
	// determined if this CDNProvider is likely to be interested in the
	// Distribution (via a Wants() check).
	//
	// It is passed a pointer to the DistributionStatus as it is expected
	// to make changes to its status. The Distribution itself is
	// immutable.
	Reconcile(
		api.DistributionClassSpec,
		api.Distribution,
		*resolver.Certificate,
		*api.DistributionStatus,
	) error

	Delete(
		api.DistributionClassSpec,
		api.Distribution,
		*api.DistributionStatus,
	) error
}
