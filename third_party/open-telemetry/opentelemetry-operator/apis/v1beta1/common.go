// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1beta1/common.go.

package v1beta1

import (
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

// MetricSpec defines a subset of metrics to be defined for the HPA's metric array
// more metric type can be supported as needed.
// See https://pkg.go.dev/k8s.io/api/autoscaling/v2#MetricSpec for reference.
type MetricSpec struct {
	Type autoscalingv2.MetricSourceType  `json:"type"`
	Pods *autoscalingv2.PodsMetricSource `json:"pods,omitempty"`
}

// AutoscalerSpec defines the OpenTelemetryCollector's pod autoscaling specification.
type AutoscalerSpec struct {
	// MinReplicas sets a lower bound to the autoscaling feature.  Set this if you are using autoscaling. It must be at least 1
	// +optional
	// +kubebuilder:validation:Minimum=1
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// MaxReplicas sets an upper bound to the autoscaling feature. If MaxReplicas is set autoscaling is enabled.
	// +optional
	// +kubebuilder:validation:Minimum=1
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
	// +kubebuilder:validation:Minimum=1
	TargetCPUUtilization *int32 `json:"targetCPUUtilization,omitempty"`
	// +optional
	// TargetMemoryUtilization sets the target average memory utilization across all replicas
	// +kubebuilder:validation:Minimum=1
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

// PortsSpec defines the OpenTelemetryCollector's container/service ports additional specifications.
type PortsSpec struct {
	// Allows defining which port to bind to the host in the Container.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	HostPort int32 `json:"hostPort,omitempty"`

	// Maintain previous fields in new struct
	v1.ServicePort `json:",inline"`
}

type OpenTelemetryCommonFields struct {
	// ManagementState defines if the CR should be managed by the operator or not.
	// Default is managed.
	//
	// +kubebuilder:default:=managed
	ManagementState ManagementStateType `json:"managementState,omitempty"`
	// Resources to set on generated pods.
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// NodeSelector to schedule generated pods.
	// This only works with the following OpenTelemetryCollector mode's: daemonset, statefulset, and deployment.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Args is the set of arguments to pass to the main container's binary.
	// +optional
	Args map[string]string `json:"args,omitempty"`
	// Replicas is the number of pod instances for the underlying replicaset. Set this if you are not using autoscaling.
	// +optional
	// +kubebuilder:default:=1
	Replicas *int32 `json:"replicas,omitempty"`
	// PodDisruptionBudget specifies the pod disruption budget configuration to use
	// for the generated workload. By default, a PDB with a MaxUnavailable of one is set.
	//
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
	// SecurityContext configures the container security context for
	// the generated main container.
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
	// generated pod, when running as a deployment, daemonset,
	// or statefulset.
	//
	// In sidecar mode, the opentelemetry-operator will ignore this setting.
	//
	// +optional
	PodSecurityContext *v1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// PodAnnotations is the set of annotations that will be attached to
	// the generated pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// ServiceAccount indicates the name of an existing service account to use with this instance. When set,
	// the operator will not automatically create a ServiceAccount.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Image indicates the container image to use for the generated pods.
	// +optional
	Image string `json:"image,omitempty"`
	// ImagePullPolicy indicates the pull policy to be used for retrieving the container image.
	// +optional
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// VolumeMounts represents the mount points to use in the underlying deployment(s).
	// +optional
	// +listType=atomic
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// Ports allows a set of ports to be exposed by the underlying v1.Service & v1.ContainerPort. By default, the operator
	// will attempt to infer the required ports by parsing the .Spec.Config property but this property can be
	// used to open additional ports that can't be inferred by the operator, like for custom receivers.
	// +optional
	// +listType=atomic
	Ports []PortsSpec `json:"ports,omitempty"`
	// Environment variables to set on the generated pods.
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`
	// List of sources to populate environment variables on the generated pods.
	// +optional
	EnvFrom []v1.EnvFromSource `json:"envFrom,omitempty"`
	// Toleration to schedule the generated pods.
	// This only works with the following OpenTelemetryCollector mode's: daemonset, statefulset, and deployment.
	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// Volumes represents which volumes to use in the underlying deployment(s).
	// +optional
	// +listType=atomic
	Volumes []v1.Volume `json:"volumes,omitempty"`
	// If specified, indicates the pod's scheduling constraints
	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`
	// Actions that the management system should take in response to container lifecycle events. Cannot be updated.
	// +optional
	Lifecycle *v1.Lifecycle `json:"lifecycle,omitempty"`
	// Duration in seconds the pod needs to terminate gracefully upon probe failure.
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
	// TopologySpreadConstraints embedded kubernetes pod configuration option,
	// controls how pods are spread across your cluster among failure-domains
	// such as regions, zones, nodes, and other user-defined topology domains
	// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/
	// This only works with the following OpenTelemetryCollector mode's: statefulset, and deployment.
	// +optional
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// HostNetwork indicates if the pod should run in the host networking namespace.
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`
	// DNSPolicy defines how a pod's DNS will be configured.
	// +optional
	DNSPolicy *v1.DNSPolicy `json:"dnsPolicy,omitempty"`
	// HostPID indicates if the pod should have access to the host process ID namespace.
	// +optional
	HostPID bool `json:"hostPID,omitempty"`
	// ShareProcessNamespace indicates if the pod's containers should share process namespace.
	// +optional
	ShareProcessNamespace bool `json:"shareProcessNamespace,omitempty"`
	// If specified, indicates the pod's priority.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// InitContainers allows injecting initContainers to the generated pod definition.
	// These init containers can be used to fetch secrets for injection into the
	// configuration from external sources, run added checks, etc. Any errors during the execution of
	// an initContainer will lead to a restart of the Pod. More info:
	// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
	// +optional
	InitContainers []v1.Container `json:"initContainers,omitempty"`
	// AdditionalContainers allows injecting additional containers into the generated pod definition.
	// These sidecar containers can be used for authentication proxies, log shipping sidecars, agents for shipping
	// metrics to their cloud, or in general sidecars that do not support automatic injection.
	// This only works with the following OpenTelemetryCollector mode's: daemonset, statefulset, and deployment.
	//
	// Container names managed by the operator:
	// * `otc-container`
	//
	// Overriding containers managed by the operator is outside the scope of what the maintainers will support and by
	// doing so, you wil accept the risk of it breaking things.
	//
	// +optional
	AdditionalContainers []v1.Container `json:"additionalContainers,omitempty"`
	// PodDNSConfig defines the DNS parameters of a pod in addition to those generated from DNSPolicy.
	PodDNSConfig v1.PodDNSConfig `json:"podDnsConfig,omitempty"`
	// IPFamily represents the IP Family (IPv4 or IPv6). This type is used
	// to express the family of an IP expressed by a type (e.g. service.spec.ipFamilies).
	// +optional
	IpFamilies []v1.IPFamily `json:"ipFamilies,omitempty"`
	// IPFamilyPolicy represents the dual-stack-ness requested or required by a Service
	// +kubebuilder:default:=SingleStack
	// +optional
	IpFamilyPolicy *v1.IPFamilyPolicy `json:"ipFamilyPolicy,omitempty"`
	// TrafficDistribution specifies how traffic to this service is routed.
	// https://kubernetes.io/docs/concepts/services-networking/service/#traffic-distribution
	// This is only applicable to Service resources.
	// +optional
	TrafficDistribution *string `json:"trafficDistribution,omitempty"`
}

type StatefulSetCommonFields struct {
	// VolumeClaimTemplates will provide stable storage using PersistentVolumes.
	// This only works with the following OpenTelemetryCollector mode's: statefulset.
	// +optional
	// +listType=atomic
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	// PersistentVolumeClaimRetentionPolicy describes the lifecycle of persistent volume claims
	// created from volumeClaimTemplates.
	// This only works with the following OpenTelemetryCollector modes: statefulset.
	// +optional
	PersistentVolumeClaimRetentionPolicy *appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy `json:"persistentVolumeClaimRetentionPolicy,omitempty"`

	// ServiceName sets the serviceName of the StatefulSet.
	// If not specified, it will default to "<name>-headless".
	// Note that the custom service name is not created by the operator.
	// +optional
	ServiceName string `json:"serviceName,omitempty"`
}
