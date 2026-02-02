// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1beta1/opentelemetrycollector_types.go.

package v1beta1

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&OpenTelemetryCollector{}, &OpenTelemetryCollectorList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=otelcol;otelcols
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.scale.replicas,selectorpath=.status.scale.selector
// +kubebuilder:printcolumn:name="Mode",type="string",JSONPath=".spec.mode",description="Deployment Mode"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="OpenTelemetry Version"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.scale.statusReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".status.image"
// +kubebuilder:printcolumn:name="Management",type="string",JSONPath=".spec.managementState",description="Management State"
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenTelemetry Collector"
// This annotation provides a hint for OLM which resources are managed by OpenTelemetryCollector kind.
// It's not mandatory to list all resources.
// +operator-sdk:csv:customresourcedefinitions:resources={{Pod,v1},{Deployment,apps/v1},{DaemonSets,apps/v1},{StatefulSets,apps/v1},{ConfigMaps,v1},{Service,v1},{Ingress,networking/v1}}

// OpenTelemetryCollector is the Schema for the opentelemetrycollectors API.
type OpenTelemetryCollector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenTelemetryCollectorSpec   `json:"spec,omitempty"`
	Status OpenTelemetryCollectorStatus `json:"status,omitempty"`
}

// Hub exists to allow for conversion.
func (*OpenTelemetryCollector) Hub() {}

// +kubebuilder:object:root=true

// OpenTelemetryCollectorList contains a list of OpenTelemetryCollector.
type OpenTelemetryCollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenTelemetryCollector `json:"items"`
}

// OpenTelemetryCollectorStatus defines the observed state of OpenTelemetryCollector.
type OpenTelemetryCollectorStatus struct {
	// Scale is the OpenTelemetryCollector's scale subresource status.
	// +optional
	Scale ScaleSubresourceStatus `json:"scale,omitempty"`

	// Version of the managed OpenTelemetry Collector (operand)
	// +optional
	Version string `json:"version,omitempty"`

	// Image indicates the container image to use for the OpenTelemetry Collector.
	// +optional
	Image string `json:"image,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!(self.mode == 'sidecar' && size(self.tolerations) > 0) || !has(self.tolerations)",message="the OpenTelemetry Collector mode is set to sidecar, which does not support the attribute 'tolerations'"
// +kubebuilder:validation:XValidation:rule="!(self.mode == 'sidecar' && self.priorityClassName != '') || !has(self.priorityClassName)",message="the OpenTelemetry Collector mode is set to sidecar, which does not support the attribute 'priorityClassName'"
// +kubebuilder:validation:XValidation:rule="!(self.mode == 'sidecar' && self.affinity != null) || !has(self.affinity)",message="the OpenTelemetry Collector mode is set to sidecar, which does not support the attribute 'affinity'"
// +kubebuilder:validation:XValidation:rule="!(self.mode == 'sidecar' && size(self.additionalContainers) > 0) || !has(self.additionalContainers)",message="the OpenTelemetry Collector mode is set to sidecar, which does not support the attribute 'additionalContainers'"

// OpenTelemetryCollectorSpec defines the desired state of OpenTelemetryCollector.
type OpenTelemetryCollectorSpec struct {
	// OpenTelemetryCommonFields are fields that are on all OpenTelemetry CRD workloads.
	OpenTelemetryCommonFields `json:",inline"`
	// StatefulSetCommonFields are fields that are on all OpenTelemetry CRD workloads.
	StatefulSetCommonFields `json:",inline"`
	// Autoscaler specifies the pod autoscaling configuration to use
	// for the workload.
	// +optional
	Autoscaler *AutoscalerSpec `json:"autoscaler,omitempty"`
	// TargetAllocator indicates a value which determines whether to spawn a target allocation resource or not.
	// +optional
	TargetAllocator TargetAllocatorEmbedded `json:"targetAllocator,omitempty"`
	// Mode represents how the collector should be deployed (deployment, daemonset, statefulset or sidecar)
	// +optional
	Mode Mode `json:"mode,omitempty"`
	// UpgradeStrategy represents how the operator will handle upgrades to the CR when a newer version of the operator is deployed
	// +optional
	UpgradeStrategy UpgradeStrategy `json:"upgradeStrategy"`
	// Config is the raw JSON to be used as the collector's configuration. Refer to the OpenTelemetry Collector documentation for details.
	// The empty objects e.g. batch: should be written as batch: {} otherwise they won't work with kustomize or kubectl edit.
	// +required
	// +kubebuilder:pruning:PreserveUnknownFields
	Config Config `json:"config"`
	// ConfigVersions defines the number versions to keep for the collector config. Each config version is stored in a separate ConfigMap.
	// Defaults to 3. The minimum value is 1.
	// +optional
	// +kubebuilder:default:=3
	// +kubebuilder:validation:Minimum:=1
	ConfigVersions int `json:"configVersions,omitempty"`
	// Ingress is used to specify how OpenTelemetry Collector is exposed. This
	// functionality is only available if one of the valid modes is set.
	// Valid modes are: deployment, daemonset and statefulset.
	// +optional
	Ingress Ingress `json:"ingress,omitempty"`
	// NetworkPolicy defines the network policy to be applied to the OpenTelemetry Collector pods.
	// +optional
	NetworkPolicy NetworkPolicy `json:"networkPolicy,omitempty"`
	// Liveness config for the OpenTelemetry Collector except the probe handler which is auto generated from the health extension of the collector.
	// It is only effective when healthcheckextension is configured in the OpenTelemetry Collector pipeline.
	// +optional
	LivenessProbe *Probe `json:"livenessProbe,omitempty"`
	// Readiness config for the OpenTelemetry Collector except the probe handler which is auto generated from the health extension of the collector.
	// It is only effective when healthcheckextension is configured in the OpenTelemetry Collector pipeline.
	// +optional
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`
	// Startup config for the OpenTelemetry Collector except the probe handler which is auto generated from the health extension of the collector.
	// It is only effective when healthcheckextension is configured in the OpenTelemetry Collector pipeline.
	// +optional
	StartupProbe *Probe `json:"startupProbe,omitempty"`

	// ObservabilitySpec defines how telemetry data gets handled.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Observability"
	Observability ObservabilitySpec `json:"observability,omitempty"`

	// ConfigMaps is a list of ConfigMaps in the same namespace as the OpenTelemetryCollector
	// object, which shall be mounted into the Collector Pods.
	// Each ConfigMap will be added to the Collector's Deployments as a volume named `configmap-<configmap-name>`.
	ConfigMaps []ConfigMapsSpec `json:"configmaps,omitempty"`
	// UpdateStrategy represents the strategy the operator will take replacing existing DaemonSet pods with new pods
	// https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/daemon-set-v1/#DaemonSetSpec
	// This is only applicable to Daemonset mode.
	// +optional
	DaemonSetUpdateStrategy appsv1.DaemonSetUpdateStrategy `json:"daemonSetUpdateStrategy,omitempty"`
	// UpdateStrategy represents the strategy the operator will take replacing existing Deployment pods with new pods
	// https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/deployment-v1/#DeploymentSpec
	// This is only applicable to Deployment mode.
	// +optional
	DeploymentUpdateStrategy appsv1.DeploymentStrategy `json:"deploymentUpdateStrategy,omitempty"`
}

// TargetAllocatorEmbedded defines the configuration for the Prometheus target allocator, embedded in the
// OpenTelemetryCollector spec.
type TargetAllocatorEmbedded struct {
	// Replicas is the number of pod instances for the underlying TargetAllocator. This should only be set to a value
	// other than 1 if a strategy that allows for high availability is chosen. Currently, the only allocation strategy
	// that can be run in a high availability mode is consistent-hashing.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// NodeSelector to schedule OpenTelemetry TargetAllocator pods.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Resources to set on the OpenTelemetryTargetAllocator containers.
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// AllocationStrategy determines which strategy the target allocator should use for allocation.
	// The current options are least-weighted, consistent-hashing and per-node. The default is
	// consistent-hashing.
	// WARNING: The per-node strategy currently ignores targets without a Node, like control plane components.
	// +optional
	// +kubebuilder:default:=consistent-hashing
	AllocationStrategy TargetAllocatorAllocationStrategy `json:"allocationStrategy,omitempty"`
	// FilterStrategy determines how to filter targets before allocating them among the collectors.
	// The only current option is relabel-config (drops targets based on prom relabel_config).
	// The default is relabel-config.
	// +optional
	// +kubebuilder:default:=relabel-config
	FilterStrategy TargetAllocatorFilterStrategy `json:"filterStrategy,omitempty"`
	// ServiceAccount indicates the name of an existing service account to use with this instance. When set,
	// the operator will not automatically create a ServiceAccount for the TargetAllocator.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Image indicates the container image to use for the OpenTelemetry TargetAllocator.
	// +optional
	Image string `json:"image,omitempty"`
	// Enabled indicates whether to use a target allocation mechanism for Prometheus targets or not.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// If specified, indicates the pod's scheduling constraints
	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`
	// PrometheusCR defines the configuration for the retrieval of PrometheusOperator CRDs ( servicemonitor.monitoring.coreos.com/v1 and podmonitor.monitoring.coreos.com/v1 )  retrieval.
	// All CR instances which the ServiceAccount has access to will be retrieved. This includes other namespaces.
	// +optional
	PrometheusCR TargetAllocatorPrometheusCR `json:"prometheusCR,omitempty"`
	// SecurityContext configures the container security context for
	// the targetallocator.
	// +optional
	SecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`
	// PodSecurityContext configures the pod security context for the
	// targetallocator.
	// +optional
	PodSecurityContext *v1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// TopologySpreadConstraints embedded kubernetes pod configuration option,
	// controls how pods are spread across your cluster among failure-domains
	// such as regions, zones, nodes, and other user-defined topology domains
	// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/
	// +optional
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// Toleration embedded kubernetes pod configuration option,
	// controls how pods can be scheduled with matching taints
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// ENV vars to set on the OpenTelemetry TargetAllocator's Pods. These can then in certain cases be
	// consumed in the config file for the TargetAllocator.
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`
	// ObservabilitySpec defines how telemetry data gets handled.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Observability"
	Observability ObservabilitySpec `json:"observability,omitempty"`
	// PodDisruptionBudget specifies the pod disruption budget configuration to use
	// for the target allocator workload. By default, a PDB with a MaxUnavailable of one is set for a valid
	// allocation strategy.
	//
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
	// CollectorNotReadyGracePeriod defines the grace period after which a TargetAllocator stops considering a collector is target assignable.
	// The default is 30s, which means that if a collector becomes not Ready, the target allocator will wait for 30 seconds before reassigning its targets. The assumption is that the state is temporary, and an expensive target reallocation should be avoided if possible.
	//
	// +optional
	// +kubebuilder:default:="30s"
	// +kubebuilder:validation:Format:=duration
	CollectorNotReadyGracePeriod *metav1.Duration `json:"collectorNotReadyGracePeriod,omitempty"`
	// CollectorTargetReloadInterval defines the interval at which the Prometheus receiver will reload targets from the target allocator.
	// The default is 30s.
	//
	// +optional
	// +kubebuilder:default:="30s"
	// +kubebuilder:validation:Format:=duration
	CollectorTargetReloadInterval *metav1.Duration `json:"collectorTargetReloadInterval,omitempty"`
}

// Probe defines the OpenTelemetry's pod probe config.
type Probe struct {
	// Number of seconds after the container has started before liveness probes are initiated.
	// Defaults to 0 seconds. Minimum value is 0.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	// +kubebuilder:validation:Minimum=0
	InitialDelaySeconds *int32 `json:"initialDelaySeconds,omitempty"`
	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	// +optional
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds *int32 `json:"periodSeconds,omitempty"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
	// +optional
	// +kubebuilder:validation:Minimum=1
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +optional
	// +kubebuilder:validation:Minimum=1
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`
	// Optional duration in seconds the pod needs to terminate gracefully upon probe failure.
	// The grace period is the duration in seconds after the processes running in the pod are sent
	// a termination signal and the time when the processes are forcibly halted with a kill signal.
	// Set this value longer than the expected cleanup time for your process.
	// If this value is nil, the pod's terminationGracePeriodSeconds will be used. Otherwise, this
	// value overrides the value provided by the pod spec.
	// Value must be non-negative integer. The value zero indicates stop immediately via
	// the kill signal (no opportunity to shut down).
	// This is a beta field and requires enabling ProbeTerminationGracePeriod feature gate.
	// Minimum value is 1. spec.terminationGracePeriodSeconds is used if unset.
	// +optional
	// +kubebuilder:validation:Minimum=1
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

// ObservabilitySpec defines how telemetry data gets handled.
type ObservabilitySpec struct {
	// Metrics defines the metrics configuration for operands.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics Config"
	Metrics MetricsConfigSpec `json:"metrics,omitempty"`
}

// MetricsConfigSpec defines a metrics config.
type MetricsConfigSpec struct {
	// EnableMetrics specifies if ServiceMonitor or PodMonitor(for sidecar mode) should be created for the service managed by the OpenTelemetry Operator.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Create ServiceMonitors for OpenTelemetry Collector"
	EnableMetrics bool `json:"enableMetrics,omitempty"`
	// ExtraLabels are additional labels to be added to the ServiceMonitor
	//
	// +optional
	// +kubebuilder:validation:Optional
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`

	// DisablePrometheusAnnotations controls the automatic addition of default Prometheus annotations
	// ('prometheus.io/scrape', 'prometheus.io/port', and 'prometheus.io/path')
	//
	// +optional
	// +kubebuilder:validation:Optional
	DisablePrometheusAnnotations bool `json:"disablePrometheusAnnotations,omitempty"`
}

// ScaleSubresourceStatus defines the observed state of the OpenTelemetryCollector's
// scale subresource.
type ScaleSubresourceStatus struct {
	// The selector used to match the OpenTelemetryCollector's
	// deployment or statefulSet pods.
	// +optional
	Selector string `json:"selector,omitempty"`

	// The total number non-terminated pods targeted by this
	// OpenTelemetryCollector's deployment or statefulSet.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// StatusReplicas is the number of pods targeted by this OpenTelemetryCollector's with a Ready Condition /
	// Total number of non-terminated pods targeted by this OpenTelemetryCollector's (their labels match the selector).
	// Deployment, Daemonset, StatefulSet.
	// +optional
	StatusReplicas string `json:"statusReplicas,omitempty"`
}

type ConfigMapsSpec struct {
	// Configmap defines name and path where the configMaps should be mounted.
	Name      string `json:"name"`
	MountPath string `json:"mountpath"`
}
