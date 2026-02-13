// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1alpha1/opentelemetrycollector_types.go.
// The import of github.com/open-telemetry/opentelemetry-operator was adjusted to github.com/gardener/gardener/third_party/open-telemetry/opentelemetry-operator.

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/gardener/gardener/third_party/open-telemetry/opentelemetry-operator/apis/v1beta1"
)

// ManagementStateType defines the type for CR management states.
//
// +kubebuilder:validation:Enum=managed;unmanaged
type ManagementStateType string

const (
	// ManagementStateManaged when the OpenTelemetryCollector custom resource should be
	// reconciled by the operator.
	ManagementStateManaged ManagementStateType = "managed"

	// ManagementStateUnmanaged when the OpenTelemetryCollector custom resource should not be
	// reconciled by the operator.
	ManagementStateUnmanaged ManagementStateType = "unmanaged"
)

// Ingress is used to specify how OpenTelemetry Collector is exposed. This
// functionality is only available if one of the valid modes is set.
// Valid modes are: deployment, daemonset and statefulset.
// NOTE: If this feature is activated, all specified receivers are exposed.
// Currently this has a few limitations. Depending on the ingress controller
// there are problems with TLS and gRPC.
// SEE: https://github.com/open-telemetry/opentelemetry-operator/issues/1306.
// NOTE: As a workaround, port name and appProtocol could be specified directly
// in the CR.
// SEE: OpenTelemetryCollector.spec.ports[index].
type Ingress struct {
	// Type default value is: ""
	// Supported types are: ingress, route
	Type IngressType `json:"type,omitempty"`

	// RuleType defines how Ingress exposes collector receivers.
	// IngressRuleTypePath ("path") exposes each receiver port on a unique path on single domain defined in Hostname.
	// IngressRuleTypeSubdomain ("subdomain") exposes each receiver port on a unique subdomain of Hostname.
	// Default is IngressRuleTypePath ("path").
	RuleType IngressRuleType `json:"ruleType,omitempty"`

	// Hostname by which the ingress proxy can be reached.
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// Annotations to add to ingress.
	// e.g. 'cert-manager.io/cluster-issuer: "letsencrypt"'
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// TLS configuration.
	// +optional
	TLS []networkingv1.IngressTLS `json:"tls,omitempty"`

	// IngressClassName is the name of an IngressClass cluster resource. Ingress
	// controller implementations use this field to know whether they should be
	// serving this Ingress resource.
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Route is an OpenShift specific section that is only considered when
	// type "route" is used.
	// +optional
	Route OpenShiftRoute `json:"route,omitempty"`
}

// OpenShiftRoute defines openshift route specific settings.
type OpenShiftRoute struct {
	// Termination indicates termination type. By default "edge" is used.
	Termination TLSRouteTerminationType `json:"termination,omitempty"`
}

// OpenTelemetryCollectorSpec defines the desired state of OpenTelemetryCollector.
type OpenTelemetryCollectorSpec struct {
	// ManagementState defines if the CR should be managed by the operator or not.
	// Default is managed.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:default:=managed
	ManagementState ManagementStateType `json:"managementState,omitempty"`
	// Resources to set on the OpenTelemetry Collector pods.
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// NodeSelector to schedule OpenTelemetry Collector pods.
	// This is only relevant to daemonset, statefulset, and deployment mode
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Args is the set of arguments to pass to the OpenTelemetry Collector binary
	// +optional
	Args map[string]string `json:"args,omitempty"`
	// Replicas is the number of pod instances for the underlying OpenTelemetry Collector. Set this if you are not using autoscaling
	// +optional
	// +kubebuilder:default:=1
	Replicas *int32 `json:"replicas,omitempty"`
	// MinReplicas sets a lower bound to the autoscaling feature.  Set this if you are using autoscaling. It must be at least 1
	// +optional
	// Deprecated: use "OpenTelemetryCollector.Spec.Autoscaler.MinReplicas" instead.
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// MaxReplicas sets an upper bound to the autoscaling feature. If MaxReplicas is set autoscaling is enabled.
	// +optional
	// Deprecated: use "OpenTelemetryCollector.Spec.Autoscaler.MaxReplicas" instead.
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
	// Autoscaler specifies the pod autoscaling configuration to use
	// for the OpenTelemetryCollector workload.
	//
	// +optional
	Autoscaler *AutoscalerSpec `json:"autoscaler,omitempty"`
	// PodDisruptionBudget specifies the pod disruption budget configuration to use
	// for the OpenTelemetryCollector workload.
	//
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
	// SecurityContext configures the container security context for
	// the opentelemetry-collector container.
	//
	// In deployment, daemonset, or statefulset mode, this controls
	// the security context settings for the primary application
	// container.
	//
	// In sidecar mode, this controls the security context for the
	// injected sidecar container.
	//
	// +optional
	SecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`
	// PodSecurityContext configures the pod security context for the
	// opentelemetry-collector pod, when running as a deployment, daemonset,
	// or statefulset.
	//
	// In sidecar mode, the opentelemetry-operator will ignore this setting.
	//
	// +optional
	PodSecurityContext *v1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// PodAnnotations is the set of annotations that will be attached to
	// Collector and Target Allocator pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// TargetAllocator indicates a value which determines whether to spawn a target allocation resource or not.
	// +optional
	TargetAllocator OpenTelemetryTargetAllocator `json:"targetAllocator,omitempty"`
	// Mode represents how the collector should be deployed (deployment, daemonset, statefulset or sidecar)
	// +optional
	Mode Mode `json:"mode,omitempty"`
	// ServiceAccount indicates the name of an existing service account to use with this instance. When set,
	// the operator will not automatically create a ServiceAccount for the collector.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Image indicates the container image to use for the OpenTelemetry Collector.
	// +optional
	Image string `json:"image,omitempty"`
	// UpgradeStrategy represents how the operator will handle upgrades to the CR when a newer version of the operator is deployed
	// +optional
	UpgradeStrategy UpgradeStrategy `json:"upgradeStrategy"`

	// ImagePullPolicy indicates the pull policy to be used for retrieving the container image (Always, Never, IfNotPresent)
	// +optional
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Config is the raw YAML to be used as the collector's configuration. Refer to the OpenTelemetry Collector documentation for details.
	// +required
	Config string `json:"config,omitempty"`
	// VolumeMounts represents the mount points to use in the underlying collector deployment(s)
	// +optional
	// +listType=atomic
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// Ports allows a set of ports to be exposed by the underlying v1.Service. By default, the operator
	// will attempt to infer the required ports by parsing the .Spec.Config property but this property can be
	// used to open additional ports that can't be inferred by the operator, like for custom receivers.
	// +optional
	// +listType=atomic
	Ports []PortsSpec `json:"ports,omitempty"`
	// ENV vars to set on the OpenTelemetry Collector's Pods. These can then in certain cases be
	// consumed in the config file for the Collector.
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`
	// List of sources to populate environment variables on the OpenTelemetry Collector's Pods.
	// These can then in certain cases be consumed in the config file for the Collector.
	// +optional
	EnvFrom []v1.EnvFromSource `json:"envFrom,omitempty"`
	// VolumeClaimTemplates will provide stable storage using PersistentVolumes. Only available when the mode=statefulset.
	// +optional
	// +listType=atomic
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	// Toleration to schedule OpenTelemetry Collector pods.
	// This is only relevant to daemonset, statefulset, and deployment mode
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// Volumes represents which volumes to use in the underlying collector deployment(s).
	// +optional
	// +listType=atomic
	Volumes []v1.Volume `json:"volumes,omitempty"`
	// Ingress is used to specify how OpenTelemetry Collector is exposed. This
	// functionality is only available if one of the valid modes is set.
	// Valid modes are: deployment, daemonset and statefulset.
	// +optional
	Ingress Ingress `json:"ingress,omitempty"`
	// HostNetwork indicates if the pod should run in the host networking namespace.
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`
	// ShareProcessNamespace indicates if the pod's containers should share process namespace.
	// +optional
	ShareProcessNamespace bool `json:"shareProcessNamespace,omitempty"`
	// If specified, indicates the pod's priority.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// If specified, indicates the pod's scheduling constraints
	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`
	// Actions that the management system should take in response to container lifecycle events. Cannot be updated.
	// +optional
	Lifecycle *v1.Lifecycle `json:"lifecycle,omitempty"`
	// Duration in seconds the pod needs to terminate gracefully upon probe failure.
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
	// Liveness config for the OpenTelemetry Collector except the probe handler which is auto generated from the health extension of the collector.
	// It is only effective when healthcheckextension is configured in the OpenTelemetry Collector pipeline.
	// +optional
	LivenessProbe *Probe `json:"livenessProbe,omitempty"`
	// InitContainers allows injecting initContainers to the Collector's pod definition.
	// These init containers can be used to fetch secrets for injection into the
	// configuration from external sources, run added checks, etc. Any errors during the execution of
	// an initContainer will lead to a restart of the Pod. More info:
	// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
	// +optional
	InitContainers []v1.Container `json:"initContainers,omitempty"`

	// ServiceName is the name of the Service to be used.
	// If not specified, it will default to "<name>-headless".
	// +optional
	ServiceName string `json:"serviceName,omitempty"`
	// TrafficDistribution specifies how traffic to this service is routed.
	// https://kubernetes.io/docs/concepts/services-networking/service/#traffic-distribution
	// This is only applicable to Service resources.
	// +optional
	TrafficDistribution *string `json:"trafficDistribution,omitempty"`

	// AdditionalContainers allows injecting additional containers into the Collector's pod definition.
	// These sidecar containers can be used for authentication proxies, log shipping sidecars, agents for shipping
	// metrics to their cloud, or in general sidecars that do not support automatic injection. This option only
	// applies to Deployment, DaemonSet, and StatefulSet deployment modes of the collector. It does not apply to the sidecar
	// deployment mode. More info about sidecars:
	// https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/
	//
	// Container names managed by the operator:
	// * `otc-container`
	//
	// Overriding containers managed by the operator is outside the scope of what the maintainers will support and by
	// doing so, you wil accept the risk of it breaking things.
	//
	// +optional
	AdditionalContainers []v1.Container `json:"additionalContainers,omitempty"`

	// ObservabilitySpec defines how telemetry data gets handled.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Observability"
	Observability ObservabilitySpec `json:"observability,omitempty"`

	// TopologySpreadConstraints embedded kubernetes pod configuration option,
	// controls how pods are spread across your cluster among failure-domains
	// such as regions, zones, nodes, and other user-defined topology domains
	// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/
	// This is only relevant to statefulset, and deployment mode
	// +optional
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// ConfigMaps is a list of ConfigMaps in the same namespace as the OpenTelemetryCollector
	// object, which shall be mounted into the Collector Pods.
	// Each ConfigMap will be added to the Collector's Deployments as a volume named `configmap-<configmap-name>`.
	ConfigMaps []ConfigMapsSpec `json:"configmaps,omitempty"`
	// UpdateStrategy represents the strategy the operator will take replacing existing DaemonSet pods with new pods
	// https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/daemon-set-v1/#DaemonSetSpec
	// This is only applicable to Daemonset mode.
	// +optional
	UpdateStrategy appsv1.DaemonSetUpdateStrategy `json:"updateStrategy,omitempty"`
	// UpdateStrategy represents the strategy the operator will take replacing existing Deployment pods with new pods
	// https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/deployment-v1/#DeploymentSpec
	// This is only applicable to Deployment mode.
	// +optional
	DeploymentUpdateStrategy appsv1.DeploymentStrategy `json:"deploymentUpdateStrategy,omitempty"`
}

// PortsSpec defines the OpenTelemetryCollector's container/service ports additional specifications.
type PortsSpec struct {
	// Allows defining which port to bind to the host in the Container.
	// +optional
	HostPort int32 `json:"hostPort,omitempty"`

	// Maintain previous fields in new struct
	v1.ServicePort `json:",inline"`
}

// OpenTelemetryTargetAllocator defines the configurations for the Prometheus target allocator.
type OpenTelemetryTargetAllocator struct {
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
	AllocationStrategy OpenTelemetryTargetAllocatorAllocationStrategy `json:"allocationStrategy,omitempty"`
	// FilterStrategy determines how to filter targets before allocating them among the collectors.
	// The only current option is relabel-config (drops targets based on prom relabel_config).
	// The default is relabel-config.
	// +optional
	// +kubebuilder:default:=relabel-config
	FilterStrategy string `json:"filterStrategy,omitempty"`
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
	PrometheusCR OpenTelemetryTargetAllocatorPrometheusCR `json:"prometheusCR,omitempty"`
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
	// for the target allocator workload.
	//
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
}

type OpenTelemetryTargetAllocatorPrometheusCR struct {
	// Enabled indicates whether to use a PrometheusOperator custom resources as targets or not.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Interval between consecutive scrapes. Equivalent to the same setting on the Prometheus CRD.
	//
	// Default: "30s"
	// +kubebuilder:default:="30s"
	// +kubebuilder:validation:Format:=duration
	ScrapeInterval *metav1.Duration `json:"scrapeInterval,omitempty"`
	// ScrapeClasses to be referenced by PodMonitors and ServiceMonitors to include common configuration.
	// If specified, expects an array of ScrapeClass objects as specified by https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.ScrapeClass.
	// +optional
	// +listType=atomic
	// +kubebuilder:pruning:PreserveUnknownFields
	ScrapeClasses []v1beta1.AnyConfig `json:"scrapeClasses,omitempty"`
	// PodMonitors to be selected for target discovery.
	// This is a map of {key,value} pairs. Each {key,value} in the map is going to exactly match a label in a
	// PodMonitor's meta labels. The requirements are ANDed.
	// Empty or nil map matches all pod monitors.
	// +optional
	PodMonitorSelector map[string]string `json:"podMonitorSelector,omitempty"`
	// ServiceMonitors to be selected for target discovery.
	// This is a map of {key,value} pairs. Each {key,value} in the map is going to exactly match a label in a
	// ServiceMonitor's meta labels. The requirements are ANDed.
	// Empty or nil map matches all service monitors.
	// +optional
	ServiceMonitorSelector map[string]string `json:"serviceMonitorSelector,omitempty"`
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

	// Messages about actions performed by the operator on this resource.
	// +optional
	// +listType=atomic
	// Deprecated: use Kubernetes events instead.
	Messages []string `json:"messages,omitempty"`

	// Replicas is currently not being set and might be removed in the next version.
	// +optional
	// Deprecated: use "OpenTelemetryCollector.Status.Scale.Replicas" instead.
	Replicas int32 `json:"replicas,omitempty"`
}

// +kubebuilder:deprecatedversion:warning="OpenTelemetryCollector v1alpha1 is deprecated. Migrate to v1beta1."
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=otelcol;otelcols
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

// +kubebuilder:object:root=true

// OpenTelemetryCollectorList contains a list of OpenTelemetryCollector.
type OpenTelemetryCollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenTelemetryCollector `json:"items"`
}

// AutoscalerSpec defines the OpenTelemetryCollector's pod autoscaling specification.
type AutoscalerSpec struct {
	// MinReplicas sets a lower bound to the autoscaling feature.  Set this if you are using autoscaling. It must be at least 1
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// MaxReplicas sets an upper bound to the autoscaling feature. If MaxReplicas is set autoscaling is enabled.
	// +optional
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
	// +optional
	Behavior *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
	// Metrics is meant to provide a customizable way to configure HPA metrics.
	// currently the only supported custom metrics is type=Pod.
	// Use TargetCPUUtilization or TargetMemoryUtilization instead if scaling on these common resource metrics.
	// +optional
	Metrics []MetricSpec `json:"metrics,omitempty"`
	// TargetCPUUtilization sets the target average CPU used across all replicas.
	// If average CPU exceeds this value, the HPA will scale up. Defaults to 90 percent.
	// +optional
	TargetCPUUtilization *int32 `json:"targetCPUUtilization,omitempty"`
	// +optional
	// TargetMemoryUtilization sets the target average memory utilization across all replicas
	TargetMemoryUtilization *int32 `json:"targetMemoryUtilization,omitempty"`
}

// PodDisruptionBudgetSpec defines the OpenTelemetryCollector's pod disruption budget specification.
type PodDisruptionBudgetSpec struct {
	// An eviction is allowed if at least "minAvailable" pods selected by
	// "selector" will still be available after the eviction, i.e. even in the
	// absence of the evicted pod.  So for example you can prevent all voluntary
	// evictions by specifying "100%".
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`

	// An eviction is allowed if at most "maxUnavailable" pods selected by
	// "selector" are unavailable after the eviction, i.e. even in absence of
	// the evicted pod. For example, one can prevent all voluntary evictions
	// by specifying 0. This is a mutually exclusive setting with "minAvailable".
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// MetricsConfigSpec defines a metrics config.
type MetricsConfigSpec struct {
	// EnableMetrics specifies if ServiceMonitor or PodMonitor(for sidecar mode) should be created for the service managed by the OpenTelemetry Operator.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Create ServiceMonitors for OpenTelemetry Collector"
	EnableMetrics bool `json:"enableMetrics,omitempty"`
	// DisablePrometheusAnnotations controls the automatic addition of default Prometheus annotations
	// ('prometheus.io/scrape', 'prometheus.io/port', and 'prometheus.io/path')
	//
	// +optional
	// +kubebuilder:validation:Optional
	DisablePrometheusAnnotations bool `json:"DisablePrometheusAnnotations,omitempty"`
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

// Probe defines the OpenTelemetry's pod probe config. Only Liveness probe is supported currently.
type Probe struct {
	// Number of seconds after the container has started before liveness probes are initiated.
	// Defaults to 0 seconds. Minimum value is 0.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	InitialDelaySeconds *int32 `json:"initialDelaySeconds,omitempty"`
	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	// +optional
	PeriodSeconds *int32 `json:"periodSeconds,omitempty"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
	// +optional
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +optional
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
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

// MetricSpec defines a subset of metrics to be defined for the HPA's metric array
// more metric type can be supported as needed.
// See https://pkg.go.dev/k8s.io/api/autoscaling/v2#MetricSpec for reference.
type MetricSpec struct {
	Type autoscalingv2.MetricSourceType  `json:"type"`
	Pods *autoscalingv2.PodsMetricSource `json:"pods,omitempty"`
}

type ConfigMapsSpec struct {
	// Configmap defines name and path where the configMaps should be mounted.
	Name      string `json:"name"`
	MountPath string `json:"mountpath"`
}

func init() {
	SchemeBuilder.Register(&OpenTelemetryCollector{}, &OpenTelemetryCollectorList{})
}
