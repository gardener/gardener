// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AdmissionControllerConfiguration defines the configuration for the Gardener admission controller.
type AdmissionControllerConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// GardenClientConnection specifies the kubeconfig file and the client connection settings
	// when communicating with the garden apiserver.
	GardenClientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:"gardenClientConnection"`
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string `json:"logLevel"`
	// Server defines the configuration of the HTTP server.
	Server ServerConfiguration `json:"server"`
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
	// HTTPS is the configuration for the HTTPS server.
	HTTPS HTTPSServer `json:"https"`
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
	// ServerCertPath is the path to the server certificate file.
	ServerCertPath string `json:"serverCertPath"`
	// ServerKeyPath is the path to the private key file.
	ServerKeyPath string `json:"serverKeyPath"`
}
