// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

const (
	// BaseDir is the directory on the worker node that contains gardener-node-agent relevant files.
	BaseDir = "/var/lib/gardener-node-agent"

	// CredentialsDir is the directory on the worker node that contains credentials for the gardener-node-agent.
	CredentialsDir = BaseDir + "/credentials"
	// BootstrapTokenFilePath is the file path on the worker node that contains the bootstrap token for the node.
	BootstrapTokenFilePath = CredentialsDir + "/bootstrap-token"
	// TokenFilePath is the file path on the worker node that contains the access token of the gardener-node-agent.
	TokenFilePath = CredentialsDir + "/token"

	// ConfigPath is the file path on the worker node that contains the configuration
	// of the gardener-node-agent.
	ConfigPath = BaseDir + "/config.yaml"
	// InitScriptPath is the file path on the worker node that contains the init script
	// of the gardener-node-agent.
	InitScriptPath = BaseDir + "/gardener-node-init.sh"

	// NodeInitUnitName is the name of the gardener-node-init systemd service.
	NodeInitUnitName = "gardener-node-init.service"
	// UnitName is the name of the gardener-node-agent systemd service.
	UnitName = "gardener-node-agent.service"

	// OSCSecretKey is the key inside the gardener-node-agent osc secret to access
	// the encoded osc.
	OSCSecretKey = "gardener-node-agent"
	// OSCOldConfigPath is the file path on the worker node that contains the
	// previous content of the osc
	OSCOldConfigPath = BaseDir + "/previous-osc.yaml"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeAgentConfiguration defines the configuration for the gardener-node-agent.
type NodeAgentConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// ClientConnection specifies the kubeconfig file and the client connection settings for the proxy server to use
	// when communicating with the kube-apiserver of the shoot cluster.
	ClientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:"clientConnection"`
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
	// OperatingSystemConfigSecretName defines the name of the secret in the shoot cluster control plane, which contains
	// the Operating System Config (OSC) for the gardener-node-agent.
	OperatingSystemConfigSecretName string `json:"operatingSystemConfigSecretName"`
	// AccessTokenSecretName defines the name of the secret in the shoot cluster control plane, which contains
	// the `kube-apiserver` access token for the gardener-node-agent.
	AccessTokenSecretName string `json:"accessTokenSecretName"`
	// Image is the container image reference to the gardener-node-agent.
	Image string `json:"image"`
	// HyperkubeImage is the container image reference to the hyperkube containing kubelet.
	HyperkubeImage string `json:"hyperkubeImage"`
	// KubernetesVersion contains the kubernetes version of the kubelet, used for annotating the corresponding node
	// resource with a kubernetes version annotation.
	KubernetesVersion *semver.Version `json:"kubernetesVersion"`
	// KubeletDataVolumeSize sets the data volume size of an unformatted disk on the worker node, which is used for
	// /var/lib on the worker.
	// +optional
	KubeletDataVolumeSize *int64 `json:"kubeletDataVolumeSize,omitempty"`
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
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
