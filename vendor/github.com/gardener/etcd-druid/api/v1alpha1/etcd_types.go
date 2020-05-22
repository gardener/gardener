// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// GarbageCollectionPolicyExponential defines the exponential policy for garbage collecting old backups
	GarbageCollectionPolicyExponential = "Exponential"
	// GarbageCollectionPolicyLimitBased defines the limit based policy for garbage collecting old backups
	GarbageCollectionPolicyLimitBased = "LimitBased"

	// Basic is a constant for metrics level basic.
	Basic MetricsLevel = "basic"
	// Extensive is a constant for metrics level extensive.
	Extensive MetricsLevel = "extensive"
)

// MetricsLevel defines the level 'basic' or 'extensive'.
// +kubebuilder:validation:Enum=basic;extensive
type MetricsLevel string

// GarbageCollectionPolicy defines the type of policy for snapshot garbage collection.
// +kubebuilder:validation:Enum=Exponential;LimitBased
type GarbageCollectionPolicy string

// StorageProvider defines the type of object store provider for storing backups.
type StorageProvider string

// StoreSpec defines parameters related to ObjectStore persisting backups
type StoreSpec struct {
	// +optional
	Container *string `json:"container,omitempty"`
	// +required
	Prefix string `json:"prefix"`
	// +optional
	Provider *StorageProvider `json:"provider,omitempty"`
	// +optional
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
}

// TLSConfig hold the TLS configuration details.
type TLSConfig struct {
	// +required
	ServerTLSSecretRef corev1.SecretReference `json:"serverTLSSecretRef"`
	// +required
	ClientTLSSecretRef corev1.SecretReference `json:"clientTLSSecretRef"`
	// +required
	TLSCASecretRef corev1.SecretReference `json:"tlsCASecretRef"`
}

// BackupSpec defines parametes associated with the full and delta snapshots of etcd
type BackupSpec struct {
	// Port define the port on which etcd-backup-restore server will exposed.
	// +optional
	Port *int `json:"port,omitempty"`
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`
	// Image defines the etcd container image and tag
	// +optional
	Image *string `json:"image,omitempty"`
	// Store defines the specification of object store provider for storing backups.
	// +optional
	Store *StoreSpec `json:"store,omitempty"`
	// Resources defines the compute Resources required by backup-restore container.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// FullSnapshotSchedule defines the cron standard schedule for full snapshots.
	// +optional
	FullSnapshotSchedule *string `json:"fullSnapshotSchedule,omitempty"`
	// GarbageCollectionPolicy defines the policy for garbage collecting old backups
	// +optional
	GarbageCollectionPolicy *GarbageCollectionPolicy `json:"garbageCollectionPolicy,omitempty"`
	// GarbageCollectionPeriod defines the period for garbage collecting old backups
	// +optional
	GarbageCollectionPeriod *metav1.Duration `json:"garbageCollectionPeriod,omitempty"`
	// DeltaSnapshotPeriod defines the period after which delta snapshots will be taken
	// +optional
	DeltaSnapshotPeriod *metav1.Duration `json:"deltaSnapshotPeriod,omitempty"`
	// DeltaSnapshotMemoryLimit defines the memory limit after which delta snapshots will be taken
	// +optional
	DeltaSnapshotMemoryLimit *resource.Quantity `json:"deltaSnapshotMemoryLimit,omitempty"`
}

// EtcdConfig defines parametes associated etcd deployed
type EtcdConfig struct {
	// Quota defines the etcd DB quota.
	// +optional
	Quota *resource.Quantity `json:"quota,omitempty"`
	// DefragmentationSchedule defines the cron standard schedule for defragmentation of etcd.
	// +optional
	DefragmentationSchedule *string `json:"defragmentationSchedule,omitempty"`
	// +optional
	ServerPort *int `json:"serverPort,omitempty"`
	// +optional
	ClientPort *int `json:"clientPort,omitempty"`
	// Image defines the etcd container image and tag
	// +optional
	Image *string `json:"image,omitempty"`
	// +optional
	AuthSecretRef *corev1.SecretReference `json:"authSecretRef,omitempty"`
	// Metrics defines the level of detail for exported metrics of etcd, specify 'extensive' to include histogram metrics.
	// +optional
	Metrics MetricsLevel `json:"metrics,omitempty"`
	// Resources defines the compute Resources required by etcd container.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`
}

// EtcdSpec defines the desired state of Etcd
type EtcdSpec struct {
	// selector is a label query over pods that should match the replica count.
	// It must match the pod template's labels.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector *metav1.LabelSelector `json:"selector"`
	// +required
	Labels map[string]string `json:"labels"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +required
	Etcd EtcdConfig `json:"etcd"`
	// +required
	Backup BackupSpec `json:"backup"`
	// +required
	Replicas int `json:"replicas"`
	// PriorityClassName is the name of a priority class that shall be used for the etcd pods.
	// +optional
	PriorityClassName *string `json:"priorityClassName,omitempty"`
	// StorageClass defines the name of the StorageClass required by the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1
	// +optional
	StorageClass *string `json:"storageClass,omitempty"`
	// StorageCapacity defines the size of persistent volume.
	// +optional
	StorageCapacity *resource.Quantity `json:"storageCapacity,omitempty"`
	// VolumeClaimTemplate defines the volume claim template to be created
	// +optional
	VolumeClaimTemplate *string `json:"volumeClaimTemplate,omitempty"`
}

// CrossVersionObjectReference contains enough information to let you identify the referred resource.
type CrossVersionObjectReference struct {
	// Kind of the referent
	// +required
	Kind string `json:"kind,omitempty"`
	// Name of the referent
	// +required
	Name string `json:"name,omitempty"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// ConditionStatus is the status of a condition.
type ConditionStatus string

// ConditionType is a string alias.
type ConditionType string

const (
	// ConditionAvailable is a condition type for indicating availability.
	ConditionAvailable ConditionType = "Available"

	// ConditionTrue means a resource is in the condition.
	ConditionTrue ConditionStatus = "True"
	// ConditionFalse means a resource is not in the condition.
	ConditionFalse ConditionStatus = "False"
	// ConditionUnknown means Gardener can't decide if a resource is in the condition or not.
	ConditionUnknown ConditionStatus = "Unknown"
	// ConditionProgressing means the condition was seen true, failed but stayed within a predefined failure threshold.
	// In the future, we could add other intermediate conditions, e.g. ConditionDegraded.
	ConditionProgressing ConditionStatus = "Progressing"

	// ConditionCheckError is a constant for a reason in condition.
	ConditionCheckError = "ConditionCheckError"
)

// Condition holds the information about the state of a resource.
type Condition struct {
	// Type of the Etcd condition.
	Type ConditionType `json:"type,omitempty"`
	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Last time the condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

// EndpointStatus is the status of a condition.
type EndpointStatus string

// LastOperationType is a string alias.
type LastOperationType string

const (
	// LastOperationTypeCreate indicates a 'create' operation.
	LastOperationTypeCreate LastOperationType = "Create"
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
	Description string `json:"description,omitempty"`
	// Last time the operation state transitioned from one to another.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// The progress in percentage (0-100) of the last operation.
	Progress int `json:"progress,omitempty"`
	// Status of the last operation, one of Aborted, Processing, Succeeded, Error, Failed.
	State LastOperationState `json:"state,omitempty"`
	// Type of the last operation, one of Create, Reconcile, Delete.
	Type LastOperationType `json:"type,omitempty"`
}

// EtcdStatus defines the observed state of Etcd
type EtcdStatus struct {
	// ObservedGeneration is the most recent generation observed for this resource.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
	// +optional
	Etcd CrossVersionObjectReference `json:"etcd,omitempty"`
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
	// +optional
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`
	// +optional
	ServiceName *string `json:"serviceName,omitempty"`
	// +optional
	LastError *string `json:"lastError,omitempty"`
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// +optional
	Ready *bool `json:"ready,omitempty"`
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
	// selector is a label query over pods that should match the replica count.
	// It must match the pod template's labels.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	//LastOperation   LastOperation               `json:"lastOperation,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.labelSelector

// Etcd is the Schema for the etcds API
type Etcd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdSpec   `json:"spec,omitempty"`
	Status EtcdStatus `json:"status,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true

// EtcdList contains a list of Etcd
type EtcdList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Etcd `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Etcd{}, &EtcdList{})
}
