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
	in.ProviderConfig.DeepCopyInto(&out.ProviderConfig)
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
