// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExposureClass represents a control plane endpoint exposure strategy.
type ExposureClass struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Handler is the name of the handler which applies the control plane endpoint exposure strategy.
	// This field is immutable.
	Handler string `json:"handler" protobuf:"bytes,2,opt,name=handler"`
	// Scheduling holds information how to select applicable Seed's for ExposureClass usage.
	// This field is immutable.
	// +optional
	Scheduling *ExposureClassScheduling `json:"scheduling,omitempty" protobuf:"bytes,3,opt,name=scheduling"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExposureClassList is a collection of ExposureClass.
type ExposureClassList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of ExposureClasses.
	Items []ExposureClass `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ExposureClassScheduling holds information to select applicable Seed's for ExposureClass usage.
type ExposureClassScheduling struct {
	// SeedSelector is an optional label selector for Seed's which are suitable to use the ExposureClass.
	// +optional
	SeedSelector *SeedSelector `json:"seedSelector,omitempty" protobuf:"bytes,1,opt,name=seedSelector"`
	// Tolerations contains the tolerations for taints on Seed clusters.
	// +patchMergeKey=key
	// +patchStrategy=merge
	// +optional
	Tolerations []Toleration `json:"tolerations,omitempty" patchStrategy:"merge" patchMergeKey:"key" protobuf:"bytes,2,rep,name=tolerations"`
}
