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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cfapi "gitlab.com/redcoat/cdn-manager/pkg/provider/cloudfront/api/v1alpha1"
)

func init() {
	SchemeBuilder.Register(&DistributionClass{}, &DistributionClassList{})
	SchemeBuilder.Register(&ClusterDistributionClass{}, &ClusterDistributionClassList{})
}

// A DistributionClass represents a namespaced-scoped configuration for
// a third party Content Delivery Network / Edge Cache (eg CloudFront).
// This holds generic configuration about a CDN integration, such as
// access tokens, or other CDN-specific configurations.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type DistributionClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DistributionClassSpec `json:"spec,omitempty"`
}

// A DistributionClass represents a cluster-scoped configuration for
// a third party Content Delivery Network / Edge Cache (eg CloudFront).
// This holds generic configuration about a CDN integration, such as
// access tokens, or other CDN-specific configurations.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
type ClusterDistributionClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DistributionClassSpec `json:"spec,omitempty"`
}

// Configuration for the DistributionClass or ClusterDistributionClass
// resource
type DistributionClassSpec struct {
	Providers ProviderList `json:"providers"`
}

type ProviderList struct {
	// If this block exists, Distributions referencing this
	// DistributionClass will be setup in CloudFront. You can specify an
	// Access Key and Secret key in this block, or authorize the pod
	// directly by setting the AWS_ACCESS_KEY_ID / AWS_ACCESS_KEY_ID
	// environment variables, or using AWS' IAM Roles For Service Accounts
	// (IRSA) Controller.
	// +optional
	CloudFront *cfapi.CloudFrontSpec `json:"cloudfront,omitempty"`
}

// DistributionClassList contains a list of DistributionClasses
// +kubebuilder:object:root=true
type DistributionClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DistributionClass `json:"items"`
}

// ClusterDistributionClassList contains a list of ClusterDistributionClass
//+kubebuilder:object:root=true
type ClusterDistributionClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterDistributionClass `json:"items"`
}
