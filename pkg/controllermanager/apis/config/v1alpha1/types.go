// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/klog"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerManagerConfiguration defines the configuration for the Gardener controller manager.
type ControllerManagerConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// GardenClientConnection specifies the kubeconfig file and the client connection settings
	// for the proxy server to use when communicating with the garden apiserver.
	GardenClientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:"gardenClientConnection"`
	// Controllers defines the configuration of the controllers.
	Controllers ControllerManagerControllerConfiguration `json:"controllers"`
	// LeaderElection defines the configuration of leader election client.
	LeaderElection LeaderElectionConfiguration `json:"leaderElection"`
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string `json:"logLevel"`
	// LogFormat is the output format for the logs. Must be one of [text,json].
	LogFormat string `json:"logFormat"`
	// KubernetesLogLevel is the log level used for Kubernetes' k8s.io/klog functions.
	KubernetesLogLevel klog.Level `json:"kubernetesLogLevel"`
	// Server defines the configuration of the HTTP server.
	Server ServerConfiguration `json:"server"`
	// Debugging holds configuration for Debugging related features.
	// +optional
	Debugging *componentbaseconfigv1alpha1.DebuggingConfiguration `json:"debugging,omitempty"`
	// FeatureGates is a map of feature names to bools that enable or disable alpha/experimental
	// features. This field modifies piecemeal the built-in default values from
	// "github.com/gardener/gardener/pkg/controllermanager/features/features.go".
	// Default: nil
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// ControllerManagerControllerConfiguration defines the configuration of the controllers.
type ControllerManagerControllerConfiguration struct {
	// Bastion defines the configuration of the Bastion controller.
	// +optional
	Bastion *BastionControllerConfiguration `json:"bastion,omitempty"`
	// CloudProfile defines the configuration of the CloudProfile controller.
	// +optional
	CloudProfile *CloudProfileControllerConfiguration `json:"cloudProfile,omitempty"`
	// ControllerDeployment defines the configuration of the ControllerDeployment controller.
	// +optional
	ControllerDeployment *ControllerDeploymentControllerConfiguration `json:"controllerDeployment,omitempty"`
	// ControllerRegistration defines the configuration of the ControllerRegistration controller.
	// +optional
	ControllerRegistration *ControllerRegistrationControllerConfiguration `json:"controllerRegistration,omitempty"`
	// Event defines the configuration of the Event controller.  If unset, the event controller will be disabled.
	// +optional
	Event *EventControllerConfiguration `json:"event,omitempty"`
	// ExposureClass defines the configuration of the ExposureClass controller.
	// +optional
	ExposureClass *ExposureClassControllerConfiguration `json:"exposureClass,omitempty"`
	// Plant defines the configuration of the Plant controller.
	// +optional
	Plant *PlantControllerConfiguration `json:"plant,omitempty"`
	// Project defines the configuration of the Project controller.
	// +optional
	Project *ProjectControllerConfiguration `json:"project,omitempty"`
	// Quota defines the configuration of the Quota controller.
	// +optional
	Quota *QuotaControllerConfiguration `json:"quota,omitempty"`
	// SecretBinding defines the configuration of the SecretBinding controller.
	// +optional
	SecretBinding *SecretBindingControllerConfiguration `json:"secretBinding,omitempty"`
	// Seed defines the configuration of the Seed lifecycle controller.
	// +optional
	Seed *SeedControllerConfiguration `json:"seed,omitempty"`
	// ShootMaintenance defines the configuration of the ShootMaintenance controller.
	ShootMaintenance ShootMaintenanceControllerConfiguration `json:"shootMaintenance"`
	// ShootQuota defines the configuration of the ShootQuota controller.
	ShootQuota ShootQuotaControllerConfiguration `json:"shootQuota"`
	// ShootHibernation defines the configuration of the ShootHibernation controller.
	ShootHibernation ShootHibernationControllerConfiguration `json:"shootHibernation"`
	// ShootReference defines the configuration of the ShootReference controller. If unspecified, it is defaulted with `concurrentSyncs=5`.
	// +optional
	ShootReference *ShootReferenceControllerConfiguration `json:"shootReference,omitempty"`
	// ShootRetry defines the configuration of the ShootRetry controller. If unspecified, it is defaulted with `concurrentSyncs=5`.
	// +optional
	ShootRetry *ShootRetryControllerConfiguration `json:"shootRetry,omitempty"`
	// ManagedSeedSet defines the configuration of the ManagedSeedSet controller.
	// +optional
	ManagedSeedSet *ManagedSeedSetControllerConfiguration `json:"managedSeedSet,omitempty"`
}

// BastionControllerConfiguration defines the configuration of the Bastion
// controller.
type BastionControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// MaxLifetime is the maximum time a Bastion resource can exist before it is
	// forcefully deleted (defaults to '24h').
	// +optional
	MaxLifetime *metav1.Duration `json:"maxLifetime,omitempty"`
}

// CloudProfileControllerConfiguration defines the configuration of the CloudProfile
// controller.
type CloudProfileControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
}

// ControllerDeploymentControllerConfiguration defines the configuration of the
// ControllerDeployment controller.
type ControllerDeploymentControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
}

// ControllerRegistrationControllerConfiguration defines the configuration of the
// ControllerRegistration controller.
type ControllerRegistrationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
}

// EventControllerConfiguration defines the configuration of the Event controller.
type EventControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// TTLNonShootEvents is the time-to-live for all non-shoot related events (defaults to `1h`).
	// +optional
	TTLNonShootEvents *metav1.Duration `json:"ttlNonShootEvents,omitempty"`
}

// ExposureClassControllerConfiguration defines the configuration of the
// ExposureClass controller.
type ExposureClassControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
}

// PlantControllerConfiguration defines the configuration of the
// PlantControllerConfiguration controller.
type PlantControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration `json:"syncPeriod"`
}

// ProjectControllerConfiguration defines the configuration of the
// Project controller.
type ProjectControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// MinimumLifetimeDays is the number of days a `Project` may exist before it is being
	// checked whether it is actively used or got stale.
	// +optional
	MinimumLifetimeDays *int `json:"minimumLifetimeDays,omitempty"`
	// Quotas is the default configuration matching projects are set up with if a quota is not already specified.
	// +optional
	Quotas []QuotaConfiguration `json:"quotas,omitempty"`
	// StaleGracePeriodDays is the number of days a `Project` may be unused before it will
	// be considered for checks whether it is actively used or got stale.
	// +optional
	StaleGracePeriodDays *int `json:"staleGracePeriodDays,omitempty"`
	// StaleExpirationTimeDays is the number of days after a `Project` that has been marked as
	// 'stale'/'unused' and passed the 'stale grace period' will be considered for auto deletion.
	// +optional
	StaleExpirationTimeDays *int `json:"staleExpirationTimeDays,omitempty"`
	// StaleSyncPeriod is the duration how often the reconciliation loop for stale Projects is executed.
	// +optional
	StaleSyncPeriod *metav1.Duration `json:"staleSyncPeriod,omitempty"`
}

// QuotaConfiguration defines quota configurations.
type QuotaConfiguration struct {
	// Config is the quota specification used for the project set-up.
	// Only v1.ResourceQuota resources are supported.
	Config runtime.RawExtension `json:"config"`
	// ProjectSelector is an optional setting to select the projects considered for quotas.
	// Defaults to empty LabelSelector, which matches all projects.
	// +optional
	ProjectSelector *metav1.LabelSelector `json:"projectSelector,omitempty"`
}

// QuotaControllerConfiguration defines the configuration of the Quota controller.
type QuotaControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
}

// SecretBindingControllerConfiguration defines the configuration of the
// SecretBinding controller.
type SecretBindingControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
}

// SeedControllerConfiguration defines the configuration of the
// Seed controller.
type SeedControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// MonitorPeriod is the duration after the seed controller will mark the `GardenletReady`
	// condition in `Seed` resources as `Unknown` in case the gardenlet did not send heartbeats.
	// +optional
	MonitorPeriod *metav1.Duration `json:"monitorPeriod,omitempty"`
	// ShootMonitorPeriod is the duration after the seed controller will mark Gardener's conditions
	// in `Shoot` resources as `Unknown` in case the gardenlet of the responsible seed cluster did
	// not send heartbeats.
	// +optional
	ShootMonitorPeriod *metav1.Duration `json:"shootMonitorPeriod,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration `json:"syncPeriod"`
}

// ShootMaintenanceControllerConfiguration defines the configuration of the
// ShootMaintenance controller.
type ShootMaintenanceControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// EnableShootControlPlaneRestarter configures whether adequate pods of the shoot control plane are restarted during maintenance.
	// +optional
	EnableShootControlPlaneRestarter *bool `json:"enableShootControlPlaneRestarter"`
	// EnableShootCoreAddonRestarter configures whether some core addons to be restarted during maintenance.
	// +optional
	EnableShootCoreAddonRestarter *bool `json:"enableShootCoreAddonRestarter"`
}

// ShootQuotaControllerConfiguration defines the configuration of the
// ShootQuota controller.
type ShootQuotaControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// SyncPeriod is the duration how often the existing resources are reconciled
	// (how often Shoots referenced Quota is checked).
	SyncPeriod metav1.Duration `json:"syncPeriod"`
}

// ShootHibernationControllerConfiguration defines the configuration of the
// ShootHibernation controller.
type ShootHibernationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
}

// ShootReferenceControllerConfiguration defines the configuration of the
// ShootReference controller.
type ShootReferenceControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// shoots.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// ProtectAuditPolicyConfigMaps controls whether the shoot reference controller shall protect ConfigMaps containing
	// audit policies and referenced in Shoots.
	// +optional
	ProtectAuditPolicyConfigMaps *bool `json:"protectAuditPolicyConfigMaps,omitempty"`
}

// ShootRetryControllerConfiguration defines the configuration of the
// ShootRetry controller.
type ShootRetryControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// RetryPeriod is the retry period for retrying failed Shoots that match certain criterion.
	// Defaults to 10m.
	// +optional
	RetryPeriod *metav1.Duration `json:"retryPeriod,omitempty"`
}

// ManagedSeedSetControllerConfiguration defines the configuration of the
// ManagedSeedSet controller.
type ManagedSeedSetControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// MaxShootRetries is the maximum number of times to retry failed shoots before giving up. Defaults to 3.
	// +optional
	MaxShootRetries *int `json:"maxShootRetries,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration `json:"syncPeriod"`
}

// LeaderElectionConfiguration defines the configuration of leader election
// clients for components that can run with leader election enabled.
type LeaderElectionConfiguration struct {
	componentbaseconfigv1alpha1.LeaderElectionConfiguration `json:",inline"`
	// LockObjectNamespace defines the namespace of the lock object.
	LockObjectNamespace string `json:"lockObjectNamespace"`
	// LockObjectName defines the lock object name.
	LockObjectName string `json:"lockObjectName"`
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
	// HTTP is the configuration for the HTTP server.
	HTTP Server `json:"http"`
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

const (
	// ControllerManagerDefaultLockObjectNamespace is the default lock namespace for leader election.
	ControllerManagerDefaultLockObjectNamespace = "garden"

	// ControllerManagerDefaultLockObjectName is the default lock name for leader election.
	ControllerManagerDefaultLockObjectName = "gardener-controller-manager-leader-election"

	// DefaultDiscoveryTTL is the default ttl for the cached discovery client.
	DefaultDiscoveryTTL = 10 * time.Second
)
