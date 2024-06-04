//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContextObject) DeepCopyInto(out *ContextObject) {
	*out = *in
	if in.Namespace != nil {
		in, out := &in.Namespace, &out.Namespace
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContextObject.
func (in *ContextObject) DeepCopy() *ContextObject {
	if in == nil {
		return nil
	}
	out := new(ContextObject)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CredentialsBinding) DeepCopyInto(out *CredentialsBinding) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Provider = in.Provider
	out.CredentialsRef = in.CredentialsRef
	if in.Quotas != nil {
		in, out := &in.Quotas, &out.Quotas
		*out = make([]v1.ObjectReference, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CredentialsBinding.
func (in *CredentialsBinding) DeepCopy() *CredentialsBinding {
	if in == nil {
		return nil
	}
	out := new(CredentialsBinding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CredentialsBinding) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CredentialsBindingList) DeepCopyInto(out *CredentialsBindingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CredentialsBinding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CredentialsBindingList.
func (in *CredentialsBindingList) DeepCopy() *CredentialsBindingList {
	if in == nil {
		return nil
	}
	out := new(CredentialsBindingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CredentialsBindingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CredentialsBindingProvider) DeepCopyInto(out *CredentialsBindingProvider) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CredentialsBindingProvider.
func (in *CredentialsBindingProvider) DeepCopy() *CredentialsBindingProvider {
	if in == nil {
		return nil
	}
	out := new(CredentialsBindingProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TargetSystem) DeepCopyInto(out *TargetSystem) {
	*out = *in
	if in.ProviderConfig != nil {
		in, out := &in.ProviderConfig, &out.ProviderConfig
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TargetSystem.
func (in *TargetSystem) DeepCopy() *TargetSystem {
	if in == nil {
		return nil
	}
	out := new(TargetSystem)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TokenRequest) DeepCopyInto(out *TokenRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TokenRequest.
func (in *TokenRequest) DeepCopy() *TokenRequest {
	if in == nil {
		return nil
	}
	out := new(TokenRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TokenRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TokenRequestSpec) DeepCopyInto(out *TokenRequestSpec) {
	*out = *in
	if in.ContextObject != nil {
		in, out := &in.ContextObject, &out.ContextObject
		*out = new(ContextObject)
		(*in).DeepCopyInto(*out)
	}
	if in.ExpirationSeconds != nil {
		in, out := &in.ExpirationSeconds, &out.ExpirationSeconds
		*out = new(int64)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TokenRequestSpec.
func (in *TokenRequestSpec) DeepCopy() *TokenRequestSpec {
	if in == nil {
		return nil
	}
	out := new(TokenRequestSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TokenRequestStatus) DeepCopyInto(out *TokenRequestStatus) {
	*out = *in
	in.ExpirationTimeStamp.DeepCopyInto(&out.ExpirationTimeStamp)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TokenRequestStatus.
func (in *TokenRequestStatus) DeepCopy() *TokenRequestStatus {
	if in == nil {
		return nil
	}
	out := new(TokenRequestStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadIdentity) DeepCopyInto(out *WorkloadIdentity) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadIdentity.
func (in *WorkloadIdentity) DeepCopy() *WorkloadIdentity {
	if in == nil {
		return nil
	}
	out := new(WorkloadIdentity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkloadIdentity) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadIdentityList) DeepCopyInto(out *WorkloadIdentityList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WorkloadIdentity, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadIdentityList.
func (in *WorkloadIdentityList) DeepCopy() *WorkloadIdentityList {
	if in == nil {
		return nil
	}
	out := new(WorkloadIdentityList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkloadIdentityList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadIdentitySpec) DeepCopyInto(out *WorkloadIdentitySpec) {
	*out = *in
	if in.Audiences != nil {
		in, out := &in.Audiences, &out.Audiences
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.TargetSystem.DeepCopyInto(&out.TargetSystem)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadIdentitySpec.
func (in *WorkloadIdentitySpec) DeepCopy() *WorkloadIdentitySpec {
	if in == nil {
		return nil
	}
	out := new(WorkloadIdentitySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadIdentityStatus) DeepCopyInto(out *WorkloadIdentityStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadIdentityStatus.
func (in *WorkloadIdentityStatus) DeepCopy() *WorkloadIdentityStatus {
	if in == nil {
		return nil
	}
	out := new(WorkloadIdentityStatus)
	in.DeepCopyInto(out)
	return out
}
