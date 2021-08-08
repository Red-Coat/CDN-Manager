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
	"git.redcoat.dev/cdn/pkg/api/provider/cloudfront"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ObjectReference struct {
	// The kind of resource to target (Ingress or Service)
	// +kubebuilder:validation=Ingress;Service
	Kind string `json:"kind"`

	// The name of the resource to target. This must be in the same
	// namespace as the Distribution resource.
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
	// If you want to target another Resource in the cluster (eg a Service
	// or an Ingress) specify it here. The Distribution will point to that
	// service's ingress load balancer.
	// +optional
	Target *ObjectReference `json:"targetRef"`

	// +optional
	Host string `json:"host,omitempty"`

	// The port this service uses for HTTP requests
	// +optional
	HTTPPort *ServicePort `json:"httpPort"`

	// The port this service uses for HTTPS requests
	// +optional
	HTTPSPort *ServicePort `json:"httpsPort"`
}

// The desired state of the Distribution
type DistributionSpec struct {
	// Reference to the Distribution Class provider to use for this
	// distribution
	DistributionClassRef ObjectReference `json:"distributionClass"`

	// The list of HTTP methods to support. Others will be rejected with
	// the CDN provider's native behaviour. NB: the controller can only
	// guarantee that methods will work if they are "standard", eg
	// CloudFront only supports HEAD, GET, OPTIONS, POST, PUT, DELETE.
	//
	// In addition, the controller cannot guarantee that methods you don't
	// specify here _won't_ be accessible. For example, CloudFront only
	// supports limited subsets, so if you specify any one of POST, PUT,
	// or DELETE, all methods are enabled.
	SupportedMethods []string `json:"supportedMethods"`

	// Information about the origin for the distribution
	Origin Origin `json:"origin"`

	// The hostnames that this distribution will listen for
	Hosts []string `json:"hosts"`

	// Optionally you can support TLS on your distribution
	// +optional
	TLS *TLSSpec `json:"tls"`
}

// Information about a specific Endpoint
//
// This must always contain a Provider identifier, as all endpoints are
// associated with one. It can then have a hostname, IP address, or
// both. If it has both, these should be the same resource, eg:
//
// Acceptable:
// - ip: 1.2.3.4
//   host: lb-1-2-3-4.provider.example.com
//
// Not Acceptable:
// - ip: 1.2.3.4
//   host: lb-4-5-6-7.provider.example.com
//
// If there are two distinct endpoints, one with an ip and one with
// host, just specify these as seperate Endpoint items:
// - ip: 1.2.3.4
// - host: lb-4-5-6-7.provider.example.com
type Endpoint struct {
	// The name of the provider that is responsible for this endpoint
	// (eg "cloudfront")
	Provider string `json:"provider"`

	// A hostname that the distribution is available at
	// +optional
	Host string `json:"host,omitempty"`

	// An IP address that the distribution is available at
	// +optional
	IP string `json:"ip,omitempty"`
}

// The current State of the Distribution
type DistributionStatus struct {
	Ready bool `json:"ready"`

	// List of one or more "endpoints" for the deployed distribution.
	// These can be either hostnames for DNS CNAMING, or direct IP
	// addresses, depending on the provider.
	//+optional
	Endpoints []Endpoint `json:"endpoints"`

	//+optional
	CloudFront *cloudfront.CloudFrontStatus `json:"cloudfront,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.ready",name=Ready,type=boolean
// +kubebuilder:printcolumn:JSONPath=".status.endpoints[0].host",name=Endpoint,type=string

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
