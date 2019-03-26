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
	"time"

	gardencore "github.com/gardener/gardener/pkg/apis/core"

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
	// +optional
	metav1.ObjectMeta
	// Spec defines the cloud environment properties.
	// +optional
	Spec CloudProfileSpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileList is a collection of CloudProfiles.
type CloudProfileList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	// +optional
	metav1.ListMeta
	// Items is the list of CloudProfiles.
	Items []CloudProfile
}

// CloudProfileSpec is the specification of a CloudProfile.
// It must contain exactly one of its defined keys.
type CloudProfileSpec struct {
	// AWS is the profile specification for the Amazon Web Services cloud.
	// +optional
	AWS *AWSProfile
	// Azure is the profile specification for the Microsoft Azure cloud.
	// +optional
	Azure *AzureProfile
	// GCP is the profile specification for the Google Cloud Platform cloud.
	// +optional
	GCP *GCPProfile
	// OpenStack is the profile specification for the OpenStack cloud.
	// +optional
	OpenStack *OpenStackProfile
	// Alicloud is the profile specification for the Alibaba cloud.
	// +optional
	Alicloud *AlicloudProfile
	// Packet is the profile specification for the Packet cloud.
	// +optional
	Packet *PacketProfile
	// Local is the profile specification for the Local provider.
	// +optional
	Local *LocalProfile
	// CABundle is a certificate bundle which will be installed onto every host machine of the Shoot cluster.
	// +optional
	CABundle *string
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
	MachineImages []AWSMachineImageMapping
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// AWSMachineImage defines the region and the AMI for a machine image.
type AWSMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName
	// AMI is the technical id of the image (region specific).
	AMI string
}

// AWSMachineImageMapping is a mapping of machine images to regions.
type AWSMachineImageMapping struct {
	// Name is the name of the image.
	Name MachineImageName
	// Regions is a list of machine images with their regional technical id.
	Regions []AWSRegionalMachineImage
}

type AWSRegionalMachineImage struct {
	// Name is the name of a region.
	Name string
	// AMI is the technical id of the image (specific for region stated in the 'Name' field).
	AMI string
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
	MachineImages []AzureMachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
}

// AzureDomainCount defines the region and the count for this domain count value.
type AzureDomainCount struct {
	// Region is a region in Azure.
	Region string
	// Count is the count value for the respective domain count.
	Count int
}

// AzureMachineImage defines the channel and the version of the machine image in the Azure environment.
type AzureMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName
	// Publisher is the publisher of the image.
	Publisher string
	// Offer is the offering of the image.
	Offer string
	// SKU is the stock keeping unit to pull images from.
	SKU string
	// Version is the version of the image.
	Version string
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
	MachineImages []GCPMachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// GCPMachineImage defines the name of the machine image in the GCP environment.
type GCPMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName
	// Image is the technical name of the image. It contains the image name and the Google Cloud project.
	// Example: projects/<name>/global/images/version23
	Image string
}

// OpenStackProfile defines certain constraints and definitions for the OpenStack cloud.
type OpenStackProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints OpenStackConstraints
	// KeyStoneURL is the URL for auth{n,z} in OpenStack (pointing to KeyStone).
	KeyStoneURL string
	// DNSServers is a list of IPs of DNS servers used while creating subnets.
	// +optional
	DNSServers []string
	// DHCPDomain is the dhcp domain of the OpenStack system configured in nova.conf. Only meaningful for
	// Kubernetes 1.10.1+. See https://github.com/kubernetes/kubernetes/pull/61890 for details.
	// +optional
	DHCPDomain *string
	// RequestTimeout specifies the HTTP timeout against the OpenStack API.
	// +optional
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
	MachineImages []OpenStackMachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []OpenStackMachineType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// FloatingPools contains constraints regarding allowed values of the 'floatingPoolName' block in the Shoot specification.
type OpenStackFloatingPool struct {
	// Name is the name of the floating pool.
	Name string
}

// LoadBalancerProviders contains constraints regarding allowed values of the 'loadBalancerProvider' block in the Shoot specification.
type OpenStackLoadBalancerProvider struct {
	// Name is the name of the load balancer provider.
	Name string
}

// OpenStackMachineImage defines the name of the machine image in the OpenStack environment.
type OpenStackMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName
	// Image is the technical name of the image.
	Image string
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
	MachineImages []AlicloudMachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []AlicloudMachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []AlicloudVolumeType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// AlicloudMachineImage defines the machine image for Alicloud.
type AlicloudMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName
	// ID is the ID of the image.
	ID string
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
	MachineImages []PacketMachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone
}

// PacketMachineImage defines the machine image for Packet.
type PacketMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName
	// ID is the ID of the image.
	ID string
}

// LocalProfile defines constraints and definitions for the local development.
type LocalProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints LocalConstraints
}

// LocalConstraints is an object containing constraints for certain values in the Shoot specification.
type LocalConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint
}

// DNSProviderConstraint contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
type DNSProviderConstraint struct {
	// Name is the name of the DNS provider.
	Name DNSProvider
}

// KubernetesConstraints contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
type KubernetesConstraints struct {
	// Versions is the list of allowed Kubernetes versions for Shoot clusters (e.g., 1.13.1).
	Versions []string
}

// MachineType contains certain properties of a machine type.
type MachineType struct {
	// Name is the name of the machine type.
	Name string
	// Usable defines if the machine type can be used for shoot clusters.
	// +optional
	Usable *bool
	// CPU is the number of CPUs for this machine type.
	CPU resource.Quantity
	// GPU is the number of GPUs for this machine type.
	GPU resource.Quantity
	// Memory is the amount of memory for this machine type.
	Memory resource.Quantity
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
	// +optional
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

// MachineImageName is a string alias.
type MachineImageName string

const (
	// MachineImageCoreOS is a constant for the CoreOS machine image.
	MachineImageCoreOS MachineImageName = "coreos"
	// MachineImageCoreOSAlicloud is a constant for the CoreOS machine image used by Alicloud.
	// The Alicloud CoreOS image is modified (e.g., it does not support cloud-config, and is therefore
	// treated like another OS).
	MachineImageCoreOSAlicloud MachineImageName = "coreos-alicloud"
)

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
	// +optional
	metav1.ObjectMeta
	// Spec defines the project properties.
	// +optional
	Spec ProjectSpec
	// Most recently observed status of the Project.
	// +optional
	Status ProjectStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectList is a collection of Projects.
type ProjectList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	// +optional
	metav1.ListMeta
	// Items is the list of Projects.
	Items []Project
}

// ProjectSpec is the specification of a Project.
type ProjectSpec struct {
	// CreatedBy is a subject representing a user name, an email address, or any other identifier of a user
	// who created the project.
	// +optional
	CreatedBy *rbacv1.Subject
	// Description is a human-readable description of what the project is used for.
	// +optional
	Description *string
	// Owner is a subject representing a user name, an email address, or any other identifier of a user owning
	// the project.
	// +optional
	Owner *rbacv1.Subject
	// Purpose is a human-readable explanation of the project's purpose.
	// +optional
	Purpose *string
	// Members is a list of subjects representing a user name, an email address, or any other identifier of a user
	// that should be part of this project.
	// +optional
	Members []rbacv1.Subject
	// Namespace is the name of the namespace that has been created for the Project object.
	// +optional
	Namespace *string
}

// ProjectStatus holds the most recently observed status of the project.
type ProjectStatus struct {
	// ObservedGeneration is the most recent generation observed for this project.
	// +optional
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
	// +optional
	metav1.ObjectMeta
	// Spec defines the Seed cluster properties.
	// +optional
	Spec SeedSpec
	// Most recently observed status of the Seed cluster.
	// +optional
	Status SeedStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SeedList is a collection of Seeds.
type SeedList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	// +optional
	metav1.ListMeta
	// Items is the list of Seeds.
	Items []Seed
}

// SeedSpec is the specification of a Seed.
type SeedSpec struct {
	// Cloud defines the cloud profile and the region this Seed cluster belongs to.
	Cloud SeedCloud
	// IngressDomain is the domain of the Seed cluster pointing to the ingress controller endpoint. It will be used
	// to construct ingress URLs for system applications running in Shoot clusters.
	IngressDomain string
	// SecretRef is a reference to a Secret object containing the Kubeconfig and the cloud provider credentials for
	// the account the Seed cluster has been deployed to.
	SecretRef corev1.SecretReference
	// Networks defines the pod, service and worker network of the Seed cluster.
	Networks SeedNetworks
	// Visible labels the Seed cluster as selectable for the seedfinder admission controller.
	// +optional
	Visible *bool
	// Protected prevent that the Seed Cluster can be used for regular Shoot cluster control planes.
	// +optional
	Protected *bool
}

// SeedStatus holds the most recently observed status of the Seed cluster.
type SeedStatus struct {
	// Conditions represents the latest available observations of a Seed's current state.
	// +optional
	Conditions []gardencore.Condition
}

// SeedCloud defines the cloud profile and the region this Seed cluster belongs to.
type SeedCloud struct {
	// Profile is the name of a cloud profile.
	Profile string
	// Region is a name of a region.
	Region string
}

// SeedNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
type SeedNetworks struct {
	// Nodes is the CIDR of the node network.
	Nodes gardencore.CIDR
	// Pods is the CIDR of the pod network.
	Pods gardencore.CIDR
	// Services is the CIDR of the service network.
	Services gardencore.CIDR
}

////////////////////////////////////////////////////
//                      QUOTAS                    //
////////////////////////////////////////////////////

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Quota holds certain information about resource usage limitations and lifetime for Shoot objects.
type Quota struct {
	metav1.TypeMeta
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta
	// Spec defines the Quota constraints.
	// +optional
	Spec QuotaSpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuotaList is a collection of Quotas.
type QuotaList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	// +optional
	metav1.ListMeta
	// Items is the list of Quotas.
	Items []Quota
}

// QuotaSpec is the specification of a Quota.
type QuotaSpec struct {
	// ClusterLifetimeDays is the lifetime of a Shoot cluster in days before it will be terminated automatically.
	// +optional
	ClusterLifetimeDays *int
	// Metrics is a list of resources which will be put under constraints.
	Metrics corev1.ResourceList
	// Scope is the scope of the Quota object, either 'project' or 'secret'.
	Scope QuotaScope
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
	// +optional
	metav1.ObjectMeta
	// SecretRef is a reference to a secret object in the same or another namespace.
	SecretRef corev1.SecretReference
	// Quotas is a list of references to Quota objects in the same or another namespace.
	// +optional
	Quotas []corev1.ObjectReference
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretBindingList is a collection of SecretBindings.
type SecretBindingList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	// +optional
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
	// +optional
	metav1.ObjectMeta
	// Specification of the Shoot cluster.
	// +optional
	Spec ShootSpec
	// Most recently observed status of the Shoot cluster.
	// +optional
	Status ShootStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootList is a list of Shoot objects.
type ShootList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	// +optional
	metav1.ListMeta
	// Items is the list of Shoots.
	Items []Shoot
}

// ShootSpec is the specification of a Shoot.
type ShootSpec struct {
	// Addons contains information about enabled/disabled addons and their configuration.
	// +optional
	Addons *Addons
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	Backup *Backup
	// Cloud contains information about the cloud environment and their specific settings.
	Cloud Cloud
	// DNS contains information about the DNS settings of the Shoot.
	DNS DNS
	// Hibernation contains information whether the Shoot is suspended or not.
	// +optional
	Hibernation *Hibernation
	// Kubernetes contains the version and configuration settings of the control plane components.
	Kubernetes Kubernetes
	// Maintenance contains information about the time window for maintenance operations and which
	// operations should be performed.
	// +optional
	Maintenance *Maintenance
}

// ShootStatus holds the most recently observed status of the Shoot cluster.
type ShootStatus struct {
	// Conditions represents the latest available observations of a Shoots's current state.
	// +optional
	Conditions []gardencore.Condition
	// Gardener holds information about the Gardener which last acted on the Shoot.
	Gardener Gardener
	// LastOperation holds information about the last operation on the Shoot.
	// +optional
	LastOperation *gardencore.LastOperation
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *gardencore.LastError
	// ObservedGeneration is the most recent generation observed for this Shoot. It corresponds to the
	// Shoot's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64
	// RetryCycleStartTime is the start time of the last retry cycle (used to determine how often an operation
	// must be retried until we give up).
	// +optional
	RetryCycleStartTime *metav1.Time
	// Seed is the name of the seed cluster that runs the control plane of the Shoot. This value is only written
	// after a successful create/reconcile operation. It will be used when control planes are moved between Seeds.
	Seed string
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
	// +optional
	Seed *string
	// AWS contains the Shoot specification for the Amazon Web Services cloud.
	// +optional
	AWS *AWSCloud
	// Azure contains the Shoot specification for the Microsoft Azure cloud.
	// +optional
	Azure *AzureCloud
	// GCP contains the Shoot specification for the Google Cloud Platform cloud.
	// +optional
	GCP *GCPCloud
	// OpenStack contains the Shoot specification for the OpenStack cloud.
	// +optional
	OpenStack *OpenStackCloud
	// Alicloud contains the Shoot specification for the Alibaba cloud.
	// +optional
	Alicloud *Alicloud
	// PacketCloud contains the Shoot specification for the Packet cloud.
	// +optional
	Packet *PacketCloud
	// Local contains the Shoot specification for the Local local provider.
	// +optional
	Local *Local
}

// AWSCloud contains the Shoot specification for AWS.

type AWSCloud struct {
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *AWSMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AWSNetworks
	// Workers is a list of worker groups.
	Workers []AWSWorker
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string
}

// AWSNetworks holds information about the Kubernetes and infrastructure networks.
type AWSNetworks struct {
	gardencore.K8SNetworks
	// VPC indicates whether to use an existing VPC or create a new one.
	VPC AWSVPC
	// Internal is a list of private subnets to create (used for internal load balancers).
	Internal []gardencore.CIDR
	// Public is a list of public subnets to create (used for bastion and load balancers).
	Public []gardencore.CIDR
	// Workers is a list of worker subnets (private) to create (used for the VMs).
	Workers []gardencore.CIDR
}

// AWSVPC contains either an id (of an existing VPC) or the CIDR (for a VPC to be created).
type AWSVPC struct {
	// ID is the AWS VPC id of an existing VPC.
	// +optional
	ID *string
	// CIDR is a CIDR range for a new VPC.
	// +optional
	CIDR *gardencore.CIDR
}

// AWSWorker is the definition of a worker group.
type AWSWorker struct {
	Worker
	// VolumeType is the type of the root volumes.
	VolumeType string
	// VolumeSize is the size of the root volume.
	VolumeSize string
}

// Alicloud contains the Shoot specification for Alibaba cloud
type Alicloud struct {
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *AlicloudMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AlicloudNetworks
	// Workers is a list of worker groups.
	Workers []AlicloudWorker
	// Zones is a list of availability zones to deploy the Shoot cluster to, currently, only one is supported.
	Zones []string
}

// AlicloudVPC contains either an id (of an existing VPC) or the CIDR (for a VPC to be created).
type AlicloudVPC struct {
	// ID is the Alicloud VPC id of an existing VPC.
	// +optional
	ID *string
	// CIDR is a CIDR range for a new VPC.
	// +optional
	CIDR *gardencore.CIDR
}

// AlicloudNetworks holds information about the Kubernetes and infrastructure networks.
type AlicloudNetworks struct {
	gardencore.K8SNetworks
	// VPC indicates whether to use an existing VPC or create a new one.
	VPC AlicloudVPC
	// Workers is a CIDR of a worker subnet (private) to create (used for the VMs).
	Workers []gardencore.CIDR
}

// AlicloudWorker is the definition of a worker group.
type AlicloudWorker struct {
	Worker
	// VolumeType is the type of the root volumes.
	VolumeType string
	// VolumeSize is the size of the root volume.
	VolumeSize string
}

// PacketCloud contains the Shoot specification for Packet cloud
type PacketCloud struct {
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *PacketMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks PacketNetworks
	// Workers is a list of worker groups.
	Workers []PacketWorker
	// Zones is a list of availability zones to deploy the Shoot cluster to, currently, only one is supported.
	Zones []string
}

// PacketNetworks holds information about the Kubernetes and infrastructure networks.
type PacketNetworks struct {
	gardencore.K8SNetworks
}

// PacketWorker is the definition of a worker group.
type PacketWorker struct {
	Worker
	// VolumeType is the type of the root volumes.
	VolumeType string
	// VolumeSize is the size of the root volume.
	VolumeSize string
}

// AzureCloud contains the Shoot specification for Azure.
type AzureCloud struct {
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *AzureMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AzureNetworks
	// ResourceGroup indicates whether to use an existing resource group or create a new one.
	// +optional
	ResourceGroup *AzureResourceGroup
	// Workers is a list of worker groups.
	Workers []AzureWorker
}

// AzureResourceGroup indicates whether to use an existing resource group or create a new one.
type AzureResourceGroup struct {
	// Name is the name of an existing resource group.
	Name string
}

// AzureNetworks holds information about the Kubernetes and infrastructure networks.
type AzureNetworks struct {
	gardencore.K8SNetworks
	// VNet indicates whether to use an existing VNet or create a new one.
	VNet AzureVNet
	// Workers is a CIDR of a worker subnet (private) to create (used for the VMs).
	Workers gardencore.CIDR
}

// AzureVNet indicates whether to use an existing VNet or create a new one.
type AzureVNet struct {
	// Name is the AWS VNet name of an existing VNet.
	// +optional
	Name *string
	// CIDR is a CIDR range for a new VNet.
	// +optional
	CIDR *gardencore.CIDR
}

// AzureWorker is the definition of a worker group.
type AzureWorker struct {
	Worker
	// VolumeType is the type of the root volumes.
	VolumeType string
	// VolumeSize is the size of the root volume.
	VolumeSize string
}

// GCPCloud contains the Shoot specification for GCP.
type GCPCloud struct {
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *GCPMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks GCPNetworks
	// Workers is a list of worker groups.
	Workers []GCPWorker
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string
}

// GCPNetworks holds information about the Kubernetes and infrastructure networks.
type GCPNetworks struct {
	gardencore.K8SNetworks
	// VPC indicates whether to use an existing VPC or create a new one.
	// +optional
	VPC *GCPVPC
	// Internal is a private subnet (used for internal load balancers).
	Internal *gardencore.CIDR
	// Workers is a list of CIDRs of worker subnets (private) to create (used for the VMs).
	Workers []gardencore.CIDR
}

// GCPVPC indicates whether to use an existing VPC or create a new one.
type GCPVPC struct {
	// Name is the name of an existing GCP VPC.
	Name string
}

// GCPWorker is the definition of a worker group.
type GCPWorker struct {
	Worker
	// VolumeType is the type of the root volumes.
	VolumeType string
	// VolumeSize is the size of the root volume.
	VolumeSize string
}

// OpenStackCloud contains the Shoot specification for OpenStack.
type OpenStackCloud struct {
	// FloatingPoolName is the name of the floating pool to get FIPs from.
	FloatingPoolName string
	// LoadBalancerProvider is the name of the load balancer provider in the OpenStack environment.
	LoadBalancerProvider string
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *OpenStackMachineImage
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks OpenStackNetworks
	// Workers is a list of worker groups.
	Workers []OpenStackWorker
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string
}

// OpenStackNetworks holds information about the Kubernetes and infrastructure networks.
type OpenStackNetworks struct {
	gardencore.K8SNetworks
	// Router indicates whether to use an existing router or create a new one.
	// +optional
	Router *OpenStackRouter
	// Workers is a list of CIDRs of worker subnets (private) to create (used for the VMs).
	Workers []gardencore.CIDR
}

// OpenStackRouter indicates whether to use an existing router or create a new one.
type OpenStackRouter struct {
	// ID is the router id of an existing OpenStack router.
	ID string
}

// OpenStackWorker is the definition of a worker group.
type OpenStackWorker struct {
	Worker
}

// Local contains the Shoot specification for local provider.
type Local struct {
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks LocalNetworks
	// Endpoint of the local service.
	Endpoint string
}

// LocalNetworks holds information about the Kubernetes and infrastructure networks.
type LocalNetworks struct {
	gardencore.K8SNetworks
	// Workers is a list of CIDRs of worker subnets (private) to create (used for the VMs).
	Workers []gardencore.CIDR
}

// Worker is the base definition of a worker group.
type Worker struct {
	// Name is the name of the worker group.
	Name string
	// MachineType is the machine type of the worker group.
	MachineType string
	// AutoScalerMin is the minimum number of VMs to create.
	AutoScalerMin int
	// AutoScalerMin is the maximum number of VMs to create.
	AutoScalerMax int
	// MaxSurge is maximum number of VMs that are created during an update.
	MaxSurge intstr.IntOrString
	//MaxUnavailable is the maximum number of VMs that can be unavailable during an update.
	MaxUnavailable intstr.IntOrString
}

// Addons is a collection of configuration for specific addons which are managed by the Gardener.
type Addons struct {
	// KubernetesDashboard holds configuration settings for the kubernetes dashboard addon.
	// +optional
	KubernetesDashboard *KubernetesDashboard
	// NginxIngress holds configuration settings for the nginx-ingress addon.
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	NginxIngress *NginxIngress

	// ClusterAutoscaler holds configuration settings for the cluster autoscaler addon.
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	ClusterAutoscaler *ClusterAutoscaler
	// Heapster holds configuration settings for the heapster addon.
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	Heapster *Heapster
	// Kube2IAM holds configuration settings for the kube2iam addon (only AWS).
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	Kube2IAM *Kube2IAM
	// KubeLego holds configuration settings for the kube-lego addon.
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	KubeLego *KubeLego
	// Monocular holds configuration settings for the monocular addon.
	// DEPRECATED: This field will be removed in a future version.
	// +optional
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
}

// ClusterAutoscaler describes configuration values for the cluster-autoscaler addon.
type ClusterAutoscaler struct {
	Addon
}

// NginxIngress describes configuration values for the nginx-ingress addon.
type NginxIngress struct {
	Addon
	// LoadBalancerSourceRanges is list of whitelist IP sources for NginxIngress
	// +optional
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
	// +optional
	Mail string
}

// Kube2IAM describes configuration values for the kube2iam addon.
type Kube2IAM struct {
	Addon
	// Roles is list of AWS IAM roles which should be created by the Gardener.
	// +optional
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

// Backup - DEPRECATED: This struct will be removed in a future version.
type Backup struct {
	// DEPRECATED: This field will be removed in a future version.
	Schedule string
	// DEPRECATED: This field will be removed in a future version.
	Maximum int
}

// DNS holds information about the provider, the hosted zone id and the domain.
type DNS struct {
	// Provider is the DNS provider type for the Shoot.
	Provider DNSProvider
	// HostedZoneID is the ID of an existing DNS Hosted Zone used to create the DNS records in.
	// +optional
	HostedZoneID *string
	// Domain is the external available domain of the Shoot cluster.
	// +optional
	Domain *string
	// SecretName is a name of a secret containing credentials for the stated HostedZoneID and the
	// provider. When not specified, the Gardener will use the cloud provider credentials referenced
	// by the Shoot and try to find respective credentials there. Specifying this field may override
	// this behaviour, i.e. forcing the Gardener to only look into the given secret.
	// +optional
	SecretName *string
}

// DNSProvider is a string alias.
type DNSProvider string

const (
	// DNSUnmanaged is a constant for the 'unmanaged' DNS provider.
	DNSUnmanaged DNSProvider = "unmanaged"
	// DNSAWSRoute53 is a constant for the 'aws-route53' DNS provider.
	DNSAWSRoute53 DNSProvider = "aws-route53"
	// DNSGoogleCloudDNS is a constant for the 'google-clouddns' DNS provider.
	DNSGoogleCloudDNS DNSProvider = "google-clouddns"
	// DNSOpenstackDesignate is a constant for the designate DNS provider
	DNSOpenstackDesignate DNSProvider = "openstack-designate"
	// DNSAlicloud is a constant for Alicloud DNS provider
	DNSAlicloud DNSProvider = "alicloud-dns"
)

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
	// CloudProviderLocal is a constant for the local development provider.
	CloudProviderLocal CloudProvider = "local"
)

// Hibernation contains information whether the Shoot is suspended or not.
type Hibernation struct {
	// Enabled is true if Shoot is hibernated, false otherwise.
	Enabled bool
	// Schedules determines the hibernation schedules.
	// +optional
	Schedules []HibernationSchedule
}

// HibernationSchedule determines the hibernation schedule of a Shoot.
// A Shoot will be regularly hibernated at each start time and will be woken up at each end time.
// Start or End can be omitted, though at least one of each has to be specified.
type HibernationSchedule struct {
	// Start is a Cron spec at which time a Shoot will be hibernated.
	// +optional
	Start *string
	// End is a Cron spec at which time a Shoot will be woken up.
	// +optional
	End *string
}

// Kubernetes contains the version and configuration variables for the Shoot control plane.
type Kubernetes struct {
	// AllowPrivilegedContainers indicates whether privileged containers are allowed in the Shoot (default: true).
	// +optional
	AllowPrivilegedContainers *bool
	// KubeAPIServer contains configuration settings for the kube-apiserver.
	// +optional
	KubeAPIServer *KubeAPIServerConfig
	// CloudControllerManager contains configuration settings for the cloud-controller-manager.
	// +optional
	CloudControllerManager *CloudControllerManagerConfig
	// KubeControllerManager contains configuration settings for the kube-controller-manager.
	// +optional
	KubeControllerManager *KubeControllerManagerConfig
	// KubeScheduler contains configuration settings for the kube-scheduler.
	// +optional
	KubeScheduler *KubeSchedulerConfig
	// KubeProxy contains configuration settings for the kube-proxy.
	// +optional
	KubeProxy *KubeProxyConfig
	// Kubelet contains configuration settings for the kubelet.
	// +optional
	Kubelet *KubeletConfig
	// Version is the semantic Kubernetes version to use for the Shoot cluster.
	Version string
}

// KubernetesConfig contains common configuration fields for the control plane components.
type KubernetesConfig struct {
	// FeatureGates contains information about enabled feature gates.
	// +optional
	FeatureGates map[string]bool
}

// KubeAPIServerConfig contains configuration settings for the kube-apiserver.
type KubeAPIServerConfig struct {
	KubernetesConfig
	// RuntimeConfig contains information about enabled or disabled APIs.
	// +optional
	RuntimeConfig map[string]bool
	// OIDCConfig contains configuration settings for the OIDC provider.
	// +optional
	OIDCConfig *OIDCConfig
	// AdmissionPlugins contains the list of user-defined admission plugins (additional to those managed by Gardener), and, if desired, the corresponding
	// configuration.
	// +optional
	AdmissionPlugins []AdmissionPlugin
	// AuditConfig contains configuration settings for the audit of the kube-apiserver.
	// +optional
	AuditConfig *AuditConfig
}

// AuditConfig contains settings for audit of the api server
type AuditConfig struct {
	// AuditPolicy contains configuration settings for audit policy of the kube-apiserver.
	// +optional
	AuditPolicy *AuditPolicy
}

// AuditPolicy contains audit policy for kube-apiserver
type AuditPolicy struct {
	// ConfigMapRef is a reference to a ConfigMap object in the same namespace,
	// which contains the audit policy for the kube-apiserver.
	// +optional
	ConfigMapRef *corev1.LocalObjectReference
}

// OIDCConfig contains configuration settings for the OIDC provider.
// Note: Descriptions were taken from the Kubernetes documentation.
type OIDCConfig struct {
	// If set, the OpenID server's certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host's root CA set will be used.
	// +optional
	CABundle *string
	// The client ID for the OpenID Connect client, must be set if oidc-issuer-url is set.
	// +optional
	ClientID *string
	// If provided, the name of a custom OpenID Connect claim for specifying user groups. The claim value is expected to be a string or array of strings. This flag is experimental, please see the authentication documentation for further details.
	// +optional
	GroupsClaim *string
	// If provided, all groups will be prefixed with this value to prevent conflicts with other authentication strategies.
	// +optional
	GroupsPrefix *string
	// The URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT).
	// +optional
	IssuerURL *string
	// ATTENTION: Only meaningful for Kubernetes >= 1.11
	// key=value pairs that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value.
	// +optional
	RequiredClaims map[string]string
	// List of allowed JOSE asymmetric signing algorithms. JWTs with a 'alg' header value not in this list will be rejected. Values are defined by RFC 7518 https://tools.ietf.org/html/rfc7518#section-3.1
	// +optional
	SigningAlgs []string
	// The OpenID claim to use as the user name. Note that claims other than the default ('sub') is not guaranteed to be unique and immutable. This flag is experimental, please see the authentication documentation for further details. (default "sub")
	// +optional
	UsernameClaim *string
	// If provided, all usernames will be prefixed with this value. If not provided, username claims other than 'email' are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value '-'.
	// +optional
	UsernamePrefix *string
}

// AdmissionPlugin contains information about a specific admission plugin and its corresponding configuration.
type AdmissionPlugin struct {
	// Name is the name of the plugin.
	Name string
	// Config is the configuration of the plugin.
	// NOTE: After a discussion with @mvladev we decided to not use the runtime.RawExtension type for the configuration
	// for now as there seems to be a bug with the OpenAPI generation which would make kubectl not correctly validate
	// the objects (see also https://github.com/kubernetes-sigs/cluster-api/issues/137). We keep it as string for now
	// and will later migrate the Go type to runtime.RawExtension once the issues have been resolved.
	// SEE ALSO: https://github.com/gardener/gardener/pull/322
	// +optional
	Config *string
}

// CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.
type CloudControllerManagerConfig struct {
	KubernetesConfig
}

// KubeControllerManagerConfig contains configuration settings for the kube-controller-manager.
type KubeControllerManagerConfig struct {
	KubernetesConfig
	// HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
	// +optional
	HorizontalPodAutoscalerConfig *HorizontalPodAutoscalerConfig
}

// HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
// Note: Descriptions were taken from the Kubernetes documentation.
type HorizontalPodAutoscalerConfig struct {
	// DownscaleDelay is the period since last downscale, before another downscale can be performed in horizontal pod autoscaler.
	// +optional
	DownscaleDelay *metav1.Duration
	// SyncPeriod is the period for syncing the number of pods in horizontal pod autoscaler.
	// +optional
	SyncPeriod *metav1.Duration
	// Tolerance is the minimum change (from 1.0) in the desired-to-actual metrics ratio for the horizontal pod autoscaler to consider scaling.
	// +optional
	Tolerance *float64
	// UpscaleDelay is the period since last upscale, before another upscale can be performed in horizontal pod autoscaler.
	// +optional
	UpscaleDelay *metav1.Duration
	// DownscaleStabilization is the period for which autoscaler will look backwards and not scale down below any recommendation it made during that period.
	// +optional
	DownscaleStabilization *metav1.Duration
	// InitialReadinessDelay is the  period after pod start during which readiness changes will be treated as initial readiness.
	// +optional
	InitialReadinessDelay *metav1.Duration
	// CPUInitializationPeriod is the period after pod start when CPU samples might be skipped.
	// +optional
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
}

// Maintenance contains information about the time window for maintenance operations and which
// operations should be performed.
type Maintenance struct {
	// AutoUpdate contains information about which constraints should be automatically updated.
	// +optional
	AutoUpdate *MaintenanceAutoUpdate
	// TimeWindow contains information about the time window for maintenance operations.
	// +optional
	TimeWindow *MaintenanceTimeWindow
}

// MaintenanceAutoUpdate contains information about which constraints should be automatically updated.
type MaintenanceAutoUpdate struct {
	// KubernetesVersion indicates whether the patch Kubernetes version may be automatically updated.
	KubernetesVersion bool
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

const (
	// DefaultETCDBackupSchedule is a constant for the default schedule to take backups of a Shoot cluster (5 minutes).
	DefaultETCDBackupSchedule = "0 */24 * * *"
	// DefaultETCDBackupMaximum is a constant for the default number of etcd backups to keep for a Shoot cluster.
	DefaultETCDBackupMaximum = 7
	// MinimumETCDFullBackupTimeInterval is the time interval between consecutive full backups.
	MinimumETCDFullBackupTimeInterval = 24 * time.Hour
)

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
	SeedAvailable gardencore.ConditionType = "Available"

	// ShootControlPlaneHealthy is a constant for a condition type indicating the control plane health.
	ShootControlPlaneHealthy gardencore.ConditionType = "ControlPlaneHealthy"
	// ShootEveryNodeReady is a constant for a condition type indicating the node health.
	ShootEveryNodeReady gardencore.ConditionType = "EveryNodeReady"
	// ShootSystemComponentsHealthy is a constant for a condition type indicating the system components health.
	ShootSystemComponentsHealthy gardencore.ConditionType = "SystemComponentsHealthy"
	// ShootAPIServerAvailable is a constant for a condition type indicating the api server is available.
	ShootAPIServerAvailable gardencore.ConditionType = "APIServerAvailable"
)

////////////////////////////////////////////////////
//              Backup Infrastructure             //
////////////////////////////////////////////////////

// BackupInfrastructure holds details about backup infrastructure
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=x-kubernetes-print-columns:custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,SEED:.spec.seed,STATUS:.status.lastOperation.state
type BackupInfrastructure struct {
	metav1.TypeMeta
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta
	// Specification of the Backup Infrastructure.
	// +optional
	Spec BackupInfrastructureSpec
	// Most recently observed status of the Backup Infrastructure.
	// +optional
	Status BackupInfrastructureStatus
}

// BackupInfrastructureList is a list of BackupInfrastructure objects.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type BackupInfrastructureList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	// +optional
	metav1.ListMeta
	// Items is the list of BackupInfrastructure.
	Items []BackupInfrastructure
}

// BackupInfrastructureSpec is the specification of a Backup Infrastructure.
type BackupInfrastructureSpec struct {
	// Seed is the name of a Seed object.
	Seed string
	// ShootUID is a unique identifier for the Shoot cluster for which the BackupInfrastructure object is created.
	ShootUID types.UID
}

// BackupInfrastructureStatus holds the most recently observed status of the Backup Infrastructure.
type BackupInfrastructureStatus struct {
	// LastOperation holds information about the last operation on the BackupInfrastructure.
	// +optional
	LastOperation *gardencore.LastOperation
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *gardencore.LastError
	// ObservedGeneration is the most recent generation observed for this BackupInfrastructure. It corresponds to the
	// BackupInfrastructure's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64
}
