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

// Details on how to authenticate with the AWS API. If this is not
// specified, we will fall back to the ambient credentials given to
// the controller pod.
// +kubebuilder:object:generate=true
type AwsAuth struct {
	// A reference to a secret containing an AWS Access ID and Secret Key
	// pair. The Secret must have data saved in the standard fields:
	// AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY. Other fields are
	// ingored.
	// +optional
	AccessKey *NamespacedName `json:"accessKeySecret,omitempty"`

	// +optional
	JWTAuth *AwsJwtAuth `json:"jwt,omitempty"`
}

// Details on how to authenticate using a ServiceAccount token as a
// web identity for AWS STS. You should already have setup the cluster
// as an identity provider in AWS IAM.
// +kubebuilder:object:generate=true
type AwsJwtAuth struct {
	// A reference to the ServiceAccount to use. A ServiceAccount token
	// will be generated for this resource, and used as the web identity.
	ServiceAccount NamespacedName `json:"serviceAccount"`

	// The audience to use for the token. It is normally save to leave
	// this as the default ("sts.amazonaws.com") unless you know you have
	// a different setup.
	// +kubebuilder:default="sts.amazonaws.com"
	// +optional
	Audience string `json:"stsAudience"`

	// The name of the annotation on the ServiceAccount to search for the
	// Role to assume. On EKS, the standard annotation is the default
	// ("eks.amazonaws.com/role-arn").
	// +kubebuilder:default="eks.amazonaws.com/role-arn"
	// +optional
	AnnotationName string `json:"annotationName"`
}

// A reference to a kuberenetes resource, possibly in another namespace
// +kubebuilder:object:generate=true
type NamespacedName struct {
	// The name of the resource
	Name string `json:"name"`

	// The namespace of the resource. For ClusterDistributionClasses, this
	// field is required.
	// +optional
	Namespace *string `json:"namespace"`
}
