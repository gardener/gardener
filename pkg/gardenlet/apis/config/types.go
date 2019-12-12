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
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/klog"
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
	// LeaderElection defines the configuration of leader election client.
	LeaderElection *LeaderElectionConfiguration
	// Discovery defines the configuration of the discovery client.
	Discovery *DiscoveryConfiguration
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel *string
	// KubernetesLogLevel is the log level used for Kubernetes' k8s.io/klog functions.
	KubernetesLogLevel *klog.Level
	// FeatureGates is a map of feature names to bools that enable or disable alpha/experimental
	// features. This field modifies piecemeal the built-in default values from
	// "github.com/gardener/gardener/pkg/features/gardener_features.go".
	// Default: nil
	FeatureGates map[string]bool
	// SeedConfig contains configuration for the seed cluster. May not be set if seed selector is set.
	// In this case the gardenlet creates the `Seed` object itself based on the provided config.
	SeedConfig *SeedConfig
	// SeedSelector contains an optional list of labels on `Seed` resources that shall be managed by
	// this gardenlet instance. In this case the `Seed` object is not managed by the Gardenlet and must
	// be created by an operator/administrator.
	SeedSelector *metav1.LabelSelector
}

// GardenClientConnection specifies the kubeconfig file and the client connection settings
// for the proxy server to use when communicating with the garden apiserver.
type GardenClientConnection struct {
	componentbaseconfig.ClientConnectionConfiguration
	// GardenClusterAddress is the external address that the gardenlets can use to remotely connect to the Garden
	// cluster. It is needed in case the gardenlet deploys itself into shooted seeds.
	GardenClusterAddress *string
	// GardenClusterCACert is the external address that the gardenlets can use to remotely connect to the Garden
	// cluster. It is needed in case the gardenlet deploys itself into shooted seeds.
	GardenClusterCACert []byte
	// BootstrapKubeconfig is a reference to a secret that contains a data key 'kubeconfig' whose value
	// is a kubeconfig that can be used for bootstrapping. If `kubeconfig` is given then only this kubeconfig
	// will be considered.
	BootstrapKubeconfig *corev1.SecretReference
	// KubeconfigSecret is the reference to a secret object that stores the gardenlet's kubeconfig that
	// it uses to communicate with the garden cluster. If `kubeconfig` is given then only this kubeconfig
	// will be considered.
	KubeconfigSecret *corev1.SecretReference
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
	// ControllerInstallation defines the configuration of the ControllerInstallation controller.
	ControllerInstallation *ControllerInstallationControllerConfiguration
	// ControllerInstallationCare defines the configuration of the ControllerInstallationCare controller.
	ControllerInstallationCare *ControllerInstallationCareControllerConfiguration
	// Seed defines the configuration of the Seed controller.
	Seed *SeedControllerConfiguration
	// Shoot defines the configuration of the Shoot controller.
	Shoot *ShootControllerConfiguration
	// ShootCare defines the configuration of the ShootCare controller.
	ShootCare *ShootCareControllerConfiguration
	// ShootStateSync defines the configuration of the ShootState controller
	ShootStateSync *ShootStateSyncControllerConfiguration
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
	// DeletionGracePeriodHours holds the period in number of days to delete the Backup Infrastructure after deletion timestamp is set.
	// If value is set to 0 then the BackupEntryController will trigger deletion immediately.
	DeletionGracePeriodHours *int
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

// SeedControllerConfiguration defines the configuration of the Seed controller.
type SeedControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// ReserveExcessCapacity indicates whether the Seed controller should reserve
	// excess capacity for Shoot control planes in the Seeds. This is done via
	// PodPriority and requires the Seed cluster to have Kubernetes version 1.11 or
	// the PodPriority feature gate as well as the scheduling.k8s.io/v1alpha1 API
	// group enabled. It defaults to true.
	ReserveExcessCapacity *bool
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod *metav1.Duration
}

// ShootControllerConfiguration defines the configuration of the CloudProfile
// controller.
type ShootControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// ReconcileInMaintenanceOnly determines whether Shoot reconciliations happen only
	// during its maintenance time window.
	ReconcileInMaintenanceOnly *bool
	// RespectSyncPeriodOverwrite determines whether a sync period overwrite of a
	// Shoot (via annotation) is respected or not. Defaults to false.
	RespectSyncPeriodOverwrite *bool
	// RetryDuration is the maximum duration how often a reconciliation will be retried
	// in case of errors.
	RetryDuration *metav1.Duration
	// RetrySyncPeriod is the duration how fast Shoots with an errornous operation are
	// re-added to the queue so that the operation can be retried. Defaults to 15s.
	RetrySyncPeriod *metav1.Duration
	// SyncPeriod is the duration how often the existing resources are reconciled.
	SyncPeriod *metav1.Duration
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
	// ConditionThresholds defines the condition threshold per condition type.
	ConditionThresholds []ConditionThreshold
}

// ConditionThreshold defines the duration how long a flappy condition stays in progressing state.
type ConditionThreshold struct {
	// Type is the type of the condition to define the threshold for.
	Type string
	// Duration is the duration how long the condition can stay in the progressing state.
	Duration *metav1.Duration
}

// ShootStateSyncControllerConfiguration defines the configuration of the
// ShootStateController controller.
type ShootStateSyncControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs *int
	// SyncPeriod is the duration how often the existing extension resources are
	// synced to the ShootState resource
	SyncPeriod *metav1.Duration
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
	LockObjectNamespace *string
	// LockObjectName defines the lock object name.
	LockObjectName *string
}

// SeedConfig contains configuration for the seed cluster.
type SeedConfig struct {
	gardencorev1beta1.Seed
}

const (
	// GardenletDefaultLockObjectNamespace is the default lock namespace for leader election.
	GardenletDefaultLockObjectNamespace = "garden"

	// GardenletDefaultLockObjectName is the default lock name for leader election.
	GardenletDefaultLockObjectName = "gardenlet-leader-election"

	// DefaultBackupEntryDeletionGracePeriodHours is a constant for the default number of hours the Backup Entry should be kept after shoot is deleted.
	// By default we set this to 0 so that then BackupEntryController will trigger deletion immediately.
	DefaultBackupEntryDeletionGracePeriodHours = 0

	// DefaultDiscoveryTTL is the default ttl for the cached discovery client.
	DefaultDiscoveryTTL = 10 * time.Second
)
