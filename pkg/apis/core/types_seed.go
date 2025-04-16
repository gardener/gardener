// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	// Settings contains certain settings for this seed cluster.
	Settings *SeedSettings
	// Taints describes taints on the seed.
	Taints []SeedTaint
	// Volume contains settings for persistentvolumes created in the seed cluster.
	Volume *SeedVolume
	// Ingress configures Ingress specific settings of the Seed cluster. This field is immutable.
	Ingress *Ingress
	// AccessRestrictions describe a list of access restrictions for this seed cluster.
	AccessRestrictions []AccessRestriction
	// Extensions contain type and provider information for Seed extensions.
	Extensions []Extension
	// Resources holds a list of named resource references that can be referred to in extension configs by their names.
	Resources []NamedResourceReference
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
	// LastOperation holds information about the last operation on the Seed.
	LastOperation *LastOperation
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
	// Deprecated: This field will be removed after v1.121.0 has been released. Use `CredentialsRef` instead.
	// Until removed, this field is synced with the `CredentialsRef` field when it refers to a secret.
	SecretRef corev1.SecretReference

	// CredentialsRef is reference to a resource holding the credentials used for
	// authentication with the object store service where the backups are stored.
	// Supported referenced resources are v1.Secrets and
	// security.gardener.cloud/v1alpha1.WorkloadIdentity
	CredentialsRef *corev1.ObjectReference
}

// SeedDNS contains the external domain and configuration for the DNS provider
type SeedDNS struct {
	// Provider configures a DNSProvider
	Provider *SeedDNSProvider
}

// SeedDNSProvider configures a DNS provider
type SeedDNSProvider struct {
	// Type describes the type of the dns-provider, for example `aws-route53`
	Type string
	// SecretRef is a reference to a Secret object containing cloud provider credentials used for registering external domains.
	SecretRef corev1.SecretReference
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
	// IPFamilies specifies the IP protocol versions to use for seed networking. This field is immutable.
	// See https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md.
	// Defaults to ["IPv4"].
	IPFamilies []IPFamily
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
	// LoadBalancerServices controls certain settings for services of type load balancer that are created in the seed.
	LoadBalancerServices *SeedSettingLoadBalancerServices
	// VerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the seed.
	VerticalPodAutoscaler *SeedSettingVerticalPodAutoscaler
	// DependencyWatchdog controls certain settings for the dependency-watchdog components deployed in the seed.
	DependencyWatchdog *SeedSettingDependencyWatchdog
	// TopologyAwareRouting controls certain settings for topology-aware traffic routing in the seed.
	// See https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md.
	TopologyAwareRouting *SeedSettingTopologyAwareRouting
}

// SeedSettingExcessCapacityReservation controls the excess capacity reservation for shoot control planes in the
// seed.
type SeedSettingExcessCapacityReservation struct {
	// Enabled controls whether the default excess capacity reservation should be enabled. When not specified, the functionality is enabled.
	Enabled *bool
	// Configs configures excess capacity reservation deployments for shoot control planes in the seed.
	Configs []SeedSettingExcessCapacityReservationConfig
}

// SeedSettingExcessCapacityReservationConfig configures excess capacity reservation deployments for shoot control planes in the seed.
type SeedSettingExcessCapacityReservationConfig struct {
	// Resources specify the resource requests and limits of the excess-capacity-reservation pod.
	Resources corev1.ResourceList
	// NodeSelector specifies the node where the excess-capacity-reservation pod should run.
	NodeSelector map[string]string
	// Tolerations specify the tolerations for the the excess-capacity-reservation pod.
	Tolerations []corev1.Toleration
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
	// ExternalTrafficPolicy describes how nodes distribute service traffic they
	// receive on one of the service's "externally-facing" addresses.
	// Defaults to "Cluster".
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicy
	// Zones controls settings, which are specific to the single-zone load balancers in a multi-zonal setup.
	// Can be empty for single-zone seeds. Each specified zone has to relate to one of the zones in seed.spec.provider.zones.
	Zones []SeedSettingLoadBalancerServicesZones
	// ProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.
	// Defaults to nil, which is equivalent to not allowing ProxyProtocol.
	ProxyProtocol *LoadBalancerServicesProxyProtocol
}

// SeedSettingLoadBalancerServicesZones controls settings, which are specific to the single-zone load balancers in a
// multi-zonal setup.
type SeedSettingLoadBalancerServicesZones struct {
	// Name is the name of the zone as specified in seed.spec.provider.zones.
	Name string
	// Annotations is a map of annotations that will be injected/merged into the zone-specific load balancer service object.
	Annotations map[string]string
	// ExternalTrafficPolicy describes how nodes distribute service traffic they
	// receive on one of the service's "externally-facing" addresses.
	// Defaults to "Cluster".
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicy
	// ProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.
	// Defaults to nil, which is equivalent to not allowing ProxyProtocol.
	ProxyProtocol *LoadBalancerServicesProxyProtocol
}

// LoadBalancerServicesProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.
type LoadBalancerServicesProxyProtocol struct {
	// Allowed controls whether the ProxyProtocol is optionally allowed for the load balancer services.
	// This should only be enabled if the load balancer services are already using ProxyProtocol or will be reconfigured to use it soon.
	// Until the load balancers are configured with ProxyProtocol, enabling this setting may allow clients to spoof their source IP addresses.
	// The option allows a migration from non-ProxyProtocol to ProxyProtocol without downtime (depending on the infrastructure).
	// Defaults to false.
	Allowed bool
}

// SeedSettingVerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
// seed.
type SeedSettingVerticalPodAutoscaler struct {
	// Enabled controls whether the VPA components shall be deployed into the garden namespace in the seed cluster. It
	// is enabled by default because Gardener heavily relies on a VPA being deployed. You should only disable this if
	// your seed cluster already has another, manually/custom managed VPA deployment.
	Enabled bool
}

// SeedSettingDependencyWatchdog controls the dependency-watchdog settings for the seed.
type SeedSettingDependencyWatchdog struct {
	// Weeder controls the weeder settings for the dependency-watchdog for the seed.
	Weeder *SeedSettingDependencyWatchdogWeeder
	// Prober controls the prober settings for the dependency-watchdog for the seed.
	Prober *SeedSettingDependencyWatchdogProber
}

// SeedSettingDependencyWatchdogWeeder controls the weeder settings for the dependency-watchdog for the seed.
type SeedSettingDependencyWatchdogWeeder struct {
	// Enabled controls whether the weeder of the dependency-watchdog should be enabled. This controller
	// helps to alleviate the delay where control plane components remain unavailable by finding the respective pods in
	// CrashLoopBackoff status and restarting them once their dependants become ready and available again.
	Enabled bool
}

// SeedSettingDependencyWatchdogProber controls the prober settings for the dependency-watchdog for the seed.
type SeedSettingDependencyWatchdogProber struct {
	// Enabled controls whether the prober of the dependency-watchdog should be enabled.
	// reachable via its external ingress in order to avoid melt-down situations.
	Enabled bool
}

// SeedSettingTopologyAwareRouting controls certain settings for topology-aware traffic routing in the seed.
// See https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md.
type SeedSettingTopologyAwareRouting struct {
	// Enabled controls whether certain Services deployed in the seed cluster should be topology-aware.
	// These Services are etcd-main-client, etcd-events-client, kube-apiserver, gardener-resource-manager and vpa-webhook.
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
