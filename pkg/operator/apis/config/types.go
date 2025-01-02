// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperatorConfiguration defines the configuration for the Gardener operator.
type OperatorConfiguration struct {
	metav1.TypeMeta
	// RuntimeClientConnection specifies the kubeconfig file and the client connection settings for the proxy server to
	// use when communicating with the kube-apiserver of the runtime cluster.
	RuntimeClientConnection componentbaseconfig.ClientConnectionConfiguration
	// VirtualClientConnection specifies the kubeconfig file and the client connection settings for the proxy server to
	// use when communicating with the kube-apiserver of the virtual cluster.
	VirtualClientConnection componentbaseconfig.ClientConnectionConfiguration
	// LeaderElection defines the configuration of leader election client.
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string
	// LogFormat is the output format for the logs. Must be one of [text,json].
	LogFormat string
	// Server defines the configuration of the HTTP server.
	Server ServerConfiguration
	// Debugging holds configuration for Debugging related features.
	Debugging *componentbaseconfig.DebuggingConfiguration
	// FeatureGates is a map of feature names to bools that enable or disable alpha/experimental features. This field
	// modifies piecemeal the built-in default values from "github.com/gardener/gardener/pkg/operator/features/features.go".
	// Default: nil
	FeatureGates map[string]bool
	// Controllers defines the configuration of the controllers.
	Controllers ControllerConfiguration
	// NodeToleration contains optional settings for default tolerations.
	NodeToleration *NodeTolerationConfiguration
}

// ConditionThreshold defines the threshold of the given condition type.
type ConditionThreshold struct {
	// Type is the type of the condition to define the threshold for.
	Type string
	// Duration is the duration how long the condition can stay in the progressing state.
	Duration metav1.Duration
}

// ControllerConfiguration defines the configuration of the controllers.
type ControllerConfiguration struct {
	// Garden is the configuration for the garden controller.
	Garden GardenControllerConfig
	// GardenCare is the configuration for the garden care controller
	GardenCare GardenCareControllerConfiguration
	// GardenletDeployer is the configuration for the gardenlet deployer controller.
	GardenletDeployer GardenletDeployerControllerConfig
	// NetworkPolicy is the configuration for the NetworkPolicy controller.
	NetworkPolicy NetworkPolicyControllerConfiguration
	// VPAEvictionRequirements is the configuration for the VPAEvictionrequirements controller.
	VPAEvictionRequirements VPAEvictionRequirementsControllerConfiguration
	// Extension defines the configuration of the extension controller.
	Extension ExtensionControllerConfiguration
	// ExtensionRequiredRuntime defines the configuration of the ExtensionRequiredRuntime controller.
	ExtensionRequiredRuntime ExtensionRequiredRuntimeControllerConfiguration
	// ExtensionRequiredVirtual defines the configuration of the ExtensionRequiredVirtual controller.
	ExtensionRequiredVirtual ExtensionRequiredVirtualControllerConfiguration
}

// GardenCareControllerConfiguration defines the configuration of the GardenCare controller.
type GardenCareControllerConfiguration struct {
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check is performed).
	SyncPeriod *metav1.Duration
	// ConditionThresholds defines the condition threshold per condition type.
	ConditionThresholds []ConditionThreshold
}

// GardenControllerConfig is the configuration for the garden controller.
type GardenControllerConfig struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the controller performs its reconciliation.
	SyncPeriod *metav1.Duration
	// ETCDConfig contains an optional configuration for the
	// backup compaction feature of ETCD backup-restore functionality.
	ETCDConfig *gardenletconfigv1alpha1.ETCDConfig
}

// GardenletDeployerControllerConfig is the configuration for the gardenlet deployer controller.
type GardenletDeployerControllerConfig struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	ConcurrentSyncs *int
}

// NetworkPolicyControllerConfiguration defines the configuration of the NetworkPolicy controller.
type NetworkPolicyControllerConfiguration struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	ConcurrentSyncs *int
	// AdditionalNamespaceSelectors is a list of label selectors for additional namespaces that should be considered by
	// the controller.
	AdditionalNamespaceSelectors []metav1.LabelSelector
}

// VPAEvictionRequirementsControllerConfiguration defines the configuration of the VPAEvictionRequirements controller.
type VPAEvictionRequirementsControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	ConcurrentSyncs *int
}

// ExtensionControllerConfiguration defines the configuration of the extension controller.
type ExtensionControllerConfiguration struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	ConcurrentSyncs *int
}

// ExtensionRequiredRuntimeControllerConfiguration defines the configuration of the extension-required-runtime controller.
type ExtensionRequiredRuntimeControllerConfiguration struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	ConcurrentSyncs *int
}

// ExtensionRequiredVirtualControllerConfiguration defines the configuration of the extension-required-virtual controller.
type ExtensionRequiredVirtualControllerConfiguration struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	ConcurrentSyncs *int
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
	// Webhooks is the configuration for the HTTPS webhook server.
	Webhooks Server
	// HealthProbes is the configuration for serving the healthz and readyz endpoints.
	HealthProbes *Server
	// Metrics is the configuration for serving the metrics endpoint.
	Metrics *Server
}

// Server contains information for HTTP(S) server configuration.
type Server struct {
	// BindAddress is the IP address on which to listen for the specified port.
	BindAddress string
	// Port is the port on which to serve requests.
	Port int
}

// NodeTolerationConfiguration contains information about node toleration options.
type NodeTolerationConfiguration struct {
	// DefaultNotReadyTolerationSeconds specifies the seconds for the `node.kubernetes.io/not-ready` toleration that
	// should be added to pods not already tolerating this taint.
	DefaultNotReadyTolerationSeconds *int64
	// DefaultUnreachableTolerationSeconds specifies the seconds for the `node.kubernetes.io/unreachable` toleration that
	// should be added to pods not already tolerating this taint.
	DefaultUnreachableTolerationSeconds *int64
}
