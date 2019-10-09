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

package garden

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

////////////////////////////////////////////////////
//                  CLOUD PROFILES                //
////////////////////////////////////////////////////

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfile represents certain properties about a cloud environment.
type CloudProfile struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec defines the cloud environment properties.
	Spec CloudProfileSpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileList is a collection of CloudProfiles.
type CloudProfileList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of CloudProfiles.
	Items []CloudProfile
}

// CloudProfileSpec is the specification of a CloudProfile.
// It must contain exactly one of its defined keys.
type CloudProfileSpec struct {
	// AWS is the profile specification for the Amazon Web Services cloud.
	AWS *AWSProfile
	// Azure is the profile specification for the Microsoft Azure cloud.
	Azure *AzureProfile
	// GCP is the profile specification for the Google Cloud Platform cloud.
	GCP *GCPProfile
	// OpenStack is the profile specification for the OpenStack cloud.
	OpenStack *OpenStackProfile
	// Alicloud is the profile specification for the Alibaba cloud.
	Alicloud *AlicloudProfile
	// Packet is the profile specification for the Packet cloud.
	Packet *PacketProfile
	// CABundle is a certificate bundle which will be installed onto every host machine of the Shoot cluster.
	CABundle *string
	//
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesSettings
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []CloudProfileMachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// ProviderConfig contains provider-specific configuration for the profile.
	ProviderConfig *ProviderConfig
	// Regions contains constraints regarding allowed values for regions and zones.
	Regions []Region
	// SeedSelector contains an optional list of labels on `Seed` resources that marks those seeds whose shoots may use this provider profile.
	// An empty list means that all seeds of the same provider type are supported.
	// This is useful for environments that are of the same type (like openstack) but may have different "instances"/landscapes.
	SeedSelector *metav1.LabelSelector
	// Type is the name of the provider.
	Type string
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
}

// KubernetesSettings contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
type KubernetesSettings struct {
	// Versions is the list of allowed Kubernetes versions with optional expiration dates for Shoot clusters.
	Versions []ExpirableVersion
}

// CloudProfileMachineImage defines the name and multiple versions of the machine image in any environment.
type CloudProfileMachineImage struct {
	// Name is the name of the image.
	Name string
	// Versions contains versions and expiration dates of the machine image
	Versions []ExpirableVersion
}

// ExpirableVersion contains a version and an expiration date.
type ExpirableVersion struct {
	// Version is the version identifier.
	Version string
	// ExpirationDate defines the time at which this version expires.
	ExpirationDate *metav1.Time
}

// Region contains certain properties of a region.
type Region struct {
	// Name is a region name.
	Name string
	// Zones is a list of availability zones in this region.
	Zones []AvailabilityZone
}

// AvailabilityZone is an availability zone.
type AvailabilityZone struct {
	// Name is an an availability zone name.
	Name string
	// UnavailableMachineTypes is a list of machine type names that are not availability in this zone.
	UnavailableMachineTypes []string
	// UnavailableVolumeTypes is a list of volume type names that are not availability in this zone.
	UnavailableVolumeTypes []string
}

// AWSProfile defines certain constraints and definitions for the AWS cloud.
type AWSProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints AWSConstraints
}

// AWSConstraints is an object containing constraints for certain values in the Shoot specification.
type AWSConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []MachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// MachineImage defines the name and multiple versions of the machine image in any environment.
type MachineImage struct {
	// Name is the name of the image.
	Name string
	// Versions contains versions and expiration dates of the machine image
	Versions []MachineImageVersion
}

// MachineImageVersion contains a version and an expiration date of a machine image
type MachineImageVersion struct {
	// Version is the version of the image.
	Version string
	// ExpirationDate defines the time at which a shoot that opted out of automatic operating system updates and
	// that is running this image version will be forcefully updated to the latest version specified in the referenced
	// cloud profile.
	ExpirationDate *metav1.Time
}

// AzureProfile defines certain constraints and definitions for the Azure cloud.
type AzureProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints AzureConstraints
	// CountUpdateDomains is list of Azure update domain counts for each region.
	CountUpdateDomains []AzureDomainCount
	// CountFaultDomains is list of Azure fault domain counts for each region.
	CountFaultDomains []AzureDomainCount
}

// AzureConstraints is an object containing constraints for certain values in the Shoot specification.
type AzureConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []MachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// AzureDomainCount defines the region and the count for this domain count value.
type AzureDomainCount struct {
	// Region is a region in Azure.
	Region string
	// Count is the count value for the respective domain count.
	Count int
}

// GCPProfile defines certain constraints and definitions for the GCP cloud.
type GCPProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints GCPConstraints
}

// GCPConstraints is an object containing constraints for certain values in the Shoot specification.
type GCPConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []MachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// OpenStackProfile defines certain constraints and definitions for the OpenStack cloud.
type OpenStackProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints OpenStackConstraints
	// KeyStoneURL is the URL for auth{n,z} in OpenStack (pointing to KeyStone).
	KeyStoneURL string
	// DNSServers is a list of IPs of DNS servers used while creating subnets.
	DNSServers []string
	// DHCPDomain is the dhcp domain of the OpenStack system configured in nova.conf. Only meaningful for
	// Kubernetes 1.10.1+. See https://github.com/kubernetes/kubernetes/pull/61890 for details.
	DHCPDomain *string
	// RequestTimeout specifies the HTTP timeout against the OpenStack API.
	RequestTimeout *string
}

// OpenStackConstraints is an object containing constraints for certain values in the Shoot specification.
type OpenStackConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint
	// FloatingPools contains constraints regarding allowed values of the 'floatingPoolName' block in the Shoot specification.
	FloatingPools []OpenStackFloatingPool
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints
	// LoadBalancerProviders contains constraints regarding allowed values of the 'loadBalancerProvider' block in the Shoot specification.
	LoadBalancerProviders []OpenStackLoadBalancerProvider
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []MachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []OpenStackMachineType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// OpenStackFloatingPool contains constraints regarding allowed values of the 'floatingPoolName' block in the Shoot specification.
type OpenStackFloatingPool struct {
	// Name is the name of the floating pool.
	Name string
	// LoadBalancerClasses contains a list of supported labeled load balancer network settings.
	LoadBalancerClasses []OpenStackLoadBalancerClass
}

// OpenStackLoadBalancerProvider contains constraints regarding allowed values of the 'loadBalancerProvider' block in the Shoot specification.
type OpenStackLoadBalancerProvider struct {
	// Name is the name of the load balancer provider.
	Name string
}

// AlicloudProfile defines constraints and definitions in Alibaba Cloud environment.
type AlicloudProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints AlicloudConstraints
}

// AlicloudConstraints is an object containing constraints for certain values in the Shoot specification
type AlicloudConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []MachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []AlicloudMachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []AlicloudVolumeType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// AlicloudMachineType defines certain machine types and zone constraints.
type AlicloudMachineType struct {
	MachineType
	Zones []string
}

// AlicloudVolumeType defines certain volume types and zone constraints.
type AlicloudVolumeType struct {
	VolumeType
	Zones []string
}

// PacketProfile defines constraints and definitions in Packet Cloud environment.
type PacketProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints PacketConstraints
}

// PacketConstraints is an object containing constraints for certain values in the Shoot specification
type PacketConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []MachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// DNSProviderConstraint contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
type DNSProviderConstraint struct {
	// Name is the name of the DNS provider.
	Name string
}

// KubernetesConstraints contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
type KubernetesConstraints struct {
	// OfferedVersions is the list of allowed Kubernetes versions with optional expiration dates for Shoot clusters
	OfferedVersions []KubernetesVersion
}

// KubernetesVersion contains the version code and optional expiration date for a kubernetes version
type KubernetesVersion struct {
	// Version is the kubernetes version
	Version string
	// ExpirationDate defines the time at which this kubernetes version is not supported any more. This has the following implications:
	// 1) A shoot that opted out of automatic kubernetes system updates and that is running this kubernetes version will be forcefully updated to the latest kubernetes patch version for the current minor version
	// 2) Shoot's with this kubernetes version cannot be created
	ExpirationDate *metav1.Time
}

// MachineType contains certain properties of a machine type.
type MachineType struct {
	// Name is the name of the machine type.
	Name string
	// Usable defines if the machine type can be used for shoot clusters.
	Usable *bool
	// CPU is the number of CPUs for this machine type.
	CPU resource.Quantity
	// GPU is the number of GPUs for this machine type.
	GPU resource.Quantity
	// Storage is the amount of storage associated with the root volume of this machine type.
	Storage *MachineTypeStorage
	// Memory is the amount of memory for this machine type.
	Memory resource.Quantity
}

// MachineTypeStorage is the amount of storage associated with the root volume of this machine type.
type MachineTypeStorage struct {
	// Size is the storage size.
	Size resource.Quantity
	// Type is the type of the storage.
	Type string
}

// OpenStackMachineType contains certain properties of a machine type in OpenStack
type OpenStackMachineType struct {
	MachineType
	// VolumeType is the type of that volume.
	VolumeType string
	// VolumeSize is the amount of disk storage for this machine type.
	VolumeSize resource.Quantity
}

// VolumeType contains certain properties of a volume type.
type VolumeType struct {
	// Name is the name of the volume type.
	Name string
	// Usable defines if the volume type can be used for shoot clusters.
	Usable *bool
	// Class is the class of the volume type.
	Class string
}

const (
	// VolumeClassStandard is a constant for the standard volume class.
	VolumeClassStandard string = "standard"
	// VolumeClassPremium is a constant for the premium volume class.
	VolumeClassPremium string = "premium"
)

// Zone contains certain properties of an availability zone.
type Zone struct {
	// Region is a region name.
	Region string
	// Names is a list of availability zone names in this region.
	Names []string
}

// SeedBackup contains the object store configuration for backups for shoot(currently only etcd).
type SeedBackup struct {
	// Provider is a provider name.
	Provider CloudProvider
	// Region is a region name.
	Region *string
	// SecretRef is a reference to a Secret object containing the cloud provider credentials for
	// the object store where backups should be stored. It should have enough privileges to manipulate
	// the objects as well as buckets.
	SecretRef corev1.SecretReference
}

////////////////////////////////////////////////////
//                    PROJECTS                    //
////////////////////////////////////////////////////

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Project holds certain properties about a Gardener project.
type Project struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec defines the project properties.
	Spec ProjectSpec
	// Most recently observed status of the Project.
	Status ProjectStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectList is a collection of Projects.
type ProjectList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of Projects.
	Items []Project
}

// ProjectSpec is the specification of a Project.
type ProjectSpec struct {
	// CreatedBy is a subject representing a user name, an email address, or any other identifier of a user
	// who created the project.
	CreatedBy *rbacv1.Subject
	// Description is a human-readable description of what the project is used for.
	Description *string
	// Owner is a subject representing a user name, an email address, or any other identifier of a user owning
	// the project.
	Owner *rbacv1.Subject
	// Purpose is a human-readable explanation of the project's purpose.
	Purpose *string
	// Members is a list of subjects representing a user name, an email address, or any other identifier of a user,
	// group, or service account that has a certain role.
	ProjectMembers []ProjectMember
	// Namespace is the name of the namespace that has been created for the Project object.
	Namespace *string
}

// ProjectMember is a member of a project.
type ProjectMember struct {
	// Subject is representing a user name, an email address, or any other identifier of a user, group, or service
	// account that has a certain role.
	rbacv1.Subject
	// Role represents the role of this member.
	Role string
}

const (
	// ProjectMemberAdmin is a const for a role that provides full admin access.
	ProjectMemberAdmin = "admin"
	// ProjectMemberViewer is a const for a role that provides limited permissions to only view some resources.
	ProjectMemberViewer = "viewer"
)

// ProjectStatus holds the most recently observed status of the project.
type ProjectStatus struct {
	// ObservedGeneration is the most recent generation observed for this project.
	ObservedGeneration int64
	// Phase is the current phase of the project.
	Phase ProjectPhase
}

// ProjectPhase is a label for the condition of a project at the current time.
type ProjectPhase string

const (
	// ProjectPending indicates that the project reconciliation is pending.
	ProjectPending ProjectPhase = "Pending"
	// ProjectReady indicates that the project reconciliation was successful.
	ProjectReady ProjectPhase = "Ready"
	// ProjectFailed indicates that the project reconciliation failed.
	ProjectFailed ProjectPhase = "Failed"
	// ProjectTerminating indicates that the project is in termination process.
	ProjectTerminating ProjectPhase = "Terminating"
)

////////////////////////////////////////////////////
//                      SEEDS                     //
////////////////////////////////////////////////////

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Seed holds certain properties about a Seed cluster.
type Seed struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec defines the Seed cluster properties.
	Spec SeedSpec
	// Most recently observed status of the Seed cluster.
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

// SeedSpec is the specification of a Seed.
type SeedSpec struct {
	// Cloud defines the cloud profile and the region this Seed cluster belongs to.
	Cloud SeedCloud
	// Provider defines the provider type and region for this Seed cluster.
	Provider SeedProvider
	// IngressDomain is the domain of the Seed cluster pointing to the ingress controller endpoint. It will be used
	// to construct ingress URLs for system applications running in Shoot clusters.
	IngressDomain string
	// SecretRef is a reference to a Secret object containing the Kubeconfig and the cloud provider credentials for
	// the account the Seed cluster has been deployed to.
	SecretRef corev1.SecretReference
	// Networks defines the pod, service and worker network of the Seed cluster.
	Networks SeedNetworks
	// BlockCIDRs is a list of network addresses tha should be blocked for shoot control plane components running
	// in the seed cluster.
	BlockCIDRs []string
	// Taints describes taints on the seed.
	Taints []SeedTaint
	// Backup holds the object store configuration for the backups of shoot(currently only etcd).
	// If it is not specified, then there won't be any backups taken for Shoots associated with this Seed.
	// If backup field is present in Seed, then backups of the etcd from Shoot controlplane will be stored under the
	// configured object store.
	Backup *SeedBackup
	// Volume contains settings for persistentvolumes created in the seed cluster.
	Volume *SeedVolume
}

const (
	MigrationSeedCloudProfile      = "migration.seed.gardener.cloud/cloudProfile"
	MigrationSeedCloudRegion       = "migration.seed.gardener.cloud/cloudRegion"
	MigrationSeedProviderType      = "migration.seed.gardener.cloud/providerType"
	MigrationSeedProviderRegion    = "migration.seed.gardener.cloud/providerRegion"
	MigrationSeedVolumeMinimumSize = "migration.seed.gardener.cloud/volumeMinimumSize"
	MigrationSeedVolumeProviders   = "migration.seed.gardener.cloud/volumeProviders"
	MigrationSeedTaints            = "migration.seed.gardener.cloud/taints"

	MigrationCloudProfileType           = "migration.cloudprofile.gardener.cloud/type"
	MigrationCloudProfileProviderConfig = "migration.cloudprofile.gardener.cloud/providerConfig"
	MigrationCloudProfileSeedSelector   = "migration.cloudprofile.gardener.cloud/seedSelector"
	MigrationCloudProfileDNSProviders   = "migration.cloudprofile.gardener.cloud/dnsProviders"
	MigrationCloudProfileRegions        = "migration.cloudprofile.gardener.cloud/regions"
	MigrationCloudProfileVolumeTypes    = "migration.cloudprofile.gardener.cloud/volumeTypes"
	MigrationCloudProfileKubernetes     = "migration.cloudprofile.gardener.cloud/kubernetes"
	MigrationCloudProfileMachineImages  = "migration.cloudprofile.gardener.cloud/machineImages"
	MigrationCloudProfileMachineTypes   = "migration.cloudprofile.gardener.cloud/machineTypes"
)

// SeedStatus holds the most recently observed status of the Seed cluster.
type SeedStatus struct {
	// Conditions represents the latest available observations of a Seed's current state.
	Conditions []Condition
	// Gardener holds information about the Gardener which last acted on the Seed.
	Gardener Gardener
	// ObservedGeneration is the most recent generation observed for this Seed. It corresponds to the
	// Seed's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64
}

// SeedCloud defines the cloud profile and the region this Seed cluster belongs to.
type SeedCloud struct {
	// Profile is the name of a cloud profile.
	Profile string
	// Region is a name of a region.
	Region string
}

// SeedProvider defines the provider type and region for this Seed cluster.
type SeedProvider struct {
	// Type is the name of the provider.
	Type string
	// Region is a name of a region.
	Region string
}

// Volume contains settings for persistentvolumes created in the seed cluster.
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

// SeedVolumeProviderPurposeEtcdMain is a constant for the etcd-main volume provider purpose.
const SeedVolumeProviderPurposeEtcdMain = "etcd-main"

// SeedNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
type SeedNetworks struct {
	// Nodes is the CIDR of the node network.
	Nodes string
	// Pods is the CIDR of the pod network.
	Pods string
	// Services is the CIDR of the service network.
	Services string
	// ShootDefaults contains the default networks CIDRs for shoots.
	ShootDefaults *ShootNetworks
}

// ShootNetworks contains the default networks CIDRs for shoots.
type ShootNetworks struct {
	// Pods is the CIDR of the pod network.
	Pods *string
	// Services is the CIDR of the service network.
	Services *string
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
	// SeedTaintInvisible is a constant for a taint key on a seed that marks it as invisible. Invisible seeds
	// are not considered by the gardener-scheduler.
	SeedTaintInvisible = "seed.gardener.cloud/invisible"
)

////////////////////////////////////////////////////
//                      QUOTAS                    //
////////////////////////////////////////////////////

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Quota holds certain information about resource usage limitations and lifetime for Shoot objects.
type Quota struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec defines the Quota constraints.
	Spec QuotaSpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuotaList is a collection of Quotas.
type QuotaList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of Quotas.
	Items []Quota
}

// QuotaSpec is the specification of a Quota.
type QuotaSpec struct {
	// ClusterLifetimeDays is the lifetime of a Shoot cluster in days before it will be terminated automatically.
	ClusterLifetimeDays *int
	// Metrics is a list of resources which will be put under constraints.
	Metrics corev1.ResourceList
	// Scope is reference to an object version/kind.
	Scope corev1.ObjectReference
}

const (
	// QuotaMetricCPU is the constraint for the amount of CPUs
	QuotaMetricCPU corev1.ResourceName = corev1.ResourceCPU
	// QuotaMetricGPU is the constraint for the amount of GPUs (e.g. from Nvidia)
	QuotaMetricGPU corev1.ResourceName = "gpu"
	// QuotaMetricMemory is the constraint for the amount of memory
	QuotaMetricMemory corev1.ResourceName = corev1.ResourceMemory
	// QuotaMetricStorageStandard is the constraint for the size of a standard disk
	QuotaMetricStorageStandard corev1.ResourceName = corev1.ResourceStorage + ".standard"
	// QuotaMetricStoragePremium is the constraint for the size of a premium disk (e.g. SSD)
	QuotaMetricStoragePremium corev1.ResourceName = corev1.ResourceStorage + ".premium"
	// QuotaMetricLoadbalancer is the constraint for the amount of loadbalancers
	QuotaMetricLoadbalancer corev1.ResourceName = "loadbalancer"
)

// QuotaScope is a string alias.
type QuotaScope string

const (
	// QuotaScopeProject indicates that the scope of a Quota object is a project.
	QuotaScopeProject QuotaScope = "project"
	// QuotaScopeSecret indicates that the scope of a Quota object is a cloud provider secret.
	QuotaScopeSecret QuotaScope = "secret"
)

////////////////////////////////////////////////////
//                 SECRET BINDINGS                //
////////////////////////////////////////////////////

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SecretBinding struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// SecretRef is a reference to a secret object in the same or another namespace.
	SecretRef corev1.SecretReference
	// Quotas is a list of references to Quota objects in the same or another namespace.
	Quotas []corev1.ObjectReference
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretBindingList is a collection of SecretBindings.
type SecretBindingList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of SecretBindings.
	Items []SecretBinding
}

////////////////////////////////////////////////////
//                      SHOOTS                    //
////////////////////////////////////////////////////

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Shoot struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Specification of the Shoot cluster.
	Spec ShootSpec
	// Most recently observed status of the Shoot cluster.
	Status ShootStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootList is a list of Shoot objects.
type ShootList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of Shoots.
	Items []Shoot
}

// ShootSpec is the specification of a Shoot.
type ShootSpec struct {
	// Addons contains information about enabled/disabled addons and their configuration.
	Addons *Addons
	// Cloud contains information about the cloud environment and their specific settings.
	Cloud Cloud
	// CloudProfileName is a name of a CloudProfile object.
	CloudProfileName string
	// DNS contains information about the DNS settings of the Shoot.
	DNS *DNS
	// Extensions contain type and provider information for Shoot extensions.
	Extensions []Extension
	// Hibernation contains information whether the Shoot is suspended or not.
	Hibernation *Hibernation
	// Kubernetes contains the version and configuration settings of the control plane components.
	Kubernetes Kubernetes
	// Networking contains information about cluster networking such as CNI Plugin type, CIDRs, ...etc.
	Networking Networking
	// Maintenance contains information about the time window for maintenance operations and which
	// operations should be performed.
	Maintenance *Maintenance
	// Provider contains all provider-specific and provider-relevant information.
	Provider Provider
	// Region is a name of a region.
	Region string
	// SecretBindingName is the name of the a SecretBinding that has a reference to the provider secret.
	// The credentials inside the provider secret will be used to create the shoot in the respective account.
	SecretBindingName string
	// SeedName is the name of the seed cluster that runs the control plane of the Shoot.
	SeedName *string
}

const (
	MigrationShootCloudControllerManager  = "migration.shoot.gardener.cloud/cloudControllerManager"
	MigrationShootDNSProviders            = "migration.shoot.gardener.cloud/dnsProviders"
	MigrationShootGlobalMachineImage      = "migration.shoot.gardener.cloud/globalMachineImage"
	MigrationShootProvider                = "migration.shoot.gardener.cloud/provider"
	MigrationShootWorkers                 = "migration.shoot.gardener.cloud/workers"
	MigrationShootAddonsClusterAutoscaler = "migration.shoot.gardener.cloud/addonsClusterAutoscaler"
	MigrationShootAddonsHeapster          = "migration.shoot.gardener.cloud/addonsHeapster"
	MigrationShootAddonsKubeLego          = "migration.shoot.gardener.cloud/addonsKubeLego"
	MigrationShootAddonsKube2IAM          = "migration.shoot.gardener.cloud/addonsKube2IAM"
	MigrationShootAddonsMonocular         = "migration.shoot.gardener.cloud/addonsMonocular"
)

// ShootStatus holds the most recently observed status of the Shoot cluster.
type ShootStatus struct {
	// Conditions represents the latest available observations of a Shoots's current state.
	Conditions []Condition
	// Gardener holds information about the Gardener which last acted on the Shoot.
	Gardener Gardener
	// LastOperation holds information about the last operation on the Shoot.
	LastOperation *LastOperation
	// LastError holds information about the last occurred error during an operation.
	LastError *LastError
	// ObservedGeneration is the most recent generation observed for this Shoot. It corresponds to the
	// Shoot's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64
	// RetryCycleStartTime is the start time of the last retry cycle (used to determine how often an operation
	// must be retried until we give up).
	RetryCycleStartTime *metav1.Time
	// Seed is the name of the seed cluster that runs the control plane of the Shoot. This value is only written
	// after a successful create/reconcile operation. It will be used when control planes are moved between Seeds.
	Seed *string
	// IsHibernated indicates whether the Shoot is currently hibernated.
	IsHibernated *bool
	// TechnicalID is the name that is used for creating the Seed namespace, the infrastructure resources, and
	// basically everything that is related to this particular Shoot.
	TechnicalID string
	// UID is a unique identifier for the Shoot cluster to avoid portability between Kubernetes clusters.
	// It is used to compute unique hashes.
	UID types.UID
}

///////////////////////////////
// Shoot Specification Types //
///////////////////////////////

// CalicoNetworkType is a constant for the calico network type.
const CalicoNetworkType = "calico"

// Networking defines networking parameters for the shoot cluster.
type Networking struct {
	// Type identifies the type of the networking plugin
	Type string
	// ProviderConfig is the configuration passed to network resource.
	ProviderConfig *ProviderConfig
	// Pods is the CIDR of the pod network.
	Pods *string
	// Nodes is the CIDR of the entire node network.
	Nodes string
	// Services is the CIDR of the service network.
	Services *string
}

// Cloud contains information about the cloud environment and their specific settings.
// It must contain exactly one key of the below cloud providers.
type Cloud struct {
	// Profile is a name of a CloudProfile object.
	Profile string
	// Region is a name of a cloud provider region.
	Region string
	// SecretBindingRef is a reference to a SecretBinding object.
	SecretBindingRef corev1.LocalObjectReference
	// Seed is the name of a Seed object.
	Seed *string
	// AWS contains the Shoot specification for the Amazon Web Services cloud.
	AWS *AWSCloud
	// Azure contains the Shoot specification for the Microsoft Azure cloud.
	Azure *AzureCloud
	// GCP contains the Shoot specification for the Google Cloud Platform cloud.
	GCP *GCPCloud
	// OpenStack contains the Shoot specification for the OpenStack cloud.
	OpenStack *OpenStackCloud
	// Alicloud contains the Shoot specification for the Alibaba cloud.
	Alicloud *Alicloud
	// PacketCloud contains the Shoot specification for the Packet cloud.
	Packet *PacketCloud
}

// AWSCloud contains the Shoot specification for AWS.

type AWSCloud struct {
	// ShootMachineImage holds information about the machine image to use for all workers.
	// It will default to the latest version of the first image stated in the referenced CloudProfile if no
	// value has been provided.
	MachineImage *ShootMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AWSNetworks
	// Workers is a list of worker groups.
	Workers []Worker
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string
}

// AWSNetworks holds information about the Kubernetes and infrastructure networks.
type AWSNetworks struct {
	K8SNetworks
	// VPC indicates whether to use an existing VPC or create a new one.
	VPC AWSVPC
	// Internal is a list of private subnets to create (used for internal load balancers).
	Internal []string
	// Public is a list of public subnets to create (used for bastion and load balancers).
	Public []string
	// Workers is a list of worker subnets (private) to create (used for the VMs).
	Workers []string
}

// AWSVPC contains either an id (of an existing VPC) or the CIDR (for a VPC to be created).
type AWSVPC struct {
	// ID is the AWS VPC id of an existing VPC.
	ID *string
	// CIDR is a CIDR range for a new VPC.
	CIDR *string
}

// Alicloud contains the Shoot specification for Alibaba cloud
type Alicloud struct {
	// ShootMachineImage holds information about the machine image to use for all workers.
	// It will default to the latest version of the first image stated in the referenced CloudProfile if no
	// value has been provided.
	MachineImage *ShootMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AlicloudNetworks
	// Workers is a list of worker groups.
	Workers []Worker
	// Zones is a list of availability zones to deploy the Shoot cluster to, currently, only one is supported.
	Zones []string
}

// AlicloudVPC contains either an id (of an existing VPC) or the CIDR (for a VPC to be created).
type AlicloudVPC struct {
	// ID is the Alicloud VPC id of an existing VPC.
	ID *string
	// CIDR is a CIDR range for a new VPC.
	CIDR *string
}

// AlicloudNetworks holds information about the Kubernetes and infrastructure networks.
type AlicloudNetworks struct {
	K8SNetworks
	// VPC indicates whether to use an existing VPC or create a new one.
	VPC AlicloudVPC
	// Workers is a CIDR of a worker subnet (private) to create (used for the VMs).
	Workers []string
}

// PacketCloud contains the Shoot specification for Packet cloud
type PacketCloud struct {
	// ShootMachineImage holds information about the machine image to use for all workers.
	// It will default to the latest version of the first image stated in the referenced CloudProfile if no
	// value has been provided.
	MachineImage *ShootMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks PacketNetworks
	// Workers is a list of worker groups.
	Workers []Worker
	// Zones is a list of availability zones to deploy the Shoot cluster to, currently, only one is supported.
	Zones []string
}

// PacketNetworks holds information about the Kubernetes and infrastructure networks.
type PacketNetworks struct {
	K8SNetworks
}

// AzureCloud contains the Shoot specification for Azure.
type AzureCloud struct {
	// ShootMachineImage holds information about the machine image to use for all workers.
	// It will default to the latest version of the first image stated in the referenced CloudProfile if no
	// value has been provided.
	MachineImage *ShootMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AzureNetworks
	// ResourceGroup indicates whether to use an existing resource group or create a new one.
	ResourceGroup *AzureResourceGroup
	// Workers is a list of worker groups.
	Workers []Worker
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string
}

// AzureResourceGroup indicates whether to use an existing resource group or create a new one.
type AzureResourceGroup struct {
	// Name is the name of an existing resource group.
	Name string
}

// AzureNetworks holds information about the Kubernetes and infrastructure networks.
type AzureNetworks struct {
	K8SNetworks
	// VNet indicates whether to use an existing VNet or create a new one.
	VNet AzureVNet
	// Workers is a CIDR of a worker subnet (private) to create (used for the VMs).
	Workers string
	// ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the worker subnet.
	ServiceEndpoints []string
}

// AzureVNet indicates whether to use an existing VNet or create a new one.
type AzureVNet struct {
	// Name is the AWS VNet name of an existing VNet.
	Name *string
	// CIDR is a CIDR range for a new VNet.
	CIDR *string
}

// GCPCloud contains the Shoot specification for GCP.
type GCPCloud struct {
	// ShootMachineImage holds information about the machine image to use for all workers.
	// It will default to the latest version of the first image stated in the referenced CloudProfile if no
	// value has been provided.
	MachineImage *ShootMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks GCPNetworks
	// Workers is a list of worker groups.
	Workers []Worker
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string
}

// GCPNetworks holds information about the Kubernetes and infrastructure networks.
type GCPNetworks struct {
	K8SNetworks
	// VPC indicates whether to use an existing VPC or create a new one.
	VPC *GCPVPC
	// Internal is a private subnet (used for internal load balancers).
	Internal *string
	// Workers is a list of strings of worker subnets (private) to create (used for the VMs).
	Workers []string
}

// GCPVPC indicates whether to use an existing VPC or create a new one.
type GCPVPC struct {
	// Name is the name of an existing GCP VPC.
	Name string
}

// OpenStackCloud contains the Shoot specification for OpenStack.
type OpenStackCloud struct {
	// FloatingPoolName is the name of the floating pool to get FIPs from.
	FloatingPoolName string
	// LoadBalancerProvider is the name of the load balancer provider in the OpenStack environment.
	LoadBalancerProvider string
	// LoadBalancerClasses available for a dedicated Shoot.
	LoadBalancerClasses []OpenStackLoadBalancerClass
	// ShootMachineImage holds information about the machine image to use for all workers.
	// It will default to the latest version of the first image stated in the referenced CloudProfile if no
	// value has been provided.
	MachineImage *ShootMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks OpenStackNetworks
	// Workers is a list of worker groups.
	Workers []Worker
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string
}

// OpenStackLoadBalancerClass defines a restricted network setting for generic LoadBalancer classes usable in CloudProfiles.
type OpenStackLoadBalancerClass struct {
	// Name is the name of the LB class
	Name string
	// FloatingSubnetID is the subnetwork ID of a dedicated subnet in floating network pool.
	FloatingSubnetID *string
	// FloatingNetworkID is the network ID of the floating network pool.
	FloatingNetworkID *string
	// SubnetID is the ID of a local subnet used for LoadBalancer provisioning. Only usable if no FloatingPool
	// configuration is done.
	SubnetID *string
}

// OpenStackNetworks holds information about the Kubernetes and infrastructure networks.
type OpenStackNetworks struct {
	K8SNetworks
	// Router indicates whether to use an existing router or create a new one.
	Router *OpenStackRouter
	// Workers is a list of CIDRs of worker subnets (private) to create (used for the VMs).
	Workers []string
}

// OpenStackRouter indicates whether to use an existing router or create a new one.
type OpenStackRouter struct {
	// ID is the router id of an existing OpenStack router.
	ID string
}

// Extension contains type and provider information for Shoot extensions.
type Extension struct {
	// Type is the type of the extension resource.
	Type string
	// ProviderConfig is the configuration passed to extension resource.
	ProviderConfig *ProviderConfig
}

// Addons is a collection of configuration for specific addons which are managed by the Gardener.
type Addons struct {
	// KubernetesDashboard holds configuration settings for the kubernetes dashboard addon.
	KubernetesDashboard *KubernetesDashboard
	// NginxIngress holds configuration settings for the nginx-ingress addon.
	// DEPRECATED: This field will be removed in a future version.
	NginxIngress *NginxIngress

	// ClusterAutoscaler holds configuration settings for the cluster autoscaler addon.
	// DEPRECATED: This field will be removed in a future version.
	ClusterAutoscaler *AddonClusterAutoscaler
	// Heapster holds configuration settings for the heapster addon.
	// DEPRECATED: This field will be removed in a future version.
	Heapster *Heapster
	// Kube2IAM holds configuration settings for the kube2iam addon (only AWS).
	// DEPRECATED: This field will be removed in a future version.
	Kube2IAM *Kube2IAM
	// KubeLego holds configuration settings for the kube-lego addon.
	// DEPRECATED: This field will be removed in a future version.
	KubeLego *KubeLego
	// Monocular holds configuration settings for the monocular addon.
	// DEPRECATED: This field will be removed in a future version.
	Monocular *Monocular
}

// Addon also enabling or disabling a specific addon and is used to derive from.
type Addon struct {
	// Enabled indicates whether the addon is enabled or not.
	Enabled bool
}

// HelmTiller describes configuration values for the helm-tiller addon.
type HelmTiller struct {
	Addon
}

// Heapster describes configuration values for the heapster addon.
type Heapster struct {
	Addon
}

// KubernetesDashboard describes configuration values for the kubernetes-dashboard addon.
type KubernetesDashboard struct {
	Addon
	// AuthenticationMode defines the authentication mode for the kubernetes-dashboard.
	AuthenticationMode *string
}

const (
	// KubernetesDashboardAuthModeBasic uses basic authentication mode for auth.
	KubernetesDashboardAuthModeBasic = "basic"
	// KubernetesDashboardAuthModeToken uses token-based mode for auth.
	KubernetesDashboardAuthModeToken = "token"
)

// AddonClusterAutoscaler describes configuration values for the cluster-autoscaler addon.
type AddonClusterAutoscaler struct {
	Addon
}

// NginxIngress describes configuration values for the nginx-ingress addon.
type NginxIngress struct {
	Addon
	// LoadBalancerSourceRanges is list of whitelist IP sources for NginxIngress
	LoadBalancerSourceRanges []string
}

// Monocular describes configuration values for the monocular addon.
type Monocular struct {
	Addon
}

// KubeLego describes configuration values for the kube-lego addon.
type KubeLego struct {
	Addon
	// Mail is the email address to register at Let's Encrypt.
	Mail string
}

// Kube2IAM describes configuration values for the kube2iam addon.
type Kube2IAM struct {
	Addon
	// Roles is list of AWS IAM roles which should be created by the Gardener.
	Roles []Kube2IAMRole
}

// Kube2IAMRole allows passing AWS IAM policies which will result in IAM roles.
type Kube2IAMRole struct {
	// Name is the name of the IAM role. Will be extended by the Shoot name.
	Name string
	// Description is a human readable message indiciating what this IAM role can be used for.
	Description string
	// Policy is an AWS IAM policy document.
	Policy string
}

// DNS holds information about the provider, the hosted zone id and the domain.
type DNS struct {
	// Domain is the external available domain of the Shoot cluster.
	Domain *string
	// Providers is a list of DNS providers that shall be enabled for this shoot cluster. Only relevant if
	// not a default domain is used.
	Providers []DNSProvider
}

// DNSProvider contains information about a DNS provider.
type DNSProvider struct {
	// Domains contains information about which domains shall be included/excluded for this provider.
	Domains *DNSIncludeExclude
	// SecretName is a name of a secret containing credentials for the stated domain and the
	// provider. When not specified, the Gardener will use the cloud provider credentials referenced
	// by the Shoot and try to find respective credentials there. Specifying this field may override
	// this behavior, i.e. forcing the Gardener to only look into the given secret.
	SecretName *string
	// Type is the DNS provider type for the Shoot. Only relevant if not the default domain is used for
	// this shoot.
	Type *string
	// Zones contains information about which hosted zones shall be included/excluded for this provider.
	Zones *DNSIncludeExclude
}

type DNSIncludeExclude struct {
	// Include is a list of resources that shall be included.
	Include []string
	// Exclude is a list of resources that shall be excluded.
	Exclude []string
}

// DNSUnmanaged is a constant for the 'unmanaged' DNS provider.
const DNSUnmanaged string = "unmanaged"

// CloudProvider is a string alias.
type CloudProvider string

const (
	// CloudProviderAWS is a constant for the AWS cloud provider.
	CloudProviderAWS CloudProvider = "aws"
	// CloudProviderAzure is a constant for the Azure cloud provider.
	CloudProviderAzure CloudProvider = "azure"
	// CloudProviderGCP is a constant for the GCP cloud provider.
	CloudProviderGCP CloudProvider = "gcp"
	// CloudProviderOpenStack is a constant for the OpenStack cloud provider.
	CloudProviderOpenStack CloudProvider = "openstack"
	// CloudProviderAlicloud is a constant for the Alibaba cloud provider.
	CloudProviderAlicloud CloudProvider = "alicloud"
	// CloudProviderPacket is a constant for the Packet cloud provider.
	CloudProviderPacket CloudProvider = "packet"
)

// Hibernation contains information whether the Shoot is suspended or not.
type Hibernation struct {
	// Enabled is true if the Shoot's desired state is hibernated, false otherwise.
	Enabled *bool
	// Schedules determines the hibernation schedules.
	Schedules []HibernationSchedule
}

// HibernationSchedule determines the hibernation schedule of a Shoot.
// A Shoot will be regularly hibernated at each start time and will be woken up at each end time.
// Start or End can be omitted, though at least one of each has to be specified.
type HibernationSchedule struct {
	// Start is a Cron spec at which time a Shoot will be hibernated.
	Start *string
	// End is a Cron spec at which time a Shoot will be woken up.
	End *string
	// Location is the time location in which both start and and shall be evaluated.
	Location *string
}

// Kubernetes contains the version and configuration variables for the Shoot control plane.
type Kubernetes struct {
	// AllowPrivilegedContainers indicates whether privileged containers are allowed in the Shoot (default: true).
	AllowPrivilegedContainers *bool
	// KubeAPIServer contains configuration settings for the kube-apiserver.
	KubeAPIServer *KubeAPIServerConfig
	// CloudControllerManager contains configuration settings for the cloud-controller-manager.
	CloudControllerManager *CloudControllerManagerConfig
	// KubeControllerManager contains configuration settings for the kube-controller-manager.
	KubeControllerManager *KubeControllerManagerConfig
	// KubeScheduler contains configuration settings for the kube-scheduler.
	KubeScheduler *KubeSchedulerConfig
	// KubeProxy contains configuration settings for the kube-proxy.
	KubeProxy *KubeProxyConfig
	// Kubelet contains configuration settings for the kubelet.
	Kubelet *KubeletConfig
	// Version is the semantic Kubernetes version to use for the Shoot cluster.
	Version string
	// ClusterAutoscaler contains the configration flags for the Kubernetes cluster autoscaler.
	ClusterAutoscaler *ClusterAutoscaler
}

// ClusterAutoscaler contains the configration flags for the Kubernetes cluster autoscaler.
type ClusterAutoscaler struct {
	// ScaleDownUtilizationThreshold defines the threshold in % under which a node is being removed
	ScaleDownUtilizationThreshold *float64
	// ScaleDownUnneededTime defines how long a node should be unneeded before it is eligible for scale down (default: 10 mins).
	ScaleDownUnneededTime *metav1.Duration
	// ScaleDownDelayAfterAdd defines how long after scale up that scale down evaluation resumes (default: 10 mins).
	ScaleDownDelayAfterAdd *metav1.Duration
	// ScaleDownDelayAfterFailure how long after scale down failure that scale down evaluation resumes (default: 3 mins).
	ScaleDownDelayAfterFailure *metav1.Duration
	// ScaleDownDelayAfterDelete how long after node deletion that scale down evaluation resumes, defaults to scanInterval (defaults to ScanInterval).
	ScaleDownDelayAfterDelete *metav1.Duration
	// ScanInterval how often cluster is reevaluated for scale up or down (default: 10 secs).
	ScanInterval *metav1.Duration
}

// KubernetesConfig contains common configuration fields for the control plane components.
type KubernetesConfig struct {
	// FeatureGates contains information about enabled feature gates.
	FeatureGates map[string]bool
}

// KubeAPIServerConfig contains configuration settings for the kube-apiserver.
type KubeAPIServerConfig struct {
	KubernetesConfig
	// AdmissionPlugins contains the list of user-defined admission plugins (additional to those managed by Gardener), and, if desired, the corresponding
	// configuration.
	AdmissionPlugins []AdmissionPlugin
	// APIAudiences are the identifiers of the API. The service account token authenticator will
	// validate that tokens used against the API are bound to at least one of these audiences.
	// If `serviceAccountConfig.issuer` is configured and this is not, this defaults to a single
	// element list containing the issuer URL.
	APIAudiences []string
	// AuditConfig contains configuration settings for the audit of the kube-apiserver.
	AuditConfig *AuditConfig
	// EnableBasicAuthentication defines whether basic authentication should be enabled for this cluster or not.
	EnableBasicAuthentication *bool
	// OIDCConfig contains configuration settings for the OIDC provider.
	OIDCConfig *OIDCConfig
	// RuntimeConfig contains information about enabled or disabled APIs.
	RuntimeConfig map[string]bool
	// ServiceAccountConfig contains configuration settings for the service account handling
	// of the kube-apiserver.
	ServiceAccountConfig *ServiceAccountConfig
}

// ServiceAccountConfig is the kube-apiserver configuration for service accounts.
type ServiceAccountConfig struct {
	// Issuer is the identifier of the service account token issuer. The issuer will assert this
	// identifier in "iss" claim of issued tokens. This value is a string or URI.
	Issuer *string
	// SigningKeySecret is a reference to a secret that contains the current private key of the
	// service account token issuer. The issuer will sign issued ID tokens with this private key.
	// (Requires the 'TokenRequest' feature gate.)
	SigningKeySecret *corev1.LocalObjectReference
}

// AuditConfig contains settings for audit of the api server
type AuditConfig struct {
	// AuditPolicy contains configuration settings for audit policy of the kube-apiserver.
	AuditPolicy *AuditPolicy
}

// AuditPolicy contains audit policy for kube-apiserver
type AuditPolicy struct {
	// ConfigMapRef is a reference to a ConfigMap object in the same namespace,
	// which contains the audit policy for the kube-apiserver.
	ConfigMapRef *corev1.ObjectReference
}

// OIDCConfig contains configuration settings for the OIDC provider.
// Note: Descriptions were taken from the Kubernetes documentation.
type OIDCConfig struct {
	// If set, the OpenID server's certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host's root CA set will be used.
	CABundle *string
	// The client ID for the OpenID Connect client, must be set if oidc-issuer-url is set.
	ClientID *string
	// If provided, the name of a custom OpenID Connect claim for specifying user groups. The claim value is expected to be a string or array of strings. This flag is experimental, please see the authentication documentation for further details.
	GroupsClaim *string
	// If provided, all groups will be prefixed with this value to prevent conflicts with other authentication strategies.
	GroupsPrefix *string
	// The URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT).
	IssuerURL *string
	// ATTENTION: Only meaningful for Kubernetes >= 1.11
	// key=value pairs that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value.
	RequiredClaims map[string]string
	// List of allowed JOSE asymmetric signing algorithms. JWTs with a 'alg' header value not in this list will be rejected. Values are defined by RFC 7518 https://tools.ietf.org/html/rfc7518#section-3.1
	SigningAlgs []string
	// The OpenID claim to use as the user name. Note that claims other than the default ('sub') is not guaranteed to be unique and immutable. This flag is experimental, please see the authentication documentation for further details. (default "sub")
	UsernameClaim *string
	// If provided, all usernames will be prefixed with this value. If not provided, username claims other than 'email' are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value '-'.
	UsernamePrefix *string
	// ClientAuthentication can optionally contain client configuration used for kubeconfig generation.
	ClientAuthentication *OpenIDConnectClientAuthentication
}

// OpenIDConnectClientAuthentication contains configuration for OIDC clients.
type OpenIDConnectClientAuthentication struct {
	// The client Secret for the OpenID Connect client.
	Secret *string
	// Extra configuration added to kubeconfig's auth-provider.
	// Must not be any of idp-issuer-url, client-id, client-secret, idp-certificate-authority, idp-certificate-authority-data, id-token or refresh-token
	ExtraConfig map[string]string
}

// AdmissionPlugin contains information about a specific admission plugin and its corresponding configuration.
type AdmissionPlugin struct {
	// Name is the name of the plugin.
	Name string
	// Config is the configuration of the plugin.
	Config *ProviderConfig
}

// CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.
type CloudControllerManagerConfig struct {
	KubernetesConfig
}

// KubeControllerManagerConfig contains configuration settings for the kube-controller-manager.
type KubeControllerManagerConfig struct {
	KubernetesConfig
	// HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
	HorizontalPodAutoscalerConfig *HorizontalPodAutoscalerConfig
	// NodeCIDRMaskSize defines the mask size for node cidr in cluster (default is 24)
	NodeCIDRMaskSize *int
}

// HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
// Note: Descriptions were taken from the Kubernetes documentation.
type HorizontalPodAutoscalerConfig struct {
	// DownscaleDelay is the period since last downscale, before another downscale can be performed in horizontal pod autoscaler.
	DownscaleDelay *metav1.Duration
	// SyncPeriod is the period for syncing the number of pods in horizontal pod autoscaler.
	SyncPeriod *metav1.Duration
	// Tolerance is the minimum change (from 1.0) in the desired-to-actual metrics ratio for the horizontal pod autoscaler to consider scaling.
	Tolerance *float64
	// UpscaleDelay is the period since last upscale, before another upscale can be performed in horizontal pod autoscaler.
	UpscaleDelay *metav1.Duration
	// DownscaleStabilization is the period for which autoscaler will look backwards and not scale down below any recommendation it made during that period.
	DownscaleStabilization *metav1.Duration
	// InitialReadinessDelay is the  period after pod start during which readiness changes will be treated as initial readiness.
	InitialReadinessDelay *metav1.Duration
	// CPUInitializationPeriod is the period after pod start when CPU samples might be skipped.
	CPUInitializationPeriod *metav1.Duration
}

// KubeSchedulerConfig contains configuration settings for the kube-scheduler.
type KubeSchedulerConfig struct {
	KubernetesConfig
}

// KubeProxyConfig contains configuration settings for the kube-proxy.
type KubeProxyConfig struct {
	KubernetesConfig
	// Mode specifies which proxy mode to use.
	// defaults to IPTables.
	Mode *ProxyMode
}

// ProxyMode available in Linux platform: 'userspace' (older, going to be EOL), 'iptables'
// (newer, faster), 'ipvs'(newest, better in performance and scalability).
//
// As of now only 'iptables' and 'ipvs' is supported by Gardener.
//
// In Linux platform, if the iptables proxy is selected, regardless of how, but the system's kernel or iptables versions are
// insufficient, this always falls back to the userspace proxy. IPVS mode will be enabled when proxy mode is set to 'ipvs',
// and the fall back path is firstly iptables and then userspace.
type ProxyMode string

const (
	// ProxyModeIPTables uses iptables as proxy implementation.
	ProxyModeIPTables ProxyMode = "IPTables"
	// ProxyModeIPVS uses ipvs as proxy implementation.
	ProxyModeIPVS ProxyMode = "IPVS"
)

// KubeletConfig contains configuration settings for the kubelet.
type KubeletConfig struct {
	KubernetesConfig
	// PodPIDsLimit is the maximum number of process IDs per pod allowed by the kubelet.
	PodPIDsLimit *int64
	// CPUCFSQuota allows you to disable/enable CPU throttling for Pods.
	CPUCFSQuota *bool
	// CPUManagerPolicy allows to set alternative CPU management policies (default: none).
	CPUManagerPolicy *string
	// MaxPods is the maximum number of Pods that are allowed by the Kubelet.
	// Default: 110
	MaxPods *int32
	// EvictionHard describes a set of eviction thresholds (e.g. memory.available<1Gi) that if met would trigger a Pod eviction.
	// Default:
	//   memory.available:   "100Mi/1Gi/5%"
	//   nodefs.available:   "5%"
	//   nodefs.inodesFree:  "5%"
	//   imagefs.available:  "5%"
	//   imagefs.inodesFree: "5%"
	EvictionHard *KubeletConfigEviction
	// EvictionSoft describes a set of eviction thresholds (e.g. memory.available<1.5Gi) that if met over a corresponding grace period would trigger a Pod eviction.
	// Default:
	//   memory.available:   "200Mi/1.5Gi/10%"
	//   nodefs.available:   "10%"
	//   nodefs.inodesFree:  "10%"
	//   imagefs.available:  "10%"
	//   imagefs.inodesFree: "10%"
	EvictionSoft *KubeletConfigEviction
	// EvictionSoftGracePeriod describes a set of eviction grace periods (e.g. memory.available=1m30s) that correspond to how long a soft eviction threshold must hold before triggering a Pod eviction.
	// Default:
	//   memory.available:   1m30s
	//   nodefs.available:   1m30s
	//   nodefs.inodesFree:  1m30s
	//   imagefs.available:  1m30s
	//   imagefs.inodesFree: 1m30s
	EvictionSoftGracePeriod *KubeletConfigEvictionSoftGracePeriod
	// EvictionMinimumReclaim configures the amount of resources below the configured eviction threshold that the kubelet attempts to reclaim whenever the kubelet observes resource pressure.
	// Default: 0 for each resource
	EvictionMinimumReclaim *KubeletConfigEvictionMinimumReclaim
	// EvictionPressureTransitionPeriod is the duration for which the kubelet has to wait before transitioning out of an eviction pressure condition.
	// Default: 4m0s
	EvictionPressureTransitionPeriod *metav1.Duration
	// EvictionMaxPodGracePeriod describes the maximum allowed grace period (in seconds) to use when terminating pods in response to a soft eviction threshold being met.
	// Default: 90
	EvictionMaxPodGracePeriod *int32
}

// KubeletConfigEviction contains kubelet eviction thresholds supporting either a resource.Quantity or a percentage based value.
type KubeletConfigEviction struct {
	// MemoryAvailable is the threshold for the free memory on the host server.
	MemoryAvailable *string
	// ImageFSAvailable is the threshold for the free disk space in the imagefs filesystem (docker images and container writable layers).
	ImageFSAvailable *string
	// ImageFSInodesFree is the threshold for the available inodes in the imagefs filesystem.
	ImageFSInodesFree *string
	// NodeFSAvailable is the threshold for the free disk space in the nodefs filesystem (docker volumes, logs, etc).
	NodeFSAvailable *string
	// NodeFSInodesFree is the threshold for the available inodes in the nodefs filesystem.
	NodeFSInodesFree *string
}

// KubeletConfigEviction contains configuration for the kubelet eviction minimum reclaim.
type KubeletConfigEvictionMinimumReclaim struct {
	// MemoryAvailable is the threshold for the memory reclaim on the host server.
	MemoryAvailable *resource.Quantity
	// ImageFSAvailable is the threshold for the disk space reclaim in the imagefs filesystem (docker images and container writable layers).
	ImageFSAvailable *resource.Quantity
	// ImageFSInodesFree is the threshold for the inodes reclaim in the imagefs filesystem.
	ImageFSInodesFree *resource.Quantity
	// NodeFSAvailable is the threshold for the disk space reclaim in the nodefs filesystem (docker volumes, logs, etc).
	NodeFSAvailable *resource.Quantity
	// NodeFSInodesFree is the threshold for the inodes reclaim in the nodefs filesystem.
	NodeFSInodesFree *resource.Quantity
}

// KubeletConfigEvictionSoftGracePeriod contains grace periods for kubelet eviction thresholds.
type KubeletConfigEvictionSoftGracePeriod struct {
	// MemoryAvailable is the grace period for the MemoryAvailable eviction threshold.
	MemoryAvailable *metav1.Duration
	// ImageFSAvailable is the grace period for the ImageFSAvailable eviction threshold.
	ImageFSAvailable *metav1.Duration
	// ImageFSInodesFree is the grace period for the ImageFSInodesFree eviction threshold.
	ImageFSInodesFree *metav1.Duration
	// NodeFSAvailable is the grace period for the NodeFSAvailable eviction threshold.
	NodeFSAvailable *metav1.Duration
	// NodeFSInodesFree is the grace period for the NodeFSInodesFree eviction threshold.
	NodeFSInodesFree *metav1.Duration
}

// Maintenance contains information about the time window for maintenance operations and which
// operations should be performed.
type Maintenance struct {
	// AutoUpdate contains information about which constraints should be automatically updated.
	AutoUpdate *MaintenanceAutoUpdate
	// TimeWindow contains information about the time window for maintenance operations.
	TimeWindow *MaintenanceTimeWindow
}

// MaintenanceAutoUpdate contains information about which constraints should be automatically updated.
type MaintenanceAutoUpdate struct {
	// KubernetesVersion indicates whether the patch Kubernetes version may be automatically updated.
	KubernetesVersion bool
	// MachineImageVersion indicates whether the machine image version may be automatically updated (default: true).
	MachineImageVersion *bool
}

// MaintenanceTimeWindow contains information about the time window for maintenance operations.
type MaintenanceTimeWindow struct {
	// Begin is the beginning of the time window in the format HHMMSS+ZONE, e.g. "220000+0100".
	// If not present, a random value will be computed.
	Begin string
	// End is the end of the time window in the format HHMMSS+ZONE, e.g. "220000+0100".
	// If not present, the value will be computed based on the "Begin" value.
	End string
}

// Provider contains provider-specific information that are handed-over to the provider-specific
// extension controller.
type Provider struct {
	// Type is the type of the provider.
	Type string
	// ControlPlaneConfig contains the provider-specific control plane config blob. Please look up the concrete
	// definition in the documentation of your provider extension.
	ControlPlaneConfig *ProviderConfig
	// InfrastructureConfig contains the provider-specific infrastructure config blob. Please look up the concrete
	// definition in the documentation of your provider extension.
	InfrastructureConfig *ProviderConfig
	// Workers is a list of worker groups.
	Workers []Worker
}

// Worker is the base definition of a worker group.
type Worker struct {
	// Annotations is a map of key/value pairs for annotations for all the `Node` objects in this worker pool.
	Annotations map[string]string
	// CABundle is a certificate bundle which will be installed onto every machine of this worker pool.
	CABundle *string
	// Kubernetes contains configuration for Kubernetes components related to this worker pool.
	Kubernetes *WorkerKubernetes
	// Labels is a map of key/value pairs for labels for all the `Node` objects in this worker pool.
	Labels map[string]string
	// Name is the name of the worker group.
	Name string
	// Machine contains information about the machine type and image.
	Machine Machine
	// Maximum is the maximum number of VMs to create.
	Maximum int
	// Minimum is the minimum number of VMs to create.
	Minimum int
	// MaxSurge is maximum number of VMs that are created during an update.
	MaxSurge *intstr.IntOrString
	// MaxUnavailable is the maximum number of VMs that can be unavailable during an update.
	MaxUnavailable *intstr.IntOrString
	// ProviderConfig is the provider-specific configuration for this worker pool.
	ProviderConfig *ProviderConfig
	// Taints is a list of taints for all the `Node` objects in this worker pool.
	Taints []corev1.Taint
	// Volume contains information about the volume type and size.
	Volume *Volume
	// Zones is a list of availability zones that are used to evenly distribute this worker pool. Optional
	// as not every provider may support availability zones.
	Zones []string
}

// WorkerMigrationInfo is used to store fields that are not present in both API versions (core/garden).
type WorkerMigrationInfo map[string]WorkerMigrationData

// WorkerMigrationInfo is used to store fields that are not present in both API versions (core/garden).
type WorkerMigrationData struct {
	// ProviderConfig is the provider-specific configuration for this worker pool.
	ProviderConfig *ProviderConfig
	// Volume contains information about the volume type and size.
	Volume *Volume
	// Zones is a list of availability zones that are used to evenly distribute this worker pool. Optional
	// as not every provider may support availability zones.
	Zones []string
}

// WorkerKubernetes contains configuration for Kubernetes components related to this worker pool.
type WorkerKubernetes struct {
	// Kubelet contains configuration settings for all kubelets of this worker pool.
	Kubelet *KubeletConfig
}

// Machine contains information about the machine type and image.
type Machine struct {
	// Type is the machine type of the worker group.
	Type string
	// Image holds information about the machine image to use for all nodes of this pool. It will default to the
	// latest version of the first image stated in the referenced CloudProfile if no value has been provided.
	Image *ShootMachineImage
}

// ShootMachineImage defines the name and the version of the shoot's machine image in any environment. Has to be
// defined in the respective CloudProfile.
type ShootMachineImage struct {
	// Name is the name of the image.
	Name string
	// ProviderConfig is the shoot's individual configuration passed to an extension resource.
	ProviderConfig *ProviderConfig
	// Version is the version of the shoot's image.
	Version string
}

// Volume contains information about the volume type and size.
type Volume struct {
	// Type is the machine type of the worker group.
	Type string
	// Size is the size of the root volume.
	Size string
}

////////////////////////
// Shoot Status Types //
////////////////////////

// Gardener holds the information about the Gardener
type Gardener struct {
	// ID is the Docker container id of the Gardener which last acted on a Shoot cluster.
	ID string
	// Name is the hostname (pod name) of the Gardener which last acted on a Shoot cluster.
	Name string
	// Version is the version of the Gardener which last acted on a Shoot cluster.
	Version string
}

const (
	// EventReconciling indicates that the a Reconcile operation started.
	EventReconciling = "Reconciling"
	// EventReconciled indicates that the a Reconcile operation was successful.
	EventReconciled = "Reconciled"
	// EventReconcileError indicates that the a Reconcile operation failed.
	EventReconcileError = "ReconcileError"
	// EventDeleting indicates that the a Delete operation started.
	EventDeleting = "Deleting"
	// EventDeleted indicates that the a Delete operation was successful.
	EventDeleted = "Deleted"
	// EventDeleteError indicates that the a Delete operation failed.
	EventDeleteError = "DeleteError"

	// ShootEventMaintenanceDone indicates that a maintenance operation has been performed.
	ShootEventMaintenanceDone = "MaintenanceDone"
	// ShootEventMaintenanceError indicates that a maintenance operation has failed.
	ShootEventMaintenanceError = "MaintenanceError"

	// ProjectEventNamespaceReconcileFailed indicates that the namespace reconciliation has failed.
	ProjectEventNamespaceReconcileFailed = "NamespaceReconcileFailed"
	// ProjectEventNamespaceReconcileSuccessful indicates that the namespace reconciliation has succeeded.
	ProjectEventNamespaceReconcileSuccessful = "NamespaceReconcileSuccessful"
	// ProjectEventNamespaceDeletionFailed indicates that the namespace deletion failed.
	ProjectEventNamespaceDeletionFailed = "NamespaceDeletionFailed"
	// ProjectEventNamespaceMarkedForDeletion indicates that the namespace has been successfully marked for deletion.
	ProjectEventNamespaceMarkedForDeletion = "NamespaceMarkedForDeletion"
)

const (
	// GardenerName is the value in a Garden resource's `.metadata.finalizers[]` array on which the Gardener will react
	// when performing a delete request on a resource.
	GardenerName = "gardener"

	// ExternalGardenerName is the value in a Kubernetes core resources `.metadata.finalizers[]` array on which the
	// Gardener will react when performing a delete request on a resource.
	ExternalGardenerName = "garden.sapcloud.io/gardener"

	// DefaultDomain is the default value in the Shoot's '.spec.dns.domain' when '.spec.dns.provider' is 'unmanaged'
	DefaultDomain = "cluster.local"
)

const (
	// SeedAvailable is a constant for a condition type indicating the Seed cluster availability.
	SeedAvailable ConditionType = "Available"

	// ShootControlPlaneHealthy is a constant for a condition type indicating the control plane health.
	ShootControlPlaneHealthy ConditionType = "ControlPlaneHealthy"
	// ShootEveryNodeReady is a constant for a condition type indicating the node health.
	ShootEveryNodeReady ConditionType = "EveryNodeReady"
	// ShootSystemComponentsHealthy is a constant for a condition type indicating the system components health.
	ShootSystemComponentsHealthy ConditionType = "SystemComponentsHealthy"
	// ShootAPIServerAvailable is a constant for a condition type indicating the api server is available.
	ShootAPIServerAvailable ConditionType = "APIServerAvailable"
)
