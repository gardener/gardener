//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package config

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServer) DeepCopyInto(out *APIServer) {
	*out = *in
	if in.CABundle != nil {
		in, out := &in.CABundle, &out.CABundle
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServer.
func (in *APIServer) DeepCopy() *APIServer {
	if in == nil {
		return nil
	}
	out := new(APIServer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeAgentConfiguration) DeepCopyInto(out *NodeAgentConfiguration) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.APIServer.DeepCopyInto(&out.APIServer)
	if in.KubeletDataVolumeSize != nil {
		in, out := &in.KubeletDataVolumeSize, &out.KubeletDataVolumeSize
		*out = new(int64)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeAgentConfiguration.
func (in *NodeAgentConfiguration) DeepCopy() *NodeAgentConfiguration {
	if in == nil {
		return nil
	}
	out := new(NodeAgentConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeAgentConfiguration) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
