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

package core

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Seed represents an installation request for an external controller.
type Seed struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec contains the specification of this installation.
	Spec SeedSpec
	// Status contains the status of this installation.
	Status SeedStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SeedList is a collection of Seeds.
type SeedList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of Seeds.
	Items []Seed
}

// SeedTemplate is a template for creating a Seed object.
type SeedTemplate struct {
	// Standard object metadata.
	metav1.ObjectMeta
	// Specification of the desired behavior of the Seed.
	Spec SeedSpec
}

// SeedSpec is the specification of a Seed.
type SeedSpec struct {
	// Backup holds the object store configuration for the backups of shoot (currently only etcd).
	// If it is not specified, then there won't be any backups taken for shoots associated with this seed.
	// If backup field is present in seed, then backups of the etcd from shoot control plane will be stored
	// under the configured object store.
	Backup *SeedBackup
	// DNS contains DNS-relevant information about this seed cluster.
	DNS SeedDNS
	// Networks defines the pod, service and worker network of the Seed cluster.
	Networks SeedNetworks
	// Provider defines the provider type and region for this Seed cluster.
	Provider SeedProvider
	// SecretRef is a reference to a Secret object containing the Kubeconfig of the Kubernetes
	// cluster to be registered as Seed.
	SecretRef *corev1.SecretReference
	// Settings contains certain settings for this seed cluster.
	Settings *SeedSettings
	// Taints describes taints on the seed.
	Taints []SeedTaint
	// Volume contains settings for persistentvolumes created in the seed cluster.
	Volume *SeedVolume
	// Ingress configures Ingress specific settings of the Seed cluster. This field is immutable.
	Ingress *Ingress
	// HighAvailability describes the high availability configuration for seed system components. A highly available
	// seed will need at least 3 nodes or 3 availability zones (depending on the configured FailureTolerance of `node` or `zone`),
	// allowing spreading of system components across the configured failure domain.
	// Deprecated: This field is deprecated and not respected at all. It will be removed in a future release. Use
	// `.spec.provider.zones` instead.
	HighAvailability *HighAvailability
}

// GetProviderType gets the type of the provider.
func (s *Seed) GetProviderType() string {
	return s.Spec.Provider.Type
}

// SeedStatus is the status of a Seed.
type SeedStatus struct {
	// Gardener holds information about the Gardener which last acted on the Shoot.
	Gardener *Gardener
	// KubernetesVersion is the Kubernetes version of the seed cluster.
	KubernetesVersion *string
	// Conditions represents the latest available observations of a Seed's current state.
	Conditions []Condition
	// ObservedGeneration is the most recent generation observed for this Seed. It corresponds to the
	// Seed's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64
	// ClusterIdentity is the identity of the Seed cluster. This field is immutable.
	ClusterIdentity *string
	// Capacity represents the total resources of a seed.
	Capacity corev1.ResourceList
	// Allocatable represents the resources of a seed that are available for scheduling.
	// Defaults to Capacity.
	Allocatable corev1.ResourceList
	// ClientCertificateExpirationTimestamp is the timestamp at which gardenlet's client certificate expires.
	ClientCertificateExpirationTimestamp *metav1.Time
}

// SeedBackup contains the object store configuration for backups for shoot (currently only etcd).
type SeedBackup struct {
	// Provider is a provider name. This field is immutable.
	Provider string
	// ProviderConfig is the configuration passed to BackupBucket resource.
	ProviderConfig *runtime.RawExtension
	// Region is a region name. This field is immutable.
	Region *string
	// SecretRef is a reference to a Secret object containing the cloud provider credentials for
	// the object store where backups should be stored. It should have enough privileges to manipulate
	// the objects as well as buckets.
	SecretRef corev1.SecretReference
}

// SeedDNS contains the external domain and configuration for the DNS provider
type SeedDNS struct {
	// IngressDomain is the domain of the Seed cluster pointing to the ingress controller endpoint. It will be used
	// to construct ingress URLs for system applications running in Shoot clusters. This field is immutable.
	// This will be removed in the next API version and replaced by spec.ingress.domain.
	IngressDomain *string
	// Provider configures a DNSProvider
	Provider *SeedDNSProvider
}

// SeedDNSProvider configures a DNS provider
type SeedDNSProvider struct {
	// Type describes the type of the dns-provider, for example `aws-route53`
	Type string
	// SecretRef is a reference to a Secret object containing cloud provider credentials used for registering external domains.
	SecretRef corev1.SecretReference
	// Domains contains information about which domains shall be included/excluded for this provider.
	Domains *DNSIncludeExclude
	// Zones contains information about which hosted zones shall be included/excluded for this provider.
	Zones *DNSIncludeExclude
}

// Ingress configures the Ingress specific settings of the Seed cluster
type Ingress struct {
	// Domain specifies the ingress domain of the Seed cluster pointing to the ingress controller endpoint. It will be used
	// to construct ingress URLs for system applications running in Shoot clusters. Once set this field is immutable.
	Domain string
	// Controller configures a Gardener managed Ingress Controller listening on the ingressDomain
	Controller IngressController
}

// IngressController enables a Gardener managed Ingress Controller listening on the ingressDomain
type IngressController struct {
	// Kind defines which kind of IngressController to use, for example `nginx`
	Kind string
	// ProviderConfig specifies infrastructure specific configuration for the ingressController
	ProviderConfig *runtime.RawExtension
}

// SeedNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
type SeedNetworks struct {
	// Nodes is the CIDR of the node network. This field is immutable.
	Nodes *string
	// Pods is the CIDR of the pod network. This field is immutable.
	Pods string
	// Services is the CIDR of the service network. This field is immutable.
	Services string
	// ShootDefaults contains the default networks CIDRs for shoots.
	ShootDefaults *ShootNetworks
	// BlockCIDRs is a list of network addresses that should be blocked for shoot control plane components running
	// in the seed cluster.
	BlockCIDRs []string
}

// ShootNetworks contains the default networks CIDRs for shoots.
type ShootNetworks struct {
	// Pods is the CIDR of the pod network.
	Pods *string
	// Services is the CIDR of the service network.
	Services *string
}

// SeedProvider defines the provider-specific information of this Seed cluster.
type SeedProvider struct {
	// Type is the name of the provider.
	Type string
	// ProviderConfig is the configuration passed to Seed resource.
	ProviderConfig *runtime.RawExtension
	// Region is a name of a region.
	Region string
	// Zones is the list of availability zones the seed cluster is deployed to.
	Zones []string
}

// SeedSettings contains certain settings for this seed cluster.
type SeedSettings struct {
	// ExcessCapacityReservation controls the excess capacity reservation for shoot control planes in the seed.
	ExcessCapacityReservation *SeedSettingExcessCapacityReservation
	// Scheduling controls settings for scheduling decisions for the seed.
	Scheduling *SeedSettingScheduling
	// ShootDNS controls the shoot DNS settings for the seed.
	// Deprecated: This field is deprecated and will be removed in a future version of Gardener. Do not use it.
	ShootDNS *SeedSettingShootDNS
	// LoadBalancerServices controls certain settings for services of type load balancer that are created in the seed.
	LoadBalancerServices *SeedSettingLoadBalancerServices
	// VerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the seed.
	VerticalPodAutoscaler *SeedSettingVerticalPodAutoscaler
	// SeedSettingOwnerChecks controls certain owner checks settings for shoots scheduled on this seed.
	OwnerChecks *SeedSettingOwnerChecks
	// DependencyWatchdog controls certain settings for the dependency-watchdog components deployed in the seed.
	DependencyWatchdog *SeedSettingDependencyWatchdog
}

// SeedSettingExcessCapacityReservation controls the excess capacity reservation for shoot control planes in the
// seed.
type SeedSettingExcessCapacityReservation struct {
	// Enabled controls whether the excess capacity reservation should be enabled.
	Enabled bool
}

// SeedSettingShootDNS controls the shoot DNS settings for the seed.
type SeedSettingShootDNS struct {
	// Enabled controls whether the DNS for shoot clusters should be enabled. When disabled then all shoots using the
	// seed won't get any DNS providers, DNS records, and no DNS extension controller is required to be installed here.
	// This is useful for environments where DNS is not required.
	Enabled bool
}

// SeedSettingScheduling controls settings for scheduling decisions for the seed.
type SeedSettingScheduling struct {
	// Visible controls whether the gardener-scheduler shall consider this seed when scheduling shoots. Invisible seeds
	// are not considered by the scheduler.
	Visible bool
}

// SeedSettingLoadBalancerServices controls certain settings for services of type load balancer that are created in the
// seed.
type SeedSettingLoadBalancerServices struct {
	// Annotations is a map of annotations that will be injected/merged into every load balancer service object.
	Annotations map[string]string
}

// SeedSettingVerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
// seed.
type SeedSettingVerticalPodAutoscaler struct {
	// Enabled controls whether the VPA components shall be deployed into the garden namespace in the seed cluster. It
	// is enabled by default because Gardener heavily relies on a VPA being deployed. You should only disable this if
	// your seed cluster already has another, manually/custom managed VPA deployment.
	Enabled bool
}

// SeedSettingOwnerChecks controls certain owner checks settings for shoots scheduled on this seed.
type SeedSettingOwnerChecks struct {
	// Enabled controls whether owner checks are enabled for shoots scheduled on this seed. It
	// is enabled by default because it is a prerequisite for control plane migration.
	Enabled bool
}

// SeedSettingDependencyWatchdog controls the dependency-watchdog settings for the seed.
type SeedSettingDependencyWatchdog struct {
	// Endpoint controls the endpoint settings for the dependency-watchdog for the seed.
	Endpoint *SeedSettingDependencyWatchdogEndpoint
	// Probe controls the probe settings for the dependency-watchdog for the seed.
	Probe *SeedSettingDependencyWatchdogProbe
}

// SeedSettingDependencyWatchdogEndpoint controls the endpoint settings for the dependency-watchdog for the seed.
type SeedSettingDependencyWatchdogEndpoint struct {
	// Enabled controls whether the endpoint controller of the dependency-watchdog should be enabled. This controller
	// helps to alleviate the delay where control plane components remain unavailable by finding the respective pods in
	// CrashLoopBackoff status and restarting them once their dependants become ready and available again.
	Enabled bool
}

// SeedSettingDependencyWatchdogProbe controls the probe settings for the dependency-watchdog for the seed.
type SeedSettingDependencyWatchdogProbe struct {
	// Enabled controls whether the probe controller of the dependency-watchdog should be enabled. This controller
	// scales down the kube-controller-manager of shoot clusters in case their respective kube-apiserver is not
	// reachable via its external ingress in order to avoid melt-down situations.
	Enabled bool
}

// SeedTaint describes a taint on a seed.
type SeedTaint struct {
	// Key is the taint key to be applied to a seed.
	Key string
	// Value is the taint value corresponding to the taint key.
	Value *string
}

const (
	// SeedTaintProtected is a constant for a taint key on a seed that marks it as protected. Protected seeds
	// may only be used by shoots in the `garden` namespace.
	SeedTaintProtected = "seed.gardener.cloud/protected"
)

// SeedVolume contains settings for persistentvolumes created in the seed cluster.
type SeedVolume struct {
	// MinimumSize defines the minimum size that should be used for PVCs in the seed.
	MinimumSize *resource.Quantity
	// Providers is a list of storage class provisioner types for the seed.
	Providers []SeedVolumeProvider
}

// SeedVolumeProvider is a storage class provisioner type.
type SeedVolumeProvider struct {
	// Purpose is the purpose of this provider.
	Purpose string
	// Name is the name of the storage class provisioner type.
	Name string
}

const (
	// SeedBackupBucketsReady is a constant for a condition type indicating that associated BackupBuckets are ready.
	SeedBackupBucketsReady ConditionType = "BackupBucketsReady"
	// SeedBootstrapped is a constant for a condition type indicating that the seed cluster has been
	// bootstrapped.
	SeedBootstrapped ConditionType = "Bootstrapped"
	// SeedExtensionsReady is a constant for a condition type indicating that the extensions are ready.
	SeedExtensionsReady ConditionType = "ExtensionsReady"
	// SeedGardenletReady is a constant for a condition type indicating that the Gardenlet is ready.
	SeedGardenletReady ConditionType = "GardenletReady"
	// SeedSystemComponentsHealthy is a constant for a condition type indicating the system components health.
	SeedSystemComponentsHealthy ConditionType = "SeedSystemComponentsHealthy"
)

// Resource constants for Gardener object types
const (
	// ResourceShoots is a resource constant for the number of shoots.
	ResourceShoots corev1.ResourceName = "shoots"
)
