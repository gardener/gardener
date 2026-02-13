// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1alpha1/opambridge_types.go.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpAMPBridgeSpec defines the desired state of OpAMPBridge.
type OpAMPBridgeSpec struct {
	// OpAMP backend Server endpoint
	// +required
	Endpoint string `json:"endpoint"`
	// Headers is an optional map of headers to use when connecting to the OpAMP Server,
	// typically used to set access tokens or other authorization headers.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`
	// Capabilities supported by the OpAMP Bridge
	// +required
	Capabilities map[OpAMPBridgeCapability]bool `json:"capabilities"`
	// ComponentsAllowed is a list of allowed OpenTelemetry components for each pipeline type (receiver, processor, etc.)
	// +optional
	ComponentsAllowed map[string][]string `json:"componentsAllowed,omitempty"`
	// Description allows the customization of the non identifying attributes for the OpAMP Bridge.
	// +optional
	Description *AgentDescription `json:"description,omitempty"`
	// Resources to set on the OpAMPBridge pods.
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// NodeSelector to schedule OpAMPBridge pods.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Replicas is the number of pod instances for the OpAMPBridge.
	// +optional
	// +kubebuilder:validation:Maximum=1
	Replicas *int32 `json:"replicas,omitempty"`
	// SecurityContext will be set as the container security context.
	// +optional
	SecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`
	// PodSecurityContext will be set as the pod security context.
	// +optional
	PodSecurityContext *v1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// PodAnnotations is the set of annotations that will be attached to
	// OpAMPBridge pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// ServiceAccount indicates the name of an existing service account to use with this instance. When set,
	// the operator will not automatically create a ServiceAccount for the OpAMPBridge.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Image indicates the container image to use for the OpAMPBridge.
	// +optional
	Image string `json:"image,omitempty"`
	// UpgradeStrategy represents how the operator will handle upgrades to the CR when a newer version of the operator is deployed
	// +optional
	UpgradeStrategy UpgradeStrategy `json:"upgradeStrategy"`
	// ImagePullPolicy indicates the pull policy to be used for retrieving the container image (Always, Never, IfNotPresent)
	// +optional
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// VolumeMounts represents the mount points to use in the underlying OpAMPBridge deployment(s)
	// +optional
	// +listType=atomic
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// Ports allows a set of ports to be exposed by the underlying v1.Service.
	// +optional
	// +listType=atomic
	Ports []v1.ServicePort `json:"ports,omitempty"`
	// ENV vars to set on the OpAMPBridge Pods.
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`
	// List of sources to populate environment variables on the OpAMPBridge Pods.
	// +optional
	EnvFrom []v1.EnvFromSource `json:"envFrom,omitempty"`
	// Toleration to schedule OpAMPBridge pods.
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// Volumes represents which volumes to use in the underlying OpAMPBridge deployment(s).
	// +optional
	// +listType=atomic
	Volumes []v1.Volume `json:"volumes,omitempty"`
	// HostNetwork indicates if the pod should run in the host networking namespace.
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`
	// If specified, indicates the pod's priority.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// If specified, indicates the pod's scheduling constraints
	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`
	// TopologySpreadConstraints embedded kubernetes pod configuration option,
	// controls how pods are spread across your cluster among failure-domains
	// such as regions, zones, nodes, and other user-defined topology domains
	// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/
	// +optional
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// PodDNSConfig defines the DNS parameters of a pod in addition to those generated from DNSPolicy.
	PodDNSConfig v1.PodDNSConfig `json:"podDnsConfig,omitempty"`
	// IPFamily represents the IP Family (IPv4 or IPv6). This type is used
	// to express the family of an IP expressed by a type (e.g. service.spec.ipFamilies).
	// +optional
	IpFamilies []v1.IPFamily `json:"ipFamilies,omitempty"`
	// IPFamilyPolicy represents the dual-stack-ness requested or required by a Service
	IpFamilyPolicy *v1.IPFamilyPolicy `json:"ipFamilyPolicy,omitempty"`
}

// OpAMPBridgeStatus defines the observed state of OpAMPBridge.
type OpAMPBridgeStatus struct {
	// Version of the managed OpAMP Bridge (operand)
	// +optional
	Version string `json:"version,omitempty"`
}

type AgentDescription struct {
	// NonIdentifyingAttributes are a map of key-value pairs that may be specified to provide
	// extra information about the agent to the OpAMP server.
	NonIdentifyingAttributes map[string]string `json:"non_identifying_attributes"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="OpenTelemetry Version"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.endpoint"
// +operator-sdk:csv:customresourcedefinitions:displayName="OpAMP Bridge"
// +operator-sdk:csv:customresourcedefinitions:resources={{Pod,v1},{Deployment,apps/v1},{ConfigMaps,v1},{Service,v1}}

// OpAMPBridge is the Schema for the opampbridges API.
type OpAMPBridge struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpAMPBridgeSpec   `json:"spec,omitempty"`
	Status OpAMPBridgeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpAMPBridgeList contains a list of OpAMPBridge.
type OpAMPBridgeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpAMPBridge `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpAMPBridge{}, &OpAMPBridgeList{})
}
