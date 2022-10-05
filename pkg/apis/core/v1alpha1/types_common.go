// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ErrorCode is a string alias.
type ErrorCode string

const (
	// ErrorInfraUnauthenticated indicates that the last error occurred due to the client request not being completed because it lacks valid authentication credentials for the requested resource.
	// It is classified as a non-retryable error code.
	ErrorInfraUnauthenticated ErrorCode = "ERR_INFRA_UNAUTHENTICATED"
	// ErrorInfraUnauthorized indicates that the last error occurred due to the server understanding the request but refusing to authorize it.
	// It is classified as a non-retryable error code.
	ErrorInfraUnauthorized ErrorCode = "ERR_INFRA_UNAUTHORIZED"
	// ErrorInfraQuotaExceeded indicates that the last error occurred due to infrastructure quota limits.
	// It is classified as a non-retryable error code.
	ErrorInfraQuotaExceeded ErrorCode = "ERR_INFRA_QUOTA_EXCEEDED"
	// ErrorInfraRateLimitsExceeded indicates that the last error occurred due to exceeded infrastructure request rate limits.
	ErrorInfraRateLimitsExceeded ErrorCode = "ERR_INFRA_RATE_LIMITS_EXCEEDED"
	// ErrorInfraDependencies indicates that the last error occurred due to dependent objects on the infrastructure level.
	// It is classified as a non-retryable error code.
	ErrorInfraDependencies ErrorCode = "ERR_INFRA_DEPENDENCIES"
	// ErrorRetryableInfraDependencies indicates that the last error occurred due to dependent objects on the infrastructure level, but operation should be retried.
	ErrorRetryableInfraDependencies ErrorCode = "ERR_RETRYABLE_INFRA_DEPENDENCIES"
	// ErrorInfraResourcesDepleted indicates that the last error occurred due to depleted resource in the infrastructure.
	ErrorInfraResourcesDepleted ErrorCode = "ERR_INFRA_RESOURCES_DEPLETED"
	// ErrorCleanupClusterResources indicates that the last error occurred due to resources in the cluster that are stuck in deletion.
	ErrorCleanupClusterResources ErrorCode = "ERR_CLEANUP_CLUSTER_RESOURCES"
	// ErrorConfigurationProblem indicates that the last error occurred due to a configuration problem.
	// It is classified as a non-retryable error code.
	ErrorConfigurationProblem ErrorCode = "ERR_CONFIGURATION_PROBLEM"
	// ErrorRetryableConfigurationProblem indicates that the last error occurred due to a retryable configuration problem.
	ErrorRetryableConfigurationProblem ErrorCode = "ERR_RETRYABLE_CONFIGURATION_PROBLEM"
	// ErrorProblematicWebhook indicates that the last error occurred due to a webhook not following the Kubernetes
	// best practices (https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#best-practices-and-warnings).
	// It is classified as a non-retryable error code.
	ErrorProblematicWebhook ErrorCode = "ERR_PROBLEMATIC_WEBHOOK"
)

// LastError indicates the last occurred error for an operation on a resource.
type LastError struct {
	// A human readable message indicating details about the last error.
	Description string `json:"description" protobuf:"bytes,1,opt,name=description"`
	// ID of the task which caused this last error
	// +optional
	TaskID *string `json:"taskID,omitempty" protobuf:"bytes,2,opt,name=taskID"`
	// Well-defined error codes of the last error(s).
	// +optional
	Codes []ErrorCode `json:"codes,omitempty" protobuf:"bytes,3,rep,name=codes,casttype=ErrorCode"`
	// Last time the error was reported
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty" protobuf:"bytes,4,opt,name=lastUpdateTime"`
}

// LastOperationType is a string alias.
type LastOperationType string

const (
	// LastOperationTypeCreate indicates a 'create' operation.
	LastOperationTypeCreate LastOperationType = "Create"
	// LastOperationTypeReconcile indicates a 'reconcile' operation.
	LastOperationTypeReconcile LastOperationType = "Reconcile"
	// LastOperationTypeDelete indicates a 'delete' operation.
	LastOperationTypeDelete LastOperationType = "Delete"
	// LastOperationTypeMigrate indicates a 'migrate' operation.
	LastOperationTypeMigrate LastOperationType = "Migrate"
	// LastOperationTypeRestore indicates a 'restore' operation.
	LastOperationTypeRestore LastOperationType = "Restore"
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
	Description string `json:"description" protobuf:"bytes,1,opt,name=description"`
	// Last time the operation state transitioned from one to another.
	LastUpdateTime metav1.Time `json:"lastUpdateTime" protobuf:"bytes,2,opt,name=lastUpdateTime"`
	// The progress in percentage (0-100) of the last operation.
	Progress int32 `json:"progress" protobuf:"varint,3,opt,name=progress"`
	// Status of the last operation, one of Aborted, Processing, Succeeded, Error, Failed.
	State LastOperationState `json:"state" protobuf:"bytes,4,opt,name=state,casttype=LastOperationState"`
	// Type of the last operation, one of Create, Reconcile, Delete, Migrate, Restore.
	Type LastOperationType `json:"type" protobuf:"bytes,5,opt,name=type,casttype=LastOperationType"`
}

// Gardener holds the information about the Gardener version that operated a resource.
type Gardener struct {
	// ID is the Docker container id of the Gardener which last acted on a resource.
	ID string `json:"id" protobuf:"bytes,1,opt,name=id"`
	// Name is the hostname (pod name) of the Gardener which last acted on a resource.
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
	// Version is the version of the Gardener which last acted on a resource.
	Version string `json:"version" protobuf:"bytes,3,opt,name=version"`
}

const (
	// GardenerName is the value in a Garden resource's `.metadata.finalizers[]` array on which the Gardener will react
	// when performing a delete request on a resource.
	GardenerName = "gardener"
	// ExternalGardenerName is the value in a Kubernetes core resources `.metadata.finalizers[]` array on which the
	// Gardener will react when performing a delete request on a resource.
	ExternalGardenerName = "gardener.cloud/gardener"
)

const (
	// EventReconciling indicates that the a Reconcile operation started.
	EventReconciling = "Reconciling"
	// EventReconciled indicates that the a Reconcile operation was successful.
	EventReconciled = "Reconciled"
	// EventReconcileError indicates that the a Reconcile operation failed.
	EventReconcileError = "ReconcileError"
	// EventDeleting indicates that the a Delete operation started.
	EventDeleting = "Deleting"
	// EventDeleted indicates that the a Delete operation was successful.
	EventDeleted = "Deleted"
	// EventDeleteError indicates that the a Delete operation failed.
	EventDeleteError = "DeleteError"
	// EventPrepareMigration indicates that a Prepare Migration operation started.
	EventPrepareMigration = "PrepareMigration"
	// EventMigrationPrepared indicates that Migration preparation was successful.
	EventMigrationPrepared = "MigrationPrepared"
	// EventMigrationPreparationFailed indicates that Migration preparation failed.
	EventMigrationPreparationFailed = "MigrationPreparationFailed"
)

//////////////////////////////////////////////////////////////////////////////////////////////////
// High Availability relevant types                                                             //
//////////////////////////////////////////////////////////////////////////////////////////////////

// HighAvailability specifies the configuration settings for high availability for a resource. Typical
// usages could be to configure HA for shoot control plane or for seed system components.
type HighAvailability struct {
	// FailureTolerance holds information about failure tolerance level of a highly available resource.
	FailureTolerance FailureTolerance `json:"failureTolerance" protobuf:"bytes,1,name=failureTolerance"`
}

// FailureTolerance describes information about failure tolerance level of a highly available resource.
type FailureTolerance struct {
	// Type specifies the type of failure that the highly available resource can tolerate
	Type FailureToleranceType `json:"type" protobuf:"bytes,1,name=type"`
}

// FailureToleranceType specifies the type of failure that a highly available
// shoot control plane that can tolerate.
type FailureToleranceType string

const (
	// FailureToleranceTypeNode specifies that a highly available resource can tolerate the
	// failure of one or more nodes within a single-zone setup and still be available.
	FailureToleranceTypeNode FailureToleranceType = "node"
	// FailureToleranceTypeZone specifies that a highly available resource can tolerate the
	// failure of one or more zones within a multi-zone setup and still be available.
	FailureToleranceTypeZone FailureToleranceType = "zone"
)
