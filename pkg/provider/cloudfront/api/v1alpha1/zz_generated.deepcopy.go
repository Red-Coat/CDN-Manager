//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import ()

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AwsAuth) DeepCopyInto(out *AwsAuth) {
	*out = *in
	if in.AccessKeyRef != nil {
		in, out := &in.AccessKeyRef, &out.AccessKeyRef
		*out = new(NamespacedName)
		(*in).DeepCopyInto(*out)
	}
	if in.JWTAuth != nil {
		in, out := &in.JWTAuth, &out.JWTAuth
		*out = new(AwsJwtAuth)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AwsAuth.
func (in *AwsAuth) DeepCopy() *AwsAuth {
	if in == nil {
		return nil
	}
	out := new(AwsAuth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AwsJwtAuth) DeepCopyInto(out *AwsJwtAuth) {
	*out = *in
	in.ServiceAccount.DeepCopyInto(&out.ServiceAccount)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AwsJwtAuth.
func (in *AwsJwtAuth) DeepCopy() *AwsJwtAuth {
	if in == nil {
		return nil
	}
	out := new(AwsJwtAuth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CloudFrontSpec) DeepCopyInto(out *CloudFrontSpec) {
	*out = *in
	if in.Auth != nil {
		in, out := &in.Auth, &out.Auth
		*out = new(AwsAuth)
		(*in).DeepCopyInto(*out)
	}
	if in.SupportedMethods != nil {
		in, out := &in.SupportedMethods, &out.SupportedMethods
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloudFrontSpec.
func (in *CloudFrontSpec) DeepCopy() *CloudFrontSpec {
	if in == nil {
		return nil
	}
	out := new(CloudFrontSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CloudFrontStatus) DeepCopyInto(out *CloudFrontStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloudFrontStatus.
func (in *CloudFrontStatus) DeepCopy() *CloudFrontStatus {
	if in == nil {
		return nil
	}
	out := new(CloudFrontStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedName) DeepCopyInto(out *NamespacedName) {
	*out = *in
	if in.Namespace != nil {
		in, out := &in.Namespace, &out.Namespace
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedName.
func (in *NamespacedName) DeepCopy() *NamespacedName {
	if in == nil {
		return nil
	}
	out := new(NamespacedName)
	in.DeepCopyInto(out)
	return out
}
