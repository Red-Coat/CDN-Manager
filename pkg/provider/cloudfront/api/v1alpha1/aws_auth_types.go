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

// +kubebuilder:object:generate=true
type AwsAuth struct {
	// +optional
	Role string `json:"role,omitempty"`

	// +optional
	AccessKey *NamespacedName `json:"accessKeySecret,omitempty"`

	// +optional
	JWTAuth *AwsJwtAuth `json:"jwt,omitempty"`
}

// +kubebuilder:object:generate=true
type AwsJwtAuth struct {
	ServiceAccount NamespacedName `json:"serviceAccount"`

	// +kubebuilder:default="sts.amazonaws.com"
	// +optional
	Audience string `json:"stsAudience"`

	// +kubebuilder:default="eks.amazonaws.com/role-arn"
	// +optional
	AnnotationName string `json:"annotationName"`
}

// A reference to a kuberenetes resource, possibly in another namespace
// +kubebuilder:object:generate=true
type NamespacedName struct {
	Name string `json:"name"`

	// +optional
	Namespace *string `json:"namespace"`
}
