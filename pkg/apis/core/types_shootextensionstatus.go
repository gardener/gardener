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

package core

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootExtensionStatus holds the status information of extensions of a Shoot cluster
type ShootExtensionStatus struct {
	metav1.TypeMeta
	// Standard object metadata.
	// Designed to have an owner reference to the associated Shoot resource
	metav1.ObjectMeta
	// Statuses holds a list of statuses of extension controllers.
	Statuses []ExtensionStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootExtensionStatusList is a list of ShootExtensionStatus objects.
type ShootExtensionStatusList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is a list of ShootExtensionStatus.
	Items []ShootExtensionStatus
}

// ExtensionStatus contains the kind, the type, the optional purpose and the last observed status
// of an extension controller.
type ExtensionStatus struct {
	// Kind of the extension resource
	Kind string
	// Type of the extension resource
	Type string
	// Purpose of the extension resource
	Purpose *string
	// Status contains the status of the extension resource
	Status runtime.Object
}
