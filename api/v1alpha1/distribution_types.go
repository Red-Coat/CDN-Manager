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
)

type ObjectReference struct {
	Kind string `json:"kind"`

	Name string `json:"name"`
}

type TLSSpec struct {
	// Sets how TLS is handled by the distribution:
	// Redirect (default) causes HTTP requests to be redirected to HTTPS
	// Only causes HTTP requests to be dropped
	// Both causes both HTTP and HTTPs requests to be respected
	// +kubebuilder:validation:Enum=redirect;only;both
	// +kubebuilder:default=redirect
	Mode string `json:"mode"`

	// The name of the secret containing the tls certificate
	SecretRef string `json:"secretName"`

	// Set this true if you need to request a dedicated IP address for
	// your distribution in order to support legacy (non-SNI) clients.
	// Warning, this may incurr high costs from many providers!
	// +kubebuilder:default=false
	DedicatedIP bool `json:"dedicatedIP"`
}

type CacheBehavior struct {
	// +kubebuilder:validation:Enum=headers;query-strings;cookies
	Type string `json:"type"`

	// +kubebuilder:validation:Enum=cache-only;send-to-origin
	// +kubebuilder:default=send-to-origin
	Action string `json:"action"`

	// +kubebuilder:validation:Enum=all;whitelist;blacklist
	Match string `json:"match"`

	// +optional
	Items []string `json:"items,omitempty"`
}

// Used to represent a port on a service. Either name or number must be
// specified
type ServicePort struct {
	// The name of the port on the service
	// +optional
	Name string `json:"name"`

	// The port number on the service
	// +optional
	Number int32 `json:"number"`
}

type Origin struct {
	// +optional
	Target *ObjectReference `json:"targetRef"`

	// +optional
	Host string `json:"host"`

	// The port this service uses for HTTP requests
	// +optional
	HTTPPort *ServicePort `json:"httpPort"`

	// The port this service uses for HTTPS requests
	// +optional
	HTTPSPort *ServicePort `json:"httpsPort"`
}

// DistributionSpec defines the desired state of Distribution
type DistributionSpec struct {
	// Reference to the Distribution Class provider to use for this
	// distribution
	DistributionClassRef ObjectReference `json:"distributionClass"`

	SupportedMethods []string `json:"supportedMethods"`

	// Series of rules for caching behaviors. By default: headers,
	// methods, query strings, and cookies are not considered when
	// caching, but this behavior can be changed.
	Rules []CacheBehavior `json:"rules"`

	// Information about the origin for the distribution
	Origin Origin `json:"origin"`

	// The hostnames that this distribution will listen for
	Hosts []string `json:"hosts"`

	// Optionally you can support TLS on your distribution
	// +optional
	TLS *TLSSpec `json:"tls"`
}

type CloudFrontStatus struct {
	//+kubebuilder:validation:Enum=Deployed;Disabled;InProgress
	State string `json:"state"`
	ID    string `json:"id"`
}

type Endpoint struct {
	Provider string `json:"provider"`

	// +optional
	Host string `json:"host,omitempty"`

	// +optional
	IP string `json:"ip,omitempty"`
}

// DistributionStatus defines the observed state of Distribution
type DistributionStatus struct {
	Ready bool `json:"ready"`

	//+optional
	Endpoints []Endpoint `json:"endpoints"`

	//+optional
	CloudFront *CloudFrontStatus `json:"cloudfront,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Distribution is the Schema for the distributions API
type Distribution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DistributionSpec `json:"spec"`

	// +optional
	Status DistributionStatus `json:"status"`
}

//+kubebuilder:object:root=true

// DistributionList contains a list of Distribution
type DistributionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Distribution `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Distribution{}, &DistributionList{})
}
