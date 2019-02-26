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

package v1beta1

import (
	"encoding/json"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"

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
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec defines the cloud environment properties.
	// +optional
	Spec CloudProfileSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileList is a collection of CloudProfiles.
type CloudProfileList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of CloudProfiles.
	Items []CloudProfile `json:"items"`
}

// CloudProfileSpec is the specification of a CloudProfile.
// It must contain exactly one of its defined keys.
type CloudProfileSpec struct {
	// AWS is the profile specification for the Amazon Web Services cloud.
	// +optional
	AWS *AWSProfile `json:"aws,omitempty"`
	// Azure is the profile specification for the Microsoft Azure cloud.
	// +optional
	Azure *AzureProfile `json:"azure,omitempty"`
	// GCP is the profile specification for the Google Cloud Platform cloud.
	// +optional
	GCP *GCPProfile `json:"gcp,omitempty"`
	// OpenStack is the profile specification for the OpenStack cloud.
	// +optional
	OpenStack *OpenStackProfile `json:"openstack,omitempty"`
	// Alicloud is the profile specification for the Alibaba cloud.
	// +optional
	Alicloud *AlicloudProfile `json:"alicloud,omitempty"`
	// Packet is the profile specification for the Packet cloud.
	// +optional
	Packet *PacketProfile `json:"packet,omitempty"`
	// Local is the profile specification for the Local provider.
	// +optional
	Local *LocalProfile `json:"local,omitempty"`
	// CABundle is a certificate bundle which will be installed onto every host machine of the Shoot cluster.
	// +optional
	CABundle *string `json:"caBundle,omitempty"`
}

// AWSProfile defines certain constraints and definitions for the AWS cloud.
type AWSProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints AWSConstraints `json:"constraints"`
}

// AWSConstraints is an object containing constraints for certain values in the Shoot specification.
type AWSConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint `json:"dnsProviders"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints `json:"kubernetes"`
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []AWSMachineImageMapping `json:"machineImages"`
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType `json:"machineTypes"`
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType `json:"volumeTypes"`
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone `json:"zones"`
}

// AWSMachineImage defines the region and the AMI for a machine image.
type AWSMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName `json:"name"`
	// AMI is the technical id of the image (region specific).
	AMI string `json:"ami"`
}

// AWSMachineImageMapping is a mapping of machine images to regions.
type AWSMachineImageMapping struct {
	// Name is the name of the image.
	Name MachineImageName `json:"name"`
	// Regions is a list of machine images with their regional technical id.
	Regions []AWSRegionalMachineImage `json:"regions"`
}

type AWSRegionalMachineImage struct {
	// Name is the name of a region.
	Name string `json:"name"`
	// AMI is the technical id of the image (specific for region stated in the 'Name' field).
	AMI string `json:"ami"`
}

// AzureProfile defines certain constraints and definitions for the Azure cloud.
type AzureProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints AzureConstraints `json:"constraints"`
	// CountUpdateDomains is list of Azure update domain counts for each region.
	CountUpdateDomains []AzureDomainCount `json:"countUpdateDomains"`
	// CountFaultDomains is list of Azure fault domain counts for each region.
	CountFaultDomains []AzureDomainCount `json:"countFaultDomains"`
}

// AzureConstraints is an object containing constraints for certain values in the Shoot specification.
type AzureConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint `json:"dnsProviders"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints `json:"kubernetes"`
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []AzureMachineImage `json:"machineImages"`
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType `json:"machineTypes"`
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType `json:"volumeTypes"`
}

// AzureDomainCount defines the region and the count for this domain count value.
type AzureDomainCount struct {
	// Region is a region in Azure.
	Region string `json:"region"`
	// Count is the count value for the respective domain count.
	Count int `json:"count"`
}

// AzureMachineImage defines the channel and the version of the machine image in the Azure environment.
type AzureMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName `json:"name"`
	// Publisher is the publisher of the image.
	Publisher string `json:"publisher"`
	// Offer is the offering of the image.
	Offer string `json:"offer"`
	// SKU is the stock keeping unit to pull images from.
	SKU string `json:"sku"`
	// Version is the version of the image.
	Version string `json:"version"`
}

// GCPProfile defines certain constraints and definitions for the GCP cloud.
type GCPProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints GCPConstraints `json:"constraints"`
}

// GCPConstraints is an object containing constraints for certain values in the Shoot specification.
type GCPConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint `json:"dnsProviders"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints `json:"kubernetes"`
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []GCPMachineImage `json:"machineImages"`
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType `json:"machineTypes"`
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType `json:"volumeTypes"`
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone `json:"zones"`
}

// GCPMachineImage defines the name of the machine image in the GCP environment.
type GCPMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName `json:"name"`
	// Image is the technical name of the image. It contains the image name and the Google Cloud project.
	// Example: projects/<name>/global/images/version23
	Image string `json:"image"`
}

// OpenStackProfile defines certain constraints and definitions for the OpenStack cloud.
type OpenStackProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints OpenStackConstraints `json:"constraints"`
	// KeyStoneURL is the URL for auth{n,z} in OpenStack (pointing to KeyStone).
	KeyStoneURL string `json:"keystoneURL"`
	// DNSServers is a list of IPs of DNS servers used while creating subnets.
	// +optional
	DNSServers []string `json:"dnsServers,omitempty"`
	// DHCPDomain is the dhcp domain of the OpenStack system configured in nova.conf. Only meaningful for
	// Kubernetes 1.10.1+. See https://github.com/kubernetes/kubernetes/pull/61890 for details.
	// +optional
	DHCPDomain *string `json:"dhcpDomain,omitempty"`
	// RequestTimeout specifies the HTTP timeout against the OpenStack API.
	// +optional
	RequestTimeout *string `json:"requestTimeout,omitempty"`
}

// OpenStackConstraints is an object containing constraints for certain values in the Shoot specification.
type OpenStackConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint `json:"dnsProviders"`
	// FloatingPools contains constraints regarding allowed values of the 'floatingPoolName' block in the Shoot specification.
	FloatingPools []OpenStackFloatingPool `json:"floatingPools"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints `json:"kubernetes"`
	// LoadBalancerProviders contains constraints regarding allowed values of the 'loadBalancerProvider' block in the Shoot specification.
	LoadBalancerProviders []OpenStackLoadBalancerProvider `json:"loadBalancerProviders"`
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []OpenStackMachineImage `json:"machineImages"`
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []OpenStackMachineType `json:"machineTypes"`
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone `json:"zones"`
}

// FloatingPools contains constraints regarding allowed values of the 'floatingPoolName' block in the Shoot specification.
type OpenStackFloatingPool struct {
	// Name is the name of the floating pool.
	Name string `json:"name"`
}

// LoadBalancerProviders contains constraints regarding allowed values of the 'loadBalancerProvider' block in the Shoot specification.
type OpenStackLoadBalancerProvider struct {
	// Name is the name of the load balancer provider.
	Name string `json:"name"`
}

// OpenStackMachineImage defines the name of the machine image in the OpenStack environment.
type OpenStackMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName `json:"name"`
	// Image is the technical name of the image.
	Image string `json:"image"`
}

// AlicloudProfile defines constraints and definitions in Alibaba Cloud environment.
type AlicloudProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints AlicloudConstraints `json:"constraints"`
}

// AlicloudConstraints is an object containing constraints for certain values in the Shoot specification
type AlicloudConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint `json:"dnsProviders"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints `json:"kubernetes"`
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []AlicloudMachineImage `json:"machineImages"`
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []AlicloudMachineType `json:"machineTypes"`
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []AlicloudVolumeType `json:"volumeTypes"`
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone `json:"zones"`
}

// AlicloudMachineImage defines the machine image for Alicloud.
type AlicloudMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName `json:"name"`
	// ID is the ID of the image.
	ID string `json:"id"`
}

// AlicloudMachineType defines certain machine types and zone constraints.
type AlicloudMachineType struct {
	MachineType `json:",inline"`
	Zones       []string `json:"zones"`
}

// AlicloudVolumeType defines certain volume types and zone constraints.
type AlicloudVolumeType struct {
	VolumeType `json:",inline"`
	Zones      []string `json:"zones"`
}

// PacketProfile defines constraints and definitions in Packet Cloud environment.
type PacketProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints PacketConstraints `json:"constraints"`
}

// PacketConstraints is an object containing constraints for certain values in the Shoot specification
type PacketConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint `json:"dnsProviders"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints `json:"kubernetes"`
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []PacketMachineImage `json:"machineImages"`
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType `json:"machineTypes"`
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType `json:"volumeTypes"`
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone `json:"zones"`
}

// PacketMachineImage defines the machine image for Packet.
type PacketMachineImage struct {
	// Name is the name of the image.
	Name MachineImageName `json:"name"`
	// ID is the ID of the image.
	ID string `json:"id"`
}

// LocalProfile defines constraints and definitions for the local development.
type LocalProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints LocalConstraints `json:"constraints"`
}

// LocalConstraints is an object containing constraints for certain values in the Shoot specification.
type LocalConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint `json:"dnsProviders"`
}

// DNSProviderConstraint contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
type DNSProviderConstraint struct {
	// Name is the name of the DNS provider.
	Name DNSProvider `json:"name"`
}

// KubernetesConstraints contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
type KubernetesConstraints struct {
	// Versions is the list of allowed Kubernetes versions for Shoot clusters (e.g., 1.13.1).
	Versions []string `json:"versions"`
}

// MachineType contains certain properties of a machine type.
type MachineType struct {
	// Name is the name of the machine type.
	Name string `json:"name"`
	// Usable defines if the machine type can be used for shoot clusters.
	// +optional
	Usable *bool `json:"usable,omitempty"`
	// CPU is the number of CPUs for this machine type.
	CPU resource.Quantity `json:"cpu"`
	// GPU is the number of GPUs for this machine type.
	GPU resource.Quantity `json:"gpu"`
	// Memory is the amount of memory for this machine type.
	Memory resource.Quantity `json:"memory"`
}

// OpenStackMachineType contains certain properties of a machine type in OpenStack
type OpenStackMachineType struct {
	MachineType `json:",inline"`
	// VolumeType is the type of that volume.
	VolumeType string `json:"volumeType"`
	// VolumeSize is the amount of disk storage for this machine type.
	VolumeSize resource.Quantity `json:"volumeSize"`
}

// VolumeType contains certain properties of a volume type.
type VolumeType struct {
	// Name is the name of the volume type.
	Name string `json:"name"`
	// Usable defines if the volume type can be used for shoot clusters.
	// +optional
	Usable *bool `json:"usable,omitempty"`
	// Class is the class of the volume type.
	Class string `json:"class"`
}

// Zone contains certain properties of an availability zone.
type Zone struct {
	// Region is a region name.
	Region string `json:"region"`
	// Names is a list of availability zone names in this region.
	Names []string `json:"names"`
}

// MachineImageName is a string alias.
type MachineImageName string

const (
	// MachineImageCoreOS is a constant for the CoreOS machine image.
	// deprecated
	MachineImageCoreOS MachineImageName = "coreos"
	// MachineImageCoreOSAlicloud is a constant for the CoreOS machine image used by Alicloud.
	// The Alicloud CoreOS image is modified (e.g., it does not support cloud-config, and is therefore
	// treated like another OS).
	// deprecated
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
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec defines the project properties.
	// +optional
	Spec ProjectSpec `json:"spec,omitempty"`
	// Most recently observed status of the Project.
	// +optional
	Status ProjectStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectList is a collection of Projects.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of Projects.
	Items []Project `json:"items"`
}

// ProjectSpec is the specification of a Project.
type ProjectSpec struct {
	// CreatedBy is a subject representing a user name, an email address, or any other identifier of a user
	// who created the project.
	// +optional
	CreatedBy *rbacv1.Subject `json:"createdBy,omitempty"`
	// Description is a human-readable description of what the project is used for.
	// +optional
	Description *string `json:"description,omitempty"`
	// Owner is a subject representing a user name, an email address, or any other identifier of a user owning
	// the project.
	// +optional
	Owner *rbacv1.Subject `json:"owner,omitempty"`
	// Purpose is a human-readable explanation of the project's purpose.
	// +optional
	Purpose *string `json:"purpose,omitempty"`
	// Members is a list of subjects representing a user name, an email address, or any other identifier of a user
	// that should be part of this project.
	// +optional
	Members []rbacv1.Subject `json:"members,omitempty"`
	// Namespace is the name of the namespace that has been created for the Project object.
	// A nil value means that Gardener will determine the name of the namespace.
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// ProjectStatus holds the most recently observed status of the project.
type ProjectStatus struct {
	// ObservedGeneration is the most recent generation observed for this project.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Phase is the current phase of the project.
	Phase ProjectPhase `json:"phase,omitempty"`
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
// +k8s:openapi-gen=x-kubernetes-print-columns:custom-columns=NAME:.metadata.name,DOMAIN:.spec.ingressDomain,CLOUDPROFILE:.spec.cloud.profile,REGION:.spec.cloud.profile,READY:.status.conditions[?(@.type == 'Available')].status
type Seed struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec defines the Seed cluster properties.
	// +optional
	Spec SeedSpec `json:"spec,omitempty"`
	// Most recently observed status of the Seed cluster.
	// +optional
	Status SeedStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SeedList is a collection of Seeds.
type SeedList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of Seeds.
	Items []Seed `json:"items"`
}

// SeedSpec is the specification of a Seed.
type SeedSpec struct {
	// Cloud defines the cloud profile and the region this Seed cluster belongs to.
	Cloud SeedCloud `json:"cloud"`
	// IngressDomain is the domain of the Seed cluster pointing to the ingress controller endpoint. It will be used
	// to construct ingress URLs for system applications running in Shoot clusters.
	IngressDomain string `json:"ingressDomain"`
	// SecretRef is a reference to a Secret object containing the Kubeconfig and the cloud provider credentials for
	// the account the Seed cluster has been deployed to.
	SecretRef corev1.SecretReference `json:"secretRef"`
	// Networks defines the pod, service and worker network of the Seed cluster.
	Networks SeedNetworks `json:"networks"`
	// Visible labels the Seed cluster as selectable for the seedfinder admission controller.
	// +optional
	Visible *bool `json:"visible,omitempty"`
	// Protected prevent that the Seed Cluster can be used for regular Shoot cluster control planes.
	// +optional
	Protected *bool `json:"protected,omitempty"`
}

// SeedStatus holds the most recently observed status of the Seed cluster.
type SeedStatus struct {
	// Conditions represents the latest available observations of a Seed's current state.
	// +optional
	Conditions []gardencorev1alpha1.Condition `json:"conditions,omitempty"`
}

// SeedCloud defines the cloud profile and the region this Seed cluster belongs to.
type SeedCloud struct {
	// Profile is the name of a cloud profile.
	Profile string `json:"profile"`
	// Region is a name of a region.
	Region string `json:"region"`
}

// SeedNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
type SeedNetworks struct {
	// Nodes is the CIDR of the node network.
	Nodes gardencorev1alpha1.CIDR `json:"nodes"`
	// Pods is the CIDR of the pod network.
	Pods gardencorev1alpha1.CIDR `json:"pods"`
	// Services is the CIDR of the service network.
	Services gardencorev1alpha1.CIDR `json:"services"`
}

////////////////////////////////////////////////////
//                      QUOTAS                    //
////////////////////////////////////////////////////

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Quota struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec defines the Quota constraints.
	// +optional
	Spec QuotaSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuotaList is a collection of Quotas.
type QuotaList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of Quotas.
	Items []Quota `json:"items"`
}

// QuotaSpec is the specification of a Quota.
type QuotaSpec struct {
	// ClusterLifetimeDays is the lifetime of a Shoot cluster in days before it will be terminated automatically.
	// +optional
	ClusterLifetimeDays *int `json:"clusterLifetimeDays,omitempty"`
	// Metrics is a list of resources which will be put under constraints.
	Metrics corev1.ResourceList `json:"metrics"`
	// Scope is the scope of the Quota object, either 'project' or 'secret'.
	Scope QuotaScope `json:"scope"`
}

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
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// SecretRef is a reference to a secret object in the same or another namespace.
	SecretRef corev1.SecretReference `json:"secretRef"`
	// Quotas is a list of references to Quota objects in the same or another namespace.
	// +optional
	Quotas []corev1.ObjectReference `json:"quotas,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretBindingList is a collection of SecretBindings.
type SecretBindingList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of SecretBindings.
	Items []SecretBinding `json:"items"`
}

////////////////////////////////////////////////////
//                      SHOOTS                    //
////////////////////////////////////////////////////

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=x-kubernetes-print-columns:custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,SEED:.spec.cloud.seed,DOMAIN:.spec.dns.domain,VERSION:.spec.kubernetes.version,CONTROL:.status.conditions[?(@.type == 'ControlPlaneHealthy')].status,NODES:.status.conditions[?(@.type == 'EveryNodeReady')].status,SYSTEM:.status.conditions[?(@.type == 'SystemComponentsHealthy')].status,LATEST:.status.lastOperation.state
type Shoot struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the Shoot cluster.
	// +optional
	Spec ShootSpec `json:"spec,omitempty"`
	// Most recently observed status of the Shoot cluster.
	// +optional
	Status ShootStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootList is a list of Shoot objects.
type ShootList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of Shoots.
	Items []Shoot `json:"items"`
}

// ShootSpec is the specification of a Shoot.
type ShootSpec struct {
	// Addons contains information about enabled/disabled addons and their configuration.
	// +optional
	Addons *Addons `json:"addons,omitempty"`
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	Backup *Backup `json:"backup,omitempty"`
	// Cloud contains information about the cloud environment and their specific settings.
	Cloud Cloud `json:"cloud"`
	// DNS contains information about the DNS settings of the Shoot.
	DNS DNS `json:"dns"`
	// Hibernation contains information whether the Shoot is suspended or not.
	// +optional
	Hibernation *Hibernation `json:"hibernation,omitempty"`
	// Kubernetes contains the version and configuration settings of the control plane components.
	Kubernetes Kubernetes `json:"kubernetes"`
	// Maintenance contains information about the time window for maintenance operations and which
	// operations should be performed.
	// +optional

	Maintenance *Maintenance `json:"maintenance,omitempty"`
}

// ShootStatus holds the most recently observed status of the Shoot cluster.
type ShootStatus struct {
	// Conditions represents the latest available observations of a Shoots's current state.
	// +optional
	Conditions []gardencorev1alpha1.Condition `json:"conditions,omitempty"`
	// Gardener holds information about the Gardener which last acted on the Shoot.
	Gardener Gardener `json:"gardener"`
	// LastOperation holds information about the last operation on the Shoot.
	// +optional
	LastOperation *gardencorev1alpha1.LastOperation `json:"lastOperation,omitempty"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *gardencorev1alpha1.LastError `json:"lastError,omitempty"`
	// ObservedGeneration is the most recent generation observed for this Shoot. It corresponds to the
	// Shoot's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// RetryCycleStartTime is the start time of the last retry cycle (used to determine how often an operation
	// must be retried until we give up).
	// +optional
	RetryCycleStartTime *metav1.Time `json:"retryCycleStartTime,omitempty"`
	// Seed is the name of the seed cluster that runs the control plane of the Shoot. This value is only written
	// after a successful create/reconcile operation. It will be used when control planes are moved between Seeds.
	Seed string `json:"seed,omitempty"`
	// TechnicalID is the name that is used for creating the Seed namespace, the infrastructure resources, and
	// basically everything that is related to this particular Shoot.
	TechnicalID string `json:"technicalID"`
	// UID is a unique identifier for the Shoot cluster to avoid portability between Kubernetes clusters.
	// It is used to compute unique hashes.
	UID types.UID `json:"uid"`
}

///////////////////////////////
// Shoot Specification Types //
///////////////////////////////

// Cloud contains information about the cloud environment and their specific settings.
// It must contain exactly one key of the below cloud providers.
type Cloud struct {
	// Profile is a name of a CloudProfile object.
	Profile string `json:"profile"`
	// Region is a name of a cloud provider region.
	Region string `json:"region"`
	// SecretBindingRef is a reference to a SecretBinding object.
	SecretBindingRef corev1.LocalObjectReference `json:"secretBindingRef"`
	// Seed is the name of a Seed object.
	// +optional
	Seed *string `json:"seed,omitempty"`
	// AWS contains the Shoot specification for the Amazon Web Services cloud.
	// +optional
	AWS *AWSCloud `json:"aws,omitempty"`
	// Azure contains the Shoot specification for the Microsoft Azure cloud.
	// +optional
	Azure *AzureCloud `json:"azure,omitempty"`
	// GCP contains the Shoot specification for the Google Cloud Platform cloud.
	// +optional
	GCP *GCPCloud `json:"gcp,omitempty"`
	// OpenStack contains the Shoot specification for the OpenStack cloud.
	// +optional
	OpenStack *OpenStackCloud `json:"openstack,omitempty"`
	// Alicloud contains the Shoot specification for the Alibaba cloud.
	// +optional
	Alicloud *Alicloud `json:"alicloud,omitempty"`
	// Packet contains the Shoot specification for the Packet cloud.
	// +optional
	Packet *PacketCloud `json:"packet,omitempty"`
	// Local contains the Shoot specification for the Local local provider.
	// +optional
	Local *Local `json:"local,omitempty"`
}

// AWSCloud contains the Shoot specification for AWS.

type AWSCloud struct {
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *AWSMachineImage `json:"machineImage,omitempty"`
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AWSNetworks `json:"networks"`
	// Workers is a list of worker groups.
	Workers []AWSWorker `json:"workers"`
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string `json:"zones"`
}

// AWSNetworks holds information about the Kubernetes and infrastructure networks.
type AWSNetworks struct {
	gardencorev1alpha1.K8SNetworks `json:",inline"`
	// VPC indicates whether to use an existing VPC or create a new one.
	VPC AWSVPC `json:"vpc"`
	// Internal is a list of private subnets to create (used for internal load balancers).
	Internal []gardencorev1alpha1.CIDR `json:"internal"`
	// Public is a list of public subnets to create (used for bastion and load balancers).
	Public []gardencorev1alpha1.CIDR `json:"public"`
	// Workers is a list of worker subnets (private) to create (used for the VMs).
	Workers []gardencorev1alpha1.CIDR `json:"workers"`
}

// AWSVPC contains either an id (of an existing VPC) or the CIDR (for a VPC to be created).
type AWSVPC struct {
	// ID is the AWS VPC id of an existing VPC.
	// +optional
	ID *string `json:"id,omitempty"`
	// CIDR is a CIDR range for a new VPC.
	// +optional
	CIDR *gardencorev1alpha1.CIDR `json:"cidr,omitempty"`
}

// AWSWorker is the definition of a worker group.
type AWSWorker struct {
	Worker `json:",inline"`
	// VolumeType is the type of the root volumes.
	VolumeType string `json:"volumeType"`
	// VolumeSize is the size of the root volume.
	VolumeSize string `json:"volumeSize"`
}

// Alicloud contains the Shoot specification for Alibaba cloud
type Alicloud struct {
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *AlicloudMachineImage `json:"machineImage,omitempty"`
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AlicloudNetworks `json:"networks"`
	// Workers is a list of worker groups.
	Workers []AlicloudWorker `json:"workers"`
	// Zones is a list of availability zones to deploy the Shoot cluster to, currently, only one is supported.
	Zones []string `json:"zones"`
}

// AlicloudVPC contains either an id (of an existing VPC) or the CIDR (for a VPC to be created).
type AlicloudVPC struct {
	// ID is the Alicloud VPC id of an existing VPC.
	// +optional
	ID *string `json:"id,omitempty"`
	// CIDR is a CIDR range for a new VPC.
	// +optional
	CIDR *gardencorev1alpha1.CIDR `json:"cidr,omitempty"`
}

// AlicloudNetworks holds information about the Kubernetes and infrastructure networks.
type AlicloudNetworks struct {
	gardencorev1alpha1.K8SNetworks `json:",inline"`
	// VPC indicates whether to use an existing VPC or create a new one.
	VPC AlicloudVPC `json:"vpc"`
	// Workers is a CIDR of a worker subnet (private) to create (used for the VMs).
	Workers []gardencorev1alpha1.CIDR `json:"workers"`
}

// AlicloudWorker is the definition of a worker group.
type AlicloudWorker struct {
	Worker `json:",inline"`
	// VolumeType is the type of the root volumes.
	VolumeType string `json:"volumeType"`
	// VolumeSize is the size of the root volume.
	VolumeSize string `json:"volumeSize"`
}

// PacketCloud contains the Shoot specification for Packet cloud
type PacketCloud struct {
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *PacketMachineImage `json:"machineImage,omitempty"`
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks PacketNetworks `json:"networks"`
	// Workers is a list of worker groups.
	Workers []PacketWorker `json:"workers"`
	// Zones is a list of availability zones to deploy the Shoot cluster to, currently, only one is supported.
	Zones []string `json:"zones"`
}

// PacketNetworks holds information about the Kubernetes and infrastructure networks.
type PacketNetworks struct {
	gardencorev1alpha1.K8SNetworks `json:",inline"`
}

// PacketWorker is the definition of a worker group.
type PacketWorker struct {
	Worker `json:",inline"`
	// VolumeType is the type of the root volumes.
	VolumeType string `json:"volumeType"`
	// VolumeSize is the size of the root volume.
	VolumeSize string `json:"volumeSize"`
}

// AzureCloud contains the Shoot specification for Azure.
type AzureCloud struct {
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *AzureMachineImage `json:"machineImage,omitempty"`
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AzureNetworks `json:"networks"`
	// ResourceGroup indicates whether to use an existing resource group or create a new one.
	// +optional
	ResourceGroup *AzureResourceGroup `json:"resourceGroup,omitempty"`
	// Workers is a list of worker groups.
	Workers []AzureWorker `json:"workers"`
}

// AzureResourceGroup indicates whether to use an existing resource group or create a new one.
type AzureResourceGroup struct {
	// Name is the name of an existing resource group.
	Name string `json:"name"`
}

// AzureNetworks holds information about the Kubernetes and infrastructure networks.
type AzureNetworks struct {
	gardencorev1alpha1.K8SNetworks `json:",inline"`
	// VNet indicates whether to use an existing VNet or create a new one.
	VNet AzureVNet `json:"vnet"`
	// Workers is a CIDR of a worker subnet (private) to create (used for the VMs).
	Workers gardencorev1alpha1.CIDR `json:"workers"`
}

// AzureVNet indicates whether to use an existing VNet or create a new one.
type AzureVNet struct {
	// Name is the AWS VNet name of an existing VNet.
	// +optional
	Name *string `json:"name,omitempty"`
	// CIDR is a CIDR range for a new VNet.
	// +optional
	CIDR *gardencorev1alpha1.CIDR `json:"cidr,omitempty"`
}

// AzureWorker is the definition of a worker group.
type AzureWorker struct {
	Worker `json:",inline"`
	// VolumeType is the type of the root volumes.
	VolumeType string `json:"volumeType"`
	// VolumeSize is the size of the root volume.
	VolumeSize string `json:"volumeSize"`
}

// GCPCloud contains the Shoot specification for GCP.
type GCPCloud struct {
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *GCPMachineImage `json:"machineImage,omitempty"`
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks GCPNetworks `json:"networks"`
	// Workers is a list of worker groups.
	Workers []GCPWorker `json:"workers"`
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string `json:"zones"`
}

// GCPNetworks holds information about the Kubernetes and infrastructure networks.
type GCPNetworks struct {
	gardencorev1alpha1.K8SNetworks `json:",inline"`
	// VPC indicates whether to use an existing VPC or create a new one.
	// +optional
	VPC *GCPVPC `json:"vpc,omitempty"`
	// Workers is a list of CIDRs of worker subnets (private) to create (used for the VMs).
	Workers []gardencorev1alpha1.CIDR `json:"workers"`
	// Internal is a private subnet (used for internal load balancers).
	// +optional
	Internal *gardencorev1alpha1.CIDR `json:"internal,omitempty"`
}

// GCPVPC indicates whether to use an existing VPC or create a new one.
type GCPVPC struct {
	// Name is the name of an existing GCP VPC.
	Name string `json:"name"`
}

// GCPWorker is the definition of a worker group.
type GCPWorker struct {
	Worker `json:",inline"`
	// VolumeType is the type of the root volumes.
	VolumeType string `json:"volumeType"`
	// VolumeSize is the size of the root volume.
	VolumeSize string `json:"volumeSize"`
}

// OpenStackCloud contains the Shoot specification for OpenStack.
type OpenStackCloud struct {
	// FloatingPoolName is the name of the floating pool to get FIPs from.
	FloatingPoolName string `json:"floatingPoolName"`
	// LoadBalancerProvider is the name of the load balancer provider in the OpenStack environment.
	LoadBalancerProvider string `json:"loadBalancerProvider"`
	// MachineImage holds information about the machine image to use for all workers.
	// It will default to the first image stated in the referenced CloudProfile if no
	// value has been provided.
	// +optional
	MachineImage *OpenStackMachineImage `json:"machineImage,omitempty"`
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks OpenStackNetworks `json:"networks"`
	// Workers is a list of worker groups.
	Workers []OpenStackWorker `json:"workers"`
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string `json:"zones"`
}

// OpenStackNetworks holds information about the Kubernetes and infrastructure networks.
type OpenStackNetworks struct {
	gardencorev1alpha1.K8SNetworks `json:",inline"`
	// Router indicates whether to use an existing router or create a new one.
	// +optional
	Router *OpenStackRouter `json:"router,omitempty"`
	// Workers is a list of CIDRs of worker subnets (private) to create (used for the VMs).
	Workers []gardencorev1alpha1.CIDR `json:"workers"`
}

// OpenStackRouter indicates whether to use an existing router or create a new one.
type OpenStackRouter struct {
	// ID is the router id of an existing OpenStack router.
	ID string `json:"id"`
}

// OpenStackWorker is the definition of a worker group.
type OpenStackWorker struct {
	Worker `json:",inline"`
}

// Local contains the Shoot specification for local provider.
type Local struct {
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks LocalNetworks `json:"networks"`
	// Endpoint of the local service.
	Endpoint string `json:"endpoint"`
}

// LocalNetworks holds information about the Kubernetes and infrastructure networks.
type LocalNetworks struct {
	gardencorev1alpha1.K8SNetworks `json:",inline"`
	// Workers is a CIDR of a worker subnet (private) to create (used for the VMs).
	Workers []gardencorev1alpha1.CIDR `json:"workers"`
}

// Worker is the base definition of a worker group.
type Worker struct {
	// Name is the name of the worker group.
	Name string `json:"name"`
	// MachineType is the machine type of the worker group.
	MachineType string `json:"machineType"`
	// AutoScalerMin is the minimum number of VMs to create.
	AutoScalerMin int `json:"autoScalerMin"`
	// AutoScalerMin is the maximum number of VMs to create.
	AutoScalerMax int `json:"autoScalerMax"`
	// MaxSurge is maximum number of VMs that are created during an update.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`
	//MaxUnavailable is the maximum number of VMs that can be unavailable during an update.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

var (
	// DefaultWorkerMaxSurge is the default value for Worker MaxSurge.
	DefaultWorkerMaxSurge = intstr.FromInt(1)

	// DefaultWorkerMaxUnavailable is the default value for Worker MaxUnavailable.
	DefaultWorkerMaxUnavailable = intstr.FromInt(0)
)

// Addons is a collection of configuration for specific addons which are managed by the Gardener.
type Addons struct {
	// KubernetesDashboard holds configuration settings for the kubernetes dashboard addon.
	// +optional
	KubernetesDashboard *KubernetesDashboard `json:"kubernetes-dashboard,omitempty"`
	// NginxIngress holds configuration settings for the nginx-ingress addon.
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	NginxIngress *NginxIngress `json:"nginx-ingress,omitempty"`

	// ClusterAutoscaler holds configuration settings for the cluster autoscaler addon.
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	ClusterAutoscaler *ClusterAutoscaler `json:"cluster-autoscaler,omitempty"`
	// Heapster holds configuration settings for the heapster addon.
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	Heapster *Heapster `json:"heapster,omitempty"`
	// Kube2IAM holds configuration settings for the kube2iam addon (only AWS).
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	Kube2IAM *Kube2IAM `json:"kube2iam,omitempty"`
	// KubeLego holds configuration settings for the kube-lego addon.
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	KubeLego *KubeLego `json:"kube-lego,omitempty"`
	// Monocular holds configuration settings for the monocular addon.
	// DEPRECATED: This field will be removed in a future version.
	// +optional
	Monocular *Monocular `json:"monocular,omitempty"`
}

// Addon also enabling or disabling a specific addon and is used to derive from.
type Addon struct {
	// Enabled indicates whether the addon is enabled or not.
	Enabled bool `json:"enabled"`
}

// HelmTiller describes configuration values for the helm-tiller addon.
type HelmTiller struct {
	Addon `json:",inline"`
}

// Heapster describes configuration values for the heapster addon.
type Heapster struct {
	Addon `json:",inline"`
}

// KubernetesDashboard describes configuration values for the kubernetes-dashboard addon.
type KubernetesDashboard struct {
	Addon `json:",inline"`
}

// ClusterAutoscaler describes configuration values for the cluster-autoscaler addon.
type ClusterAutoscaler struct {
	Addon `json:",inline"`
}

// NginxIngress describes configuration values for the nginx-ingress addon.
type NginxIngress struct {
	Addon `json:",inline"`
	// LoadBalancerSourceRanges is list of whitelist IP sources for NginxIngress
	// +optional
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`
}

// Monocular describes configuration values for the monocular addon.
type Monocular struct {
	Addon `json:",inline"`
}

// KubeLego describes configuration values for the kube-lego addon.
type KubeLego struct {
	Addon `json:",inline"`
	// Mail is the email address to register at Let's Encrypt.
	// +optional
	Mail string `json:"email,omitempty"`
}

// Kube2IAM describes configuration values for the kube2iam addon.
type Kube2IAM struct {
	Addon `json:",inline"`
	// Roles is list of AWS IAM roles which should be created by the Gardener.
	// +optional
	Roles []Kube2IAMRole `json:"roles,omitempty"`
}

// Kube2IAMRole allows passing AWS IAM policies which will result in IAM roles.
type Kube2IAMRole struct {
	// Name is the name of the IAM role. Will be extended by the Shoot name.
	Name string `json:"name"`
	// Description is a human readable message indiciating what this IAM role can be used for.
	Description string `json:"description"`
	// Policy is an AWS IAM policy document.
	Policy string `json:"policy"`
}

// Backup - DEPRECATED: This struct will be removed in a future version.
type Backup struct {
	// DEPRECATED: This field will be removed in a future version.
	Schedule string `json:"schedule"`
	// DEPRECATED: This field will be removed in a future version.
	Maximum int `json:"maximum"`
}

// DNS holds information about the provider, the hosted zone id and the domain.
type DNS struct {
	// Provider is the DNS provider type for the Shoot.
	Provider DNSProvider `json:"provider"`
	// HostedZoneID is the ID of an existing DNS Hosted Zone used to create the DNS records in.
	// +optional
	HostedZoneID *string `json:"hostedZoneID,omitempty"`
	// Domain is the external available domain of the Shoot cluster.
	// +optional
	Domain *string `json:"domain,omitempty"`
	// SecretName is a name of a secret containing credentials for the stated HostedZoneID and the
	// provider. When not specified, the Gardener will use the cloud provider credentials referenced
	// by the Shoot and try to find respective credentials there. Specifying this field may override
	// this behaviour, i.e. forcing the Gardener to only look into the given secret.
	// +optional
	SecretName *string `json:"secretName,omitempty"`
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
	// CloudProviderLocal is a constant for the development provider.
	CloudProviderLocal CloudProvider = "local"
)

// Hibernation contains information whether the Shoot is suspended or not.
type Hibernation struct {
	// Enabled is true if Shoot is hibernated, false otherwise.
	Enabled bool `json:"enabled"`
	// Schedules determine the hibernation schedules.
	// +optional
	Schedules []HibernationSchedule `json:"schedules,omitempty"`
}

// HibernationSchedule determines the hibernation schedule of a Shoot.
// A Shoot will be regularly hibernated at each start time and will be woken up at each end time.
// Start or End can be omitted, though at least one of each has to be specified.
type HibernationSchedule struct {
	// Start is a Cron spec at which time a Shoot will be hibernated.
	// +optional
	Start *string `json:"start,omitempty"`
	// End is a Cron spec at which time a Shoot will be woken up.
	// +optional
	End *string `json:"end,omitempty"`
}

// Kubernetes contains the version and configuration variables for the Shoot control plane.
type Kubernetes struct {
	// AllowPrivilegedContainers indicates whether privileged containers are allowed in the Shoot (default: true).
	// +optional
	AllowPrivilegedContainers *bool `json:"allowPrivilegedContainers,omitempty"`
	// KubeAPIServer contains configuration settings for the kube-apiserver.
	// +optional
	KubeAPIServer *KubeAPIServerConfig `json:"kubeAPIServer,omitempty"`
	// CloudControllerManager contains configuration settings for the cloud-controller-manager.
	// +optional
	CloudControllerManager *CloudControllerManagerConfig `json:"cloudControllerManager,omitempty"`
	// KubeControllerManager contains configuration settings for the kube-controller-manager.
	// +optional
	KubeControllerManager *KubeControllerManagerConfig `json:"kubeControllerManager,omitempty"`
	// KubeScheduler contains configuration settings for the kube-scheduler.
	// +optional
	KubeScheduler *KubeSchedulerConfig `json:"kubeScheduler,omitempty"`
	// KubeProxy contains configuration settings for the kube-proxy.
	// +optional
	KubeProxy *KubeProxyConfig `json:"kubeProxy,omitempty"`
	// Kubelet contains configuration settings for the kubelet.
	// +optional
	Kubelet *KubeletConfig `json:"kubelet,omitempty"`
	// Version is the semantic Kubernetes version to use for the Shoot cluster.
	Version string `json:"version"`
}

// KubernetesConfig contains common configuration fields for the control plane components.
type KubernetesConfig struct {
	// FeatureGates contains information about enabled feature gates.
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// KubeAPIServerConfig contains configuration settings for the kube-apiserver.
type KubeAPIServerConfig struct {
	KubernetesConfig `json:",inline"`
	// RuntimeConfig contains information about enabled or disabled APIs.
	// +optional
	RuntimeConfig map[string]bool `json:"runtimeConfig,omitempty"`
	// OIDCConfig contains configuration settings for the OIDC provider.
	// +optional
	OIDCConfig *OIDCConfig `json:"oidcConfig,omitempty"`
	// AdmissionPlugins contains the list of user-defined admission plugins (additional to those managed by Gardener), and, if desired, the corresponding
	// configuration.
	// +optional
	AdmissionPlugins []AdmissionPlugin `json:"admissionPlugins,omitempty"`
	// AuditConfig contains configuration settings for the audit of the kube-apiserver.
	// +optional
	AuditConfig *AuditConfig `json:"auditConfig,omitempty"`
}

// AuditConfig contains settings for audit of the api server
type AuditConfig struct {
	// AuditPolicy contains configuration settings for audit policy of the kube-apiserver.
	// +optional
	AuditPolicy *AuditPolicy `json:"auditPolicy,omitempty"`
}

// AuditPolicy contains audit policy for kube-apiserver
type AuditPolicy struct {
	// ConfigMapRef is a reference to a ConfigMap object in the same namespace,
	// which contains the audit policy for the kube-apiserver.
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`
}

// OIDCConfig contains configuration settings for the OIDC provider.
// Note: Descriptions were taken from the Kubernetes documentation.
type OIDCConfig struct {
	// If set, the OpenID server's certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host's root CA set will be used.
	// +optional
	CABundle *string `json:"caBundle,omitempty"`
	// The client ID for the OpenID Connect client, must be set if oidc-issuer-url is set.
	// +optional
	ClientID *string `json:"clientID,omitempty"`
	// If provided, the name of a custom OpenID Connect claim for specifying user groups. The claim value is expected to be a string or array of strings. This flag is experimental, please see the authentication documentation for further details.
	// +optional
	GroupsClaim *string `json:"groupsClaim,omitempty"`
	// If provided, all groups will be prefixed with this value to prevent conflicts with other authentication strategies.
	// +optional
	GroupsPrefix *string `json:"groupsPrefix,omitempty"`
	// The URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT).
	// +optional
	IssuerURL *string `json:"issuerURL,omitempty"`
	// ATTENTION: Only meaningful for Kubernetes >= 1.11
	// key=value pairs that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value.
	// +optional
	RequiredClaims map[string]string `json:"requiredClaims,omitempty"`
	// List of allowed JOSE asymmetric signing algorithms. JWTs with a 'alg' header value not in this list will be rejected. Values are defined by RFC 7518 https://tools.ietf.org/html/rfc7518#section-3.1
	// +optional
	SigningAlgs []string `json:"signingAlgs,omitempty"`
	// The OpenID claim to use as the user name. Note that claims other than the default ('sub') is not guaranteed to be unique and immutable. This flag is experimental, please see the authentication documentation for further details. (default "sub")
	// +optional
	UsernameClaim *string `json:"usernameClaim,omitempty"`
	// If provided, all usernames will be prefixed with this value. If not provided, username claims other than 'email' are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value '-'.
	// +optional
	UsernamePrefix *string `json:"usernamePrefix,omitempty"`
}

// AdmissionPlugin contains information about a specific admission plugin and its corresponding configuration.
type AdmissionPlugin struct {
	// Name is the name of the plugin.
	Name string `json:"name"`
	// Config is the configuration of the plugin.
	// NOTE: After a discussion with @mvladev we decided to not use the runtime.RawExtension type for the configuration
	// for now as there seems to be a bug with the OpenAPI generation which would make kubectl not correctly validate
	// the objects (see also https://github.com/kubernetes-sigs/cluster-api/issues/137). We keep it as string for now
	// and will later migrate the Go type to runtime.RawExtension once the issues have been resolved.
	// SEE ALSO: https://github.com/gardener/gardener/pull/322
	// +optional
	Config *string `json:"config,omitempty"`
}

// CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.
type CloudControllerManagerConfig struct {
	KubernetesConfig `json:",inline"`
}

// KubeControllerManagerConfig contains configuration settings for the kube-controller-manager.
type KubeControllerManagerConfig struct {
	KubernetesConfig `json:",inline"`
	// HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
	// +optional
	HorizontalPodAutoscalerConfig *HorizontalPodAutoscalerConfig `json:"horizontalPodAutoscaler,omitempty"`
}

// GardenerDuration is a workaround for missing OpenAPI functions on metav1.Duration struct.
type GardenerDuration struct {
	time.Duration `protobuf:"varint,1,opt,name=duration,casttype=time.Duration"`
}

// OpenAPISchemaType is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
//
// See: https://github.com/kubernetes/kube-openapi/tree/master/pkg/generators
func (GardenerDuration) OpenAPISchemaType() []string { return []string{"string"} }

// OpenAPISchemaFormat is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
func (GardenerDuration) OpenAPISchemaFormat() string { return "date-time" }

// UnmarshalJSON implements the json.Unmarshaller interface.
func (d *GardenerDuration) UnmarshalJSON(b []byte) error {
	var str string
	err := json.Unmarshal(b, &str)
	if err != nil {
		return err
	}

	pd, err := time.ParseDuration(str)
	if err != nil {
		return err
	}
	d.Duration = pd
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (d *GardenerDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

// HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
// Note: Descriptions were taken from the Kubernetes documentation.
type HorizontalPodAutoscalerConfig struct {
	// The period since last downscale, before another downscale can be performed in horizontal pod autoscaler.
	// +optional
	DownscaleDelay *GardenerDuration `json:"downscaleDelay,omitempty"`
	// The period for syncing the number of pods in horizontal pod autoscaler.
	// +optional
	SyncPeriod *GardenerDuration `json:"syncPeriod,omitempty"`
	// The minimum change (from 1.0) in the desired-to-actual metrics ratio for the horizontal pod autoscaler to consider scaling.
	// +optional
	Tolerance *float64 `json:"tolerance,omitempty"`
	// The period since last upscale, before another upscale can be performed in horizontal pod autoscaler.
	// +optional
	UpscaleDelay *GardenerDuration `json:"upscaleDelay,omitempty"`
	// The configurable window at which the controller will choose the highest recommendation for autoscaling.
	// +optional
	DownscaleStabilization *GardenerDuration `json:"downscaleStabilization,omitempty"`
	// The configurable period at which the horizontal pod autoscaler considers a Pod “not yet ready” given that it’s unready and it has  transitioned to unready during that time.
	// +optional
	InitialReadinessDelay *GardenerDuration `json:"initialReadinessDelay,omitempty"`
	// The period after which a ready pod transition is considered to be the first.
	// +optional
	CPUInitializationPeriod *GardenerDuration `json:"cpuInitializationPeriod,omitempty"`
}

const (
	// DefaultHPADownscaleDelay is a constant for the default HPA downscale delay for a Shoot cluster.
	DefaultHPADownscaleDelay = 15 * time.Minute
	// DefaultHPASyncPeriod is a constant for the default HPA sync period for a Shoot cluster.
	DefaultHPASyncPeriod = 30 * time.Second
	// DefaultHPATolerance is a constant for the default HPA tolerance for a Shoot cluster.
	DefaultHPATolerance = 0.1
	// DefaultHPAUpscaleDelay is for the default HPA upscale delay for a Shoot cluster.
	DefaultHPAUpscaleDelay = 1 * time.Minute
	// DefaultDownscaleStabilization is the default HPA downscale stabilization window for a Shoot cluster
	DefaultDownscaleStabilization = 5 * time.Minute
	// DefaultInitialReadinessDelay is for the default HPA  ReadinessDelay value in the Shoot cluster
	DefaultInitialReadinessDelay = 30 * time.Second
	// DefaultCPUInitializationPeriod is the for the default value of the CPUInitializationPeriod in the Shoot cluster
	DefaultCPUInitializationPeriod = 5 * time.Minute
)

// KubeSchedulerConfig contains configuration settings for the kube-scheduler.
type KubeSchedulerConfig struct {
	KubernetesConfig `json:",inline"`
}

// KubeProxyConfig contains configuration settings for the kube-proxy.
type KubeProxyConfig struct {
	KubernetesConfig `json:",inline"`
	// Mode specifies which proxy mode to use.
	// defaults to IPTables.
	// +optional
	Mode *ProxyMode `json:"mode,omitempty"`
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
	KubernetesConfig `json:",inline"`
	// PodPIDsLimit is the maximum number of process IDs per pod allowed by the kubelet.
	// +optional
	PodPIDsLimit *int64 `json:"podPidsLimit,omitempty"`
}

// Maintenance contains information about the time window for maintenance operations and which
// operations should be performed.
type Maintenance struct {
	// AutoUpdate contains information about which constraints should be automatically updated.
	// +optional
	AutoUpdate *MaintenanceAutoUpdate `json:"autoUpdate,omitempty"`
	// TimeWindow contains information about the time window for maintenance operations.
	// +optional
	TimeWindow *MaintenanceTimeWindow `json:"timeWindow,omitempty"`
}

// MaintenanceAutoUpdate contains information about which constraints should be automatically updated.
type MaintenanceAutoUpdate struct {
	// KubernetesVersion indicates whether the patch Kubernetes version may be automatically updated.
	KubernetesVersion bool `json:"kubernetesVersion"`
}

// MaintenanceTimeWindow contains information about the time window for maintenance operations.
type MaintenanceTimeWindow struct {
	// Begin is the beginning of the time window in the format HHMMSS+ZONE, e.g. "220000+0100".
	// If not present, a random value will be computed.
	Begin string `json:"begin"`
	// End is the end of the time window in the format HHMMSS+ZONE, e.g. "220000+0100".
	// If not present, the value will be computed based on the "Begin" value.
	End string `json:"end"`
}

////////////////////////
// Shoot Status Types //
////////////////////////

// Gardener holds the information about the Gardener
type Gardener struct {
	// ID is the Docker container id of the Gardener which last acted on a Shoot cluster.
	ID string `json:"id"`
	// Name is the hostname (pod name) of the Gardener which last acted on a Shoot cluster.
	Name string `json:"name"`
	// Version is the version of the Gardener which last acted on a Shoot cluster.
	Version string `json:"version"`
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
	SeedAvailable gardencorev1alpha1.ConditionType = "Available"

	// ShootControlPlaneHealthy is a constant for a condition type indicating the control plane health.
	ShootControlPlaneHealthy gardencorev1alpha1.ConditionType = "ControlPlaneHealthy"
	// ShootEveryNodeReady is a constant for a condition type indicating the node health.
	ShootEveryNodeReady gardencorev1alpha1.ConditionType = "EveryNodeReady"
	// ShootSystemComponentsHealthy is a constant for a condition type indicating the system components health.
	ShootSystemComponentsHealthy gardencorev1alpha1.ConditionType = "SystemComponentsHealthy"
	// ShootAlertsInactive is a constant for a condition type indicating the Shoot cluster alert states.
	ShootAlertsInactive gardencorev1alpha1.ConditionType = "AlertsInactive"
	// ShootAPIServerAvailable is a constant for a condition type indicating that the Shoot clusters API server is available.
	ShootAPIServerAvailable gardencorev1alpha1.ConditionType = "APIServerAvailable"
)

////////////////////////////////////////////////////
//              Backup Infrastructure             //
////////////////////////////////////////////////////

// BackupInfrastructure holds details about backup infrastructure
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=x-kubernetes-print-columns:custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,SEED:.spec.seed,STATUS:.status.lastOperation.state
type BackupInfrastructure struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the Backup Infrastructure.
	// +optional
	Spec BackupInfrastructureSpec `json:"spec,omitempty"`
	// Most recently observed status of the Backup Infrastructure.
	// +optional
	Status BackupInfrastructureStatus `json:"status,omitempty"`
}

// BackupInfrastructureList is a list of BackupInfrastructure objects.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type BackupInfrastructureList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of BackupInfrastructure.
	Items []BackupInfrastructure `json:"items"`
}

// BackupInfrastructureSpec is the specification of a Backup Infrastructure.
type BackupInfrastructureSpec struct {
	// Seed is the name of a Seed object.
	Seed string `json:"seed"`
	// ShootUID is a unique identifier for the Shoot cluster for which the BackupInfrastructure object is created.
	ShootUID types.UID `json:"shootUID"`
}

// BackupInfrastructureStatus holds the most recently observed status of the Backup Infrastructure.
type BackupInfrastructureStatus struct {
	// LastOperation holds information about the last operation on the BackupInfrastructure.
	// +optional
	LastOperation *gardencorev1alpha1.LastOperation `json:"lastOperation,omitempty"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *gardencorev1alpha1.LastError `json:"lastError,omitempty"`
	// ObservedGeneration is the most recent generation observed for this BackupInfrastructure. It corresponds to the
	// BackupInfrastructure's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
