// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	// TempDir is the directory on the worker node that contains temporary directories of files.
	TempDir = BaseDir + "/tmp"
	// BinaryDir is the directory on the worker node that contains the binary for the gardener-node-agent.
	BinaryDir = "/opt/bin"

	// AccessSecretName is a constant for the secret name for the gardener-node-agent's shoot access secret.
	AccessSecretName = "gardener-node-agent"
	// BootstrapTokenFilePath is the file path on the worker node that contains the bootstrap token for the node.
	BootstrapTokenFilePath = CredentialsDir + "/bootstrap-token"
	// TokenFilePath is the file path on the worker node that contains the access token of the gardener-node-agent.
	TokenFilePath = CredentialsDir + "/token"
	// ConfigFilePath is the file path on the worker node that contains the configuration of the gardener-node-agent.
	ConfigFilePath = BaseDir + "/config.yaml"
	// KubeconfigFilePath is the file path on the worker node that contains the kubeconfig of the gardener-node-agent.
	KubeconfigFilePath = CredentialsDir + "/kubeconfig"
	// MachineNameFilePath is the file path on the worker node that contains the machine name.
	MachineNameFilePath = BaseDir + "/machine-name"

	// UnitName is the name of the gardener-node-agent systemd service.
	UnitName = "gardener-node-agent.service"
	// InitUnitName is the name of the gardener-node-agent systemd service.
	InitUnitName = "gardener-node-init.service"

	// DataKeyOperatingSystemConfig is the constant for a key in the data map of an OSC secret which contains the
	// encoded operating system config.
	DataKeyOperatingSystemConfig = "osc.yaml"
	// AnnotationKeyChecksumDownloadedOperatingSystemConfig is a constant for an annotation key on a Secret describing
	// the checksum of the operating system configuration in the data map.
	AnnotationKeyChecksumDownloadedOperatingSystemConfig = "checksum/data-script"
	// AnnotationKeyChecksumAppliedOperatingSystemConfig is a constant for an annotation key on a Node describing the
	// checksum of the last applied operating system configuration.
	AnnotationKeyChecksumAppliedOperatingSystemConfig = "checksum/cloud-config-data"
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
	// APIServer contains information about the API server.
	APIServer APIServer `json:"apiServer"`
	// Bootstrap contains configuration for the bootstrap command.
	// +optional
	Bootstrap *BootstrapConfiguration `json:"bootstrap,omitempty"`
	// Controllers defines the configuration of the controllers.
	Controllers ControllerConfiguration `json:"controllers"`
}

// APIServer contains information about the API server.
type APIServer struct {
	// Server is the address of the API server.
	Server string `json:"server"`
	// CABundle is the certificate authority bundle for the API server.
	CABundle []byte `json:"caBundle"`
}

// BootstrapConfiguration contains configuration for the bootstrap command.
type BootstrapConfiguration struct {
	// KubeletDataVolumeSize sets the data volume size of an unformatted disk on the worker node, which is used for
	// /var/lib on the worker.
	// +optional
	KubeletDataVolumeSize *int64 `json:"kubeletDataVolumeSize,omitempty"`
}

// ControllerConfiguration defines the configuration of the controllers.
type ControllerConfiguration struct {
	// OperatingSystemConfig is the configuration for the operating system config controller.
	OperatingSystemConfig OperatingSystemConfigControllerConfig `json:"operatingSystemConfig"`
	// Token is the configuration for the access token controller.
	Token TokenControllerConfig `json:"token"`
}

// OperatingSystemConfigControllerConfig defines the configuration of the operating system config controller.
type OperatingSystemConfigControllerConfig struct {
	// SyncPeriod is the duration how often the operating system config is applied.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// SecretName defines the name of the secret in the shoot cluster control plane, which contains the operating system
	// config (OSC) for the gardener-node-agent.
	SecretName string `json:"secretName"`
	// KubernetesVersion contains the Kubernetes version of the kubelet, used for annotating the corresponding node
	// resource with a kubernetes version annotation.
	KubernetesVersion *semver.Version `json:"kubernetesVersion"`
}

// TokenControllerConfig defines the configuration of the access token controller.
type TokenControllerConfig struct {
	// SyncConfigs is the list of configurations for syncing access tokens.
	// +optional
	SyncConfigs []TokenSecretSyncConfig `json:"syncConfigs,omitempty"`
	// SyncPeriod is the duration how often the access token secrets are synced to the disk.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
}

// TokenSecretSyncConfig contains configurations for syncing access tokens.
type TokenSecretSyncConfig struct {
	// SecretName defines the name of the secret in the shoot cluster's kube-system namespace which contains the access
	// token.
	SecretName string `json:"secretName"`
	// Path is the path on the machine where the access token content should be synced.
	Path string `json:"path"`
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
