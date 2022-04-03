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

func init() {
	SchemeBuilder.Register(&Distribution{}, &DistributionList{})
}

// A distribution resource should be created to ensure an up to date
// Distribution is setup. If targetted at a service or ingress, the
// Distribution will be kept up to date with its external ingress
// address. The Distribution will also keep up to date with any changes
// or renewals made the TLS certificate secret it is given.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.ready",name=Ready,type=boolean
// +kubebuilder:printcolumn:JSONPath=".status.endpoints[0].host",name=Endpoint,type=string
type Distribution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DistributionSpec `json:"spec"`

	// +optional
	Status DistributionStatus `json:"status"`
}

// The desired state of the Distribution
type DistributionSpec struct {
	// Reference to the Distribution Class provider to use for this
	// distribution
	DistributionClassRef ObjectReference `json:"distributionClass"`

	// Information about the "Origin" for the distribution, ie where the CDN
	// should be setup to point to. We would normally expect this to be this
	// kubernetes cluster, although it does not have to be.
	Origin Origin `json:"origin"`

	// The list of host names that this distribition is for, following the
	// same rules as the host field of an IngressRule, namely that it is a
	// fully qualified domain name, as defined by RFC 3986 with two
	// exceptions: 1. IP address are not allowed. Content Delivery
	// Networks and edge caches manage their own IP addresses, and
	// listening on these specifically neither makes sense, or is allowed
	// by providers. 2. The `:` delimeter is not allowed. Most CDNs always
	// use port 80 for HTTP and 443 for HTTPS and these are not
	// configurable. If you are using TLS, each of these host names _must_
	// exist within the given certificate. Warning: If you are using
	// CloudFront, you must also specify a TLS certificate if you give a
	// list of hosts.
	Hosts []string `json:"hosts"`

	// The TLS configuration for this distribution (eg the secret name for
	// the TLS certificate, and how to handle insecure requests).
	// +optional
	TLS *TLSSpec `json:"tls"`
}

// Options for the "origin" of the distribition - ie where the CDN
// points to.
type Origin struct {
	// If you specify this, this takes precendence over any detected
	// ingress load balancer hostnames. Use this to override the target's
	// hostname, or if you have not specified a kubernetes target.
	Host string `json:"host"`

	// The port to target for HTTP requests. If not given, this defaults
	// to 80.
	// +kubebuilder:default=80
	// +optional
	HTTPPort int32 `json:"httpPort"`

	// The port to target for HTTPS requests. If not given, this defaults
	// to 443.
	// +kubebuilder:default=443
	// +optional
	HTTPSPort int32 `json:"httpsPort"`
}

// Options to control the way TLS works within this distribution
type TLSSpec struct {
	// Sets how TLS is handled by the distribution:
	// Redirect (default) causes HTTP requests to be redirected to HTTPS,
	// Only causes HTTP requests to be dropped,
	// Both causes both HTTP and HTTPs requests to be respected.
	// NB: Different cloud providers may vary in the way they interpret
	// or respect this directive.
	// +kubebuilder:validation:Enum=redirect;only;both
	// +kubebuilder:default=redirect
	Mode string `json:"mode"`

	// The name of the kubernetes secret containing the TLS certificate
	// to be used by the distribution. This should be of type
	// kubernetes.io/tls and have the required fields (tls.crt and
	// tls.key). Other fields are ignored.
	SecretRef string `json:"secretName"`
}

// The current State of the Distribution
type DistributionStatus struct {
	Ready bool `json:"ready"`

	// List of one or more "endpoints" for the deployed distribution.
	// These can be either hostnames for DNS CNAMING, or direct IP
	// addresses, depending on the provider.
	//+optional
	Endpoints []Endpoint `json:"endpoints"`

	// The external provider's Identifier for the distribution
	// +optional
	ExternalId string `json:"externalId"`

	// If a TLS certificate is associated with the distribution, this is
	// its identifier in the external provider
	// +optional
	ExternalCertificateId string `json:"externalCertificateId"`

	// A status message from the external provider
	// +optional
	ExternalStatus string `json:"externalStatus,omitempty"`
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
	// A hostname that the distribution is available at. This is what you
	// would use in an DNS CNAME record. At least one of "Host" and "IP"
	// must be set for each Endpoint.
	// +optional
	Host string `json:"host,omitempty"`

	// An IP address that the distribution is available at. This is what
	// you would use in a DNS A or AAAA record. At least one of "Host" and
	// "IP" must be set for each Endpoint.
	// +optional
	IP string `json:"ip,omitempty"`
}

// DistributionList contains a list of Distributions
// +kubebuilder:object:root=true
type DistributionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Distribution `json:"items"`
}
