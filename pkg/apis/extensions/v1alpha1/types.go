// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status is the status of an Object.
type Status interface {
	// GetLastOperation retrieves the LastOperation of a status.
	// LastOperation may be nil.
	GetLastOperation() LastOperation
	// GetObservedGeneration retrieves the last generation observed by the extension controller.
	GetObservedGeneration() int64
	// GetLastError retrieves the LastError of a status.
	// LastError may be nil.
	GetLastError() LastError
}

// LastOperation is the last operation on an object.
type LastOperation interface {
	// GetDescription returns the description of the last operation.
	GetDescription() string
	// GetLastUpdateTime returns the last update time of the last operation.
	GetLastUpdateTime() metav1.Time
	// GetProgress returns progress of the last operation.
	GetProgress() int
	// GetState returns the LastOperationState of the last operation.
	GetState() gardencorev1alpha1.LastOperationState
	// GetType returns the LastOperationType of the last operation.
	GetType() gardencorev1alpha1.LastOperationType
}

// LastError is the last error on an object.
type LastError interface {
	// GetDescription gets the description of the last occurred error.
	GetDescription() string
	// GetCodes gets the error codes of the last error.
	GetCodes() []gardencorev1alpha1.ErrorCode
	// GetLastUpdateTime retrieves the last time the error was updated.
	GetLastUpdateTime() *metav1.Time
}

// Spec is the spec section of an Object.
type Spec interface {
	// GetExtensionType retrieves the extension type.
	GetExtensionType() string
}

// Object is an extension object resource.
type Object interface {
	metav1.Object
	// GetExtensionStatus retrieves the object's status.
	GetExtensionStatus() Status
	// GetExtensionSpec retrieves the object's spec.
	GetExtensionSpec() Spec
}
