// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

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
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec contains the specification of this installation.
	Spec SeedSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Status contains the status of this installation.
	Status SeedStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SeedList is a collection of Seeds.
type SeedList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of Seeds.
	Items []Seed `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// SeedTemplate is a template for creating a Seed object.
type SeedTemplate struct {
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Specification of the desired behavior of the Seed.
	// +optional
	Spec SeedSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// SeedSpec is the specification of a Seed.
type SeedSpec struct {
	// Backup holds the object store configuration for the backups of shoot (currently only etcd).
	// If it is not specified, then there won't be any backups taken for shoots associated with this seed.
	// If backup field is present in seed, then backups of the etcd from shoot control plane will be stored
	// under the configured object store.
	// +optional
	Backup *Backup `json:"backup,omitempty" protobuf:"bytes,1,opt,name=backup"`
	// DNS contains DNS-relevant information about this seed cluster.
	DNS SeedDNS `json:"dns" protobuf:"bytes,2,opt,name=dns"`
	// Networks defines the pod, service and worker network of the Seed cluster.
	Networks SeedNetworks `json:"networks" protobuf:"bytes,3,opt,name=networks"`
	// Provider defines the provider type and region for this Seed cluster.
	Provider SeedProvider `json:"provider" protobuf:"bytes,4,opt,name=provider"`

	// SecretRef is tombstoned to show why 5 is reserved protobuf tag.
	// SecretRef *corev1.SecretReference `json:"secretRef,omitempty" protobuf:"bytes,5,opt,name=secretRef"`

	// Taints describes taints on the seed.
	// +optional
	Taints []SeedTaint `json:"taints,omitempty" protobuf:"bytes,6,rep,name=taints"`
	// Volume contains settings for persistentvolumes created in the seed cluster.
	// +optional
	Volume *SeedVolume `json:"volume,omitempty" protobuf:"bytes,7,opt,name=volume"`
	// Settings contains certain settings for this seed cluster.
	// +optional
	Settings *SeedSettings `json:"settings,omitempty" protobuf:"bytes,8,opt,name=settings"`
	// Ingress configures Ingress specific settings of the Seed cluster. This field is immutable.
	// +optional
	Ingress *Ingress `json:"ingress,omitempty" protobuf:"bytes,9,opt,name=ingress"`
	// AccessRestrictions describe a list of access restrictions for this seed cluster.
	// +optional
	AccessRestrictions []AccessRestriction `json:"accessRestrictions,omitempty" protobuf:"bytes,10,rep,name=accessRestrictions"`
	// Extensions contain type and provider information for Seed extensions.
	// +optional
	Extensions []Extension `json:"extensions,omitempty" protobuf:"bytes,11,rep,name=extensions"`
	// Resources holds a list of named resource references that can be referred to in extension configs by their names.
	// +optional
	Resources []NamedResourceReference `json:"resources,omitempty" protobuf:"bytes,12,rep,name=resources"`
}

// SeedStatus is the status of a Seed.
type SeedStatus struct {
	// Gardener holds information about the Gardener which last acted on the Shoot.
	// +optional
	Gardener *Gardener `json:"gardener,omitempty" protobuf:"bytes,1,opt,name=gardener"`
	// KubernetesVersion is the Kubernetes version of the seed cluster.
	// +optional
	KubernetesVersion *string `json:"kubernetesVersion,omitempty" protobuf:"bytes,2,opt,name=kubernetesVersion"`
	// Conditions represents the latest available observations of a Seed's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,3,rep,name=conditions"`
	// ObservedGeneration is the most recent generation observed for this Seed. It corresponds to the
	// Seed's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,4,opt,name=observedGeneration"`
	// ClusterIdentity is the identity of the Seed cluster. This field is immutable.
	// +optional
	ClusterIdentity *string `json:"clusterIdentity,omitempty" protobuf:"bytes,5,opt,name=clusterIdentity"`
	// Capacity represents the total resources of a seed.
	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty" protobuf:"bytes,6,rep,name=capacity"`
	// Allocatable represents the resources of a seed that are available for scheduling.
	// Defaults to Capacity.
	// +optional
	Allocatable corev1.ResourceList `json:"allocatable,omitempty" protobuf:"bytes,7,rep,name=allocatable"`
	// ClientCertificateExpirationTimestamp is the timestamp at which gardenlet's client certificate expires.
	// +optional
	ClientCertificateExpirationTimestamp *metav1.Time `json:"clientCertificateExpirationTimestamp,omitempty" protobuf:"bytes,8,opt,name=clientCertificateExpirationTimestamp"`
	// LastOperation holds information about the last operation on the Seed.
	// +optional
	LastOperation *LastOperation `json:"lastOperation,omitempty" protobuf:"bytes,9,opt,name=lastOperation"`
}

// Backup contains the object store configuration for backups for shoot (currently only etcd).
type Backup struct {
	// Provider is a provider name. This field is immutable.
	Provider string `json:"provider" protobuf:"bytes,1,opt,name=provider"`
	// ProviderConfig is the configuration passed to BackupBucket resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
	// Region is a region name. This field is immutable.
	// +optional
	Region *string `json:"region,omitempty" protobuf:"bytes,3,opt,name=region"`

	// SecretRef is tombstoned to show why 4 is reserved protobuf tag.
	// SecretRef corev1.SecretReference `json:"secretRef" protobuf:"bytes,4,opt,name=secretRef"`

	// CredentialsRef is reference to a resource holding the credentials used for
	// authentication with the object store service where the backups are stored.
	// Supported referenced resources are v1.Secrets and
	// security.gardener.cloud/v1alpha1.WorkloadIdentity
	// +optional
	CredentialsRef *corev1.ObjectReference `json:"credentialsRef,omitempty" protobuf:"bytes,5,opt,name=credentialsRef"`
}

// SeedDNS contains DNS-relevant information about this seed cluster.
type SeedDNS struct {
	// IngressDomain is tombstoned to show why 1 is reserved protobuf tag.
	// IngressDomain *string `json:"ingressDomain,omitempty" protobuf:"bytes,1,opt,name=ingressDomain"`

	// Provider configures a DNSProvider
	// +optional
	Provider *SeedDNSProvider `json:"provider,omitempty" protobuf:"bytes,2,opt,name=provider"`
}

// SeedDNSProvider configures a DNSProvider for Seeds
type SeedDNSProvider struct {
	// Type describes the type of the dns-provider, for example `aws-route53`
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// SecretRef is a reference to a Secret object containing cloud provider credentials used for registering external domains.
	SecretRef corev1.SecretReference `json:"secretRef" protobuf:"bytes,2,opt,name=secretRef"`

	// Domains is tombstoned to show why 3 is reserved protobuf tag.
	// Domains *DNSIncludeExclude `json:"domains,omitempty" protobuf:"bytes,3,opt,name=domains"`

	// Zones is tombstoned to show why 4 is reserved protobuf tag.
	// Zones *DNSIncludeExclude `json:"zones,omitempty" protobuf:"bytes,4,opt,name=zones"`
}

// Ingress configures the Ingress specific settings of the cluster
type Ingress struct {
	// Domain specifies the IngressDomain of the cluster pointing to the ingress controller endpoint. It will be used
	// to construct ingress URLs for system applications running in Shoot/Garden clusters. Once set this field is immutable.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
	Domain string `json:"domain" protobuf:"bytes,1,name=domain"`
	// Controller configures a Gardener managed Ingress Controller listening on the ingressDomain
	Controller IngressController `json:"controller" protobuf:"bytes,2,name=controller"`
}

// IngressController enables a Gardener managed Ingress Controller listening on the ingressDomain
type IngressController struct {
	// Kind defines which kind of IngressController to use. At the moment only `nginx` is supported
	// +kubebuilder:validation:Enum="nginx"
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// ProviderConfig specifies infrastructure specific configuration for the ingressController
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
}

// SeedNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
type SeedNetworks struct {
	// Nodes is the CIDR of the node network. This field is immutable.
	// +optional
	Nodes *string `json:"nodes,omitempty" protobuf:"bytes,1,opt,name=nodes"`
	// Pods is the CIDR of the pod network. This field is immutable.
	Pods string `json:"pods" protobuf:"bytes,2,opt,name=pods"`
	// Services is the CIDR of the service network. This field is immutable.
	Services string `json:"services" protobuf:"bytes,3,opt,name=services"`
	// ShootDefaults contains the default networks CIDRs for shoots.
	// +optional
	ShootDefaults *ShootNetworks `json:"shootDefaults,omitempty" protobuf:"bytes,4,opt,name=shootDefaults"`
	// BlockCIDRs is a list of network addresses that should be blocked for shoot control plane components running
	// in the seed cluster.
	// +optional
	BlockCIDRs []string `json:"blockCIDRs,omitempty" protobuf:"bytes,5,rep,name=blockCIDRs"`
	// IPFamilies specifies the IP protocol versions to use for seed networking. This field is immutable.
	// See https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md.
	// Defaults to ["IPv4"].
	// +optional
	IPFamilies []IPFamily `json:"ipFamilies,omitempty" protobuf:"bytes,6,rep,name=ipFamilies,casttype=IPFamily"`
}

// ShootNetworks contains the default networks CIDRs for shoots.
type ShootNetworks struct {
	// Pods is the CIDR of the pod network.
	// +optional
	Pods *string `json:"pods,omitempty" protobuf:"bytes,1,opt,name=pods"`
	// Services is the CIDR of the service network.
	// +optional
	Services *string `json:"services,omitempty" protobuf:"bytes,2,opt,name=services"`
}

// SeedProvider defines the provider-specific information of this Seed cluster.
type SeedProvider struct {
	// Type is the name of the provider.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// ProviderConfig is the configuration passed to Seed resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
	// Region is a name of a region.
	Region string `json:"region" protobuf:"bytes,3,opt,name=region"`
	// Zones is the list of availability zones the seed cluster is deployed to.
	// +optional
	Zones []string `json:"zones,omitempty" protobuf:"bytes,4,rep,name=zones"`
}

// SeedSettings contains certain settings for this seed cluster.
type SeedSettings struct {
	// ExcessCapacityReservation controls the excess capacity reservation for shoot control planes in the seed.
	// +optional
	ExcessCapacityReservation *SeedSettingExcessCapacityReservation `json:"excessCapacityReservation,omitempty" protobuf:"bytes,1,opt,name=excessCapacityReservation"`
	// Scheduling controls settings for scheduling decisions for the seed.
	// +optional
	Scheduling *SeedSettingScheduling `json:"scheduling,omitempty" protobuf:"bytes,2,opt,name=scheduling"`

	// ShootDNS is tombstoned to show why 3 is reserved protobuf tag.
	// ShootDNS *SeedSettingShootDNS `json:"shootDNS,omitempty" protobuf:"bytes,3,opt,name=shootDNS"`

	// LoadBalancerServices controls certain settings for services of type load balancer that are created in the seed.
	// +optional
	LoadBalancerServices *SeedSettingLoadBalancerServices `json:"loadBalancerServices,omitempty" protobuf:"bytes,4,opt,name=loadBalancerServices"`
	// VerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the seed.
	// +optional
	VerticalPodAutoscaler *SeedSettingVerticalPodAutoscaler `json:"verticalPodAutoscaler,omitempty" protobuf:"bytes,5,opt,name=verticalPodAutoscaler"`

	// OwnerChecks is tombstoned to show why 6 is reserved protobuf tag.
	// OwnerChecks *SeedSettingOwnerChecks `json:"ownerChecks,omitempty" protobuf:"bytes,6,opt,name=ownerChecks"`

	// DependencyWatchdog controls certain settings for the dependency-watchdog components deployed in the seed.
	// +optional
	DependencyWatchdog *SeedSettingDependencyWatchdog `json:"dependencyWatchdog,omitempty" protobuf:"bytes,7,opt,name=dependencyWatchdog"`
	// TopologyAwareRouting controls certain settings for topology-aware traffic routing in the seed.
	// See https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md.
	// +optional
	TopologyAwareRouting *SeedSettingTopologyAwareRouting `json:"topologyAwareRouting,omitempty" protobuf:"bytes,8,opt,name=topologyAwareRouting"`
}

// SeedSettingExcessCapacityReservation controls the excess capacity reservation for shoot control planes in the seed.
type SeedSettingExcessCapacityReservation struct {
	// Enabled controls whether the default excess capacity reservation should be enabled. When not specified, the functionality is enabled.
	// +optional
	Enabled *bool `json:"enabled,omitempty" protobuf:"bytes,1,opt,name=enabled"`
	// Configs configures excess capacity reservation deployments for shoot control planes in the seed.
	// +optional
	Configs []SeedSettingExcessCapacityReservationConfig `json:"configs,omitempty" protobuf:"bytes,2,rep,name=configs"`
}

// SeedSettingExcessCapacityReservationConfig configures excess capacity reservation deployments for shoot control planes in the seed.
type SeedSettingExcessCapacityReservationConfig struct {
	// Resources specify the resource requests and limits of the excess-capacity-reservation pod.
	Resources corev1.ResourceList `json:"resources" protobuf:"bytes,1,rep,name=resources,casttype=k8s.io/api/core/v1.ResourceList,castkey=k8s.io/api/core/v1.ResourceName"`
	// NodeSelector specifies the node where the excess-capacity-reservation pod should run.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,2,rep,name=nodeSelector"`
	// Tolerations specify the tolerations for the the excess-capacity-reservation pod.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty" protobuf:"bytes,3,rep,name=tolerations"`
}

// SeedSettingScheduling controls settings for scheduling decisions for the seed.
type SeedSettingScheduling struct {
	// Visible controls whether the gardener-scheduler shall consider this seed when scheduling shoots. Invisible seeds
	// are not considered by the scheduler.
	Visible bool `json:"visible" protobuf:"bytes,1,opt,name=visible"`
}

// SeedSettingLoadBalancerServices controls certain settings for services of type load balancer that are created in the
// seed.
type SeedSettingLoadBalancerServices struct {
	// Annotations is a map of annotations that will be injected/merged into every load balancer service object.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,1,rep,name=annotations"`
	// ExternalTrafficPolicy describes how nodes distribute service traffic they
	// receive on one of the service's "externally-facing" addresses.
	// Defaults to "Cluster".
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty" protobuf:"bytes,2,opt,name=externalTrafficPolicy"`
	// Zones controls settings, which are specific to the single-zone load balancers in a multi-zonal setup.
	// Can be empty for single-zone seeds. Each specified zone has to relate to one of the zones in seed.spec.provider.zones.
	// +optional
	Zones []SeedSettingLoadBalancerServicesZones `json:"zones,omitempty" protobuf:"bytes,3,rep,name=zones"`
	// ProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.
	// Defaults to nil, which is equivalent to not allowing ProxyProtocol.
	// +optional
	ProxyProtocol *LoadBalancerServicesProxyProtocol `json:"proxyProtocol,omitempty" protobuf:"bytes,4,opt,name=proxyProtocol"`
}

// SeedSettingLoadBalancerServicesZones controls settings, which are specific to the single-zone load balancers in a
// multi-zonal setup.
type SeedSettingLoadBalancerServicesZones struct {
	// Name is the name of the zone as specified in seed.spec.provider.zones.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Annotations is a map of annotations that will be injected/merged into the zone-specific load balancer service object.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,2,rep,name=annotations"`
	// ExternalTrafficPolicy describes how nodes distribute service traffic they
	// receive on one of the service's "externally-facing" addresses.
	// Defaults to "Cluster".
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty" protobuf:"bytes,3,opt,name=externalTrafficPolicy"`
	// ProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.
	// Defaults to nil, which is equivalent to not allowing ProxyProtocol.
	// +optional
	ProxyProtocol *LoadBalancerServicesProxyProtocol `json:"proxyProtocol,omitempty" protobuf:"bytes,4,opt,name=proxyProtocol"`
}

// LoadBalancerServicesProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.
type LoadBalancerServicesProxyProtocol struct {
	// Allowed controls whether the ProxyProtocol is optionally allowed for the load balancer services.
	// This should only be enabled if the load balancer services are already using ProxyProtocol or will be reconfigured to use it soon.
	// Until the load balancers are configured with ProxyProtocol, enabling this setting may allow clients to spoof their source IP addresses.
	// The option allows a migration from non-ProxyProtocol to ProxyProtocol without downtime (depending on the infrastructure).
	// Defaults to false.
	Allowed bool `json:"allowed" protobuf:"bytes,1,opt,name=allowed"`
}

// SeedSettingVerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
// seed.
type SeedSettingVerticalPodAutoscaler struct {
	// Enabled controls whether the VPA components shall be deployed into the garden namespace in the seed cluster. It
	// is enabled by default because Gardener heavily relies on a VPA being deployed. You should only disable this if
	// your seed cluster already has another, manually/custom managed VPA deployment.
	Enabled bool `json:"enabled" protobuf:"bytes,1,opt,name=enabled"`
}

// SeedSettingDependencyWatchdog controls the dependency-watchdog settings for the seed.
type SeedSettingDependencyWatchdog struct {
	// Endpoint is tombstoned to show why 1 is reserved protobuf tag.
	// Endpoint *SeedSettingDependencyWatchdogEndpoint `json:"endpoint,omitempty" protobuf:"bytes,1,opt,name=endpoint"`
	// Probe is tombstoned to show why 2 is reserved protobuf tag.
	// Probe *SeedSettingDependencyWatchdogProbe `json:"probe,omitempty" protobuf:"bytes,2,opt,name=probe"`

	// Weeder controls the weeder settings for the dependency-watchdog for the seed.
	// +optional
	Weeder *SeedSettingDependencyWatchdogWeeder `json:"weeder,omitempty" protobuf:"bytes,3,opt,name=weeder"`
	// Prober controls the prober settings for the dependency-watchdog for the seed.
	// +optional
	Prober *SeedSettingDependencyWatchdogProber `json:"prober,omitempty" protobuf:"bytes,4,opt,name=prober"`
}

// SeedSettingDependencyWatchdogWeeder controls the weeder settings for the dependency-watchdog for the seed.
type SeedSettingDependencyWatchdogWeeder struct {
	// Enabled controls whether the endpoint controller(weeder) of the dependency-watchdog should be enabled. This controller
	// helps to alleviate the delay where control plane components remain unavailable by finding the respective pods in
	// CrashLoopBackoff status and restarting them once their dependants become ready and available again.
	Enabled bool `json:"enabled" protobuf:"bytes,1,opt,name=enabled"`
}

// SeedSettingDependencyWatchdogProber controls the prober settings for the dependency-watchdog for the seed.
type SeedSettingDependencyWatchdogProber struct {
	// Enabled controls whether the probe controller(prober) of the dependency-watchdog should be enabled. This controller
	// scales down the kube-controller-manager, machine-controller-manager and cluster-autoscaler of shoot clusters in case their respective kube-apiserver is not
	// reachable via its external ingress in order to avoid melt-down situations.
	Enabled bool `json:"enabled" protobuf:"bytes,1,opt,name=enabled"`
}

// SeedSettingTopologyAwareRouting controls certain settings for topology-aware traffic routing in the seed.
// See https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md.
type SeedSettingTopologyAwareRouting struct {
	// Enabled controls whether certain Services deployed in the seed cluster should be topology-aware.
	// These Services are etcd-main-client, etcd-events-client, kube-apiserver, gardener-resource-manager and vpa-webhook.
	Enabled bool `json:"enabled" protobuf:"bytes,1,opt,name=enabled"`
}

// SeedTaint describes a taint on a seed.
type SeedTaint struct {
	// Key is the taint key to be applied to a seed.
	Key string `json:"key" protobuf:"bytes,1,opt,name=key"`
	// Value is the taint value corresponding to the taint key.
	// +optional
	Value *string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
}

const (
	// SeedTaintProtected is a constant for a taint key on a seed that marks it as protected. Protected seeds
	// may only be used by shoots in the `garden` namespace.
	SeedTaintProtected = "seed.gardener.cloud/protected"
)

// SeedVolume contains settings for persistentvolumes created in the seed cluster.
type SeedVolume struct {
	// MinimumSize defines the minimum size that should be used for PVCs in the seed.
	// +optional
	MinimumSize *resource.Quantity `json:"minimumSize,omitempty" protobuf:"bytes,1,opt,name=minimumSize"`
	// Providers is a list of storage class provisioner types for the seed.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	Providers []SeedVolumeProvider `json:"providers,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,2,rep,name=providers"`
}

// SeedVolumeProvider is a storage class provisioner type.
type SeedVolumeProvider struct {
	// Purpose is the purpose of this provider.
	Purpose string `json:"purpose" protobuf:"bytes,1,opt,name=purpose"`
	// Name is the name of the storage class provisioner type.
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
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
