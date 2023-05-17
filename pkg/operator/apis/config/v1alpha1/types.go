// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
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

// ControllerConfiguration defines the configuration of the controllers.
type ControllerConfiguration struct {
	// Garden is the configuration for the garden controller.
	Garden GardenControllerConfig `json:"garden"`
	// NetworkPolicy is the configuration for the NetworkPolicy controller.
	NetworkPolicy NetworkPolicyControllerConfiguration `json:"networkPolicy"`
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
	ETCDConfig *gardenletv1alpha1.ETCDConfig `json:"etcdConfig,omitempty"`
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
