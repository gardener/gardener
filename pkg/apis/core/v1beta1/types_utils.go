// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// EventSchedulingSuccessful is an event reason for successful scheduling.
	EventSchedulingSuccessful = "SchedulingSuccessful"
	// EventSchedulingFailed is an event reason for failed scheduling.
	EventSchedulingFailed = "SchedulingFailed"
)

// ConditionStatus is the status of a condition.
type ConditionStatus string

// ConditionType is a string alias.
type ConditionType string

// Condition holds the information about the state of a resource.
type Condition struct {
	// Type of the condition.
	Type ConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=ConditionType"`
	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=ConditionStatus"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime" protobuf:"bytes,3,opt,name=lastTransitionTime"`
	// Last time the condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime" protobuf:"bytes,4,opt,name=lastUpdateTime"`
	// The reason for the condition's last transition.
	Reason string `json:"reason" protobuf:"bytes,5,opt,name=reason"`
	// A human readable message indicating details about the transition.
	Message string `json:"message" protobuf:"bytes,6,opt,name=message"`
	// Well-defined error codes in case the condition reports a problem.
	// +optional
	Codes []ErrorCode `json:"codes,omitempty" protobuf:"bytes,7,rep,name=codes,casttype=ErrorCode"`
}

const (
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
	// ManagedResourceMissingConditionError is a constant for a reason in a condition that indicates
	// one or multiple missing conditions in the observed managed resource.
	ManagedResourceMissingConditionError = "MissingManagedResourceCondition"
	// OutdatedStatusError is a constant for a reason in a condition that indicates
	// that the observed generation in a status is outdated.
	OutdatedStatusError = "OutdatedStatus"
	// ManagedResourceProgressingRolloutStuck is a constant for a reason in a condition that indicates
	// managed resource progressing condition is stuck in the true state for more than the threshold time.
	ManagedResourceProgressingRolloutStuck = "ProgressingRolloutStuck"
)
