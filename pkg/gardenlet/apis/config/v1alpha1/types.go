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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GardenletConfiguration defines the configuration for the Gardenlet.
type GardenletConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// GardenClientConnection specifies the kubeconfig file and the client connection settings
	// for the proxy server to use when communicating with the garden apiserver.
	// +optional
	GardenClientConnection *GardenClientConnection `json:"gardenClientConnection,omitempty"`
	// SeedClientConnection specifies the client connection settings for the proxy server
	// to use when communicating with the seed apiserver.
	// +optional
	SeedClientConnection *SeedClientConnection `json:"seedClientConnection,omitempty"`
	// ShootClientConnection specifies the client connection settings for the proxy server
	// to use when communicating with the shoot apiserver.
	// +optional
	ShootClientConnection *ShootClientConnection `json:"shootClientConnection,omitempty"`
	// Controllers defines the configuration of the controllers.
	// +optional
	Controllers *GardenletControllerConfiguration `json:"controllers,omitempty"`
	// Resources defines the total capacity for seed resources and the amount reserved for use by Gardener.
	// +optional
	Resources *ResourcesConfiguration `json:"resources,omitempty"`
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
	// "github.com/gardener/gardener/pkg/gardenlet/features/features.go".
	// Default: nil
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
	// SeedConfig contains configuration for the seed cluster.
	// +optional
	SeedConfig *SeedConfig `json:"seedConfig,omitempty"`
	// Logging contains an optional configurations for the logging stack deployed
	// by the Gardenlet in the seed clusters.
	// +optional
	Logging *Logging `json:"logging,omitempty"`
	// SNI contains an optional configuration for the APIServerSNI feature used
	// by the Gardenlet in the seed clusters.
	// +optional
	SNI *SNI `json:"sni,omitempty"`
	// ETCDConfig contains an optional configuration for the
	// backup compaction feature in etcdbr
	// +optional
	ETCDConfig *ETCDConfig `json:"etcdConfig,omitempty"`
	// ExposureClassHandlers is a list of optional of exposure class handlers.
	// +optional
	ExposureClassHandlers []ExposureClassHandler `json:"exposureClassHandlers,omitempty"`
	// MonitoringConfig is optional and adds additional settings for the monitoring stack.
	// +optional
	Monitoring *MonitoringConfig `json:"monitoring,omitempty"`
}

// GardenClientConnection specifies the kubeconfig file and the client connection settings
// for the proxy server to use when communicating with the garden apiserver.
type GardenClientConnection struct {
	componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:",inline"`
	// GardenClusterAddress is the external address that the gardenlets can use to remotely connect to the Garden
	// cluster. It is needed in case the gardenlet deploys itself into ManagedSeeds.
	// +optional
	GardenClusterAddress *string `json:"gardenClusterAddress,omitempty"`
	// GardenClusterCACert is the external address that the gardenlets can use to remotely connect to the Garden
	// cluster. It is needed in case the gardenlet deploys itself into ManagedSeeds.
	// +optional
	GardenClusterCACert []byte `json:"gardenClusterCACert,omitempty"`
	// BootstrapKubeconfig is a reference to a secret that contains a data key 'kubeconfig' whose value
	// is a kubeconfig that can be used for bootstrapping. If `kubeconfig` is given then only this kubeconfig
	// will be considered.
	// +optional
	BootstrapKubeconfig *corev1.SecretReference `json:"bootstrapKubeconfig,omitempty"`
	// KubeconfigSecret is the reference to a secret object that stores the gardenlet's kubeconfig that
	// it uses to communicate with the garden cluster. If `kubeconfig` is given then only this kubeconfig
	// will be considered.
	// +optional
	KubeconfigSecret *corev1.SecretReference `json:"kubeconfigSecret,omitempty"`
	// KubeconfigValidity allows configuring certain settings related to the validity and rotation of kubeconfig
	// secrets.
	// +optional
	KubeconfigValidity *KubeconfigValidity `json:"kubeconfigValidity,omitempty"`
}

// KubeconfigValidity allows configuring certain settings related to the validity and rotation of kubeconfig secrets.
type KubeconfigValidity struct {
	// Validity specifies the validity time for the client certificate issued by gardenlet. It will be set as
	// .spec.expirationSeconds in the created CertificateSigningRequest resource.
	// This value is not defaulted, meaning that the value configured via `--cluster-signing-duration` on
	// kube-controller-manager is used.
	// Note that using this value will only have effect for garden clusters >= Kubernetes 1.22.
	// Note that changing this value will only have effect after the next rotation of the gardenlet's kubeconfig secret.
	// +optional
	Validity *metav1.Duration `json:"validity,omitempty"`
	// AutoRotationJitterPercentageMin is the minimum percentage when it comes to compute a random jitter value for the
	// automatic rotation deadline of expiring certificates. Defaults to 70. This means that gardenlet will renew its
	// client certificate when 70% of its lifetime is reached the earliest.
	// +optional
	AutoRotationJitterPercentageMin *int32 `json:"autoRotationJitterPercentageMin,omitempty"`
	// AutoRotationJitterPercentageMax is the maximum percentage when it comes to compute a random jitter value for the
	// automatic rotation deadline of expiring certificates. Defaults to 90. This means that gardenlet will renew its
	// client certificate when 90% of its lifetime is reached at the latest.
	// +optional
	AutoRotationJitterPercentageMax *int32 `json:"autoRotationJitterPercentageMax,omitempty"`
}

// SeedClientConnection specifies the client connection settings
// for the proxy server to use when communicating with the seed apiserver.
type SeedClientConnection struct {
	componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:",inline"`
}

// ShootClientConnection specifies the client connection settings
// for the proxy server to use when communicating with the shoot apiserver.
type ShootClientConnection struct {
	componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:",inline"`
}

// GardenletControllerConfiguration defines the configuration of the controllers.
type GardenletControllerConfiguration struct {
	// BackupBucket defines the configuration of the BackupBucket controller.
	// +optional
	BackupBucket *BackupBucketControllerConfiguration `json:"backupBucket,omitempty"`
	// BackupEntry defines the configuration of the BackupEntry controller.
	// +optional
	BackupEntry *BackupEntryControllerConfiguration `json:"backupEntry,omitempty"`
	// BackupEntryMigration defines the configuration of the BackupEntryMigration controller.
	// +optional
	BackupEntryMigration *BackupEntryMigrationControllerConfiguration `json:"backupEntryMigration,omitempty"`
	// Bastion defines the configuration of the Bastion controller.
	// +optional
	Bastion *BastionControllerConfiguration `json:"bastion,omitempty"`
	// ControllerInstallation defines the configuration of the ControllerInstallation controller.
	// +optional
	ControllerInstallation *ControllerInstallationControllerConfiguration `json:"controllerInstallation,omitempty"`
	// ControllerInstallationCare defines the configuration of the ControllerInstallationCare controller.
	// +optional
	ControllerInstallationCare *ControllerInstallationCareControllerConfiguration `json:"controllerInstallationCare,omitempty"`
	// ControllerInstallationRequired defines the configuration of the ControllerInstallationRequired controller.
	// +optional
	ControllerInstallationRequired *ControllerInstallationRequiredControllerConfiguration `json:"controllerInstallationRequired,omitempty"`
	// Seed defines the configuration of the Seed controller.
	// +optional
	Seed *SeedControllerConfiguration `json:"seed,omitempty"`
	// SeedCare defines the configuration of the SeedCare controller.
	// +optional
	SeedCare *SeedCareControllerConfiguration `json:"seedCare,omitempty"`
	// Shoot defines the configuration of the Shoot controller.
	// +optional
	Shoot *ShootControllerConfiguration `json:"shoot,omitempty"`
	// ShootCare defines the configuration of the ShootCare controller.
	// +optional
	ShootCare *ShootCareControllerConfiguration `json:"shootCare,omitempty"`
	// ShootMigration defines the configuration of the ShootMigration controller.
	// +optional
	ShootMigration *ShootMigrationControllerConfiguration `json:"shootMigration,omitempty"`
	// ShootStateSync defines the configuration of the ShootState controller
	// +optional
	ShootStateSync *ShootStateSyncControllerConfiguration `json:"shootStateSync,omitempty"`
	// SeedAPIServerNetworkPolicy defines the configuration of the SeedAPIServerNetworkPolicy controller
	// +optional
	SeedAPIServerNetworkPolicy *SeedAPIServerNetworkPolicyControllerConfiguration `json:"seedAPIServerNetworkPolicy,omitempty"`
	// ManagedSeedControllerConfiguration defines the configuration of the ManagedSeed controller.
	// +optional
	ManagedSeed *ManagedSeedControllerConfiguration `json:"managedSeed,omitempty"`
	// ShootSecretControllerConfiguration defines the configuration of the ShootSecret controller.
	// +optional
	ShootSecret *ShootSecretControllerConfiguration `json:"shootSecret,omitempty"`
}

// BackupBucketControllerConfiguration defines the configuration of the BackupBucket
// controller.
type BackupBucketControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// BackupEntryControllerConfiguration defines the configuration of the BackupEntry
// controller.
type BackupEntryControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// DeletionGracePeriodHours holds the period in number of hours to delete the BackupEntry after deletion timestamp is set.
	// If value is set to 0 then the BackupEntryController will trigger deletion immediately.
	// +optional
	DeletionGracePeriodHours *int `json:"deletionGracePeriodHours,omitempty"`
	// DeletionGracePeriodShootPurposes is a list of shoot purposes for which the deletion grace period applies. All
	// BackupEntries corresponding to Shoots with different purposes will be deleted immediately.
	// +optional
	DeletionGracePeriodShootPurposes []gardencorev1beta1.ShootPurpose `json:"deletionGracePeriodShootPurposes,omitempty"`
}

// BackupEntryMigrationControllerConfiguration defines the configuration of the BackupEntryMigration
// controller.
type BackupEntryMigrationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	// It is only relevant for backup entries that are currently being migrated.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// GracePeriod is the period to wait before forcing the restoration after the migration has started.
	// +optional
	GracePeriod *metav1.Duration `json:"gracePeriod,omitempty"`
	// LastOperationStaleDuration is the duration to consider the last operation stale after it was last updated.
	// +optional
	LastOperationStaleDuration *metav1.Duration `json:"lastOperationStaleDuration,omitempty"`
}

// BastionControllerConfiguration defines the configuration of the Bastion
// controller.
type BastionControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ControllerInstallationControllerConfiguration defines the configuration of the
// ControllerInstallation controller.
type ControllerInstallationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ControllerInstallationCareControllerConfiguration defines the configuration of the ControllerInstallationCare
// controller.
type ControllerInstallationCareControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of ControllerInstallations is performed.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
}

// ControllerInstallationRequiredControllerConfiguration defines the configuration of the ControllerInstallationRequired
// controller.
type ControllerInstallationRequiredControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// SeedControllerConfiguration defines the configuration of the Seed controller.
type SeedControllerConfiguration struct {
	// SyncPeriod is the duration how often the existing resources are reconciled.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// LeaseResyncSeconds defines how often (in seconds) the seed lease is renewed.
	// Defaults to 2
	// +optional
	LeaseResyncSeconds *int32 `json:"leaseResyncSeconds,omitempty"`
	// LeaseResyncMissThreshold is the amount of missed lease resyncs before the health status
	// is changed to false.
	// Defaults to 10
	// +optional
	LeaseResyncMissThreshold *int32 `json:"leaseResyncMissThreshold,omitempty"`
}

// ShootControllerConfiguration defines the configuration of the Shoot
// controller.
type ShootControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// ProgressReportPeriod is the period how often the progress of a shoot operation will be reported in the
	// Shoot's `.status.lastOperation` field. By default, the progress will be reported immediately after a task of the
	// respective flow has been completed. If you set this to a value > 0 (e.g., 5s) then it will be only reported every
	// 5 seconds. Any tasks that were completed in the meantime will not be reported.
	// +optional
	ProgressReportPeriod *metav1.Duration `json:"progressReportPeriod,omitempty"`
	// ReconcileInMaintenanceOnly determines whether Shoot reconciliations happen only
	// during its maintenance time window.
	// +optional
	ReconcileInMaintenanceOnly *bool `json:"reconcileInMaintenanceOnly,omitempty"`
	// RespectSyncPeriodOverwrite determines whether a sync period overwrite of a
	// Shoot (via annotation) is respected or not. Defaults to false.
	// +optional
	RespectSyncPeriodOverwrite *bool `json:"respectSyncPeriodOverwrite,omitempty"`
	// RetryDuration is the maximum duration how often a reconciliation will be retried
	// in case of errors.
	// +optional
	RetryDuration *metav1.Duration `json:"retryDuration,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// DNSEntryTTLSeconds is the TTL in seconds that is being used for DNS entries when reconciling shoots.
	// Default: 120s
	// +optional
	DNSEntryTTLSeconds *int64 `json:"dnsEntryTTLSeconds,omitempty"`
}

// ShootCareControllerConfiguration defines the configuration of the ShootCare
// controller.
type ShootCareControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of Shoot clusters is performed (only if no operation is
	// already running on them).
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// StaleExtensionHealthChecks defines the configuration of the check for stale extension health checks.
	// +optional
	StaleExtensionHealthChecks *StaleExtensionHealthChecks `json:"staleExtensionHealthChecks,omitempty"`
	// ConditionThresholds defines the condition threshold per condition type.
	// +optional
	ConditionThresholds []ConditionThreshold `json:"conditionThresholds,omitempty"`
	// WebhookRemediatorEnabled specifies whether the remediator for webhooks not following the Kubernetes best
	// practices (https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#best-practices-and-warnings)
	// is enabled.
	// +optional
	WebhookRemediatorEnabled *bool `json:"webhookRemediatorEnabled,omitempty"`
}

// SeedCareControllerConfiguration defines the configuration of the SeedCare
// controller.
type SeedCareControllerConfiguration struct {
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of Seed clusters is performed
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// ConditionThresholds defines the condition threshold per condition type.
	// +optional
	ConditionThresholds []ConditionThreshold `json:"conditionThresholds,omitempty"`
}

// ShootMigrationControllerConfiguration defines the configuration of the ShootMigration
// controller.
type ShootMigrationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// GracePeriod is the period to wait before forcing the restoration after the migration has started.
	// It is only relevant for shoots that are currently being migrated.
	// +optional
	GracePeriod *metav1.Duration `json:"gracePeriod,omitempty"`
	// LastOperationStaleDuration is the duration to consider the last operation stale after it was last updated.
	// +optional
	LastOperationStaleDuration *metav1.Duration `json:"lastOperationStaleDuration,omitempty"`
}

// ShootSecretControllerConfiguration defines the configuration of the ShootSecret controller.
type ShootSecretControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// StaleExtensionHealthChecks defines the configuration of the check for stale extension health checks.
type StaleExtensionHealthChecks struct {
	// Enabled specifies whether the check for stale extensions health checks is enabled.
	// Defaults to true.
	Enabled bool `json:"enabled"`
	// Threshold configures the threshold when gardenlet considers a health check report of an extension CRD as outdated.
	// The threshold should have some leeway in case a Gardener extension is temporarily unavailable.
	// Defaults to 5m.
	// +optional
	Threshold *metav1.Duration `json:"threshold,omitempty"`
}

// ConditionThreshold defines the duration how long a flappy condition stays in progressing state.
type ConditionThreshold struct {
	// Type is the type of the condition to define the threshold for.
	Type string `json:"type"`
	// Duration is the duration how long the condition can stay in the progressing state.
	Duration metav1.Duration `json:"duration"`
}

// ShootStateSyncControllerConfiguration defines the configuration of the ShootState Sync controller.
type ShootStateSyncControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// SeedAPIServerNetworkPolicyControllerConfiguration defines the configuration of the SeedAPIServerNetworkPolicy
// controller.
type SeedAPIServerNetworkPolicyControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// ManagedSeedControllerConfiguration defines the configuration of the ManagedSeed controller.
type ManagedSeedControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// WaitSyncPeriod is the duration how often an existing resource is reconciled when the controller is waiting for an event.
	// +optional
	WaitSyncPeriod *metav1.Duration `json:"waitSyncPeriod,omitempty"`
	// SyncJitterPeriod is a jitter duration for the reconciler sync that can be used to distribute the syncs randomly.
	// If its value is greater than 0 then the managed seeds will not be enqueued immediately but only after a random
	// duration between 0 and the configured value. It is defaulted to 5m.
	// +optional
	SyncJitterPeriod *metav1.Duration `json:"syncJitterPeriod,omitempty"`
	// JitterUpdates enables enqueuing managed seeds with a random duration(jitter) in case of an update to the spec.
	// The applied jitterPeriod is taken from SyncJitterPeriod.
	// +optional
	JitterUpdates *bool `json:"jitterUpdates,omitempty"`
}

// ResourcesConfiguration defines the total capacity for seed resources and the amount reserved for use by Gardener.
type ResourcesConfiguration struct {
	// Capacity defines the total resources of a seed.
	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty"`
	// Reserved defines the resources of a seed that are reserved for use by Gardener.
	// Defaults to 0.
	// +optional
	Reserved corev1.ResourceList `json:"reserved,omitempty"`
}

// SeedConfig contains configuration for the seed cluster.
type SeedConfig struct {
	gardencorev1beta1.SeedTemplate `json:",inline"`
}

// FluentBit contains configuration for Fluent Bit.
type FluentBit struct {
	// ServiceSection defines [SERVICE] configuration for the fluent-bit.
	// If it is nil, fluent-bit uses default service configuration.
	// +optional
	ServiceSection *string `json:"service,omitempty" yaml:"service,omitempty"`
	// InputSection defines [INPUT] configuration for the fluent-bit.
	// If it is nil, fluent-bit uses default input configuration.
	// +optional
	InputSection *string `json:"input,omitempty" yaml:"input,omitempty"`
	// OutputSection defines [OUTPUT] configuration for the fluent-bit.
	// If it is nil, fluent-bit uses default output configuration.
	// +optional
	OutputSection *string `json:"output,omitempty" yaml:"output,omitempty"`
	// NetworkPolicy defines settings for the fluent-bit NetworkPolicy.
	// +optional
	NetworkPolicy *FluentBitNetworkPolicy `json:"networkPolicy,omitempty" yaml:"networkPolicy,omitempty"`
}

// FluentBitNetworkPolicy defines settings for the fluent-bit NetworkPolicy.
type FluentBitNetworkPolicy struct {
	// AdditionalEgressIPBlocks contains IP CIDRs for the egress network policy.
	// +optional
	AdditionalEgressIPBlocks []string `json:"additionalEgressIPBlocks,omitempty" yaml:"additionalEgressIPBlocks,omitempty"`
}

// Loki contains configuration for the Loki.
type Loki struct {
	// Enabled is used to enable or disable the shoot and seed Loki.
	// If FluentBit is used with a custom output the Loki can, Loki is maybe unused and can be disabled.
	// If not set, by default Loki is enabled
	// +optional
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Garden contains configuration for the Loki in garden namespace.
	// +optional
	Garden *GardenLoki `json:"garden,omitempty" yaml:"garden,omitempty"`
}

// GardenLoki contains configuration for the Loki in garden namespace.
type GardenLoki struct {
	// Storage is the disk storage capacity of the central Loki.
	// Defaults to 100Gi.
	// +optional
	Storage *resource.Quantity `json:"storage,omitempty" yaml:"storage,omitempty"`
}

// ShootNodeLogging contains configuration for the shoot node logging.
type ShootNodeLogging struct {
	// ShootPurposes determines which shoots can have node logging by their purpose
	// +optional
	ShootPurposes []gardencorev1beta1.ShootPurpose `json:"shootPurposes,omitempty" yaml:"shootPurposes,omitempty"`
}

// ShootEventLogging contains configurations for the shoot event logger.
type ShootEventLogging struct {
	// Enabled is used to enable or disable shoot event logger.
	// +optional
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// Logging contains configuration for the logging stack.
type Logging struct {
	// Enabled is used to enable or disable logging stack for clusters.
	// +optional
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// FluentBit contains configurations for the fluent-bit
	// +optional
	FluentBit *FluentBit `json:"fluentBit,omitempty" yaml:"fluentBit,omitempty"`
	// Loki contains configuration for the Loki
	// +optional
	Loki *Loki `json:"loki,omitempty" yaml:"loki,omitempty"`
	// ShootNodeLogging contains configurations for the shoot node logging
	// +optional
	ShootNodeLogging *ShootNodeLogging `json:"shootNodeLogging,omitempty" yaml:"shootNodeLogging,omitempty"`
	// ShootEventLogging contains configurations for the shoot event logger.
	// +optional
	ShootEventLogging *ShootEventLogging `json:"shootEventLogging,omitempty" yaml:"shootEventLogging,omitempty"`
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
	// Port is the port on which to serve unsecured, unauthenticated access.
	Port int `json:"port"`
}

// SNI contains an optional configuration for the APIServerSNI feature used
// by the Gardenlet in the seed clusters.
type SNI struct {
	// Ingress is the ingressgateway configuration.
	// +optional
	Ingress *SNIIngress `json:"ingress,omitempty"`
}

// SNIIngress contains configuration of the ingressgateway.
type SNIIngress struct {
	// ServiceName is the name of the ingressgateway Service.
	// Defaults to "istio-ingressgateway".
	// +optional
	ServiceName *string `json:"serviceName,omitempty"`
	// ServiceExternalIP is the external ip which should be assigned to the
	// load balancer service of the ingress gateway.
	// Compatibility is depending on the respective provider cloud-controller-manager.
	// +optional
	ServiceExternalIP *string `json:"serviceExternalIP,omitempty"`
	// Namespace is the namespace in which the ingressgateway is deployed in.
	// Defaults to "istio-ingress".
	// +optional
	Namespace *string `json:"namespace,omitempty"`
	// Labels of the ingressgateway
	// Defaults to "istio: ingressgateway".
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// ETCDConfig contains ETCD related configs
type ETCDConfig struct {
	// ETCDController contains config specific to ETCD controller
	// +optional
	ETCDController *ETCDController `json:"etcdController,omitempty"`
	// CustodianController contains config specific to custodian controller
	// +optional
	CustodianController *CustodianController `json:"custodianController,omitempty"`
	// BackupCompactionController contains config specific to backup compaction controller
	// +optional
	BackupCompactionController *BackupCompactionController `json:"backupCompactionController,omitempty"`
	// BackupLeaderElection contains configuration for the leader election for the etcd backup-restore sidecar.
	// +optional
	BackupLeaderElection *ETCDBackupLeaderElection `json:"backupLeaderElection,omitempty"`
}

// ETCDController contains config specific to ETCD controller
type ETCDController struct {
	// Workers specify number of worker threads in ETCD controller
	// Defaults to 50
	// +optional
	Workers *int64 `json:"workers,omitempty"`
}

// CustodianController contains config specific to custodian controller
type CustodianController struct {
	// Workers specify number of worker threads in custodian controller
	// Defaults to 10
	// +optional
	Workers *int64 `json:"workers,omitempty"`
}

// BackupCompactionController contains config specific to backup compaction controller
type BackupCompactionController struct {
	// Workers specify number of worker threads in backup compaction controller
	// Defaults to 3
	// +optional
	Workers *int64 `json:"workers,omitempty"`
	// EnableBackupCompaction enables automatic compaction of etcd backups
	// Defaults to false
	// +optional
	EnableBackupCompaction *bool `json:"enableBackupCompaction,omitempty"`
	// EventsThreshold defines total number of etcd events that can be allowed before a backup compaction job is triggered
	// Defaults to 1 Million events
	// +optional
	EventsThreshold *int64 `json:"eventsThreshold,omitempty"`
	// ActiveDeadlineDuration defines duration after which a running backup compaction job will be killed
	// Defaults to 3 hours
	// +optional
	ActiveDeadlineDuration *metav1.Duration `json:"activeDeadlineDuration,omitempty"`
}

// ETCDBackupLeaderElection contains configuration for the leader election for the etcd backup-restore sidecar.
type ETCDBackupLeaderElection struct {
	// ReelectionPeriod defines the Period after which leadership status of corresponding etcd is checked.
	// +optional
	ReelectionPeriod *metav1.Duration `json:"reelectionPeriod,omitempty"`
	// EtcdConnectionTimeout defines the timeout duration for etcd client connection during leader election.
	// +optional
	EtcdConnectionTimeout *metav1.Duration `json:"etcdConnectionTimeout,omitempty"`
}

// ExposureClassHandler contains configuration for an exposure class handler.
type ExposureClassHandler struct {
	// Name is the name of the exposure class handler.
	Name string `json:"name"`
	// LoadBalancerService contains configuration which is used to configure the underlying
	// load balancer to apply the control plane endpoint exposure strategy.
	LoadBalancerService LoadBalancerServiceConfig `json:"loadBalancerService"`
	// SNI contains optional configuration for a dedicated ingressgateway belonging to
	// an exposure class handler. This is only required in context of the APIServerSNI feature of the gardenlet.
	// +optional
	SNI *SNI `json:"sni,omitempty"`
}

// LoadBalancerServiceConfig contains configuration which is used to configure the underlying
// load balancer to apply the control plane endpoint exposure strategy.
type LoadBalancerServiceConfig struct {
	// Annotations is a key value map to annotate the underlying load balancer services.
	Annotations map[string]string `json:"annotations"`
}

// MonitoringConfig contains settings for the monitoring stack.
type MonitoringConfig struct {
	// Shoot is optional and contains settings for the shoot monitoring stack.
	// +optional
	Shoot *ShootMonitoringConfig `json:"shoot,omitempty"`
}

// ShootMonitoringConfig contains settings for the shoot monitoring stack.
type ShootMonitoringConfig struct {
	// Enabled is used to enable or disable the shoot monitoring stack.
	// Defaults to true.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// RemoteWrite is optional and contains remote write setting.
	// +optional
	RemoteWrite *RemoteWriteMonitoringConfig `json:"remoteWrite,omitempty"`
	// ExternalLabels is optional and sets additional external labels for the monitoring stack.
	// +optional
	ExternalLabels map[string]string `json:"externalLabels,omitempty"`
}

// RemoteWriteMonitoringConfig contains settings for the remote write setting for monitoring stack.
type RemoteWriteMonitoringConfig struct {
	// URL contains an Url for remote write setting in prometheus.
	URL string `json:"url"`
	// Keep contains a list of metrics that will be remote written
	// +optional
	Keep []string `json:"keep,omitempty"`
	// QueueConfig contains the queue_config for prometheus remote write.
	// +optional
	QueueConfig *string `json:"queueConfig,omitempty"`
}

const (
	// GardenletDefaultLockObjectNamespace is the default lock namespace for leader election.
	GardenletDefaultLockObjectNamespace = "garden"

	// GardenletDefaultLockObjectName is the default lock name for leader election.
	GardenletDefaultLockObjectName = "gardenlet-leader-election"

	// DefaultBackupEntryDeletionGracePeriodHours is a constant for the default number of hours the Backup Entry should be kept after shoot is deleted.
	// By default we set this to 0 so that then BackupEntryController will trigger deletion immediately.
	DefaultBackupEntryDeletionGracePeriodHours = 0

	// DefaultDiscoveryDirName is the name of the default directory used for discovering Kubernetes APIs.
	DefaultDiscoveryDirName = "gardenlet-discovery"

	// DefaultDiscoveryCacheDirName is the name of the default directory used for the discovery cache.
	DefaultDiscoveryCacheDirName = "cache"

	// DefaultDiscoveryHTTPCacheDirName is the name of the default directory used for the discovery HTTP cache.
	DefaultDiscoveryHTTPCacheDirName = "http-cache"

	// DefaultDiscoveryTTL is the default ttl for the cached discovery client.
	DefaultDiscoveryTTL = 10 * time.Second

	// DefaultControllerConcurrentSyncs is a default value for concurrent syncs for controllers.
	DefaultControllerConcurrentSyncs = 20

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

// DefaultControllerSyncPeriod is a default value for sync period for controllers.
var DefaultControllerSyncPeriod = metav1.Duration{Duration: time.Minute}

// DefaultCentralLokiStorage is a default value for garden/loki's storage.
var DefaultCentralLokiStorage = resource.MustParse("100Gi")
