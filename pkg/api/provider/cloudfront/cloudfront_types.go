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

package cloudfront

import (
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
)

// The access details for cloudfront distributions
// If this section is provided, a cloudfront distribution will be setup,
// even if access details are not given in this block.
//+kubebuilder:object:generate=true
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

	// Set this true if you need to request a dedicated IP address for
	// your distribution in order to support legacy (non-SNI) clients.
	// Warning, this will incurr high costs from AWS!
	// +kubebuilder:default=false
	// +optional
	DedicatedIP bool `json:"dedicatedIP"`
}

// The status of a CloudFront Distribution, if one was requested by the
// DistributionClass
//+kubebuilder:object:generate=true
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
