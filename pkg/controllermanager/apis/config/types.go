// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfig "k8s.io/component-base/config"
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
	LeaderElection *componentbaseconfig.LeaderElectionConfiguration
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string
	// LogFormat is the output format for the logs. Must be one of [text,json].
	LogFormat string
	// Server defines the configuration of the HTTP server.
	Server ServerConfiguration
	// Debugging holds configuration for Debugging related features.
	Debugging *componentbaseconfig.DebuggingConfiguration
	// FeatureGates is a map of feature names to bools that enable or disable alpha/experimental
	// features. This field modifies piecemeal the built-in default values from
	// "github.com/gardener/gardener/pkg/controllermanager/features/features.go".
	// Default: nil
	FeatureGates map[string]bool
}

// ControllerManagerControllerConfiguration defines the configuration of the controllers.
type ControllerManagerControllerConfiguration struct {
	// Bastion defines the configuration of the Bastion controller.
	Bastion *BastionControllerConfiguration
	// CertificateSigningRequest defines the configuration of the CertificateSigningRequest controller.
	CertificateSigningRequest *CertificateSigningRequestControllerConfiguration
	// CloudProfile defines the configuration of the CloudProfile controller.
	CloudProfile *CloudProfileControllerConfiguration
	// NamespacedCloudProfile defines the configuration of the NamespacedCloudProfile controller.
	NamespacedCloudProfile *NamespacedCloudProfileControllerConfiguration
	// ControllerDeployment defines the configuration of the ControllerDeployment controller.
	ControllerDeployment *ControllerDeploymentControllerConfiguration
	// ControllerRegistration defines the configuration of the ControllerRegistration controller.
	ControllerRegistration *ControllerRegistrationControllerConfiguration
	// Event defines the configuration of the Event controller.  If unset, the event controller will be disabled.
	Event *EventControllerConfiguration
	// ExposureClass defines the configuration of the ExposureClass controller.
	ExposureClass *ExposureClassControllerConfiguration
	// Project defines the configuration of the Project controller.
	Project *ProjectControllerConfiguration
	// Quota defines the configuration of the Quota controller.
	Quota *QuotaControllerConfiguration
	// SecretBinding defines the configuration of the SecretBinding controller.
	SecretBinding *SecretBindingControllerConfiguration
	// CredentialsBinding defines the configuration of the CredentialsBinding controller.
	CredentialsBinding *CredentialsBindingControllerConfiguration
	// Seed defines the configuration of the Seed controller.
	Seed *SeedControllerConfiguration
	// SeedExtensionsCheck defines the configuration of the SeedExtensionsCheck controller.
	SeedExtensionsCheck *SeedExtensionsCheckControllerConfiguration
	// SeedBackupBucketsCheck defines the configuration of the SeedBackupBucketsCheck controller.
	SeedBackupBucketsCheck *SeedBackupBucketsCheckControllerConfiguration
	// ShootMaintenance defines the configuration of the ShootMaintenance controller.
	ShootMaintenance ShootMaintenanceControllerConfiguration
	// ShootQuota defines the configuration of the ShootQuota controller.
	ShootQuota *ShootQuotaControllerConfiguration
	// ShootHibernation defines the configuration of the ShootHibernation controller.
	ShootHibernation ShootHibernationControllerConfiguration
	// ShootReference defines the configuration of the ShootReference controller. If unspecified, it is defaulted with `concurrentSyncs=5`.
	ShootReference *ShootReferenceControllerConfiguration
	// ShootRetry defines the configuration of the ShootRetry controller. If unspecified, it is defaulted with `concurrentSyncs=5`.
	ShootRetry *ShootRetryControllerConfiguration
	// ShootConditions defines the configuration of the ShootConditions controller. If unspecified, it is defaulted with `concurrentSyncs=5`.
	ShootConditions *ShootConditionsControllerConfiguration
	// ShootStatusLabel defines the configuration of the ShootStatusLabel controller.
	ShootStatusLabel *ShootStatusLabelControllerConfiguration
	// ManagedSeedSet defines the configuration of the ManagedSeedSet controller.
	ManagedSeedSet *ManagedSeedSetControllerConfiguration
}

// BastionControllerConfiguration defines the configuration of the Bastion
// controller.
type BastionControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// MaxLifetime is the maximum time a Bastion resource can exist before it is
	// forcefully deleted (defaults to '24h').
	MaxLifetime *metav1.Duration
}

// CertificateSigningRequestControllerConfiguration defines the configuration of the CertificateSigningRequest
// controller.
type CertificateSigningRequestControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// CloudProfileControllerConfiguration defines the configuration of the CloudProfile
// controller.
type CloudProfileControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// NamespacedCloudProfileControllerConfiguration defines the configuration of the NamespacedCloudProfile
// controller.
type NamespacedCloudProfileControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// ControllerDeploymentControllerConfiguration defines the configuration of the
// ControllerDeployment controller.
type ControllerDeploymentControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// ControllerRegistrationControllerConfiguration defines the configuration of the
// ControllerRegistration controller.
type ControllerRegistrationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// EventControllerConfiguration defines the configuration of the Event controller.
type EventControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// TTLNonShootEvents is the time-to-live for all non-shoot related events (defaults to `1h`).
	TTLNonShootEvents *metav1.Duration
}

// ExposureClassControllerConfiguration defines the configuration of the
// ExposureClass controller.
type ExposureClassControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// ProjectControllerConfiguration defines the configuration of the
// Project controller.
type ProjectControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// MinimumLifetimeDays is the number of days a `Project` may exist before it is being
	// checked whether it is actively used or got stale.
	MinimumLifetimeDays *int
	// Quotas is the default configuration matching projects are set up with if a quota is not already specified.
	Quotas []QuotaConfiguration
	// StaleGracePeriodDays is the number of days a `Project` may be unused/stale before a
	// timestamp for an auto deletion is computed.
	StaleGracePeriodDays *int
	// StaleExpirationTimeDays is the number of days after a `Project` that has been marked as
	// 'stale'/'unused' and passed the 'stale grace period' will be considered for auto deletion.
	StaleExpirationTimeDays *int
	// StaleSyncPeriod is the duration how often the reconciliation loop for stale Projects is executed.
	StaleSyncPeriod *metav1.Duration
}

// QuotaConfiguration defines quota configurations.
type QuotaConfiguration struct {
	// Config is the quota specification used for the project set-up.
	// Only v1.ResourceQuota resources are supported.
	Config runtime.Object
	// ProjectSelector is an optional setting to select the projects considered for quotas.
	// Defaults to empty LabelSelector, which matches all projects.
	ProjectSelector *metav1.LabelSelector
}

// QuotaControllerConfiguration defines the configuration of the Quota controller.
type QuotaControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// SecretBindingControllerConfiguration defines the configuration of the
// SecretBinding controller.
type SecretBindingControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// CredentialsBindingControllerConfiguration defines the configuration of the
// CredentialsBinding controller.
type CredentialsBindingControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// SeedControllerConfiguration defines the configuration of the
// Seed controller.
type SeedControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// MonitorPeriod is the duration after the seed controller will mark the `GardenletReady`
	// condition in `Seed` resources as `Unknown` in case the gardenlet did not send heartbeats.
	MonitorPeriod *metav1.Duration
	// ShootMonitorPeriod is the duration after the seed controller will mark Gardener's conditions
	// in `Shoot` resources as `Unknown` in case the gardenlet of the responsible seed cluster did
	// not send heartbeats.
	ShootMonitorPeriod *metav1.Duration
	// SyncPeriod is the duration how often the seed controller will check for active gardenlet hearbeats.
	SyncPeriod *metav1.Duration
}

// SeedExtensionsCheckControllerConfiguration defines the configuration of the SeedExtensionsCheck
// controller.
type SeedExtensionsCheckControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of Seed Extensions is performed).
	SyncPeriod *metav1.Duration
	// ConditionThresholds defines the condition threshold per condition type.
	ConditionThresholds []ConditionThreshold
}

// SeedBackupBucketsCheckControllerConfiguration defines the configuration of the
// SeedBackupBucketsCheck controller.
type SeedBackupBucketsCheckControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of BackupBuckets is performed).
	SyncPeriod *metav1.Duration
	// ConditionThresholds defines the condition threshold per condition type.
	ConditionThresholds []ConditionThreshold
}

// ShootMaintenanceControllerConfiguration defines the configuration of the
// ShootMaintenance controller.
type ShootMaintenanceControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// EnableShootControlPlaneRestarter configures whether adequate pods of the shoot control plane are restarted during maintenance.
	EnableShootControlPlaneRestarter *bool
	// EnableShootCoreAddonRestarter configures whether some core addons to be restarted during maintenance.
	EnableShootCoreAddonRestarter *bool
}

// ShootQuotaControllerConfiguration defines the configuration of the
// ShootQuota controller.
type ShootQuotaControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the existing resources are reconciled
	// (how often Shoots referenced Quota is checked).
	SyncPeriod *metav1.Duration
}

// ShootHibernationControllerConfiguration defines the configuration of the
// ShootHibernation controller.
type ShootHibernationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// TriggerDeadlineDuration is an optional deadline for triggering hibernation if scheduled
	// time is missed for any reason (defaults to '2h').
	TriggerDeadlineDuration *metav1.Duration
}

// ShootReferenceControllerConfiguration defines the configuration of the
// ShootReference controller.
type ShootReferenceControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// shoots.
	ConcurrentSyncs *int
}

// ShootRetryControllerConfiguration defines the configuration of the
// ShootRetry controller.
type ShootRetryControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// RetryPeriod is the retry period for retrying failed Shoots that match certain criterion.
	RetryPeriod *metav1.Duration
	// RetryJitterPeriod is a jitter duration for the reconciler retry that can be used to distribute the retries randomly.
	// If its value is greater than 0 then the shoot will not be retried with the configured retry period but a random
	// duration between 0 and the configured value will be added. It is defaulted to 5m.
	RetryJitterPeriod *metav1.Duration
}

// ShootConditionsControllerConfiguration defines the configuration of the
// ShootConditions controller.
type ShootConditionsControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// ShootStatusLabelControllerConfiguration defines the configuration of the
// ShootStatusLabel controller.
type ShootStatusLabelControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// ManagedSeedSetControllerConfiguration defines the configuration of the
// ManagedSeedSet controller.
type ManagedSeedSetControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// MaxShootRetries is the maximum number of times to retry failed shoots before giving up. Defaults to 3.
	MaxShootRetries *int
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration
}

// ConditionThreshold defines the duration how long a flappy condition stays in progressing state.
type ConditionThreshold struct {
	// Type is the type of the condition to define the threshold for.
	Type string
	// Duration is the duration how long the condition can stay in the progressing state.
	Duration metav1.Duration
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
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
