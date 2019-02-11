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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ErrorCode is a string alias.
type ErrorCode string

const (
	// ErrorInfraUnauthorized indicates that the last error occurred due to invalid cloud provider credentials.
	ErrorInfraUnauthorized ErrorCode = "ERR_INFRA_UNAUTHORIZED"
	// ErrorInfraInsufficientPrivileges indicates that the last error occurred due to insufficient cloud provider privileges.
	ErrorInfraInsufficientPrivileges ErrorCode = "ERR_INFRA_INSUFFICIENT_PRIVILEGES"
	// ErrorInfraQuotaExceeded indicates that the last error occurred due to cloud provider quota limits.
	ErrorInfraQuotaExceeded ErrorCode = "ERR_INFRA_QUOTA_EXCEEDED"
	// ErrorInfraDependencies indicates that the last error occurred due to dependent objects on the cloud provider level.
	ErrorInfraDependencies ErrorCode = "ERR_INFRA_DEPENDENCIES"
)

// LastError indicates the last occurred error for an operation on a resource.
type LastError struct {
	// A human readable message indicating details about the last error.
	Description string `json:"description"`
	// Well-defined error codes of the last error(s).
	// +optional
	Codes []ErrorCode `json:"codes,omitempty"`
}

// LastOperationType is a string alias.
type LastOperationType string

const (
	// LastOperationTypeReconcile indicates a 'reconcile' operation.
	LastOperationTypeReconcile LastOperationType = "Reconcile"
	// LastOperationTypeDelete indicates a 'delete' operation.
	LastOperationTypeDelete LastOperationType = "Delete"
)

// LastOperationState is a string alias.
type LastOperationState string

const (
	// LastOperationStateProcessing indicates that an operation is ongoing.
	LastOperationStateProcessing LastOperationState = "Processing"
	// LastOperationStateSucceeded indicates that an operation has completed successfully.
	LastOperationStateSucceeded LastOperationState = "Succeeded"
	// LastOperationStateError indicates that an operation is completed with errors and will be retried.
	LastOperationStateError LastOperationState = "Error"
	// LastOperationStateFailed indicates that an operation is completed with errors and won't be retried.
	LastOperationStateFailed LastOperationState = "Failed"
	// LastOperationStatePending indicates that an operation cannot be done now, but will be tried in future.
	LastOperationStatePending LastOperationState = "Pending"
	// LastOperationStateAborted indicates that an operation has been aborted.
	LastOperationStateAborted LastOperationState = "Aborted"
)

// LastOperation indicates the type and the state of the last operation, along with a description
// message and a progress indicator.
type LastOperation struct {
	// A human readable message indicating details about the last operation.
	Description string `json:"description"`
	// Last time the operation state transitioned from one to another.
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	// The progress in percentage (0-100) of the last operation.
	Progress int `json:"progress"`
	// Status of the last operation, one of Aborted, Processing, Succeeded, Error, Failed.
	State LastOperationState `json:"state"`
	// Type of the last operation, one of Create, Reconcile, Delete.
	Type LastOperationType `json:"type"`
}

// DefaultSpec contains common status fields for every extension resource.
type DefaultSpec struct {
	// Type contains the instance of the resource's kind.
	Type string `json:"type"`
	// Shoot contains information about the shoot cluster's state.
	Shoot Shoot `json:"shoot"`
}

// Shoot contains information about the shoot cluster's state.
type Shoot struct {
	// Hibernated describes whether the shoot is hibernated or not.
	Hibernated bool `json:"hibernated"`
	// Failed describes whether the shoot is in a failed state, i.e. whether Gardener does not want any operations to be
	// retried automatically.
	Failed bool `json:"failed"`
}

// DefaultStatus contains common status fields for every extension resource.
type DefaultStatus struct {
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// State can be filled by the operating controller with what ever data it needs.
	State string `json:"state,omitempty"`
	// LastOperation holds information about the last operation on the resource.
	// +optional
	LastOperation *LastOperation `json:"lastOperation,omitempty"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *LastError `json:"lastError,omitempty"`
}
