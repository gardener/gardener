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
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootExtensionStatus holds the status information of extensions of a Shoot cluster
type ShootExtensionStatus struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// Designed to have an owner reference to the associated Shoot resource
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Statuses holds a list of statuses of extension controllers.
	// +optional
	Statuses []ExtensionStatus `json:"statuses,omitempty" protobuf:"bytes,2,rep,name=statuses"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootExtensionStatusList is a list of ShootExtensionStatus objects.
type ShootExtensionStatusList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is a list of ShootExtensionStatus.
	// +optional
	Items []ShootExtensionStatus `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ExtensionStatus contains the kind, the type, the optional purpose and the last observed status
// of an extension controller.
type ExtensionStatus struct {
	// Kind of the extension resource
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// Type of the extension resource
	Type string `json:"type" protobuf:"bytes,2,opt,name=type"`
	// Purpose of the extension resource
	// +optional
	Purpose *string `json:"purpose,omitempty" protobuf:"bytes,3,opt,name=purpose"`
	// Status contains the status of the extension resource
	Status runtime.RawExtension `json:"status" protobuf:"bytes,4,opt,name=status"`
}
