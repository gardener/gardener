// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
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
	// +optional
	LeaderElection *componentbaseconfigv1alpha1.LeaderElectionConfiguration `json:"leaderElection,omitempty"`
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string `json:"logLevel"`
	// LogFormat is the output format for the logs. Must be one of [text,json].
	LogFormat string `json:"logFormat"`
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
	// CertificateSigningRequest defines the configuration of the CertificateSigningRequest controller.
	// +optional
	CertificateSigningRequest *CertificateSigningRequestControllerConfiguration `json:"certificateSigningRequest,omitempty"`
	// CloudProfile defines the configuration of the CloudProfile controller.
	// +optional
	CloudProfile *CloudProfileControllerConfiguration `json:"cloudProfile,omitempty"`
	// NamespacedCloudProfile defines the configuration of the NamespacedCloudProfile controller.
	// +optional
	NamespacedCloudProfile *NamespacedCloudProfileControllerConfiguration `json:"namespacedCloudProfile,omitempty"`
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
	// Project defines the configuration of the Project controller.
	// +optional
	Project *ProjectControllerConfiguration `json:"project,omitempty"`
	// Quota defines the configuration of the Quota controller.
	// +optional
	Quota *QuotaControllerConfiguration `json:"quota,omitempty"`
	// SecretBinding defines the configuration of the SecretBinding controller.
	// +optional
	SecretBinding *SecretBindingControllerConfiguration `json:"secretBinding,omitempty"`
	// CredentialsBinding defines the configuration of the CredentialsBinding controller.
	// +optional
	CredentialsBinding *CredentialsBindingControllerConfiguration `json:"credentialsBinding,omitempty"`
	// Seed defines the configuration of the Seed lifecycle controller.
	// +optional
	Seed *SeedControllerConfiguration `json:"seed,omitempty"`
	// SeedExtensionsCheck defines the configuration of the SeedExtensionsCheck controller.
	// +optional
	SeedExtensionsCheck *SeedExtensionsCheckControllerConfiguration `json:"seedExtensionsCheck,omitempty"`
	// SeedBackupBucketsCheck defines the configuration of the SeedBackupBucketsCheck controller.
	// +optional
	SeedBackupBucketsCheck *SeedBackupBucketsCheckControllerConfiguration `json:"seedBackupBucketsCheck,omitempty"`
	// ShootMaintenance defines the configuration of the ShootMaintenance controller.
	ShootMaintenance ShootMaintenanceControllerConfiguration `json:"shootMaintenance"`
	// ShootQuota defines the configuration of the ShootQuota controller.
	// +optional
	ShootQuota *ShootQuotaControllerConfiguration `json:"shootQuota,omitempty"`
	// ShootHibernation defines the configuration of the ShootHibernation controller.
	ShootHibernation ShootHibernationControllerConfiguration `json:"shootHibernation"`
	// ShootReference defines the configuration of the ShootReference controller. If unspecified, it is defaulted with `concurrentSyncs=5`.
	// +optional
	ShootReference *ShootReferenceControllerConfiguration `json:"shootReference,omitempty"`
	// ShootRetry defines the configuration of the ShootRetry controller. If unspecified, it is defaulted with `concurrentSyncs=5`.
	// +optional
	ShootRetry *ShootRetryControllerConfiguration `json:"shootRetry,omitempty"`
	// ShootConditions defines the configuration of the ShootConditions controller. If unspecified, it is defaulted with `concurrentSyncs=5`.
	// +optional
	ShootConditions *ShootConditionsControllerConfiguration `json:"shootConditions,omitempty"`
	// ShootStatusLabel defines the configuration of the ShootStatusLabel controller.
	// +optional
	ShootStatusLabel *ShootStatusLabelControllerConfiguration `json:"shootStatusLabel,omitempty"`
	// ShootMigration defines the configuration of the ShootMigration controller. If unspecified, it is defaulted with `concurrentSyncs=5`.
	// +optional
	ShootMigration *ShootMigrationControllerConfiguration `json:"shootMigration,omitempty"`
	// ManagedSeedSet defines the configuration of the ManagedSeedSet controller.
	// +optional
	ManagedSeedSet *ManagedSeedSetControllerConfiguration `json:"managedSeedSet,omitempty"`
	// ShootState defines the configuration of the ShootState finalizer controller.
	// +optional
	ShootState *ShootStateControllerConfiguration `json:"shootState,omitempty"`
}

// BastionControllerConfiguration defines the configuration of the Bastion
// controller.
type BastionControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// MaxLifetime is the maximum time a Bastion resource can exist before it is
	// forcefully deleted (defaults to '24h').
	// +optional
	MaxLifetime *metav1.Duration `json:"maxLifetime,omitempty"`
}

// CertificateSigningRequestControllerConfiguration defines the configuration of the CertificateSigningRequest
// controller.
type CertificateSigningRequestControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// CloudProfileControllerConfiguration defines the configuration of the CloudProfile
// controller.
type CloudProfileControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// NamespacedCloudProfileControllerConfiguration defines the configuration of the NamespacedCloudProfile
// controller.
type NamespacedCloudProfileControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ControllerDeploymentControllerConfiguration defines the configuration of the
// ControllerDeployment controller.
type ControllerDeploymentControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ControllerRegistrationControllerConfiguration defines the configuration of the
// ControllerRegistration controller.
type ControllerRegistrationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// EventControllerConfiguration defines the configuration of the Event controller.
type EventControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// TTLNonShootEvents is the time-to-live for all non-shoot related events (defaults to `1h`).
	// +optional
	TTLNonShootEvents *metav1.Duration `json:"ttlNonShootEvents,omitempty"`
}

// ExposureClassControllerConfiguration defines the configuration of the
// ExposureClass controller.
type ExposureClassControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ProjectControllerConfiguration defines the configuration of the
// Project controller.
type ProjectControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
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
	// Config is the corev1.ResourceQuota specification used for the project set-up.
	Config corev1.ResourceQuota `json:"config"`
	// ProjectSelector is an optional setting to select the projects considered for quotas.
	// Defaults to empty LabelSelector, which matches all projects.
	// +optional
	ProjectSelector *metav1.LabelSelector `json:"projectSelector,omitempty"`
}

// QuotaControllerConfiguration defines the configuration of the Quota controller.
type QuotaControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// SecretBindingControllerConfiguration defines the configuration of the
// SecretBinding controller.
type SecretBindingControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// CredentialsBindingControllerConfiguration defines the configuration of the
// CredentialsBinding controller.
type CredentialsBindingControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// SeedControllerConfiguration defines the configuration of the
// Seed controller.
type SeedControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// MonitorPeriod is the duration after the seed controller will mark the `GardenletReady`
	// condition in `Seed` resources as `Unknown` in case the gardenlet did not send heartbeats.
	// +optional
	MonitorPeriod *metav1.Duration `json:"monitorPeriod,omitempty"`
	// ShootMonitorPeriod is the duration after the seed controller will mark Gardener's conditions
	// in `Shoot` resources as `Unknown` in case the gardenlet of the responsible seed cluster did
	// not send heartbeats.
	// +optional
	ShootMonitorPeriod *metav1.Duration `json:"shootMonitorPeriod,omitempty"`
	// SyncPeriod is the duration how often the seed controller will check for active gardenlet hearbeats.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
}

// SeedExtensionsCheckControllerConfiguration defines the configuration of the SeedExtensionsCheck
// controller.
type SeedExtensionsCheckControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of Seed Extensions is performed).
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// ConditionThresholds defines the condition threshold per condition type.
	// +optional
	ConditionThresholds []ConditionThreshold `json:"conditionThresholds,omitempty"`
}

// SeedBackupBucketsCheckControllerConfiguration defines the configuration of the SeedBackupBucketsCheck
// controller.
type SeedBackupBucketsCheckControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of BackupBuckets is performed).
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// ConditionThresholds defines the condition threshold per condition type.
	// +optional
	ConditionThresholds []ConditionThreshold `json:"conditionThresholds,omitempty"`
}

// ShootMaintenanceControllerConfiguration defines the configuration of the
// ShootMaintenance controller.
type ShootMaintenanceControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
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
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled
	// (how often Shoots referenced Quota is checked).
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
}

// ShootHibernationControllerConfiguration defines the configuration of the
// ShootHibernation controller.
type ShootHibernationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// TriggerDeadlineDuration is an optional deadline for triggering hibernation if scheduled
	// time is missed for any reason (defaults to '2h').
	// +optional
	TriggerDeadlineDuration *metav1.Duration `json:"triggerDeadlineDuration,omitempty"`
}

// ShootReferenceControllerConfiguration defines the configuration of the
// ShootReference controller.
type ShootReferenceControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// shoots.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ShootRetryControllerConfiguration defines the configuration of the
// ShootRetry controller.
type ShootRetryControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// RetryPeriod is the retry period for retrying failed Shoots that match certain criterion.
	// Defaults to 10m.
	// +optional
	RetryPeriod *metav1.Duration `json:"retryPeriod,omitempty"`
	// RetryJitterPeriod is a jitter duration for the reconciler retry that can be used to distribute the retries randomly.
	// If its value is greater than 0 then the shoot will not be retried with the configured retry period but a random
	// duration between 0 and the configured value will be added. It is defaulted to 5m.
	// +optional
	RetryJitterPeriod *metav1.Duration `json:"retryJitterPeriod,omitempty"`
}

// ShootConditionsControllerConfiguration defines the configuration of the
// ShootConditions controller.
type ShootConditionsControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ShootStatusLabelControllerConfiguration defines the configuration of the
// ShootStatusLabel controller.
type ShootStatusLabelControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ShootMigrationControllerConfiguration defines the configuration of the
// ShootMigration controller.
type ShootMigrationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ManagedSeedSetControllerConfiguration defines the configuration of the
// ManagedSeedSet controller.
type ManagedSeedSetControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// MaxShootRetries is the maximum number of times to retry failed shoots before giving up. Defaults to 3.
	// +optional
	MaxShootRetries *int `json:"maxShootRetries,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod metav1.Duration `json:"syncPeriod"`
}

// ShootStateControllerConfiguration defines the configuration of the
// ShootState finalizer controller.
type ShootStateControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ConditionThreshold defines the duration how long a flappy condition stays in progressing state.
type ConditionThreshold struct {
	// Type is the type of the condition to define the threshold for.
	Type string `json:"type"`
	// Duration is the duration how long the condition can stay in the progressing state.
	Duration metav1.Duration `json:"duration"`
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

const (
	// ControllerManagerDefaultLockObjectNamespace is the default lock namespace for leader election.
	ControllerManagerDefaultLockObjectNamespace = "garden"

	// ControllerManagerDefaultLockObjectName is the default lock name for leader election.
	ControllerManagerDefaultLockObjectName = "gardener-controller-manager-leader-election"

	// DefaultControllerConcurrentSyncs is a default value for concurrent syncs for controllers.
	DefaultControllerConcurrentSyncs = 5

	// LogLevelDebug is the debug log level, i.e. the most verbose.
	LogLevelDebug = "debug"
	// LogLevelInfo is the default log level.
	LogLevelInfo = "info"
	// LogLevelError is a log level where only errors are logged.
	LogLevelError = "error"

	// LogFormatJSON is the output type that produces a JSON object per log line.
	LogFormatJSON = "json"
	// LogFormatText outputs the log as human-readable text.
	LogFormatText = "text"
)
