/*
Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved.

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
	autoscaling "k8s.io/api/autoscaling/v2beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpa_api "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VpaTemplateSpec defines the spec for VPA
type VpaTemplateSpec struct {
	// Controls how the autoscaler computes recommended resources.
	// The resource policy may be used to set constraints on the recommendations
	// for individual containers. If not specified, the autoscaler computes recommended
	// resources for all containers in the pod, without additional constraints.
	// +optional
	ResourcePolicy *vpa_api.PodResourcePolicy `json:"resourcePolicy,omitempty" protobuf:"bytes,2,opt,name=resourcePolicy"`
}

// UpdatePolicy describes the rules on how changes are applied.
type UpdatePolicy struct {
	// Controls when autoscaler applies changes to the resources.
	// The default is 'On'.
	// +optional
	UpdateMode *string `json:"updateMode,omitempty" protobuf:"bytes,1,opt,name=updateMode"`
}

const (
	// UpdateModePurge means that HPA/VPA will not be created.
	UpdateModePurge string = "Purge"
	// UpdateModeOff means that autoscaler never changes resources.
	UpdateModeOff string = "Off"
	// UpdateModeAuto means that autoscaler can update resources during the lifetime of the resource.
	UpdateModeAuto string = "Auto"
	// UpdateModeScaleUp means that HPA/VPA will never scale down resources vertically.
	UpdateModeScaleUp string = "ScaleUp"
)

// HpaTemplateSpec defines the spec for HPA
type HpaTemplateSpec struct {
	// minReplicas is the lower limit for the number of replicas to which the autoscaler can scale down.
	// It defaults to 1 pod.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty" protobuf:"varint,1,opt,name=minReplicas"`

	// maxReplicas is the upper limit for the number of replicas to which the autoscaler can scale up.
	// It cannot be less that minReplicas.
	MaxReplicas int32 `json:"maxReplicas" protobuf:"varint,2,opt,name=maxReplicas"`

	// metrics contains the specifications for which to use to calculate the
	// desired replica count (the maximum replica count across all metrics will
	// be used).  The desired replica count is calculated multiplying the
	// ratio between the target value and the current value by the current
	// number of pods.  Ergo, metrics used must decrease as the pod count is
	// increased, and vice-versa.  See the individual metric source types for
	// more information about how each type of metric must respond.
	// If not set, the default metric will be set to 80% average CPU utilization.
	// +optional
	Metrics []autoscaling.MetricSpec `json:"metrics,omitempty" protobuf:"bytes,3,rep,name=metrics"`
}

// WeightBasedScalingInterval defines the interval of replica counts in which VpaWeight is applied to VPA scaling
type WeightBasedScalingInterval struct {
	// VpaWeight defines the weight (in percentage) to be given to VPA's recommendationd for the interval of number of replicas provided
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	VpaWeight VpaWeight `json:"vpaWeight,omitempty"`
	// StartReplicaCount is the number of replicas from which VpaWeight is applied to VPA scaling
	// If this field is not provided, it will default to minReplicas of HPA
	// +optional
	StartReplicaCount int32 `json:"startReplicaCount,omitempty"`
	// LastReplicaCount is the number of replicas till which VpaWeight is applied to VPA scaling
	// If this field is not provided, it will default to maxReplicas of HPA
	// +optional
	LastReplicaCount int32 `json:"lastReplicaCount,omitempty"`
}

// ScaleStabilization defines stabilization parameters after last scaling
type ScaleStabilization struct {
	// Duration defines the minimum delay in minutes between 2 consecutive scale operations
	// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"
	Duration *string `json:"duration,omitempty"`
	// MinCpuChange is the minimum change in CPU on which HVPA acts
	// HVPA uses minimum of the Value and Percentage value
	MinCPUChange *ChangeThreshold `json:"minCpuChange,omitempty"`
	// MinMemChange is the minimum change in memory on which HVPA acts
	// HVPA uses minimum of the Value and Percentage value
	MinMemChange *ChangeThreshold `json:"minMemChange,omitempty"`
}

// VpaSpec defines spec for VPA
type VpaSpec struct {
	// Selector is a label query that should match VPA.
	// Must match in order to be controlled.
	// If empty, defaulted to labels on VPA template.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Describes the rules on how changes are applied.
	// If not specified, all fields in the `UpdatePolicy` are set to their
	// default values.
	// +optional
	UpdatePolicy *UpdatePolicy `json:"updatePolicy,omitempty"`

	// Template is the object that describes the VPA that will be created.
	// +optional
	Template VpaTemplate `json:"template,omitempty"`
}

// VpaTemplate defines the template for VPA
type VpaTemplate struct {
	// Metadata of the pods created from this template.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a VPA.
	// +optional
	Spec VpaTemplateSpec `json:"spec,omitempty"`
}

// HpaTemplate defines the template for HPA
type HpaTemplate struct {
	// Metadata of the pods created from this template.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a HPA.
	// +optional
	Spec HpaTemplateSpec `json:"spec,omitempty"`
}

// HpaSpec defines spec for HPA
type HpaSpec struct {
	// Selector is a label query that should match HPA.
	// Must match in order to be controlled.
	// If empty, defaulted to labels on HPA template.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Describes the rules on how changes are applied.
	// If not specified, all fields in the `UpdatePolicy` are set to their
	// default values.
	// +optional
	UpdatePolicy *UpdatePolicy `json:"updatePolicy,omitempty"`

	// Template is the object that describes the HPA that will be created.
	// +optional
	Template HpaTemplate `json:"template,omitempty"`
}

// HvpaSpec defines the desired state of Hvpa
type HvpaSpec struct {
	// Replicas is the number of replicas of target resource
	Replicas *int32 `json:"replicas,omitempty"`

	// Hpa defines the spec of HPA
	Hpa HpaSpec `json:"hpa,omitempty"`

	// Vpa defines the spec of VPA
	Vpa VpaSpec `json:"vpa,omitempty"`

	// WeightBasedScalingIntervals defines the intervals of replica counts, and the weights for scaling a deployment vertically
	// If there are overlapping intervals, then the vpaWeight will be taken from the first matching interval
	WeightBasedScalingIntervals []WeightBasedScalingInterval `json:"weightBasedScalingIntervals,omitempty"`

	// TargetRef points to the controller managing the set of pods for the autoscaler to control
	TargetRef *autoscaling.CrossVersionObjectReference `json:"targetRef"`

	// ScaleUpStabilization defines stabilization parameters after last scaling
	ScaleUpStabilization *ScaleStabilization `json:"scaleUpStabilization,omitempty"`

	// ScaleDownStabilization defines stabilization parameters after last scaling
	ScaleDownStabilization *ScaleStabilization `json:"scaleDownStabilization,omitempty"`
}

// ChangeThreshold defines the thresholds for HVPA to apply VPA's recommendations
type ChangeThreshold struct {
	// Value is the absolute value of the threshold
	// +optional
	Value *string `json:"value,omitempty"`
	// Percentage is the percentage of currently allocated value to be used as threshold
	// +optional
	Percentage *int32 `json:"percentage,omitempty"`
}

// VpaWeight - weight to provide to VPA scaling
type VpaWeight int32

const (
	// VpaOnly - only vertical scaling
	VpaOnly VpaWeight = 100
	// HpaOnly - only horizontal scaling
	HpaOnly VpaWeight = 0
)

// LastError has detailed information of the error
type LastError struct {
	// Description of the error
	Description string `json:"description,omitempty"`

	// Time at which the error occurred
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// LastOperation is the type of operation for which error occurred
	LastOperation string `json:"lastOperation,omitempty"`
}

// HvpaStatus defines the observed state of Hvpa
type HvpaStatus struct {
	// Replicas is the number of replicas of the target resource.
	Replicas *int32 `json:"replicas,omitempty"`
	// TargetSelector is the string form of the label selector of HPA. This is required for HPA to work with scale subresource.
	TargetSelector *string `json:"targetSelector,omitempty"`
	// Current HPA UpdatePolicy set in the spec
	HpaUpdatePolicy *UpdatePolicy `json:"hpaUpdatePolicy,omitempty"`
	// Current VPA UpdatePolicy set in the spec
	VpaUpdatePolicy *UpdatePolicy `json:"vpaUpdatePolicy,omitempty"`

	HpaWeight VpaWeight `json:"hpaWeight,omitempty"`
	VpaWeight VpaWeight `json:"vpaWeight,omitempty"`

	// Override scale up stabilization window
	OverrideScaleUpStabilization bool `json:"overrideScaleUpStabilization,omitempty"`

	LastBlockedScaling []*BlockedScaling `json:"lastBlockedScaling,omitempty"`
	LastScaling        ScalingStatus     `json:"lastScaling,omitempty"`

	// LastError has details of any errors that occured
	LastError *LastError `json:"lastError,omitempty"`
}

// BlockingReason defines the reason for blocking.
type BlockingReason string

const (
	// BlockingReasonStabilizationWindow - HVPA is in stabilization window
	BlockingReasonStabilizationWindow BlockingReason = "StabilizationWindow"
	// BlockingReasonMaintenanceWindow - Resource is in maintenance window
	BlockingReasonMaintenanceWindow BlockingReason = "MaintenanceWindow"
	// BlockingReasonUpdatePolicy - Update policy doesn't support scaling
	BlockingReasonUpdatePolicy BlockingReason = "UpdatePolicy"
	// BlockingReasonWeight  - VpaWeight doesn't support scaling
	BlockingReasonWeight BlockingReason = "Weight"
	// BlockingReasonMinChange - Min change doesn't support scaling
	BlockingReasonMinChange BlockingReason = "MinChange"
)

// BlockedScaling defines the details for blocked scaling
type BlockedScaling struct {
	Reason        BlockingReason `json:"reason,omitempty"`
	ScalingStatus `json:"scalingStatus,omitempty"`
}

// ScalingStatus defines the status of scaling
type ScalingStatus struct {
	LastScaleTime *metav1.Time                        `json:"lastScaleTime,omitempty"`
	HpaStatus     HpaStatus                           `json:"hpaStatus,omitempty" protobuf:"bytes,1,opt,name=hpaStatus"`
	VpaStatus     vpa_api.VerticalPodAutoscalerStatus `json:"vpaStatus,omitempty" protobuf:"bytes,2,opt,name=vpaStatus"`
}

// HpaStatus defines the status of HPA
type HpaStatus struct {
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`
}

// Hvpa is the Schema for the hvpas API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.targetSelector
type Hvpa struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HvpaSpec   `json:"spec,omitempty"`
	Status HvpaStatus `json:"status,omitempty"`
}

// HvpaList contains a list of Hvpa
// +kubebuilder:object:root=true
type HvpaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Hvpa `json:"items"`
}

/*func init() {
	SchemeBuilder.Register(&Hvpa{}, &HvpaList{})
}*/
