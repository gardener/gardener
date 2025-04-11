// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperatorConfiguration defines the configuration for the Gardener operator.
type OperatorConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// RuntimeClientConnection specifies the kubeconfig file and the client connection settings for the proxy server to
	// use when communicating with the kube-apiserver of the runtime cluster.
	RuntimeClientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:"runtimeClientConnection"`
	// VirtualClientConnection specifies the kubeconfig file and the client connection settings for the proxy server to
	// use when communicating with the kube-apiserver of the virtual cluster.
	VirtualClientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:"virtualClientConnection"`
	// LeaderElection defines the configuration of leader election client.
	LeaderElection componentbaseconfigv1alpha1.LeaderElectionConfiguration `json:"leaderElection"`
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string `json:"logLevel"`
	// LogFormat is the output format for the logs. Must be one of [text,json].
	LogFormat string `json:"logFormat"`
	// Server defines the configuration of the HTTP server.
	Server ServerConfiguration `json:"server"`
	// Debugging holds configuration for Debugging related features.
	// +optional
	Debugging *componentbaseconfigv1alpha1.DebuggingConfiguration `json:"debugging,omitempty"`
	// FeatureGates is a map of feature names to bools that enable or disable alpha/experimental features. This field
	// modifies piecemeal the built-in default values from "github.com/gardener/gardener/pkg/operator/features/features.go".
	// Default: nil
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
	// Controllers defines the configuration of the controllers.
	Controllers ControllerConfiguration `json:"controllers"`
	// NodeToleration contains optional settings for default tolerations.
	// +optional
	NodeToleration *NodeTolerationConfiguration `json:"nodeToleration,omitempty"`
}

// ConditionThreshold defines the threshold of the given condition type.
type ConditionThreshold struct {
	// Type is the type of the condition to define the threshold for.
	Type string `json:"type"`
	// Duration is the duration how long the condition can stay in the progressing state.
	Duration metav1.Duration `json:"duration"`
}

// ControllerConfiguration defines the configuration of the controllers.
type ControllerConfiguration struct {
	// Garden is the configuration for the garden controller.
	Garden GardenControllerConfig `json:"garden"`
	// GardenCare is the configuration for the garden care controller
	GardenCare GardenCareControllerConfiguration `json:"gardenCare"`
	// GardenletDeployer is the configuration for the gardenlet deployer controller.
	GardenletDeployer GardenletDeployerControllerConfig `json:"gardenletDeployer"`
	// NetworkPolicy is the configuration for the NetworkPolicy controller.
	NetworkPolicy NetworkPolicyControllerConfiguration `json:"networkPolicy"`
	// VPAEvictionRequirements is the configuration for the VPAEvictionrequirements controller.
	VPAEvictionRequirements VPAEvictionRequirementsControllerConfiguration `json:"vpaEvictionRequirements"`
	// Extension defines the configuration of the extension controller.
	Extension ExtensionControllerConfiguration `json:"extension"`
	// ExtensionCare is the configuration for the extension care controller
	ExtensionCare ExtensionCareControllerConfiguration `json:"extensionCare"`
	// ExtensionRequiredRuntime defines the configuration of the ExtensionRequiredRuntime controller.
	ExtensionRequiredRuntime ExtensionRequiredRuntimeControllerConfiguration `json:"extensionRequiredRuntime"`
	// ExtensionRequiredVirtual defines the configuration of the ExtensionRequiredVirtual controller.
	ExtensionRequiredVirtual ExtensionRequiredVirtualControllerConfiguration `json:"extensionRequiredVirtual"`
}

// GardenCareControllerConfiguration defines the configuration of the GardenCare controller.
type GardenCareControllerConfiguration struct {
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check is performed).
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// ConditionThresholds defines the condition threshold per condition type.
	// +optional
	ConditionThresholds []ConditionThreshold `json:"conditionThresholds,omitempty"`
}

// GardenControllerConfig is the configuration for the garden controller.
type GardenControllerConfig struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the controller performs its reconciliation.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// ETCDConfig contains an optional configuration for the
	// backup compaction feature of ETCD backup-restore functionality.
	// +optional
	ETCDConfig *gardenletconfigv1alpha1.ETCDConfig `json:"etcdConfig,omitempty"`
}

// GardenletDeployerControllerConfig is the configuration for the gardenlet deployer controller.
type GardenletDeployerControllerConfig struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// NetworkPolicyControllerConfiguration defines the configuration of the NetworkPolicy controller.
type NetworkPolicyControllerConfiguration struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// AdditionalNamespaceSelectors is a list of label selectors for additional namespaces that should be considered by
	// the controller.
	// +optional
	AdditionalNamespaceSelectors []metav1.LabelSelector `json:"additionalNamespaceSelectors,omitempty"`
}

// VPAEvictionRequirementsControllerConfiguration defines the configuration of the VPAEvictionRequirements controller.
type VPAEvictionRequirementsControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ExtensionControllerConfiguration defines the configuration of the extension controller.
type ExtensionControllerConfiguration struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ExtensionCareControllerConfiguration defines the configuration of the ExtensionCare controller.
type ExtensionCareControllerConfiguration struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check is performed).
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// ConditionThresholds defines the condition threshold per condition type.
	// +optional
	ConditionThresholds []ConditionThreshold `json:"conditionThresholds,omitempty"`
}

// ExtensionRequiredRuntimeControllerConfiguration defines the configuration of the extension-required-runtime controller.
type ExtensionRequiredRuntimeControllerConfiguration struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ExtensionRequiredVirtualControllerConfiguration defines the configuration of the extension-required-virtual controller.
type ExtensionRequiredVirtualControllerConfiguration struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
	// Webhooks is the configuration for the HTTPS webhook server.
	Webhooks Server `json:"webhooks"`
	// HealthProbes is the configuration for serving the healthz and readyz endpoints.
	// +optional
	HealthProbes *Server `json:"healthProbes,omitempty"`
	// Metrics is the configuration for serving the metrics endpoint.
	// +optional
	Metrics *Server `json:"metrics,omitempty"`
}

// Server contains information for HTTP(S) server configuration.
type Server struct {
	// BindAddress is the IP address on which to listen for the specified port.
	BindAddress string `json:"bindAddress"`
	// Port is the port on which to serve requests.
	Port int `json:"port"`
}

// NodeTolerationConfiguration contains information about node toleration options.
type NodeTolerationConfiguration struct {
	// DefaultNotReadyTolerationSeconds specifies the seconds for the `node.kubernetes.io/not-ready` toleration that
	// should be added to pods not already tolerating this taint.
	// +optional
	DefaultNotReadyTolerationSeconds *int64 `json:"defaultNotReadyTolerationSeconds,omitempty"`
	// DefaultUnreachableTolerationSeconds specifies the seconds for the `node.kubernetes.io/unreachable` toleration that
	// should be added to pods not already tolerating this taint.
	// +optional
	DefaultUnreachableTolerationSeconds *int64 `json:"defaultUnreachableTolerationSeconds,omitempty"`
}

const (
	// DefaultLockObjectNamespace is the default lock namespace for leader election.
	DefaultLockObjectNamespace = "garden"
	// DefaultLockObjectName is the default lock name for leader election.
	DefaultLockObjectName = "gardener-operator-leader-election"
)
