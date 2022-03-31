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

// The access details for cloudfront distributions
// If this section is provided, a cloudfront distribution will be setup,
// even if access details are not given in this block.
// +kubebuilder:object:generate=true
type CloudFrontSpec struct {
	Auth *AwsAuth `json:"auth,omitempty"`

	// Set this true if you need to request a dedicated IP address for
	// your distribution in order to support legacy (non-SNI) clients.
	// Warning, this will incurr high costs from AWS!
	// +kubebuilder:default=sni-only
	// +kubebuilder:validation:Enum=sni-only;vpi;static-ip
	// +optional
	SSLMode string `json:"sslMode"`

	// The Policy ID of the CloudFront Cache Policy you want to use on
	// distributions. This value is normally optional, but required if you
	// want to set an originRequestPolicyId.
	// +optional
	CachePolicyId string `json:"cachePolicyId,omitempty"`

	// The Policy ID of the CloudFront Origin Request Policy you want to
	// use on distributions. If you specify this value, cachePolicyId is
	// required.
	// +optional
	OriginRequestPolicyId string `json:"originRequestPolicyId,omitempty"`

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
}

// The status of a CloudFront Distribution, if one was requested by the
// DistributionClass
// +kubebuilder:object:generate=true
type CloudFrontStatus struct {
	// The CloudFront State as reported by the AWS API. NB: "InProgress"
	// does not always mean the distribution is not yet available as
	// sometimes this occurs when minor items are being updated.
	//+kubebuilder:validation:Enum=Deployed;Disabled;InProgress;Unknown
	State string `json:"state"`

	// The ID of the CloudFront Distribution (you can use this in any
	// interaction with the aws cloudfront api).
	ID string `json:"id"`

	// The ARN of the ACM Certificate created for the distribution
	CertificateArn string `json:"certificateArn"`
}
