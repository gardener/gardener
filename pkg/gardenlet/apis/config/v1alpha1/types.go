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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/klog"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GardenletConfiguration defines the configuration for the Gardener controller manager.
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
	// LeaderElection defines the configuration of leader election client.
	// +optional
	LeaderElection *LeaderElectionConfiguration `json:"leaderElection,omitempty"`
	// Discovery defines the configuration of the discovery client.
	// +optional
	Discovery *DiscoveryConfiguration `json:"discovery,omitempty"`
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	// +optional
	LogLevel *string `json:"logLevel,omitempty"`
	// KubernetesLogLevel is the log level used for Kubernetes' k8s.io/klog functions.
	// +optional
	KubernetesLogLevel *klog.Level `json:"kubernetesLogLevel,omitempty"`
	// Server defines the configuration of the HTTP server.
	// +optional
	Server *ServerConfiguration `json:"server,omitempty"`
	// FeatureGates is a map of feature names to bools that enable or disable alpha/experimental
	// features. This field modifies piecemeal the built-in default values from
	// "github.com/gardener/gardener/pkg/gardenlet/features/features.go".
	// Default: nil
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
	// SeedConfig contains configuration for the seed cluster. May not be set if seed selector is set.
	// In this case the gardenlet creates the `Seed` object itself based on the provided config.
	// +optional
	SeedConfig *SeedConfig `json:"seedConfig,omitempty"`
	// SeedSelector contains an optional list of labels on `Seed` resources that shall be managed by
	// this gardenlet instance. In this case the `Seed` object is not managed by the Gardenlet and must
	// be created by an operator/administrator.
	// +optional
	SeedSelector *metav1.LabelSelector `json:"seedSelector,omitempty"`
}

// GardenClientConnection specifies the kubeconfig file and the client connection settings
// for the proxy server to use when communicating with the garden apiserver.
type GardenClientConnection struct {
	componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:",inline"`
	// GardenClusterAddress is the external address that the gardenlets can use to remotely connect to the Garden
	// cluster. It is needed in case the gardenlet deploys itself into shooted seeds.
	// +optional
	GardenClusterAddress *string `json:"gardenClusterAddress,omitempty"`
	// GardenClusterCACert is the external address that the gardenlets can use to remotely connect to the Garden
	// cluster. It is needed in case the gardenlet deploys itself into shooted seeds.
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
	BackupBucket *BackupBucketControllerConfiguration `json:"backupBucket"`
	// BackupEntry defines the configuration of the BackupEntry controller.
	// +optional
	BackupEntry *BackupEntryControllerConfiguration `json:"backupEntry"`
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
	// Shoot defines the configuration of the Shoot controller.
	// +optional
	Shoot *ShootControllerConfiguration `json:"shoot,omitempty"`
	// ShootCare defines the configuration of the ShootCare controller.
	// +optional
	ShootCare *ShootCareControllerConfiguration `json:"shootCare,omitempty"`
	// ShootStateSync defines the configuration of the ShootStateController controller
	// +optional
	ShootStateSync *ShootStateSyncControllerConfiguration `json:"shootStateSync,omitempty"`
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
	// DeletionGracePeriodHours holds the period in number of hours to delete the Backup Entry after deletion timestamp is set.
	// If value is set to 0 then the BackupEntryController will trigger deletion immediately.
	// +optional
	DeletionGracePeriodHours *int `json:"deletionGracePeriodHours,omitempty"`
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
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
}

// ShootStateSyncControllerConfiguration defines the configuration of the ShootState Sync controller.
type ShootStateSyncControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the existing extension resources are synced to the ShootState resource
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
}

// ShootControllerConfiguration defines the configuration of the Shoot
// controller.
type ShootControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
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
	// StaleExtensionHealthCheckThreshold configures the threshold when Gardener considers a Health check report of an
	// Extension CRD as outdated.
	// The StaleExtensionHealthCheckThreshold should have some leeway in case a Gardener extension is temporarily unavailable.
	// If not set, Gardener does not verify for outdated health check reports. This is for backwards-compatibility reasons
	// and will become default in a future version.
	// +optional
	StaleExtensionHealthCheckThreshold *metav1.Duration `json:"staleExtensionHealthCheckThreshold,omitempty"`
	// ConditionThresholds defines the condition threshold per condition type.
	// +optional
	ConditionThresholds []ConditionThreshold `json:"conditionThresholds,omitempty"`
}

// ConditionThreshold defines the duration how long a flappy condition stays in progressing state.
type ConditionThreshold struct {
	// Type is the type of the condition to define the threshold for.
	Type string `json:"type"`
	// Duration is the duration how long the condition can stay in the progressing state.
	Duration metav1.Duration `json:"duration"`
}

// DiscoveryConfiguration defines the configuration of how to discover API groups.
// It allows to set where to store caching data and to specify the TTL of that data.
type DiscoveryConfiguration struct {
	// DiscoveryCacheDir is the directory to store discovery cache information.
	// If unset, the discovery client will use the current working directory.
	// +optional
	DiscoveryCacheDir *string `json:"discoveryCacheDir,omitempty"`
	// HTTPCacheDir is the directory to store discovery HTTP cache information.
	// If unset, no HTTP caching will be done.
	// +optional
	HTTPCacheDir *string `json:"httpCacheDir,omitempty"`
	// TTL is the ttl how long discovery cache information shall be valid.
	// +optional
	TTL *metav1.Duration `json:"ttl,omitempty"`
}

// LeaderElectionConfiguration defines the configuration of leader election
// clients for components that can run with leader election enabled.
type LeaderElectionConfiguration struct {
	componentbaseconfigv1alpha1.LeaderElectionConfiguration `json:",inline"`
	// LockObjectNamespace defines the namespace of the lock object.
	// +optional
	LockObjectNamespace *string `json:"lockObjectNamespace,omitempty"`
	// LockObjectName defines the lock object name.
	// +optional
	LockObjectName *string `json:"lockObjectName,omitempty"`
}

// SeedConfig contains configuration for the seed cluster.
type SeedConfig struct {
	gardencorev1beta1.Seed `json:",inline"`
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
	// Port is the port on which to serve unsecured, unauthenticated access.
	Port int `json:"port"`
}

// HTTPSServer is the configuration for the HTTPSServer server.
type HTTPSServer struct {
	// Server is the configuration for the bind address and the port.
	Server `json:",inline"`
	// TLSServer contains information about the TLS configuration for a HTTPS server. If empty then a proper server
	// certificate will be self-generated during startup.
	// +optional
	TLS *TLSServer `json:"tls,omitempty"`
}

// TLSServer contains information about the TLS configuration for a HTTPS server.
type TLSServer struct {
	// ServerCertPath is the path to the server certificate file.
	ServerCertPath string `json:"serverCertPath"`
	// ServerKeyPath is the path to the private key file.
	ServerKeyPath string `json:"serverKeyPath"`
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

	// DefaultLogLevel is the default log level.
	DefaultLogLevel = "info"

	// DefaultKubernetesLogLevel is the default Kubernetes log level.
	DefaultKubernetesLogLevel klog.Level = 0

	// DefaultControllerConcurrentSyncs is a default value for concurrent syncs for controllers.
	DefaultControllerConcurrentSyncs = 20
)

var (
	// DefaultControllerSyncPeriod is a default value for sync period for controllers.
	DefaultControllerSyncPeriod = metav1.Duration{Duration: time.Minute}
)
