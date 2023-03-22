// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AdmissionControllerConfiguration defines the configuration for the Gardener admission controller.
type AdmissionControllerConfiguration struct {
	metav1.TypeMeta
	// GardenClientConnection specifies the kubeconfig file and the client connection settings
	// when communicating with the garden apiserver.
	GardenClientConnection componentbaseconfig.ClientConnectionConfiguration
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	// Defaults to "info".
	LogLevel string
	// LogFormat is the format for the logs. Must be one of [json,text].
	// Defaults to "json".
	LogFormat string
	// Server defines the configuration of the HTTP server.
	Server ServerConfiguration
	// Debugging holds configuration for Debugging related features.
	Debugging *componentbaseconfig.DebuggingConfiguration
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
	// Webhooks is the configuration for the HTTPS webhook server.
	Webhooks HTTPSServer
	// HealthProbes is the configuration for serving the healthz and readyz endpoints.
	HealthProbes *Server
	// Metrics is the configuration for serving the metrics endpoint.
	Metrics *Server
	// ResourceAdmissionConfiguration is the configuration for the resource admission.
	ResourceAdmissionConfiguration *ResourceAdmissionConfiguration
	// EnableDebugHandlers determines whether the /debug/ handlers are enabled.
	EnableDebugHandlers *bool
}

// ResourceAdmissionConfiguration contains settings about arbitrary kinds and the size each resource should have at most.
type ResourceAdmissionConfiguration struct {
	// Limits contains configuration for resources which are subjected to size limitations.
	Limits []ResourceLimit
	// UnrestrictedSubjects contains references to users, groups, or service accounts which aren't subjected to any resource size limit.
	UnrestrictedSubjects []rbacv1.Subject
	// OperationMode specifies the mode the webhooks operates in. Allowed values are "block" and "log". Defaults to "block".
	OperationMode *ResourceAdmissionWebhookMode
}

// ResourceAdmissionWebhookMode is an alias type for the resource admission webhook mode.
type ResourceAdmissionWebhookMode string

// WildcardAll is a character which represents all elements in a set.
const WildcardAll = "*"

// ResourceLimit contains settings about a kind and the size each resource should have at most.
type ResourceLimit struct {
	// APIGroup is the name of the APIGroup that contains the limited resource. WildcardAll represents all groups.
	APIGroups []string
	// APIVersions is the version of the resource. WildcardAll represents all versions.
	APIVersions []string
	// Resource is the name of the resource this rule applies to. WildcardAll represents all resources.
	Resources []string
	// Size specifies the imposed limit.
	Size resource.Quantity
}

// Server contains information for HTTP(S) server configuration.
type Server struct {
	// BindAddress is the IP address on which to listen for the specified port.
	BindAddress string
	// Port is the port on which to serve requests.
	Port int
}

// HTTPSServer is the configuration for the HTTPSServer server.
type HTTPSServer struct {
	// Server is the configuration for the bind address and the port.
	Server
	// TLSServer contains information about the TLS configuration for a HTTPS server.
	TLS TLSServer
}

// TLSServer contains information about the TLS configuration for a HTTPS server.
type TLSServer struct {
	// ServerCertDir is the path to a directory containing the server's TLS certificate and key (the files must be
	// named tls.crt and tls.key respectively).
	ServerCertDir string
}

const (
	// AdmissionModeBlock specifies that the webhook should block violating requests.
	AdmissionModeBlock ResourceAdmissionWebhookMode = "block"
	// AdmissionModeLog specifies that the webhook should only log violating requests.
	AdmissionModeLog ResourceAdmissionWebhookMode = "log"
)
