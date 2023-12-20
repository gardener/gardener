//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	time "time"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HealthCheckConfig) DeepCopyInto(out *HealthCheckConfig) {
	*out = *in
	out.SyncPeriod = in.SyncPeriod
	if in.ShootRESTOptions != nil {
		in, out := &in.ShootRESTOptions, &out.ShootRESTOptions
		*out = new(RESTOptions)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HealthCheckConfig.
func (in *HealthCheckConfig) DeepCopy() *HealthCheckConfig {
	if in == nil {
		return nil
	}
	out := new(HealthCheckConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RESTOptions) DeepCopyInto(out *RESTOptions) {
	*out = *in
	if in.QPS != nil {
		in, out := &in.QPS, &out.QPS
		*out = new(float32)
		**out = **in
	}
	if in.Burst != nil {
		in, out := &in.Burst, &out.Burst
		*out = new(int)
		**out = **in
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(time.Duration)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RESTOptions.
func (in *RESTOptions) DeepCopy() *RESTOptions {
	if in == nil {
		return nil
	}
	out := new(RESTOptions)
	in.DeepCopyInto(out)
	return out
}
