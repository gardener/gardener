// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
)

// NamedResourceReference is a named reference to a resource.
type NamedResourceReference struct {
	// Name of the resource reference.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// ResourceRef is a reference to a resource.
	ResourceRef autoscalingv1.CrossVersionObjectReference `json:"resourceRef" protobuf:"bytes,2,opt,name=resourceRef"`
}
