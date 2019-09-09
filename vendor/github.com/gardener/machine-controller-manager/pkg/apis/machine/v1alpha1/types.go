/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// WARNING!
// IF YOU MODIFY ANY OF THE TYPES HERE COPY THEM TO ../types.go
// AND RUN  ./hack/generate-code

/********************** Machine APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Machine TODO
type Machine struct {
	// ObjectMeta for machine object
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// TypeMeta for machine object
	metav1.TypeMeta `json:",inline"`

	// Spec contains the specification of the machine
	Spec MachineSpec `json:"spec,omitempty"`

	// Status contains fields depicting the status
	Status MachineStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineList is a collection of Machines.
type MachineList struct {
	// ObjectMeta for MachineList object
	metav1.TypeMeta `json:",inline"`

	// TypeMeta for MachineList object
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items contains the list of machines
	Items []Machine `json:"items"`
}

// MachineSpec is the specification of a machine.
type MachineSpec struct {

	// Class contains the machineclass attributes of a machine
	// +optional
	Class ClassSpec `json:"class,omitempty"`

	// ProviderID represents the provider's unique ID given to a machine
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// NodeTemplateSpec describes the data a node should have when created from a template
	// +optional
	NodeTemplateSpec NodeTemplateSpec `json:"nodeTemplate,omitempty"`
}

// NodeTemplateSpec describes the data a node should have when created from a template
type NodeTemplateSpec struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// NodeSpec describes the attributes that a node is created with.
	// +optional
	Spec corev1.NodeSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// MachineTemplateSpec describes the data a machine should have when created from a template
type MachineTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the machine.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	// +optional
	Spec MachineSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineTemplate describes a template for creating copies of a predefined machine.
type MachineTemplate struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Template defines the machines that will be created from this machine template.
	// https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	// +optional
	Template MachineTemplateSpec `json:"template,omitempty" protobuf:"bytes,2,opt,name=template"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineTemplateList is a list of MachineTemplates.
type MachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`

	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// List of machine templates
	Items []MachineTemplate `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ClassSpec is the class specification of machine
type ClassSpec struct {
	// API group to which it belongs
	APIGroup string `json:"apiGroup,omitempty"`

	// Kind for machine class
	Kind string `json:"kind,omitempty"`

	// Name of machine class
	Name string `json:"name,omitempty"`
}

//type CurrentStatus
type CurrentStatus struct {
	// API group to which it belongs
	Phase MachinePhase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase,casttype=MachinePhase"`

	// Name of machine class
	TimeoutActive bool `json:"timeoutActive,omitempty"`

	// Last update time of current status
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
}

// MachineStatus TODO
type MachineStatus struct {
	// Node string
	Node string `json:"node,omitempty"`

	// Conditions of this machine, same as node
	Conditions []corev1.NodeCondition `json:"conditions,omitempty"`

	// Last operation refers to the status of the last operation performed
	LastOperation LastOperation `json:"lastOperation,omitempty"`

	// Current status of the machine object
	CurrentStatus CurrentStatus `json:"currentStatus,omitempty"`
}

// LastOperation suggests the last operation performed on the object
type LastOperation struct {
	// Description of the current operation
	Description string `json:"description,omitempty"`

	// Last update time of current operation
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// State of operation
	State MachineState `json:"state,omitempty"`

	// Type of operation
	Type MachineOperationType `json:"type,omitempty"`
}

// MachinePhase is a label for the condition of a machines at the current time.
type MachinePhase string

// These are the valid statuses of machines.
const (
	// MachinePending means that the machine is being created
	MachinePending MachinePhase = "Pending"

	// MachineAvailable means that machine is present on provider but hasn't joined cluster yet
	MachineAvailable MachinePhase = "Available"

	// MachineRunning means node is ready and running successfully
	MachineRunning MachinePhase = "Running"

	// MachineRunning means node is terminating
	MachineTerminating MachinePhase = "Terminating"

	// MachineUnknown indicates that the node is not ready at the movement
	MachineUnknown MachinePhase = "Unknown"

	// MachineFailed means operation failed leading to machine status failure
	MachineFailed MachinePhase = "Failed"
)

// MachinePhase is a label for the condition of a machines at the current time.
type MachineState string

// These are the valid statuses of machines.
const (
	// MachineStatePending means there are operations pending on this machine state
	MachineStateProcessing MachineState = "Processing"

	// MachineStateFailed means operation failed leading to machine status failure
	MachineStateFailed MachineState = "Failed"

	// MachineStateSuccessful indicates that the node is not ready at the moment
	MachineStateSuccessful MachineState = "Successful"
)

// MachineOperationType is a label for the operation performed on a machine object.
type MachineOperationType string

// These are the valid statuses of machines.
const (
	// MachineOperationCreate indicates that the operation was a create
	MachineOperationCreate MachineOperationType = "Create"

	// MachineOperationUpdate indicates that the operation was an update
	MachineOperationUpdate MachineOperationType = "Update"

	// MachineOperationHealthCheck indicates that the operation was a create
	MachineOperationHealthCheck MachineOperationType = "HealthCheck"

	// MachineOperationDelete indicates that the operation was a create
	MachineOperationDelete MachineOperationType = "Delete"
)

// The below types are used by kube_client and api_server.

type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in the condition;
// "ConditionFalse" means a resource is not in the condition; "ConditionUnknown" means kubernetes
// can't decide if a resource is in the condition or not. In the future, we could add other
// intermediate conditions, e.g. ConditionDegraded.
const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

/********************** MachineSet APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineSet TODO
type MachineSet struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec MachineSetSpec `json:"spec,omitempty"`

	// +optional
	Status MachineSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineSetList is a collection of MachineSet.
type MachineSetList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []MachineSet `json:"items"`
}

// MachineSetSpec is the specification of a cluster.
type MachineSetSpec struct {
	// +optional
	Replicas int32 `json:"replicas,inline"`

	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// +optional
	MachineClass ClassSpec `json:"machineClass,omitempty"`

	// +optional
	Template MachineTemplateSpec `json:"template,omitempty" protobuf:"bytes,2,opt,name=template"`

	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,inline"`
}

// MachineSetConditionType is the condition on machineset object
type MachineSetConditionType string

// These are valid conditions of a machine set.
const (
	// MachineSetReplicaFailure is added in a machine set when one of its machines fails to be created
	// due to insufficient quota, limit ranges, machine security policy, node selectors, etc. or deleted
	// due to kubelet being down or finalizers are failing.
	MachineSetReplicaFailure MachineSetConditionType = "ReplicaFailure"
	// MachineSetFrozen is set when the machineset has exceeded its replica threshold at the safety controller
	MachineSetFrozen MachineSetConditionType = "Frozen"
)

// MachineSetCondition describes the state of a machine set at a certain point.
type MachineSetCondition struct {
	// Type of machine set condition.
	Type MachineSetConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=MachineSetConditionType"`

	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/api/core/v1.ConditionStatus"`

	// The last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,3,opt,name=lastTransitionTime"`

	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`

	// A human readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,5,opt,name=message"`
}

// MachineSetStatus represents the status of a machineSet object
type MachineSetStatus struct {
	// Replicas is the number of actual replicas.
	Replicas int32 `json:"replicas,inline"`

	// The number of pods that have labels matching the labels of the pod template of the replicaset.
	// +optional
	FullyLabeledReplicas int32 `json:"fullyLabeledReplicas,inline"`

	// The number of ready replicas for this replica set.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,inline"`

	// The number of available replicas (ready for at least minReadySeconds) for this replica set.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,inline"`

	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,inline"`

	// Represents the latest available observations of a replica set's current state.
	// +optional
	Conditions []MachineSetCondition `json:"machineSetCondition,inline"`

	// LastOperation performed
	LastOperation LastOperation `json:"lastOperation,omitempty"`

	// FailedMachines has summary of machines on which lastOperation Failed
	// +optional
	FailedMachines *[]MachineSummary `json:"failedMachines,inline"`
}

// MachineSummary store the summary of machine.
type MachineSummary struct {
	// Name of the machine object
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// ProviderID represents the provider's unique ID given to a machine
	ProviderID string `json:"providerID,omitempty"`

	// Last operation refers to the status of the last operation performed
	LastOperation LastOperation `json:"lastOperation,omitempty"`

	// OwnerRef
	OwnerRef string `json:"ownerRef,omitempty"`
}

/********************** MachineDeployment APIs ***************/

// +genclient
// +genclient:method=GetScale,verb=get,subresource=scale,result=Scale
// +genclient:method=UpdateScale,verb=update,subresource=scale,input=Scale,result=Scale
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Deployment enables declarative updates for machines and MachineSets.
type MachineDeployment struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the MachineDeployment.
	// +optional
	Spec MachineDeploymentSpec `json:"spec,omitempty"`

	// Most recently observed status of the MachineDeployment.
	// +optional
	Status MachineDeploymentStatus `json:"status,omitempty"`
}

// MachineDeploymentSpec is the specification of the desired behavior of the MachineDeployment.
type MachineDeploymentSpec struct {
	// Number of desired machines. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`

	// Label selector for machines. Existing MachineSets whose machines are
	// selected by this will be the ones affected by this MachineDeployment.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,2,opt,name=selector"`

	// Template describes the machines that will be created.
	Template MachineTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`

	// The MachineDeployment strategy to use to replace existing machines with new ones.
	// +optional
	// +patchStrategy=retainKeys
	Strategy MachineDeploymentStrategy `json:"strategy,omitempty" patchStrategy:"retainKeys" protobuf:"bytes,4,opt,name=strategy"`

	// Minimum number of seconds for which a newly created machine should be ready
	// without any of its container crashing, for it to be considered available.
	// Defaults to 0 (machine will be considered available as soon as it is ready)
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty" protobuf:"varint,5,opt,name=minReadySeconds"`

	// The number of old MachineSets to retain to allow rollback.
	// This is a pointer to distinguish between explicit zero and not specified.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty" protobuf:"varint,6,opt,name=revisionHistoryLimit"`

	// Indicates that the MachineDeployment is paused and will not be processed by the
	// MachineDeployment controller.
	// +optional
	Paused bool `json:"paused,omitempty" protobuf:"varint,7,opt,name=paused"`

	// DEPRECATED.
	// The config this MachineDeployment is rolling back to. Will be cleared after rollback is done.
	// +optional
	RollbackTo *RollbackConfig `json:"rollbackTo,omitempty" protobuf:"bytes,8,opt,name=rollbackTo"`

	// The maximum time in seconds for a MachineDeployment to make progress before it
	// is considered to be failed. The MachineDeployment controller will continue to
	// process failed MachineDeployments and a condition with a ProgressDeadlineExceeded
	// reason will be surfaced in the MachineDeployment status. Note that progress will
	// not be estimated during the time a MachineDeployment is paused. This is not set
	// by default.
	// +optional
	ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty" protobuf:"varint,9,opt,name=progressDeadlineSeconds"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DEPRECATED.
// MachineDeploymentRollback stores the information required to rollback a MachineDeployment.
type MachineDeploymentRollback struct {
	metav1.TypeMeta `json:",inline"`

	// Required: This must match the Name of a MachineDeployment.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// The annotations to be updated to a MachineDeployment
	// +optional
	UpdatedAnnotations map[string]string `json:"updatedAnnotations,omitempty" protobuf:"bytes,2,rep,name=updatedAnnotations"`

	// The config of this MachineDeployment rollback.
	RollbackTo RollbackConfig `json:"rollbackTo" protobuf:"bytes,3,opt,name=rollbackTo"`
}

type RollbackConfig struct {
	// The revision to rollback to. If set to 0, rollback to the last revision.
	// +optional
	Revision int64 `json:"revision,omitempty" protobuf:"varint,1,opt,name=revision"`
}

const (
	// DefaultDeploymentUniqueLabelKey is the default key of the selector that is added
	// to existing MCs (and label key that is added to its machines) to prevent the existing MCs
	// to select new machines (and old machines being select by new MC).
	DefaultMachineDeploymentUniqueLabelKey string = "machine-template-hash"
)

// MachineDeploymentStrategy describes how to replace existing machines with new ones.
type MachineDeploymentStrategy struct {
	// Type of MachineDeployment. Can be "Recreate" or "RollingUpdate". Default is RollingUpdate.
	// +optional
	Type MachineDeploymentStrategyType `json:"type,omitempty" protobuf:"bytes,1,opt,name=type,casttype=DeploymentStrategyType"`

	// Rolling update config params. Present only if MachineDeploymentStrategyType =
	// RollingUpdate.
	//---
	// TODO: Update this to follow our convention for oneOf, whatever we decide it
	// to be.
	// +optional
	RollingUpdate *RollingUpdateMachineDeployment `json:"rollingUpdate,omitempty" protobuf:"bytes,2,opt,name=rollingUpdate"`
}

type MachineDeploymentStrategyType string

const (
	// Kill all existing machines before creating new ones.
	RecreateMachineDeploymentStrategyType MachineDeploymentStrategyType = "Recreate"

	// Replace the old MCs by new one using rolling update i.e gradually scale down the old MCs and scale up the new one.
	RollingUpdateMachineDeploymentStrategyType MachineDeploymentStrategyType = "RollingUpdate"
)

// Spec to control the desired behavior of rolling update.
type RollingUpdateMachineDeployment struct {
	// The maximum number of machines that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// By default, a fixed value of 1 is used.
	// Example: when this is set to 30%, the old MC can be scaled down to 70% of desired machines
	// immediately when the rolling update starts. Once new machines are ready, old MC
	// can be scaled down further, followed by scaling up the new MC, ensuring
	// that the total number of machines available at all times during the update is at
	// least 70% of desired machines.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty" protobuf:"bytes,1,opt,name=maxUnavailable"`

	// The maximum number of machines that can be scheduled above the desired number of
	// machines.
	// Value can be an absolute number (ex: 5) or a percentage of desired machines (ex: 10%).
	// This can not be 0 if MaxUnavailable is 0.
	// Absolute number is calculated from percentage by rounding up.
	// By default, a value of 1 is used.
	// Example: when this is set to 30%, the new MC can be scaled up immediately when
	// the rolling update starts, such that the total number of old and new machines do not exceed
	// 130% of desired machines. Once old machines have been killed,
	// new MC can be scaled up further, ensuring that total number of machines running
	// at any time during the update is atmost 130% of desired machines.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty" protobuf:"bytes,2,opt,name=maxSurge"`
}

// MachineDeploymentStatus is the most recently observed status of the MachineDeployment.
type MachineDeploymentStatus struct {
	// The generation observed by the MachineDeployment controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,1,opt,name=observedGeneration"`

	// Total number of non-terminated machines targeted by this MachineDeployment (their labels match the selector).
	// +optional
	Replicas int32 `json:"replicas,omitempty" protobuf:"varint,2,opt,name=replicas"`

	// Total number of non-terminated machines targeted by this MachineDeployment that have the desired template spec.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty" protobuf:"varint,3,opt,name=updatedReplicas"`

	// Total number of ready machines targeted by this MachineDeployment.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty" protobuf:"varint,7,opt,name=readyReplicas"`

	// Total number of available machines (ready for at least minReadySeconds) targeted by this MachineDeployment.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty" protobuf:"varint,4,opt,name=availableReplicas"`

	// Total number of unavailable machines targeted by this MachineDeployment. This is the total number of
	// machines that are still required for the MachineDeployment to have 100% available capacity. They may
	// either be machines that are running but not yet available or machines that still have not been created.
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty" protobuf:"varint,5,opt,name=unavailableReplicas"`

	// Represents the latest available observations of a MachineDeployment's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []MachineDeploymentCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,6,rep,name=conditions"`

	// Count of hash collisions for the MachineDeployment. The MachineDeployment controller uses this
	// field as a collision avoidance mechanism when it needs to create the name for the
	// newest MachineSet.
	// +optional
	CollisionCount *int32 `json:"collisionCount,omitempty" protobuf:"varint,8,opt,name=collisionCount"`

	// FailedMachines has summary of machines on which lastOperation Failed
	// +optional
	FailedMachines []*MachineSummary `json:"failedMachines,omitempty" protobuf:"bytes,9,rep,name=failedMachines"`
}

type MachineDeploymentConditionType string

// These are valid conditions of a MachineDeployment.
const (
	// Available means the MachineDeployment is available, ie. at least the minimum available
	// replicas required are up and running for at least minReadySeconds.
	MachineDeploymentAvailable MachineDeploymentConditionType = "Available"

	// Progressing means the MachineDeployment is progressing. Progress for a MachineDeployment is
	// considered when a new machine set is created or adopted, and when new machines scale
	// up or old machines scale down. Progress is not estimated for paused MachineDeployments or
	// when progressDeadlineSeconds is not specified.
	MachineDeploymentProgressing MachineDeploymentConditionType = "Progressing"

	// ReplicaFailure is added in a MachineDeployment when one of its machines fails to be created
	// or deleted.
	MachineDeploymentReplicaFailure MachineDeploymentConditionType = "ReplicaFailure"

	// MachineDeploymentFrozen is added in a MachineDeployment when one of its machines fails to be created
	// or deleted.
	MachineDeploymentFrozen MachineDeploymentConditionType = "Frozen"
)

// MachineDeploymentCondition describes the state of a MachineDeployment at a certain point.
type MachineDeploymentCondition struct {
	// Type of MachineDeployment condition.
	Type MachineDeploymentConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=MachineDeploymentConditionType"`

	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=corev1.ConditionStatus"`

	// The last time this condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty" protobuf:"bytes,6,opt,name=lastUpdateTime"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,7,opt,name=lastTransitionTime"`

	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`

	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty" protobuf:"bytes,5,opt,name=message"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MachineDeploymentList is a list of MachineDeployments.
type MachineDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is the list of MachineDeployments.
	Items []MachineDeployment `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// describes the attributes of a scale subresource
type ScaleSpec struct {
	// desired number of machines for the scaled object.
	// +optional
	Replicas int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
}

// represents the current status of a scale subresource.
type ScaleStatus struct {
	// actual number of observed machines of the scaled object.
	Replicas int32 `json:"replicas" protobuf:"varint,1,opt,name=replicas"`

	// label query over machines that should match the replicas count. More info: http://kubernetes.io/docs/user-guide/labels#label-selectors
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,2,rep,name=selector"`

	// label selector for machines that should match the replicas count. This is a serializated
	// version of both map-based and more expressive set-based selectors. This is done to
	// avoid introspection in the clients. The string will be in the same format as the
	// query-param syntax. If the target type only supports map-based selectors, both this
	// field and map-based selector field are populated.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// +optional
	TargetSelector string `json:"targetSelector,omitempty" protobuf:"bytes,3,opt,name=targetSelector"`
}

// +genclient
// +genclient:noVerbs
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// represents a scaling request for a resource.
type Scale struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// defines the behavior of the scale. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status.
	// +optional
	Spec ScaleSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// current status of the scale. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status. Read-only.
	// +optional
	Status ScaleStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

/********************** OpenStackMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenStackMachineClass TODO
type OpenStackMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec OpenStackMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenStackMachineClassList is a collection of OpenStackMachineClasses.
type OpenStackMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []OpenStackMachineClass `json:"items"`
}

// OpenStackMachineClassSpec is the specification of a cluster.
type OpenStackMachineClassSpec struct {
	ImageName        string                  `json:"imageName"`
	Region           string                  `json:"region"`
	AvailabilityZone string                  `json:"availabilityZone"`
	FlavorName       string                  `json:"flavorName"`
	KeyName          string                  `json:"keyName"`
	SecurityGroups   []string                `json:"securityGroups"`
	Tags             map[string]string       `json:"tags,omitempty"`
	NetworkID        string                  `json:"networkID"`
	SecretRef        *corev1.SecretReference `json:"secretRef,omitempty"`
	PodNetworkCidr   string                  `json:"podNetworkCidr"`
}

/********************** AWSMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AWSMachineClass TODO
type AWSMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec AWSMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AWSMachineClassList is a collection of AWSMachineClasses.
type AWSMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []AWSMachineClass `json:"items"`
}

// AWSMachineClassSpec is the specification of a cluster.
type AWSMachineClassSpec struct {
	AMI               string                      `json:"ami,omitempty"`
	Region            string                      `json:"region,omitempty"`
	BlockDevices      []AWSBlockDeviceMappingSpec `json:"blockDevices,omitempty"`
	EbsOptimized      bool                        `json:"ebsOptimized,omitempty"`
	IAM               AWSIAMProfileSpec           `json:"iam,omitempty"`
	MachineType       string                      `json:"machineType,omitempty"`
	KeyName           string                      `json:"keyName,omitempty"`
	Monitoring        bool                        `json:"monitoring,omitempty"`
	NetworkInterfaces []AWSNetworkInterfaceSpec   `json:"networkInterfaces,omitempty"`
	Tags              map[string]string           `json:"tags,omitempty"`
	SecretRef         *corev1.SecretReference     `json:"secretRef,omitempty"`

	// TODO add more here
}

type AWSBlockDeviceMappingSpec struct {

	// The device name exposed to the machine (for example, /dev/sdh or xvdh).
	DeviceName string `json:"deviceName,omitempty"`

	// Parameters used to automatically set up EBS volumes when the machine is
	// launched.
	Ebs AWSEbsBlockDeviceSpec `json:"ebs,omitempty"`

	// Suppresses the specified device included in the block device mapping of the
	// AMI.
	NoDevice string `json:"noDevice,omitempty"`

	// The virtual device name (ephemeralN). Machine store volumes are numbered
	// starting from 0. An machine type with 2 available machine store volumes
	// can specify mappings for ephemeral0 and ephemeral1.The number of available
	// machine store volumes depends on the machine type. After you connect to
	// the machine, you must mount the volume.
	//
	// Constraints: For M3 machines, you must specify machine store volumes in
	// the block device mapping for the machine. When you launch an M3 machine,
	// we ignore any machine store volumes specified in the block device mapping
	// for the AMI.
	VirtualName string `json:"virtualName,omitempty"`
}

// Describes a block device for an EBS volume.
// Please also see https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/EbsBlockDevice
type AWSEbsBlockDeviceSpec struct {

	// Indicates whether the EBS volume is deleted on machine termination.
	DeleteOnTermination bool `json:"deleteOnTermination,omitempty"`

	// Indicates whether the EBS volume is encrypted. Encrypted Amazon EBS volumes
	// may only be attached to machines that support Amazon EBS encryption.
	Encrypted bool `json:"encrypted,omitempty"`

	// The number of I/O operations per second (IOPS) that the volume supports.
	// For io1, this represents the number of IOPS that are provisioned for the
	// volume. For gp2, this represents the baseline performance of the volume and
	// the rate at which the volume accumulates I/O credits for bursting. For more
	// information about General Purpose SSD baseline performance, I/O credits,
	// and bursting, see Amazon EBS Volume Types (http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html)
	// in the Amazon Elastic Compute Cloud User Guide.
	//
	// Constraint: Range is 100-20000 IOPS for io1 volumes and 100-10000 IOPS for
	// gp2 volumes.
	//
	// Condition: This parameter is required for requests to create io1 volumes;
	// it is not used in requests to create gp2, st1, sc1, or standard volumes.
	Iops int64 `json:"iops,omitempty"`

	// The size of the volume, in GiB.
	//
	// Constraints: 1-16384 for General Purpose SSD (gp2), 4-16384 for Provisioned
	// IOPS SSD (io1), 500-16384 for Throughput Optimized HDD (st1), 500-16384 for
	// Cold HDD (sc1), and 1-1024 for Magnetic (standard) volumes. If you specify
	// a snapshot, the volume size must be equal to or larger than the snapshot
	// size.
	//
	// Default: If you're creating the volume from a snapshot and don't specify
	// a volume size, the default is the snapshot size.
	VolumeSize int64 `json:"volumeSize,omitempty"`

	// The volume type: gp2, io1, st1, sc1, or standard.
	//
	// Default: standard
	VolumeType string `json:"volumeType,omitempty"`
}

// Describes an IAM machine profile.
type AWSIAMProfileSpec struct {
	// The Amazon Resource Name (ARN) of the machine profile.
	ARN string `json:"arn,omitempty"`

	// The name of the machine profile.
	Name string `json:"name,omitempty"`
}

// Describes a network interface.
// Please also see https://docs.aws.amazon.com/goto/WebAPI/ec2-2016-11-15/MachineAWSNetworkInterfaceSpecification
type AWSNetworkInterfaceSpec struct {

	// Indicates whether to assign a public IPv4 address to an machine you launch
	// in a VPC. The public IP address can only be assigned to a network interface
	// for eth0, and can only be assigned to a new network interface, not an existing
	// one. You cannot specify more than one network interface in the request. If
	// launching into a default subnet, the default value is true.
	AssociatePublicIPAddress bool `json:"associatePublicIPAddress,omitempty"`

	// If set to true, the interface is deleted when the machine is terminated.
	// You can specify true only if creating a new network interface when launching
	// an machine.
	DeleteOnTermination bool `json:"deleteOnTermination,omitempty"`

	// The description of the network interface. Applies only if creating a network
	// interface when launching an machine.
	Description string `json:"description,omitempty"`

	// The IDs of the security groups for the network interface. Applies only if
	// creating a network interface when launching an machine.
	SecurityGroupIDs []string `json:"securityGroupIDs,omitempty"`

	// The ID of the subnet associated with the network string. Applies only if
	// creating a network interface when launching an machine.
	SubnetID string `json:"subnetID,omitempty"`
}

/********************** AzureMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AzureMachineClass TODO
type AzureMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec AzureMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AzureMachineClassList is a collection of AzureMachineClasses.
type AzureMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []AzureMachineClass `json:"items"`
}

// AzureMachineClassSpec is the specification of a cluster.
type AzureMachineClassSpec struct {
	Location      string                        `json:"location,omitempty"`
	Tags          map[string]string             `json:"tags,omitempty"`
	Properties    AzureVirtualMachineProperties `json:"properties,omitempty"`
	ResourceGroup string                        `json:"resourceGroup,omitempty"`
	SubnetInfo    AzureSubnetInfo               `json:"subnetInfo,omitempty"`
	SecretRef     *corev1.SecretReference       `json:"secretRef,omitempty"`
}

// AzureVirtualMachineProperties is describes the properties of a Virtual Machine.
type AzureVirtualMachineProperties struct {
	HardwareProfile AzureHardwareProfile `json:"hardwareProfile,omitempty"`
	StorageProfile  AzureStorageProfile  `json:"storageProfile,omitempty"`
	OsProfile       AzureOSProfile       `json:"osProfile,omitempty"`
	NetworkProfile  AzureNetworkProfile  `json:"networkProfile,omitempty"`
	AvailabilitySet AzureSubResource     `json:"availabilitySet,omitempty"`
}

// AzureHardwareProfile is specifies the hardware settings for the virtual machine.
// Refer github.com/Azure/azure-sdk-for-go/arm/compute/models.go for VMSizes
type AzureHardwareProfile struct {
	VMSize string `json:"vmSize,omitempty"`
}

// AzureStorageProfile is specifies the storage settings for the virtual machine disks.
type AzureStorageProfile struct {
	ImageReference AzureImageReference `json:"imageReference,omitempty"`
	OsDisk         AzureOSDisk         `json:"osDisk,omitempty"`
}

// AzureImageReference is specifies information about the image to use. You can specify information about platform images,
// marketplace images, or virtual machine images. This element is required when you want to use a platform image,
// marketplace image, or virtual machine image, but is not used in other creation operations.
type AzureImageReference struct {
	ID        string `json:"id,omitempty"`
	Publisher string `json:"publisher,omitempty"`
	Offer     string `json:"offer,omitempty"`
	Sku       string `json:"sku,omitempty"`
	Version   string `json:"version,omitempty"`
}

// AzureOSDisk is specifies information about the operating system disk used by the virtual machine. <br><br> For more
// information about disks, see [About disks and VHDs for Azure virtual
// machines](https://docs.microsoft.com/azure/virtual-machines/virtual-machines-windows-about-disks-vhds?toc=%2fazure%2fvirtual-machines%2fwindows%2ftoc.json).
type AzureOSDisk struct {
	Name         string                     `json:"name,omitempty"`
	Caching      string                     `json:"caching,omitempty"`
	ManagedDisk  AzureManagedDiskParameters `json:"managedDisk,omitempty"`
	DiskSizeGB   int32                      `json:"diskSizeGB,omitempty"`
	CreateOption string                     `json:"createOption,omitempty"`
}

// AzureManagedDiskParameters is the parameters of a managed disk.
type AzureManagedDiskParameters struct {
	ID                 string `json:"id,omitempty"`
	StorageAccountType string `json:"storageAccountType,omitempty"`
}

// AzureOSProfile is specifies the operating system settings for the virtual machine.
type AzureOSProfile struct {
	ComputerName       string                  `json:"computerName,omitempty"`
	AdminUsername      string                  `json:"adminUsername,omitempty"`
	AdminPassword      string                  `json:"adminPassword,omitempty"`
	CustomData         string                  `json:"customData,omitempty"`
	LinuxConfiguration AzureLinuxConfiguration `json:"linuxConfiguration,omitempty"`
}

// AzureLinuxConfiguration is specifies the Linux operating system settings on the virtual machine. <br><br>For a list of
// supported Linux distributions, see [Linux on Azure-Endorsed
// Distributions](https://docs.microsoft.com/azure/virtual-machines/virtual-machines-linux-endorsed-distros?toc=%2fazure%2fvirtual-machines%2flinux%2ftoc.json)
// <br><br> For running non-endorsed distributions, see [Information for Non-Endorsed
// Distributions](https://docs.microsoft.com/azure/virtual-machines/virtual-machines-linux-create-upload-generic?toc=%2fazure%2fvirtual-machines%2flinux%2ftoc.json).
type AzureLinuxConfiguration struct {
	DisablePasswordAuthentication bool                  `json:"disablePasswordAuthentication,omitempty"`
	SSH                           AzureSSHConfiguration `json:"ssh,omitempty"`
}

// AzureSSHConfiguration is SSH configuration for Linux based VMs running on Azure
type AzureSSHConfiguration struct {
	PublicKeys AzureSSHPublicKey `json:"publicKeys,omitempty"`
}

// AzureSSHPublicKey is contains information about SSH certificate public key and the path on the Linux VM where the public
// key is placed.
type AzureSSHPublicKey struct {
	Path    string `json:"path,omitempty"`
	KeyData string `json:"keyData,omitempty"`
}

// AzureNetworkProfile is specifies the network interfaces of the virtual machine.
type AzureNetworkProfile struct {
	NetworkInterfaces AzureNetworkInterfaceReference `json:"networkInterfaces,omitempty"`
}

// AzureNetworkInterfaceReference is describes a network interface reference.
type AzureNetworkInterfaceReference struct {
	ID                                        string `json:"id,omitempty"`
	*AzureNetworkInterfaceReferenceProperties `json:"properties,omitempty"`
}

// AzureNetworkInterfaceReferenceProperties is describes a network interface reference properties.
type AzureNetworkInterfaceReferenceProperties struct {
	Primary bool `json:"primary,omitempty"`
}

// AzureSubResource is the Sub Resource definition.
type AzureSubResource struct {
	ID string `json:"id,omitempty"`
}

// AzureSubnetInfo is the information containing the subnet details
type AzureSubnetInfo struct {
	VnetName   string `json:"vnetName,omitempty"`
	SubnetName string `json:"subnetName,omitempty"`
}

/********************** GCPMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GCPMachineClass TODO
type GCPMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec GCPMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GCPMachineClassList is a collection of GCPMachineClasses.
type GCPMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []GCPMachineClass `json:"items"`
}

// GCPMachineClassSpec is the specification of a cluster.
type GCPMachineClassSpec struct {
	CanIpForward       bool                    `json:"canIpForward"`
	DeletionProtection bool                    `json:"deletionProtection"`
	Description        *string                 `json:"description,omitempty"`
	Disks              []*GCPDisk              `json:"disks,omitempty"`
	Labels             map[string]string       `json:"labels,omitempty"`
	MachineType        string                  `json:"machineType"`
	Metadata           []*GCPMetadata          `json:"metadata,omitempty"`
	NetworkInterfaces  []*GCPNetworkInterface  `json:"networkInterfaces,omitempty"`
	Scheduling         GCPScheduling           `json:"scheduling"`
	SecretRef          *corev1.SecretReference `json:"secretRef,omitempty"`
	ServiceAccounts    []GCPServiceAccount     `json:"serviceAccounts"`
	Tags               []string                `json:"tags,omitempty"`
	Region             string                  `json:"region"`
	Zone               string                  `json:"zone"`
}

// GCPDisk describes disks for GCP.
type GCPDisk struct {
	AutoDelete bool              `json:"autoDelete"`
	Boot       bool              `json:"boot"`
	SizeGb     int64             `json:"sizeGb"`
	Type       string            `json:"type"`
	Image      string            `json:"image"`
	Labels     map[string]string `json:"labels"`
}

// GCPMetadata describes metadata for GCP.
type GCPMetadata struct {
	Key   string  `json:"key"`
	Value *string `json:"value"`
}

// GCPNetworkInterface describes network interfaces for GCP
type GCPNetworkInterface struct {
	Network    string `json:"network,omitempty"`
	Subnetwork string `json:"subnetwork,omitempty"`
}

// GCPScheduling describes scheduling configuration for GCP.
type GCPScheduling struct {
	AutomaticRestart  bool   `json:"automaticRestart"`
	OnHostMaintenance string `json:"onHostMaintenance"`
	Preemptible       bool   `json:"preemptible"`
}

// GCPServiceAccount describes service accounts for GCP.
type GCPServiceAccount struct {
	Email  string   `json:"email"`
	Scopes []string `json:"scopes"`
}

const (
	// AWSAccessKeyID is a constant for a key name that is part of the AWS cloud credentials.
	AWSAccessKeyID string = "providerAccessKeyId"
	// AWSSecretAccessKey is a constant for a key name that is part of the AWS cloud credentials.
	AWSSecretAccessKey string = "providerSecretAccessKey"

	// AzureClientID is a constant for a key name that is part of the Azure cloud credentials.
	AzureClientID string = "azureClientId"
	// AzureClientSecret is a constant for a key name that is part of the Azure cloud credentials.
	AzureClientSecret string = "azureClientSecret"
	// AzureSubscriptionID is a constant for a key name that is part of the Azure cloud credentials.
	AzureSubscriptionID string = "azureSubscriptionId"
	// AzureTenantID is a constant for a key name that is part of the Azure cloud credentials.
	AzureTenantID string = "azureTenantId"

	// GCPServiceAccountJSON is a constant for a key name that is part of the GCP cloud credentials.
	GCPServiceAccountJSON string = "serviceAccountJSON"

	// OpenStackAuthURL is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackAuthURL string = "authURL"
	// OpenStackCACert is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackCACert string = "caCert"
	// OpenStackInsecure is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackInsecure string = "insecure"
	// OpenStackDomainName is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackDomainName string = "domainName"
	// OpenStackTenantName is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackTenantName string = "tenantName"
	// OpenStackUsername is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackUsername string = "username"
	// OpenStackPassword is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackPassword string = "password"
	// OpenStackClientCert is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackClientCert string = "clientCert"
	// OpenStackClientKey is a constant for a key name that is part of the OpenStack cloud credentials.
	OpenStackClientKey string = "clientKey"

	// AlicloudAccessKeyID is a constant for a key name that is part of the Alibaba cloud credentials.
	AlicloudAccessKeyID string = "alicloudAccessKeyID"
	// AlicloudAccessKeySecret is a constant for a key name that is part of the Alibaba cloud credentials.
	AlicloudAccessKeySecret string = "alicloudAccessKeySecret"

	// PacketAPIKey is a constant for a key name that is part of the Packet cloud credentials
	PacketAPIKey string = "apiToken"
)

/********************** AlicloudMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AlicloudMachineClass TODO
type AlicloudMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec AlicloudMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AlicloudMachineClassList is a collection of AlicloudMachineClasses.
type AlicloudMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []AlicloudMachineClass `json:"items"`
}

// AlicloudMachineClassSpec is the specification of a cluster.
type AlicloudMachineClassSpec struct {
	ImageID                 string                  `json:"imageID"`
	InstanceType            string                  `json:"instanceType"`
	Region                  string                  `json:"region"`
	ZoneID                  string                  `json:"zoneID,omitempty"`
	SecurityGroupID         string                  `json:"securityGroupID,omitempty"`
	VSwitchID               string                  `json:"vSwitchID"`
	PrivateIPAddress        string                  `json:"privateIPAddress,omitempty"`
	SystemDisk              *AlicloudSystemDisk     `json:"systemDisk,omitempty"`
	InstanceChargeType      string                  `json:"instanceChargeType,omitempty"`
	InternetChargeType      string                  `json:"internetChargeType,omitempty"`
	InternetMaxBandwidthIn  *int                    `json:"internetMaxBandwidthIn,omitempty"`
	InternetMaxBandwidthOut *int                    `json:"internetMaxBandwidthOut,omitempty"`
	SpotStrategy            string                  `json:"spotStrategy,omitempty"`
	IoOptimized             string                  `json:"IoOptimized,omitempty"`
	Tags                    map[string]string       `json:"tags,omitempty"`
	KeyPairName             string                  `json:"keyPairName"`
	SecretRef               *corev1.SecretReference `json:"secretRef,omitempty"`
}

// AlicloudSystemDisk describes SystemDisk for Alicloud.
type AlicloudSystemDisk struct {
	Category string `json:"category"`
	Size     int    `json:"size"`
}

/********************** PacketMachineClass APIs ***************/

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PacketMachineClass TODO
type PacketMachineClass struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	Spec PacketMachineClassSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PacketMachineClassList is a collection of PacketMachineClasses.
type PacketMachineClassList struct {
	// +optional
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// +optional
	Items []PacketMachineClass `json:"items"`
}

// PacketMachineClassSpec is the specification of a cluster.
type PacketMachineClassSpec struct {
	Facility     []string           `json:"facility"`
	MachineType  string             `json:"machineType"`
	BillingCycle string             `json:"billingCycle"`
	OS           string             `json:"OS"`
	ProjectID    string             `json:"projectID"`
	Tags         []string  `json:"tags,omitempty"`
	SSHKeys      []string `json:"sshKeys,omitempty"`
	UserData     string             `json:"userdata,omitempty"`

	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
}
