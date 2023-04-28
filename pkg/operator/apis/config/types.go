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

package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"

	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperatorConfiguration defines the configuration for the Gardener oeprator.
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
}

// ControllerConfiguration defines the configuration of the controllers.
type ControllerConfiguration struct {
	// Garden is the configuration for the garden controller.
	Garden GardenControllerConfig
	// NetworkPolicy is the configuration for the NetworkPolicy controller.
	NetworkPolicy NetworkPolicyControllerConfiguration
}

// GardenControllerConfig is the configuration for the garden controller.
type GardenControllerConfig struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the controller performs its reconciliation.
	SyncPeriod *metav1.Duration
	// ETCDConfig contains an optional configuration for the
	// backup compaction feature of ETCD backup-restore functionality.
	ETCDConfig *gardenletconfig.ETCDConfig
}

// NetworkPolicyControllerConfiguration defines the configuration of the NetworkPolicy controller.
type NetworkPolicyControllerConfiguration struct {
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
