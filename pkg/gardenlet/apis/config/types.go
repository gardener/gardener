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
	gardencore "github.com/gardener/gardener/pkg/apis/core"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GardenletConfiguration defines the configuration for the Gardenlet.
type GardenletConfiguration struct {
	metav1.TypeMeta
	// GardenClientConnection specifies the kubeconfig file and the client connection settings
	// for the proxy server to use when communicating with the garden apiserver.
	GardenClientConnection *GardenClientConnection
	// SeedClientConnection specifies the client connection settings for the proxy server
	// to use when communicating with the seed apiserver.
	SeedClientConnection *SeedClientConnection
	// ShootClientConnection specifies the client connection settings for the proxy server
	// to use when communicating with the shoot apiserver.
	ShootClientConnection *ShootClientConnection
	// Controllers defines the configuration of the controllers.
	Controllers *GardenletControllerConfiguration
	// Resources defines the total capacity for seed resources and the amount reserved for use by Gardener.
	Resources *ResourcesConfiguration
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
	// "github.com/gardener/gardener/pkg/gardenlet/features/features.go".
	// Default: nil
	FeatureGates map[string]bool
	// SeedConfig contains configuration for the seed cluster.
	SeedConfig *SeedConfig
	// Logging contains an optional configurations for the logging stack deployed
	// by the Gardenlet in the seed clusters.
	Logging *Logging
	// SNI contains an optional configuration for the APIServerSNI feature used
	// by the Gardenlet in the seed clusters.
	SNI *SNI
	// ETCDConfig contains an optional configuration for the
	// backup compaction feature of ETCD backup-restore functionality.
	ETCDConfig *ETCDConfig
	// ExposureClassHandlers is a list of optional of exposure class handlers.
	ExposureClassHandlers []ExposureClassHandler
	// MonitoringConfig is optional and adds additional settings for the monitoring stack.
	Monitoring *MonitoringConfig
}

// GardenClientConnection specifies the kubeconfig file and the client connection settings
// for the proxy server to use when communicating with the garden apiserver.
type GardenClientConnection struct {
	componentbaseconfig.ClientConnectionConfiguration
	// GardenClusterAddress is the external address that the gardenlets can use to remotely connect to the Garden
	// cluster. It is needed in case the gardenlet deploys itself into ManagedSeeds.
	GardenClusterAddress *string
	// GardenClusterCACert is the external address that the gardenlets can use to remotely connect to the Garden
	// cluster. It is needed in case the gardenlet deploys itself into ManagedSeeds.
	GardenClusterCACert []byte
	// BootstrapKubeconfig is a reference to a secret that contains a data key 'kubeconfig' whose value
	// is a kubeconfig that can be used for bootstrapping. If `kubeconfig` is given then only this kubeconfig
	// will be considered.
	BootstrapKubeconfig *corev1.SecretReference
	// KubeconfigSecret is the reference to a secret object that stores the gardenlet's kubeconfig that
	// it uses to communicate with the garden cluster. If `kubeconfig` is given then only this kubeconfig
	// will be considered.
	KubeconfigSecret *corev1.SecretReference
	// KubeconfigValidity allows configuring certain settings related to the validity and rotation of kubeconfig
	// secrets.
	KubeconfigValidity *KubeconfigValidity
}

// KubeconfigValidity allows configuring certain settings related to the validity and rotation of kubeconfig secrets.
type KubeconfigValidity struct {
	// Validity specifies the validity time for the client certificate issued by gardenlet. It will be set as
	// .spec.expirationSeconds in the created CertificateSigningRequest resource.
	// This value is not defaulted, meaning that the value configured via `--cluster-signing-duration` on
	// kube-controller-manager is used.
	// Note that using this value will only have effect for garden clusters >= Kubernetes 1.22.
	// Note that changing this value will only have effect after the next rotation of the gardenlet's kubeconfig secret.
	Validity *metav1.Duration
	// AutoRotationJitterPercentageMin is the minimum percentage when it comes to compute a random jitter value for the
	// automatic rotation deadline of expiring certificates. Defaults to 70. This means that gardenlet will renew its
	// client certificate when 70% of its lifetime is reached the earliest.
	AutoRotationJitterPercentageMin *int32
	// AutoRotationJitterPercentageMax is the maximum percentage when it comes to compute a random jitter value for the
	// automatic rotation deadline of expiring certificates. Defaults to 90. This means that gardenlet will renew its
	// client certificate when 90% of its lifetime is reached at the latest.
	AutoRotationJitterPercentageMax *int32
}

// SeedClientConnection specifies the client connection settings
// for the proxy server to use when communicating with the seed apiserver.
type SeedClientConnection struct {
	componentbaseconfig.ClientConnectionConfiguration
}

// ShootClientConnection specifies the client connection settings
// for the proxy server to use when communicating with the shoot apiserver.
type ShootClientConnection struct {
	componentbaseconfig.ClientConnectionConfiguration
}

// GardenletControllerConfiguration defines the configuration of the controllers.
type GardenletControllerConfiguration struct {
	// BackupBucket defines the configuration of the BackupBucket controller.
	BackupBucket *BackupBucketControllerConfiguration
	// BackupEntry defines the configuration of the BackupEntry controller.
	BackupEntry *BackupEntryControllerConfiguration
	// BackupEntryMigration defines the configuration of the BackupEntryMigration controller.
	BackupEntryMigration *BackupEntryMigrationControllerConfiguration
	// Bastion defines the configuration of the Bastion controller.
	Bastion *BastionControllerConfiguration
	// ControllerInstallation defines the configuration of the ControllerInstallation controller.
	ControllerInstallation *ControllerInstallationControllerConfiguration
	// ControllerInstallationCare defines the configuration of the ControllerInstallationCare controller.
	ControllerInstallationCare *ControllerInstallationCareControllerConfiguration
	// ControllerInstallationRequired defines the configuration of the ControllerInstallationRequired controller.
	ControllerInstallationRequired *ControllerInstallationRequiredControllerConfiguration
	// Seed defines the configuration of the Seed controller.
	Seed *SeedControllerConfiguration
	// SeedCare defines the configuration of the SeedCare controller.
	SeedCare *SeedCareControllerConfiguration
	// Shoot defines the configuration of the Shoot controller.
	Shoot *ShootControllerConfiguration
	// ShootCare defines the configuration of the ShootCare controller.
	ShootCare *ShootCareControllerConfiguration
	// ShootMigration defines the configuration of the ShootMigration controller.
	ShootMigration *ShootMigrationControllerConfiguration
	// ShootStateSync defines the configuration of the ShootState controller.
	ShootStateSync *ShootStateSyncControllerConfiguration
	// SeedAPIServerNetworkPolicy defines the configuration of the SeedAPIServerNetworkPolicy controller.
	SeedAPIServerNetworkPolicy *SeedAPIServerNetworkPolicyControllerConfiguration
	// ManagedSeedControllerConfiguration defines the configuration of the ManagedSeed controller.
	ManagedSeed *ManagedSeedControllerConfiguration
	// ShootSecretControllerConfiguration defines the configuration of the ShootSecret controller.
	ShootSecret *ShootSecretControllerConfiguration
}

// BackupBucketControllerConfiguration defines the configuration of the BackupBucket
// controller.
type BackupBucketControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	ConcurrentSyncs *int
}

// BackupEntryControllerConfiguration defines the configuration of the BackupEntry
// controller.
type BackupEntryControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	ConcurrentSyncs *int
	// DeletionGracePeriodHours holds the period in number of hours to delete the BackupEntry after deletion timestamp is set.
	// If value is set to 0 then the BackupEntryController will trigger deletion immediately.
	DeletionGracePeriodHours *int
	// DeletionGracePeriodShootPurposes is a list of shoot purposes for which the deletion grace period applies. All
	// BackupEntries corresponding to Shoots with different purposes will be deleted immediately.
	DeletionGracePeriodShootPurposes []gardencore.ShootPurpose
}

// BackupEntryMigrationControllerConfiguration defines the configuration of the BackupEntryMigration
// controller.
type BackupEntryMigrationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the existing resources are reconciled.
	// It is only relevant for backup entries that are currently being migrated.
	SyncPeriod *metav1.Duration
	// GracePeriod is the period to wait before forcing the restoration after the migration has started.
	GracePeriod *metav1.Duration
	// LastOperationStaleDuration is the duration to consider the last operation stale after it was last updated.
	LastOperationStaleDuration *metav1.Duration
}

// BastionControllerConfiguration defines the configuration of the Bastion
// controller.
type BastionControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	ConcurrentSyncs *int
}

// ControllerInstallationControllerConfiguration defines the configuration of the
// ControllerInstallation controller.
type ControllerInstallationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// ControllerInstallationCareControllerConfiguration defines the configuration of the ControllerInstallationCare
// controller.
type ControllerInstallationCareControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of ControllerInstallations is performed.
	SyncPeriod *metav1.Duration
}

// ControllerInstallationRequiredControllerConfiguration defines the configuration of the ControllerInstallationRequired
// controller.
type ControllerInstallationRequiredControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// SeedControllerConfiguration defines the configuration of the Seed controller.
type SeedControllerConfiguration struct {
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod *metav1.Duration
	// LeaseResyncSeconds defines how often (in seconds) the seed lease is renewed.
	// Default: 2s
	LeaseResyncSeconds *int32
	// LeaseResyncMissThreshold is the amount of missed lease resyncs before the health status
	// is changed to false.
	// Default: 10
	LeaseResyncMissThreshold *int32
}

// ShootControllerConfiguration defines the configuration of the Shoot
// controller.
type ShootControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// ProgressReportPeriod is the period how often the progress of a shoot operation will be reported in the
	// Shoot's `.status.lastOperation` field. By default, the progress will be reported immediately after a task of the
	// respective flow has been completed. If you set this to a value > 0 (e.g., 5s) then it will be only reported every
	// 5 seconds. Any tasks that were completed in the meantime will not be reported.
	ProgressReportPeriod *metav1.Duration
	// ReconcileInMaintenanceOnly determines whether Shoot reconciliations happen only
	// during its maintenance time window.
	ReconcileInMaintenanceOnly *bool
	// RespectSyncPeriodOverwrite determines whether a sync period overwrite of a
	// Shoot (via annotation) is respected or not. Defaults to false.
	RespectSyncPeriodOverwrite *bool
	// RetryDuration is the maximum duration how often a reconciliation will be retried
	// in case of errors.
	RetryDuration *metav1.Duration
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod *metav1.Duration
	// DNSEntryTTLSeconds is the TTL in seconds that is being used for DNS entries when reconciling shoots.
	// Default: 120s
	DNSEntryTTLSeconds *int64
}

// ShootCareControllerConfiguration defines the configuration of the ShootCare
// controller.
type ShootCareControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of Shoot clusters is performed (only if no operation is
	// already running on them).
	SyncPeriod *metav1.Duration
	// StaleExtensionHealthChecks defines the configuration of the check for stale extension health checks.
	StaleExtensionHealthChecks *StaleExtensionHealthChecks
	// ConditionThresholds defines the condition threshold per condition type.
	ConditionThresholds []ConditionThreshold
	// WebhookRemediatorEnabled specifies whether the remediator for webhooks not following the Kubernetes best
	// practices (https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#best-practices-and-warnings)
	// is enabled.
	WebhookRemediatorEnabled *bool
}

// SeedCareControllerConfiguration defines the configuration of the SeedCare
// controller.
type SeedCareControllerConfiguration struct {
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of Seed clusters is performed.
	SyncPeriod *metav1.Duration
	// ConditionThresholds defines the condition threshold per condition type.
	ConditionThresholds []ConditionThreshold
}

// ShootMigrationControllerConfiguration defines the configuration of the ShootMigration
// controller.
type ShootMigrationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the existing resources are reconciled.
	// It is only relevant for shoots that are currently being migrated.
	SyncPeriod *metav1.Duration
	// GracePeriod is the period to wait before forcing the restoration after the migration has started.
	GracePeriod *metav1.Duration
	// LastOperationStaleDuration is the duration to consider the last operation stale after it was last updated.
	LastOperationStaleDuration *metav1.Duration
}

// ShootSecretControllerConfiguration defines the configuration of the ShootSecret controller.
type ShootSecretControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	ConcurrentSyncs *int
}

// StaleExtensionHealthChecks defines the configuration of the check for stale extension health checks.
type StaleExtensionHealthChecks struct {
	// Enabled specifies whether the check for stale extensions health checks is enabled.
	// Defaults to true.
	Enabled bool
	// Threshold configures the threshold when gardenlet considers a health check report of an extension CRD as outdated.
	// The threshold should have some leeway in case a Gardener extension is temporarily unavailable.
	// Defaults to 5m.
	Threshold *metav1.Duration
}

// ConditionThreshold defines the duration how long a flappy condition stays in progressing state.
type ConditionThreshold struct {
	// Type is the type of the condition to define the threshold for.
	Type string
	// Duration is the duration how long the condition can stay in the progressing state.
	Duration metav1.Duration
}

// ShootStateSyncControllerConfiguration defines the configuration of the ShootState Sync controller.
type ShootStateSyncControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
}

// SeedAPIServerNetworkPolicyControllerConfiguration defines the configuration of the SeedAPIServerNetworkPolicy
// controller.
type SeedAPIServerNetworkPolicyControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	ConcurrentSyncs *int
}

// ManagedSeedControllerConfiguration defines the configuration of the ManagedSeed controller.
type ManagedSeedControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod *metav1.Duration
	// WaitSyncPeriod is the duration how often an existing resource is reconciled when the controller is waiting for an event.
	WaitSyncPeriod *metav1.Duration
	// SyncJitterPeriod is a jitter duration for the reconciler sync that can be used to distribute the syncs randomly.
	// If its value is greater than 0 then the managed seeds will not be enqueued immediately but only after a random
	// duration between 0 and the configured value. It is defaulted to 5m.
	SyncJitterPeriod *metav1.Duration
	// JitterUpdates enables enqueuing managed seeds with a random duration(jitter) in case of an update to the spec.
	// The applied jitterPeriod is taken from SyncJitterPeriod.
	// Defaults to false.
	JitterUpdates *bool
}

// ResourcesConfiguration defines the total capacity for seed resources and the amount reserved for use by Gardener.
type ResourcesConfiguration struct {
	// Capacity defines the total resources of a seed.
	Capacity corev1.ResourceList
	// Reserved defines the resources of a seed that are reserved for use by Gardener.
	// Defaults to 0.
	Reserved corev1.ResourceList
}

// SeedConfig contains configuration for the seed cluster.
type SeedConfig struct {
	gardencore.SeedTemplate
}

// FluentBit contains configuration for Fluent Bit.
type FluentBit struct {
	// ServiceSection defines [SERVICE] configuration for the fluent-bit.
	// If it is nil, fluent-bit uses default service configuration.
	ServiceSection *string
	// InputSection defines [INPUT] configuration for the fluent-bit.
	// If it is nil, fluent-bit uses default input configuration.
	InputSection *string
	// OutputSection defines [OUTPUT] configuration for the fluent-bit.
	// If it is nil, fluent-bit uses default output configuration.
	OutputSection *string
	// NetworkPolicy defines settings for the fluent-bit NetworkPolicy.
	NetworkPolicy *FluentBitNetworkPolicy
}

// FluentBitNetworkPolicy defines settings for the fluent-bit NetworkPolicy.
type FluentBitNetworkPolicy struct {
	// AdditionalEgressIPBlocks contains IP CIDRs for the egress network policy.
	AdditionalEgressIPBlocks []string
}

// Loki contains configuration for the Loki.
type Loki struct {
	// Enabled is used to enable or disable the shoot and seed Loki.
	// If FluentBit is used with a custom output the Loki can, Loki is maybe unused and can be disabled.
	// If not set, by default Loki is enabled.
	Enabled *bool
	// Garden contains configuration for the Loki in garden namespace.
	Garden *GardenLoki
}

// GardenLoki contains configuration for the Loki in garden namespace.
type GardenLoki struct {
	// Storage is the disk storage capacity of the central Loki.
	// Defaults to 100Gi.
	Storage *resource.Quantity
}

// ShootNodeLogging contains configuration for the shoot node logging.
type ShootNodeLogging struct {
	// ShootPurposes determines which shoots can have node logging by their purpose.
	ShootPurposes []gardencore.ShootPurpose
}

// ShootEventLogging contains configurations for the shoot event logger.
type ShootEventLogging struct {
	// Enabled is used to enable or disable shoot event logger.
	Enabled *bool
}

// Logging contains configuration for the logging stack.
type Logging struct {
	// Enabled is used to enable or disable logging stack for clusters.
	Enabled *bool
	// FluentBit contains configurations for the fluent-bit.
	FluentBit *FluentBit
	// Loki contains configuration for the Loki.
	Loki *Loki
	// ShootNodeLogging contains configurations for the shoot node logging.
	ShootNodeLogging *ShootNodeLogging
	// ShootEventLogging contains configurations for the shoot event logger.
	ShootEventLogging *ShootEventLogging
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
	// Port is the port on which to serve unsecured, unauthenticated access.
	Port int
}

// SNI contains an optional configuration for the APIServerSNI feature used
// by the Gardenlet in the seed clusters.
type SNI struct {
	// Ingress is the ingressgateway configuration.
	Ingress *SNIIngress
}

// SNIIngress contains configuration of the ingressgateway.
type SNIIngress struct {
	// ServiceName is the name of the ingressgateway Service.
	// Defaults to "istio-ingressgateway".
	ServiceName *string
	// ServiceExternalIP is the external ip which should be assigned to the
	// load balancer service of the ingress gateway.
	// Compatibility is depending on the respective provider cloud-controller-manager.
	ServiceExternalIP *string
	// Namespace is the namespace in which the ingressgateway is deployed in.
	// Defaults to "istio-ingress".
	Namespace *string
	// Labels of the ingressgateway
	// Defaults to "istio: ingressgateway".
	Labels map[string]string
}

// ETCDConfig contains ETCD related configs
type ETCDConfig struct {
	// ETCDController contains config specific to ETCD controller
	ETCDController *ETCDController
	// CustodianController contains config specific to custodian controller
	CustodianController *CustodianController
	// BackupCompactionController contains config specific to backup compaction controller
	BackupCompactionController *BackupCompactionController
	// BackupLeaderElection contains configuration for the leader election for the etcd backup-restore sidecar.
	BackupLeaderElection *ETCDBackupLeaderElection
}

// ETCDController contains config specific to ETCD controller
type ETCDController struct {
	// Workers specify number of worker threads in ETCD controller
	// Defaults to 50
	Workers *int64
}

// CustodianController contains config specific to custodian controller
type CustodianController struct {
	// Workers specify number of worker threads in custodian controller
	// Defaults to 10
	Workers *int64
}

// BackupCompactionController contains config specific to backup compaction controller
type BackupCompactionController struct {
	// Workers specify number of worker threads in backup compaction controller
	// Defaults to 3
	Workers *int64
	// EnableBackupCompaction enables automatic compaction of etcd backups
	// Defaults to false
	EnableBackupCompaction *bool
	// EventsThreshold defines total number of etcd events that can be allowed before a backup compaction job is triggered
	// Defaults to 1 Million events
	EventsThreshold *int64
	// ActiveDeadlineDuration defines duration after which a running backup compaction job will be killed
	// Defaults to 3 hours
	ActiveDeadlineDuration *metav1.Duration
}

// ETCDBackupLeaderElection contains configuration for the leader election for the etcd backup-restore sidecar.
type ETCDBackupLeaderElection struct {
	// ReelectionPeriod defines the Period after which leadership status of corresponding etcd is checked.
	ReelectionPeriod *metav1.Duration
	// EtcdConnectionTimeout defines the timeout duration for etcd client connection during leader election.
	EtcdConnectionTimeout *metav1.Duration
}

// ExposureClassHandler contains configuration for an exposure class handler.
type ExposureClassHandler struct {
	// Name is the name of the exposure class handler.
	Name string
	// LoadBalancerService contains configuration which is used to configure the underlying
	// load balancer to apply the control plane endpoint exposure strategy.
	LoadBalancerService LoadBalancerServiceConfig
	// SNI contains optional configuration for a dedicated ingressgateway belonging to
	// an exposure class handler. This is only required in context of the APIServerSNI feature of the gardenlet.
	SNI *SNI
}

// LoadBalancerServiceConfig contains configuration which is used to configure the underlying
// load balancer to apply the control plane endpoint exposure strategy.
type LoadBalancerServiceConfig struct {
	// Annotations is a key value map to annotate the underlying load balancer services.
	Annotations map[string]string
}

// MonitoringConfig contains settings for the monitoring stack.
type MonitoringConfig struct {
	// Shoot is optional and contains settings for the shoot monitoring stack.
	Shoot *ShootMonitoringConfig
}

// ShootMonitoringConfig contains settings for the shoot monitoring stack.
type ShootMonitoringConfig struct {
	// Enabled is used to enable or disable the shoot monitoring stack.
	// Defaults to true.
	Enabled *bool
	// RemoteWrite is optional and contains remote write setting.
	RemoteWrite *RemoteWriteMonitoringConfig
	// ExternalLabels is optional and sets additional external labels for the monitoring stack.
	ExternalLabels map[string]string
}

// RemoteWriteMonitoringConfig contains settings for the remote write setting for monitoring stack.
type RemoteWriteMonitoringConfig struct {
	// URL contains an Url for remote write setting in prometheus.
	URL string
	// Keep contains a list of metrics that will be remote written
	Keep []string
	// QueueConfig contains the queue_config for prometheus remote write.
	QueueConfig *string
}
