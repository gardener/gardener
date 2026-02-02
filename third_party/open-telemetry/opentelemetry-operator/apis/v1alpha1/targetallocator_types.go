// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1alpha1/targetallocator_types.go.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/third_party/open-telemetry/opentelemetry-operator/apis/v1beta1"
)

func init() {
	SchemeBuilder.Register(&TargetAllocator{}, &TargetAllocatorList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".status.image"
// +kubebuilder:printcolumn:name="Management",type="string",JSONPath=".spec.managementState",description="Management State"
// +operator-sdk:csv:customresourcedefinitions:displayName="Target Allocator"
// +operator-sdk:csv:customresourcedefinitions:resources={{Pod,v1},{Deployment,apps/v1},{ConfigMaps,v1},{Service,v1},{ServiceAccount,v1},{PodDisruptionBudget,policy/v1}}

// TargetAllocator is the Schema for the targetallocators API.
type TargetAllocator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TargetAllocatorSpec   `json:"spec,omitempty"`
	Status TargetAllocatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TargetAllocatorList contains a list of TargetAllocator.
type TargetAllocatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TargetAllocator `json:"items"`
}

// TargetAllocatorStatus defines the observed state of Target Allocator.
type TargetAllocatorStatus struct {
	// Version of the managed Target Allocator (operand)
	// +optional
	Version string `json:"version,omitempty"`

	// Image indicates the container image to use for the Target Allocator.
	// +optional
	Image string `json:"image,omitempty"`
}

// TargetAllocatorSpec defines the desired state of TargetAllocator.
type TargetAllocatorSpec struct {
	// Common defines fields that are common to all OpenTelemetry CRD workloads.
	v1beta1.OpenTelemetryCommonFields `json:",inline"`
	// AllocationStrategy determines which strategy the target allocator should use for allocation.
	// The current options are least-weighted, consistent-hashing and per-node. The default is
	// consistent-hashing.
	// WARNING: The per-node strategy currently ignores targets without a Node, like control plane components.
	// +optional
	// +kubebuilder:default:=consistent-hashing
	AllocationStrategy v1beta1.TargetAllocatorAllocationStrategy `json:"allocationStrategy,omitempty"`
	// FilterStrategy determines how to filter targets before allocating them among the collectors.
	// The only current option is relabel-config (drops targets based on prom relabel_config).
	// The default is relabel-config.
	// +optional
	// +kubebuilder:default:=relabel-config
	FilterStrategy v1beta1.TargetAllocatorFilterStrategy `json:"filterStrategy,omitempty"`
	// GlobalConfig configures the global configuration for Prometheus
	// For more info, see https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file.
	GlobalConfig v1beta1.AnyConfig `json:"global,omitempty"`
	// ScrapeConfigs define static Prometheus scrape configurations for the target allocator.
	// To use dynamic configurations from ServiceMonitors and PodMonitors, see the PrometheusCR section.
	// For the exact format, see https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config.
	// +optional
	// +listType=atomic
	// +kubebuilder:pruning:PreserveUnknownFields
	ScrapeConfigs []v1beta1.AnyConfig `json:"scrapeConfigs,omitempty"`
	// PrometheusCR defines the configuration for the retrieval of PrometheusOperator CRDs ( servicemonitor.monitoring.coreos.com/v1 and podmonitor.monitoring.coreos.com/v1 ).
	// +optional
	PrometheusCR v1beta1.TargetAllocatorPrometheusCR `json:"prometheusCR,omitempty"`
	// ObservabilitySpec defines how telemetry data gets handled.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Observability"
	Observability v1beta1.ObservabilitySpec `json:"observability,omitempty"`
	// NetworkPolicy defines the network policy to be applied to the Target Allocator pods.
	// +optional
	NetworkPolicy v1beta1.NetworkPolicy `json:"networkPolicy,omitempty"`
	// CollectorNotReadyGracePeriod defines the grace period after which a TargetAllocator stops considering a collector is target assignable.
	// The default is 30s, which means that if a collector becomes not Ready, the target allocator will wait for 30 seconds before reassigning its targets. The assumption is that the state is temporary, and an expensive target reallocation should be avoided if possible.
	//
	// +optional
	// +kubebuilder:default:="30s"
	// +kubebuilder:validation:Format:=duration
	CollectorNotReadyGracePeriod *metav1.Duration `json:"collectorNotReadyGracePeriod,omitempty"`
}
