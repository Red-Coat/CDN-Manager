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
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The access details for cloudfront distributions
// If this section is provided, a cloudfront distribution will be setup,
// even if access details are not given in this block.
type CloudFrontSpec struct {
	// The AccessKeyID is used for authentication.
	// If not set we fall-back to using env vars, shared credentials file
	// or AWS Instance metadata
	// see: https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials
	// +optional
	AccessKeyID string `json:"accessKeyID,omitempty"`

	// The SecretAccessKey is used for authentication. If not set we
	// fall-back to using env vars, shared credentials file or AWS
	// Instance metadata
	// https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials
	// +optional
	SecretAccessKey *cmmeta.SecretKeySelector `json:"secretAccessKeySecretRef"`
}

type ProviderList struct {
	// Deploy Distributions to AWS CloudFront
	// +optional
	CloudFront *CloudFrontSpec `json:"cloudfront,omitempty"`
}

// Desired state of the DistributionClass Resource
type DistributionClassSpec struct {
	// List of one or more providers to deploy Distributions to. At least
	// one must be provided
	Providers ProviderList `json:"providers"`
}

// The current state of the DistributionClass Resource
type DistributionClassStatus struct {
	// Currently unused, this exists for possible future usage
	Ready bool `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Represents a type of distribution scheme, to one or more cdn
// providers.
type DistributionClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DistributionClassSpec   `json:"spec,omitempty"`
	Status DistributionClassStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DistributionClassList contains a list of DistributionClass
type DistributionClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DistributionClass `json:"items"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status

// Represents a type of distribution scheme, to one or more cdn
// providers. This resource is cluster wide, so can be referenced by
// distributions in any namespace.
type ClusterDistributionClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DistributionClassSpec   `json:"spec,omitempty"`
	Status DistributionClassStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterDistributionClassList contains a list of ClusterDistributionClass
type ClusterDistributionClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterDistributionClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DistributionClass{}, &DistributionClassList{})
	SchemeBuilder.Register(&ClusterDistributionClass{}, &ClusterDistributionClassList{})
}
