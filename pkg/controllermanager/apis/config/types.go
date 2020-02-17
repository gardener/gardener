// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/klog"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerManagerConfiguration defines the configuration for the Gardener controller manager.
type ControllerManagerConfiguration struct {
	metav1.TypeMeta
	// GardenClientConnection specifies the kubeconfig file and the client connection settings
	// for the proxy server to use when communicating with the garden apiserver.
	GardenClientConnection componentbaseconfig.ClientConnectionConfiguration
	// Controllers defines the configuration of the controllers.
	Controllers ControllerManagerControllerConfiguration
	// LeaderElection defines the configuration of leader election client.
	LeaderElection LeaderElectionConfiguration
	// Discovery defines the configuration of the discovery client.
	Discovery DiscoveryConfiguration
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string
	// KubernetesLogLevel is the log level used for Kubernetes' k8s.io/klog functions.
	KubernetesLogLevel klog.Level
	// Server defines the configuration of the HTTP server.
	Server ServerConfiguration
	// FeatureGates is a map of feature names to bools that enable or disable alpha/experimental
	// features. This field modifies piecemeal the built-in default values from
	// "github.com/gardener/gardener/pkg/features/gardener_features.go".
	// Default: nil
	FeatureGates map[string]bool
}

// ControllerManagerControllerConfiguration defines the configuration of the controllers.
type ControllerManagerControllerConfiguration struct {
	// CloudProfile defines the configuration of the CloudProfile controller.
	CloudProfile *CloudProfileControllerConfiguration
	// ControllerRegistration defines the configuration of the ControllerRegistration controller.
	ControllerRegistration *ControllerRegistrationControllerConfiguration
	// Plant defines the configuration of the Plant controller.
	Plant *PlantControllerConfiguration
	// Project defines the configuration of the Project controller.
	Project *ProjectControllerConfiguration
	// Quota defines the configuration of the Quota controller.
	Quota *QuotaControllerConfiguration
	// SecretBinding defines the configuration of the SecretBinding controller.
	SecretBinding *SecretBindingControllerConfiguration
	// Seed defines the configuration of the Seed controller.
	Seed *SeedControllerConfiguration
	// ShootMaintenance defines the configuration of the ShootMaintenance controller.
	ShootMaintenance ShootMaintenanceControllerConfiguration
	// ShootQuota defines the configuration of the ShootQuota controller.
	ShootQuota ShootQuotaControllerConfiguration
	// ShootHibernation defines the configuration of the ShootHibernation controller.
	ShootHibernation ShootHibernationControllerConfiguration
}

// CloudProfileControllerConfiguration defines the configuration of the CloudProfile
// controller.
type CloudProfileControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
	// KubernetesVersionManagement configures the version policy that applies to
	// Kubernetes versions in the CloudProfile
	KubernetesVersionManagement *KubernetesVersionManagement
	// MachineImageVersionManagement configures the version policy that applies to
	// MachineImage versions in the CloudProfile
	MachineImageVersionManagement *MachineImageVersionManagement
}

// KubernetesVersionManagement configures the version policy that applies to
// Kubernetes versions in the CloudProfile
// You can read more about it here: https://github.com/gardener/gardener/blob/master/docs/proposals/05-versioning-policy.md
type KubernetesVersionManagement struct {
	// Enabled defines whether the KubernetesVersionManagement is enabled
	Enabled bool
	// MaintainedKubernetesVersions is the amount of minor Kubernetes versions that are considered to be "maintained"
	// refers to versions existing in the CloudProfile
	// defaults to 3 as this is common practice in the Kubernetes Community
	// e.g Versions in CloudProfile: 1.17, 1.15, 1.14, 1.13 -> Maintained: 1.17 & 1.15 & 1.14, Unmaintained: 1.13
	MaintainedKubernetesVersions *int
	// ExpirationDurationMaintainedVersion is the time until a deprecated Kubernetes patch version
	// of a supported minor version expires
	// defaults to 4 months (with each 30 days)
	ExpirationDurationMaintainedVersion *metav1.Duration
	// ExpirationDurationUnmaintainedVersion is the time until a deprecated Kubernetes patch version
	// of an unsupported minor version expires
	// defaults to 1 month (with 30 days)
	ExpirationDurationUnmaintainedVersion *metav1.Duration
}

// MachineImageVersionManagement configures the version policy that applies to
// MachineImage versions in the CloudProfile
// You can read more about it here: https://github.com/gardener/gardener/blob/master/docs/proposals/05-versioning-policy.md
type MachineImageVersionManagement struct {
	// Enabled defines whether the MachineImageVersionManagement is enabled
	Enabled bool
	// ExpirationDuration is the time until a deprecated machine image version expires
	// defaults to 4 months
	ExpirationDuration *metav1.Duration
}

// ControllerRegistrationControllerConfiguration defines the configuration of the
// ControllerRegistration controller.
type ControllerRegistrationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
}

// PlantControllerConfiguration defines the configuration of the
// PlantControllerConfiguration controller.
type PlantControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration
}

// ProjectControllerConfiguration defines the configuration of the
// Project controller.
type ProjectControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
}

// QuotaControllerConfiguration defines the configuration of the Quota controller.
type QuotaControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
}

// SecretBindingControllerConfiguration defines the configuration of the
// SecretBinding controller.
type SecretBindingControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
}

// SeedControllerConfiguration defines the configuration of the
// Seed controller.
type SeedControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
	// MonitorPeriod is the duration after the seed controller will mark the `GardenletReady`
	// condition in `Seed` resources as `Unknown` in case the gardenlet did not send heartbeats.
	// +optional
	MonitorPeriod *metav1.Duration
	// ShootMonitorPeriod is the duration after the seed controller will mark Gardener's conditions
	// in `Shoot` resources as `Unknown` in case the gardenlet of the responsible seed cluster did
	// not send heartbeats.
	ShootMonitorPeriod *metav1.Duration
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration
}

// ShootMaintenanceControllerConfiguration defines the configuration of the
// ShootMaintenance controller.
type ShootMaintenanceControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
}

// ShootQuotaControllerConfiguration defines the configuration of the
// ShootQuota controller.
type ShootQuotaControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
	// SyncPeriod is the duration how often the existing resources are reconciled
	// (how often Shoots referenced Quota is checked).
	SyncPeriod metav1.Duration
}

// ShootHibernationControllerConfiguration defines the configuration of the
// ShootHibernation controller.
type ShootHibernationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
}

// DiscoveryConfiguration defines the configuration of how to discover API groups.
// It allows to set where to store caching data and to specify the TTL of that data.
type DiscoveryConfiguration struct {
	// DiscoveryCacheDir is the directory to store discovery cache information.
	// If unset, the discovery client will use the current working directory.
	DiscoveryCacheDir *string
	// HTTPCacheDir is the directory to store discovery HTTP cache information.
	// If unset, no HTTP caching will be done.
	HTTPCacheDir *string
	// TTL is the ttl how long discovery cache information shall be valid.
	TTL *metav1.Duration
}

// LeaderElectionConfiguration defines the configuration of leader election
// clients for components that can run with leader election enabled.
type LeaderElectionConfiguration struct {
	componentbaseconfig.LeaderElectionConfiguration
	// LockObjectNamespace defines the namespace of the lock object.
	LockObjectNamespace string
	// LockObjectName defines the lock object name.
	LockObjectName string
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
	// HTTP is the configuration for the HTTP server.
	HTTP Server
	// HTTPS is the configuration for the HTTPS server.
	HTTPS HTTPSServer
}

// Server contains information for HTTP(S) server configuration.
type Server struct {
	// BindAddress is the IP address on which to listen for the specified port.
	BindAddress string
	// Port is the port on which to serve unsecured, unauthenticated access.
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
	// ServerCertPath is the path to the server certificate file.
	ServerCertPath string
	// ServerKeyPath is the path to the private key file.
	ServerKeyPath string
}

const (
	// ControllerManagerDefaultLockObjectNamespace is the default lock namespace for leader election.
	ControllerManagerDefaultLockObjectNamespace = "garden"

	// ControllerManagerDefaultLockObjectName is the default lock name for leader election.
	ControllerManagerDefaultLockObjectName = "gardener-controller-manager-leader-election"
)
