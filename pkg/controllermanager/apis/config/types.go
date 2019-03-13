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
	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the gardener-apiserver.
	ClientConnection componentbaseconfig.ClientConnectionConfiguration
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
	// ShootBackup contains configuration settings for the etcd backups.
	ShootBackup *ShootBackup
	// FeatureGates is a map of feature names to bools that enable or disable alpha/experimental
	// features. This field modifies piecemeal the built-in default values from
	// "github.com/gardener/gardener/pkg/features/gardener_features.go".
	// Default: nil
	FeatureGates map[string]bool
}

// ControllerManagerControllerConfiguration defines the configuration of the controllers.
type ControllerManagerControllerConfiguration struct {
	// BackupInfrastructure defines the configuration of the BackupInfrastructure controller.
	BackupInfrastructure BackupInfrastructureControllerConfiguration
	// CloudProfile defines the configuration of the CloudProfile controller.
	// +optional
	CloudProfile *CloudProfileControllerConfiguration
	// ControllerRegistration defines the configuration of the ControllerRegistration controller.
	// +optional
	ControllerRegistration *ControllerRegistrationControllerConfiguration
	// ControllerInstallation defines the configuration of the ControllerInstallation controller.
	// +optional
	ControllerInstallation *ControllerInstallationControllerConfiguration
	// Plant defines the configuration of the Plant controller.
	// +optional
	Plant *PlantConfiguration
	// SecretBinding defines the configuration of the SecretBinding controller.
	// +optional
	SecretBinding *SecretBindingControllerConfiguration
	// Project defines the configuration of the Project controller.
	// +optional
	Project *ProjectControllerConfiguration
	// Quota defines the configuration of the Quota controller.
	// +optional
	Quota *QuotaControllerConfiguration
	// Seed defines the configuration of the Seed controller.
	// +optional
	Seed *SeedControllerConfiguration
	// Shoot defines the configuration of the Shoot controller.
	Shoot ShootControllerConfiguration
	// ShootCare defines the configuration of the ShootCare controller.
	ShootCare ShootCareControllerConfiguration
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
}

// ControllerRegistrationControllerConfiguration defines the configuration of the
// ControllerRegistration controller.
type ControllerRegistrationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
}

// ControllerInstallationControllerConfiguration defines the configuration of the
// ControllerInstallation controller.
type ControllerInstallationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
}

// PlantConfiguration defines the configuration of the
// PlantConfiguration controller.
type PlantConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration
}

// SecretBindingControllerConfiguration defines the configuration of the
// SecretBinding controller.
type SecretBindingControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
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

// SeedControllerConfiguration defines the configuration of the Seed controller.
type SeedControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
	// ReserveExcessCapacity indicates whether the Seed controller should reserve
	// excess capacity for Shoot control planes in the Seeds. This is done via
	// PodPriority and requires the Seed cluster to have Kubernetes version 1.11 or
	// the PodPriority feature gate as well as the scheduling.k8s.io/v1alpha1 API
	// group enabled. It defaults to true.
	// +optional
	ReserveExcessCapacity *bool
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration
}

// ShootControllerConfiguration defines the configuration of the CloudProfile
// controller.
type ShootControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
	// RespectSyncPeriodOverwrite determines whether a sync period overwrite of a
	// Shoot (via annotation) is respected or not. Defaults to false.
	// +optional
	RespectSyncPeriodOverwrite *bool
	// RetryDuration is the maximum duration how often a reconciliation will be retried
	// in case of errors.
	RetryDuration metav1.Duration
	// RetrySyncPeriod is the duration how fast Shoots with an errornous operation are
	// re-added to the queue so that the operation can be retried. Defaults to 15s.
	// +optional
	RetrySyncPeriod *metav1.Duration
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration
}

// ShootCareControllerConfiguration defines the configuration of the ShootCare
// controller.
type ShootCareControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of Shoot clusters is performed (only if no operation is
	// already running on them).
	SyncPeriod metav1.Duration
	// ConditionThresholds defines the condition threshold per condition type.
	// +optional
	ConditionThresholds []ConditionThreshold
}

// ConditionThreshold defines the duration how long a flappy condition stays in progressing state.
type ConditionThreshold struct {
	// Type is the type of the condition to define the threshold for.
	Type string
	// Duration is the duration how long the condition can stay in the progressing state.
	Duration metav1.Duration
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

// BackupInfrastructureControllerConfiguration defines the configuration of the BackupInfrastructure
// controller.
type BackupInfrastructureControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	ConcurrentSyncs int
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration
	// DeletionGracePeriodDays holds the period in number of days to delete the Backup Infrastructure after deletion timestamp is set.
	// If value is set to 0 then the BackupInfrastructureController will trigger deletion immediately.
	// +optional
	DeletionGracePeriodDays *int
}

// DiscoveryConfiguration defines the configuration of how to discover API groups.
// It allows to set where to store caching data and to specify the TTL of that data.
type DiscoveryConfiguration struct {
	// DiscoveryCacheDir is the directory to store discovery cache information.
	// If unset, the discovery client will use the current working directory.
	// +optional
	DiscoveryCacheDir *string
	// HTTPCacheDir is the directory to store discovery HTTP cache information.
	// If unset, no HTTP caching will be done.
	// +optional
	HTTPCacheDir *string
	// TTL is the ttl how long discovery cache information shall be valid.
	// +optional
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

// ShootBackup holds information about backup settings.
type ShootBackup struct {
	// Schedule defines the cron schedule according to which a backup is taken from etcd.
	Schedule string
}

const (
	// ControllerManagerDefaultLockObjectNamespace is the default lock namespace for leader election.
	ControllerManagerDefaultLockObjectNamespace = "garden"

	// ControllerManagerDefaultLockObjectName is the default lock name for leader election.
	ControllerManagerDefaultLockObjectName = "gardener-controller-manager-leader-election"
)
