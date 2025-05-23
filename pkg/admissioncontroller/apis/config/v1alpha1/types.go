// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AdmissionControllerConfiguration defines the configuration for the Gardener admission controller.
type AdmissionControllerConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// GardenClientConnection specifies the kubeconfig file and the client connection settings
	// when communicating with the garden apiserver.
	GardenClientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:"gardenClientConnection"`
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	// Defaults to "info".
	LogLevel string `json:"logLevel"`
	// LogFormat is the format for the logs. Must be one of [json,text].
	// Defaults to "json".
	LogFormat string `json:"logFormat"`
	// Server defines the configuration of the HTTP server.
	Server ServerConfiguration `json:"server"`
	// Debugging holds configuration for Debugging related features.
	// +optional
	Debugging *componentbaseconfigv1alpha1.DebuggingConfiguration `json:"debugging,omitempty"`
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
	// Webhooks is the configuration for the HTTPS webhook server.
	Webhooks HTTPSServer `json:"webhooks"`
	// HealthProbes is the configuration for serving the healthz and readyz endpoints.
	// +optional
	HealthProbes *Server `json:"healthProbes,omitempty"`
	// Metrics is the configuration for serving the metrics endpoint.
	// +optional
	Metrics *Server `json:"metrics,omitempty"`
	// ResourceAdmissionConfiguration is the configuration for the resource admission.
	// +optional
	ResourceAdmissionConfiguration *ResourceAdmissionConfiguration `json:"resourceAdmissionConfiguration,omitempty"`
	// EnableDebugHandlers determines whether the /debug/ handlers are enabled.
	// +optional
	EnableDebugHandlers *bool `json:"enableDebugHandlers,omitempty"`
}

// ResourceAdmissionConfiguration contains settings about arbitrary kinds and the size each resource should have at most.
type ResourceAdmissionConfiguration struct {
	// Limits contains configuration for resources which are subjected to size limitations.
	Limits []ResourceLimit `json:"limits"`
	// UnrestrictedSubjects contains references to users, groups, or service accounts which aren't subjected to any resource size limit.
	// +optional
	UnrestrictedSubjects []rbacv1.Subject `json:"unrestrictedSubjects,omitempty"`
	// OperationMode specifies the mode the webhooks operates in. Allowed values are "block" and "log". Defaults to "block".
	// +optional
	OperationMode *ResourceAdmissionWebhookMode `json:"operationMode,omitempty"`
}

// ResourceAdmissionWebhookMode is an alias type for the resource admission webhook mode.
type ResourceAdmissionWebhookMode string

// WildcardAll is a character which represents all elements in a set.
const WildcardAll = "*"

// ResourceLimit contains settings about a kind and the size each resource should have at most.
type ResourceLimit struct {
	// APIGroups is the name of the APIGroup that contains the limited resource. WildcardAll represents all groups.
	// +optional
	APIGroups []string `json:"apiGroups,omitempty"`
	// APIVersions is the version of the resource. WildcardAll represents all versions.
	// +optional
	APIVersions []string `json:"apiVersions,omitempty"`
	// Resources is the name of the resource this rule applies to. WildcardAll represents all resources.
	Resources []string `json:"resources"`
	// Size specifies the imposed limit.
	Size resource.Quantity `json:"size"`
}

// Server contains information for HTTP(S) server configuration.
type Server struct {
	// BindAddress is the IP address on which to listen for the specified port.
	BindAddress string `json:"bindAddress"`
	// Port is the port on which to serve requests.
	Port int `json:"port"`
}

// HTTPSServer is the configuration for the HTTPSServer server.
type HTTPSServer struct {
	// Server is the configuration for the bind address and the port.
	Server `json:",inline"`
	// TLSServer contains information about the TLS configuration for a HTTPS server.
	TLS TLSServer `json:"tls"`
}

// TLSServer contains information about the TLS configuration for a HTTPS server.
type TLSServer struct {
	// ServerCertDir is the path to a directory containing the server's TLS certificate and key (the files must be
	// named tls.crt and tls.key respectively).
	ServerCertDir string `json:"serverCertDir"`
}

const (
	// AdmissionModeBlock specifies that the webhook should block violating requests.
	AdmissionModeBlock ResourceAdmissionWebhookMode = "block"
	// AdmissionModeLog specifies that the webhook should only log violating requests.
	AdmissionModeLog ResourceAdmissionWebhookMode = "log"
)
