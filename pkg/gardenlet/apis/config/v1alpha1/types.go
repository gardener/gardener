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

// GardenletConfiguration defines the configuration for the Gardenlet.
type GardenletConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// GardenClientConnection specifies the kubeconfig file and the client connection settings
	// for the proxy server to use when communicating with the garden apiserver.
	// +optional
	GardenClientConnection *GardenClientConnection `json:"gardenClientConnection,omitempty" protobuf:"bytes,1,opt,name=gardenClientConnection"`
	// SeedClientConnection specifies the client connection settings for the proxy server
	// to use when communicating with the seed apiserver.
	// +optional
	SeedClientConnection *SeedClientConnection `json:"seedClientConnection,omitempty" protobuf:"bytes,2,opt,name=seedClientConnection"`
	// ShootClientConnection specifies the client connection settings for the proxy server
	// to use when communicating with the shoot apiserver.
	// +optional
	ShootClientConnection *ShootClientConnection `json:"shootClientConnection,omitempty" protobuf:"bytes,3,opt,name=shootClientConnection"`
	// Controllers defines the configuration of the controllers.
	// +optional
	Controllers *GardenletControllerConfiguration `json:"controllers,omitempty" protobuf:"bytes,4,opt,name=controllers"`
	// Resources defines the total capacity for seed resources and the amount reserved for use by Gardener.
	// +optional
	Resources *ResourcesConfiguration `json:"resources,omitempty" protobuf:"bytes,5,opt,name=resources"`
	// LeaderElection defines the configuration of leader election client.
	// +optional
	LeaderElection *GardenletLeaderElectionConfiguration `json:"leaderElection,omitempty" protobuf:"bytes,6,opt,name=leaderElection"`
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	// +optional
	LogLevel *string `json:"logLevel,omitempty" protobuf:"bytes,7,opt,name=logLevel"`
	// KubernetesLogLevel is the log level used for Kubernetes' k8s.io/klog functions.
	// +optional
	KubernetesLogLevel *klog.Level `json:"kubernetesLogLevel,omitempty" protobuf:"bytes,8,opt,name=kubernetesLogLevel"`
	// Server defines the configuration of the HTTP server.
	// +optional
	Server *ServerConfiguration `json:"server,omitempty" protobuf:"bytes,9,opt,name=server"`
	// FeatureGates is a map of feature names to bools that enable or disable alpha/experimental
	// features. This field modifies piecemeal the built-in default values from
	// "github.com/gardener/gardener/pkg/gardenlet/features/features.go".
	// Default: nil
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty" protobuf:"bytes,10,opt,name=featureGates"`
	// SeedConfig contains configuration for the seed cluster. Must not be set if seed selector is set.
	// In this case the gardenlet creates the `Seed` object itself based on the provided config.
	// +optional
	SeedConfig *SeedConfig `json:"seedConfig,omitempty" protobuf:"bytes,11,opt,name=seedConfig"`
	// SeedSelector contains an optional list of labels on `Seed` resources that shall be managed by
	// this gardenlet instance. In this case the `Seed` object is not managed by the Gardenlet and must
	// be created by an operator/administrator.
	// +optional
	SeedSelector *metav1.LabelSelector `json:"seedSelector,omitempty" protobuf:"bytes,12,opt,name=seedSelector"`
	// Logging contains an optional configurations for the logging stack deployed
	// by the Gardenlet in the seed clusters.
	// +optional
	Logging *Logging `json:"logging,omitempty" protobuf:"bytes,13,opt,name=logging"`
	// SNI contains an optional configuration for the APIServerSNI feature used
	// by the Gardenlet in the seed clusters.
	// +optional
	SNI *SNI `json:"sni,omitempty" protobuf:"bytes,14,opt,name=sni"`
}

// GardenClientConnection specifies the kubeconfig file and the client connection settings
// for the proxy server to use when communicating with the garden apiserver.
type GardenClientConnection struct {
	componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:",inline" protobuf:"bytes,1,opt,name=clientConnectionConfiguration"`
	// GardenClusterAddress is the external address that the gardenlets can use to remotely connect to the Garden
	// cluster. It is needed in case the gardenlet deploys itself into shooted seeds.
	// +optional
	GardenClusterAddress *string `json:"gardenClusterAddress,omitempty" protobuf:"bytes,2,opt,name=gardenClusterAddress"`
	// GardenClusterCACert is the external address that the gardenlets can use to remotely connect to the Garden
	// cluster. It is needed in case the gardenlet deploys itself into shooted seeds.
	// +optional
	GardenClusterCACert []byte `json:"gardenClusterCACert,omitempty" protobuf:"bytes,3,opt,name=gardenClusterCACert"`
	// BootstrapKubeconfig is a reference to a secret that contains a data key 'kubeconfig' whose value
	// is a kubeconfig that can be used for bootstrapping. If `kubeconfig` is given then only this kubeconfig
	// will be considered.
	// +optional
	BootstrapKubeconfig *corev1.SecretReference `json:"bootstrapKubeconfig,omitempty" protobuf:"bytes,4,opt,name=bootstrapKubeconfig"`
	// KubeconfigSecret is the reference to a secret object that stores the gardenlet's kubeconfig that
	// it uses to communicate with the garden cluster. If `kubeconfig` is given then only this kubeconfig
	// will be considered.
	// +optional
	KubeconfigSecret *corev1.SecretReference `json:"kubeconfigSecret,omitempty" protobuf:"bytes,5,opt,name=kubeconfigSecret"`
}

// SeedClientConnection specifies the client connection settings
// for the proxy server to use when communicating with the seed apiserver.
type SeedClientConnection struct {
	componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:",inline" protobuf:"bytes,1,opt,name=clientConnectionConfiguration"`
}

// ShootClientConnection specifies the client connection settings
// for the proxy server to use when communicating with the shoot apiserver.
type ShootClientConnection struct {
	componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:",inline" protobuf:"bytes,1,opt,name=clientConnectionConfiguration"`
}

// GardenletControllerConfiguration defines the configuration of the controllers.
type GardenletControllerConfiguration struct {
	// BackupBucket defines the configuration of the BackupBucket controller.
	// +optional
	BackupBucket *BackupBucketControllerConfiguration `json:"backupBucket" protobuf:"bytes,1,opt,name=backupBucket"`
	// BackupEntry defines the configuration of the BackupEntry controller.
	// +optional
	BackupEntry *BackupEntryControllerConfiguration `json:"backupEntry" protobuf:"bytes,2,opt,name=backupEntry"`
	// ControllerInstallation defines the configuration of the ControllerInstallation controller.
	// +optional
	ControllerInstallation *ControllerInstallationControllerConfiguration `json:"controllerInstallation,omitempty" protobuf:"bytes,3,opt,name=controllerInstallation"`
	// ControllerInstallationCare defines the configuration of the ControllerInstallationCare controller.
	// +optional
	ControllerInstallationCare *ControllerInstallationCareControllerConfiguration `json:"controllerInstallationCare,omitempty" protobuf:"bytes,4,opt,name=controllerInstallationCare"`
	// ControllerInstallationRequired defines the configuration of the ControllerInstallationRequired controller.
	// +optional
	ControllerInstallationRequired *ControllerInstallationRequiredControllerConfiguration `json:"controllerInstallationRequired,omitempty" protobuf:"bytes,5,opt,name=controllerInstallationRequired"`
	// Seed defines the configuration of the Seed controller.
	// +optional
	Seed *SeedControllerConfiguration `json:"seed,omitempty" protobuf:"bytes,6,opt,name=seed"`
	// Shoot defines the configuration of the Shoot controller.
	// +optional
	Shoot *ShootControllerConfiguration `json:"shoot,omitempty" protobuf:"bytes,7,opt,name=shoot"`
	// ShootCare defines the configuration of the ShootCare controller.
	// +optional
	ShootCare *ShootCareControllerConfiguration `json:"shootCare,omitempty" protobuf:"bytes,8,opt,name=shootCare"`
	// ShootStateSync defines the configuration of the ShootState controller
	// +optional
	ShootStateSync *ShootStateSyncControllerConfiguration `json:"shootStateSync,omitempty" protobuf:"bytes,9,opt,name=shootStateSync"`
	// ShootedSeedRegistration the configuration of the shooted seed registration controller.
	// +optional
	ShootedSeedRegistration *ShootedSeedRegistrationControllerConfiguration `json:"shootedSeedRegistration,omitempty" protobuf:"bytes,10,opt,name=shootedSeedRegistration"`
	// SeedAPIServerNetworkPolicy defines the configuration of the SeedAPIServerNetworkPolicy controller
	// +optional
	SeedAPIServerNetworkPolicy *SeedAPIServerNetworkPolicyControllerConfiguration `json:"seedAPIServerNetworkPolicy,omitempty" protobuf:"bytes,11,opt,name=seedAPIServerNetworkPolicy"`
}

// BackupBucketControllerConfiguration defines the configuration of the BackupBucket
// controller.
type BackupBucketControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty" protobuf:"varint,1,opt,name=concurrentSyncs"`
}

// BackupEntryControllerConfiguration defines the configuration of the BackupEntry
// controller.
type BackupEntryControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty" protobuf:"varint,1,opt,name=concurrentSyncs"`
	// DeletionGracePeriodHours holds the period in number of hours to delete the BackupEntry after deletion timestamp is set.
	// If value is set to 0 then the BackupEntryController will trigger deletion immediately.
	// +optional
	DeletionGracePeriodHours *int `json:"deletionGracePeriodHours,omitempty" protobuf:"varint,2,opt,name=deletionGracePeriodHours"`
}

// ControllerInstallationControllerConfiguration defines the configuration of the
// ControllerInstallation controller.
type ControllerInstallationControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty" protobuf:"varint,1,opt,name=concurrentSyncs"`
}

// ControllerInstallationCareControllerConfiguration defines the configuration of the ControllerInstallationCare
// controller.
type ControllerInstallationCareControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty" protobuf:"varint,1,opt,name=concurrentSyncs"`
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of ControllerInstallations is performed.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty" protobuf:"bytes,2,opt,name=syncPeriod"`
}

// ControllerInstallationRequiredControllerConfiguration defines the configuration of the ControllerInstallationRequired
// controller.
type ControllerInstallationRequiredControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty" protobuf:"varint,1,opt,name=concurrentSyncs"`
}

// SeedControllerConfiguration defines the configuration of the Seed controller.
type SeedControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty" protobuf:"varint,1,opt,name=concurrentSyncs"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty" protobuf:"bytes,2,opt,name=syncPeriod"`
}

// ShootControllerConfiguration defines the configuration of the Shoot
// controller.
type ShootControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty" protobuf:"varint,1,opt,name=concurrentSyncs"`
	// ProgressReportPeriod is the period how often the progress of a shoot operation will be reported in the
	// Shoot's `.status.lastOperation` field. By default, the progress will be reported immediately after a task of the
	// respective flow has been completed. If you set this to a value > 0 (e.g., 5s) then it will be only reported every
	// 5 seconds. Any tasks that were completed in the meantime will not be reported.
	// +optional
	ProgressReportPeriod *metav1.Duration `json:"progressReportPeriod,omitempty" protobuf:"bytes,2,opt,name=progressReportPeriod"`
	// ReconcileInMaintenanceOnly determines whether Shoot reconciliations happen only
	// during its maintenance time window.
	// +optional
	ReconcileInMaintenanceOnly *bool `json:"reconcileInMaintenanceOnly,omitempty" protobuf:"varint,3,opt,name=reconcileInMaintenanceOnly"`
	// RespectSyncPeriodOverwrite determines whether a sync period overwrite of a
	// Shoot (via annotation) is respected or not. Defaults to false.
	// +optional
	RespectSyncPeriodOverwrite *bool `json:"respectSyncPeriodOverwrite,omitempty" protobuf:"varint,4,opt,name=respectSyncPeriodOverwrite"`
	// RetryDuration is the maximum duration how often a reconciliation will be retried
	// in case of errors.
	// +optional
	RetryDuration *metav1.Duration `json:"retryDuration,omitempty" protobuf:"bytes,5,opt,name=retryDuration"`
	// SyncPeriod is the duration how often the existing resources are reconciled.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty" protobuf:"bytes,6,opt,name=syncPeriod"`
	// DNSEntryTTLSeconds is the TTL in seconds that is being used for DNS entries when reconciling shoots.
	// Default: 120s
	// +optional
	DNSEntryTTLSeconds *int64 `json:"dnsEntryTTLSeconds,omitempty" protobuf:"varint,7,opt,name=dnsEntryTTLSeconds"`
}

// ShootCareControllerConfiguration defines the configuration of the ShootCare
// controller.
type ShootCareControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty" protobuf:"varint,1,opt,name=concurrentSyncs"`
	// SyncPeriod is the duration how often the existing resources are reconciled (how
	// often the health check of Shoot clusters is performed (only if no operation is
	// already running on them).
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty" protobuf:"bytes,2,opt,name=syncPeriod"`
	// StaleExtensionHealthCheckThreshold configures the threshold when Gardener considers a Health check report of an
	// Extension CRD as outdated.
	// The StaleExtensionHealthCheckThreshold should have some leeway in case a Gardener extension is temporarily unavailable.
	// If not set, Gardener does not verify for outdated health check reports. This is for backwards-compatibility reasons
	// and will become default in a future version.
	// +optional
	StaleExtensionHealthCheckThreshold *metav1.Duration `json:"staleExtensionHealthCheckThreshold,omitempty" protobuf:"bytes,3,opt,name=staleExtensionHealthCheckThreshold"`
	// ConditionThresholds defines the condition threshold per condition type.
	// +optional
	ConditionThresholds []ConditionThreshold `json:"conditionThresholds,omitempty" protobuf:"bytes,4,rep,name=conditionThresholds"`
}

// ShootedSeedRegistrationControllerConfiguration defines the configuration of the shooted seed registration controller.
type ShootedSeedRegistrationControllerConfiguration struct {
	// SyncJitterPeriod is a jitter duration for the reconciler sync that can be used to distribute the syncs randomly.
	// If its value is greater than 0 then the shooted seeds will not be enqueued immediately but only after a random
	// duration between 0 and the configured value. It is defaulted to 5m.
	// +optional
	SyncJitterPeriod *metav1.Duration `json:"syncJitterPeriod,omitempty" protobuf:"bytes,1,opt,name=syncJitterPeriod"`
}

// ConditionThreshold defines the duration how long a flappy condition stays in progressing state.
type ConditionThreshold struct {
	// Type is the type of the condition to define the threshold for.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// Duration is the duration how long the condition can stay in the progressing state.
	Duration metav1.Duration `json:"duration" protobuf:"bytes,2,opt,name=duration"`
}

// ShootStateSyncControllerConfiguration defines the configuration of the ShootState Sync controller.
type ShootStateSyncControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty" protobuf:"varint,1,opt,name=concurrentSyncs"`
	// SyncPeriod is the duration how often the existing extension resources are synced to the ShootState resource
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty" protobuf:"bytes,2,opt,name=syncPeriod"`
}

// SeedAPIServerNetworkPolicyControllerConfiguration defines the configuration of the SeedAPIServerNetworkPolicy
// controller.
type SeedAPIServerNetworkPolicyControllerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on events.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty" protobuf:"varint,1,opt,name=concurrentSyncs"`
}

// ResourcesConfiguration defines the total capacity for seed resources and the amount reserved for use by Gardener.
type ResourcesConfiguration struct {
	// Capacity defines the total resources of a seed.
	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty" protobuf:"bytes,1,opt,name=capacity"`
	// Reserved defines the resources of a seed that are reserved for use by Gardener.
	// Defaults to 0.
	// +optional
	Reserved corev1.ResourceList `json:"reserved,omitempty" protobuf:"bytes,2,opt,name=reserved"`
}

// GardenletLeaderElectionConfiguration defines the configuration of leader election
// clients for components that can run with leader election enabled.
type GardenletLeaderElectionConfiguration struct {
	componentbaseconfigv1alpha1.LeaderElectionConfiguration `json:",inline" protobuf:"bytes,1,opt,name=leaderElectionConfiguration"`
	// LockObjectNamespace defines the namespace of the lock object.
	// +optional
	LockObjectNamespace *string `json:"lockObjectNamespace,omitempty" protobuf:"bytes,2,opt,name=lockObjectNamespace"`
	// LockObjectName defines the lock object name.
	// +optional
	LockObjectName *string `json:"lockObjectName,omitempty" protobuf:"bytes,3,opt,name=lockObjectName"`
}

// SeedConfig contains configuration for the seed cluster.
type SeedConfig struct {
	gardencorev1beta1.Seed `json:",inline" protobuf:"bytes,1,opt,name=seed"`
}

// FluentBit contains configuration for Fluent Bit.
type FluentBit struct {
	// ServiceSection defines [SERVICE] configuration for the fluent-bit.
	// If it is nil, fluent-bit uses default service configuration.
	// +optional
	ServiceSection *string `json:"service,omitempty" yaml:"service,omitempty" protobuf:"bytes,1,opt,name=seed"`
	// InputSection defines [INPUT] configuration for the fluent-bit.
	// If it is nil, fluent-bit uses default input configuration.
	// +optional
	InputSection *string `json:"input,omitempty" yaml:"input,omitempty" protobuf:"bytes,2,opt,name=input"`
	// OutputSection defines [OUTPUT] configuration for the fluent-bit.
	// If it is nil, fluent-bit uses default output configuration.
	// +optional
	OutputSection *string `json:"output,omitempty" yaml:"output,omitempty" protobuf:"bytes,3,opt,name=output"`
}

// Logging contains configuration for the logging stack.
type Logging struct {
	// FluentBit contains configurations for the fluent-bit
	// +optional
	FluentBit *FluentBit `json:"fluentBit,omitempty" yaml:"fluentBit,omitempty" protobuf:"bytes,1,opt,name=fluentBit"`
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
	// HTTPS is the configuration for the HTTPS server.
	HTTPS HTTPSServer `json:"https" protobuf:"bytes,1,opt,name=https"`
}

// Server contains information for HTTP(S) server configuration.
type Server struct {
	// BindAddress is the IP address on which to listen for the specified port.
	BindAddress string `json:"bindAddress" protobuf:"bytes,1,opt,name=bindAddress"`
	// Port is the port on which to serve unsecured, unauthenticated access.
	Port int `json:"port" protobuf:"varint,2,opt,name=port"`
}

// HTTPSServer is the configuration for the HTTPSServer server.
type HTTPSServer struct {
	// Server is the configuration for the bind address and the port.
	Server `json:",inline" protobuf:"bytes,1,opt,name=server"`
	// TLSServer contains information about the TLS configuration for a HTTPS server. If empty then a proper server
	// certificate will be self-generated during startup.
	// +optional
	TLS *TLSServer `json:"tls,omitempty" protobuf:"bytes,2,opt,name=tls"`
}

// TLSServer contains information about the TLS configuration for a HTTPS server.
type TLSServer struct {
	// ServerCertPath is the path to the server certificate file.
	ServerCertPath string `json:"serverCertPath" protobuf:"bytes,1,opt,name=serverCertPath"`
	// ServerKeyPath is the path to the private key file.
	ServerKeyPath string `json:"serverKeyPath" protobuf:"bytes,2,opt,name=serverKeyPath"`
}

// SNI contains an optional configuration for the APIServerSNI feature used
// by the Gardenlet in the seed clusters.
type SNI struct {
	// Ingress is the ingressgateway configuration.
	// +optional
	Ingress *SNIIngress `json:"ingress,omitempty" protobuf:"bytes,1,opt,name=ingress"`
}

// SNIIngress contains configuration of the ingressgateway.
type SNIIngress struct {
	// ServiceName is the name of the ingressgateway Service.
	// Defaults to "istio-ingressgateway".
	// +optional
	ServiceName *string `json:"serviceName,omitempty" protobuf:"bytes,1,opt,name=serviceName"`
	// Namespace is the namespace in which the ingressgateway is deployed in.
	// Defaults to "istio-ingress".
	// +optional
	Namespace *string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
	// Labels of the ingressgateway
	// Defaults to "istio: ingressgateway".
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,3,opt,name=labels"`
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

	// DefaultSNIIngresNamespace is the default sni ingress namespace.
	DefaultSNIIngresNamespace = "istio-ingress"

	// DefaultSNIIngresServiceName is the default sni ingress service name.
	DefaultSNIIngresServiceName = "istio-ingressgateway"
)

// DefaultControllerSyncPeriod is a default value for sync period for controllers.
var DefaultControllerSyncPeriod = metav1.Duration{Duration: time.Minute}
