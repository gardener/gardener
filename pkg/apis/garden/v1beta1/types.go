// Copyright 2018 The Gardener Authors.
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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
}

// AWSProfile defines certain constraints and definitions for the AWS cloud.
type AWSProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints AWSConstraints `json:"constraints"`
	// MachineImages is a list of AWS machine images for each region.
	MachineImages []AWSMachineImage `json:"machineImages"`
}

// AWSConstraints is an object containing constraints for certain values in the Shoot specification.
type AWSConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint `json:"dnsProviders"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints `json:"kubernetes"`
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType `json:"machineTypes"`
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType `json:"volumeTypes"`
	// Zones contains constraints regarding allowed values for 'zones' block in the Shoot specification.
	Zones []Zone `json:"zones"`
}

// AWSMachineImage defines the region and the AMI for a machine image.
type AWSMachineImage struct {
	// Region is a region in AWS.
	Region string `json:"region"`
	// AMI is the technical id of the image.
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
	// MachineImage defines the channel and the version of the machine image in the Azure environment.
	MachineImage AzureMachineImage `json:"machineImage"`
}

// AzureConstraints is an object containing constraints for certain values in the Shoot specification.
type AzureConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint `json:"dnsProviders"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints `json:"kubernetes"`
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
	// Channel is the channel to pull images from (one of Alpha, Beta, Stable).
	Channel string `json:"channel"`
	// Version is the version of the image.
	Version string `json:"version"`
}

// GCPProfile defines certain constraints and definitions for the GCP cloud.
type GCPProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints GCPConstraints `json:"constraints"`
	// MachineImage defines the name of the machine image in the GCP environment.
	MachineImage GCPMachineImage `json:"machineImage"`
}

// GCPConstraints is an object containing constraints for certain values in the Shoot specification.
type GCPConstraints struct {
	// DNSProviders contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
	DNSProviders []DNSProviderConstraint `json:"dnsProviders"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesConstraints `json:"kubernetes"`
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
	Name string `json:"name"`
}

// OpenStackProfile defines certain constraints and definitions for the OpenStack cloud.
type OpenStackProfile struct {
	// Constraints is an object containing constraints for certain values in the Shoot specification.
	Constraints OpenStackConstraints `json:"constraints"`
	// KeyStoneURL is the URL for auth{n,z} in OpenStack (pointing to KeyStone).
	KeyStoneURL string `json:"keystoneURL"`
	// MachineImage defines the name of the machine image in the OpenStack environment.
	MachineImage OpenStackMachineImage `json:"machineImage"`
	// CABundle is a certificate bundle which will be installed onto every host machine of the Shoot cluster.
	CABundle string `json:"caBundle"`
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
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType `json:"machineTypes"`
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
	Name string `json:"name"`
}

// DNSProviderConstraint contains constraints regarding allowed values of the 'dns.provider' block in the Shoot specification.
type DNSProviderConstraint struct {
	// Name is the name of the DNS provider.
	Name DNSProvider `json:"name"`
}

// KubernetesConstraints contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
type KubernetesConstraints struct {
	// Versions is the list of allowed Kubernetes versions for Shoot clusters (e.g., 1.9.1).
	Versions []string `json:"versions"`
}

// MachineType contains certain properties of a machine type.
type MachineType struct {
	// Name is the name of the machine type.
	Name string `json:"name"`
	// CPUs is the number of CPUs for this machine type.
	CPUs int `json:"cpus"`
	// GPUs is the number of GPUs for this machine type.
	GPUs int `json:"gpus"`
	// Memory is the amount of memory for this machine type.
	Memory resource.Quantity `json:"memory"`
}

// VolumeType contains certain properties of a volume type.
type VolumeType struct {
	// Name is the name of the volume type.
	Name string `json:"name"`
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

////////////////////////////////////////////////////
//                      SEEDS                     //
////////////////////////////////////////////////////

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Seed holds certain properties about a Seed cluster.
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
	// Domain is the domain of the Seed cluster. It will be used to construct ingress URLs for system applications
	// running in Shoot clusters.
	Domain string `json:"domain"`
	// SecretRef is a reference to a Secret object containing the Kubeconfig and the cloud provider credentials for
	// the account the Seed cluster has been deployed to.
	SecretRef CrossReference `json:"secretRef"`
	// Networks defines the pod, service and worker network of the Seed cluster.
	Networks K8SNetworks `json:"networks"`
}

// SeedStatus holds the most recently observed status of the Seed cluster.
type SeedStatus struct {
	// Conditions represents the latest available observations of a Seed's current state.
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

// SeedCloud defines the cloud profile and the region this Seed cluster belongs to.
type SeedCloud struct {
	// Profile is the name of a cloud profile.
	Profile string `json:"profile"`
	// Region is a name of a region.
	Region string `json:"region"`
}

// LocalReference is a reference to an object in the same Kubernetes namespace.
type LocalReference struct {
	// Name is the name of the object.
	Name string `json:"name"`
}

// CrossReference is a reference to an object in a different Kubernetes namespace.
type CrossReference struct {
	// Name is the name of the object.
	Name string `json:"name"`
	// Namespace is the namespace of the object.
	Namespace string `json:"namespace"`
}

// K8SNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
type K8SNetworks struct {
	// Nodes is the CIDR of the node network.
	Nodes CIDR `json:"nodes"`
	// Pods is the CIDR of the pod network.
	Pods CIDR `json:"pods"`
	// Services is the CIDR of the service network.
	Services CIDR `json:"services"`
}

////////////////////////////////////////////////////
//                     QUOTAS                    //
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
	// Most recently observed status of the Quota constraints.
	// +optional
	Status QuotaStatus `json:"status,omitempty"`
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

// QuotaStatus holds the most recently observed status of the Quota constraints.
type QuotaStatus struct {
	// Metrics holds the current status of the constraints defined in the spec. Only used for Quotas whose scope
	// is 'secret'.
	Metrics corev1.ResourceList `json:"metrics"`
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

type PrivateSecretBinding struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// SecretRef is a reference to a secret object in the same namespace.
	// +optional
	SecretRef LocalReference `json:"secretRef,omitempty"`
	// Quotas is a list of references to Quota objects in other namespaces.
	// +optional
	Quotas []CrossReference `json:"quotas,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PrivateSecretBindingList is a collection of PrivateSecretBindings.
type PrivateSecretBindingList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of PrivateSecretBindings.
	Items []PrivateSecretBinding `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CrossSecretBinding struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// SecretRef is a reference to a secret object in another namespace.
	// +optional
	SecretRef CrossReference `json:"secretRef,omitempty"`
	// Quotas is a list of references to Quota objects in other namespaces.
	// +optional
	Quotas []CrossReference `json:"quotas,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CrossSecretBindingList is a collection of CrossSecretBindings.
type CrossSecretBindingList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of CrossSecretBindings.
	Items []CrossSecretBinding `json:"items"`
}

////////////////////////////////////////////////////
//                      SHOOTS                    //
////////////////////////////////////////////////////

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
	Addons Addons `json:"addons"`
	// Backup contains configuration settings for the etcd backups.
	// +optional
	Backup *Backup `json:"backup,omitempty"`
	// Cloud contains information about the cloud environment and their specific settings.
	Cloud Cloud `json:"cloud"`
	// DNS contains information about the DNS settings of the Shoot.
	DNS DNS `json:"dns"`
	// Kubernetes contains the version and configuration settings of the control plane components.
	Kubernetes Kubernetes `json:"kubernetes"`
}

// ShootStatus holds the most recently observed status of the Shoot cluster.
type ShootStatus struct {
	// Conditions represents the latest available observations of a Shoots's current state.
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
	// Gardener holds information about the Gardener which last acted on the Shoot.
	Gardener Gardener `json:"gardener"`
	// LastOperation holds information about the last operation on the Shoot.
	// +optional
	LastOperation *LastOperation `json:"lastOperation,omitempty"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *LastError `json:"lastError,omitempty"`
	// OperationStartTime is the start time of the last operation (used to determine how often it should
	// be retried)
	// +optional
	OperationStartTime *metav1.Time `json:"operationStartTime,omitempty"`
	// ObservedGeneration is the most recent generation observed for this Shoot. It corresponds to the
	// Shoot's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
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
	// SecretBindingRef is a reference to a PrivateSecretBinding or a CrossSecretBinding object.
	SecretBindingRef corev1.ObjectReference `json:"secretBindingRef"`
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
}

// AWSCloud contains the Shoot specification for AWS.

type AWSCloud struct {
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AWSNetworks `json:"networks"`
	// Workers is a list of worker groups.
	Workers []AWSWorker `json:"workers"`
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string `json:"zones"`
}

// AWSNetworks holds information about the Kubernetes and infrastructure networks.
type AWSNetworks struct {
	K8SNetworks
	// VPC indicates whether to use an existing VPC or create a new one.
	VPC AWSVPC `json:"vpc"`
	// Internal is a list of private subnets to create (used for internal load balancers).
	Internal []CIDR `json:"internal"`
	// Public is a list of public subnets to create (used for bastion and load balancers).
	Public []CIDR `json:"public"`
	// Workers is a list of worker subnets (private) to create (used for the VMs).
	Workers []CIDR `json:"workers"`
}

// AWSVPC contains either an id (of an existing VPC) or the CIDR (for a VPC to be created).
type AWSVPC struct {
	// ID is the AWS VPC id of an existing VPC.
	// +optional
	ID string `json:"id,omitempty"`
	// CIDR is a CIDR range for a new VPC.
	// +optional
	CIDR CIDR `json:"cidr,omitempty"`
}

// AWSWorker is the definition of a worker group.
type AWSWorker struct {
	Worker
	// VolumeType is the type of the root volumes.
	VolumeType string `json:"volumeType"`
	// VolumeSize is the size of the root volume.
	VolumeSize string `json:"volumeSize"`
}

// AzureCloud contains the Shoot specification for Azure.
type AzureCloud struct {
	// ResourceGroup indicates whether to use an existing resource group or create a new one.
	// +optional
	ResourceGroup *AzureResourceGroup `json:"resourceGroup,omitempty"`
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks AzureNetworks `json:"networks"`
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
	K8SNetworks
	// VNet indicates whether to use an existing VNet or create a new one.
	VNet AzureVNet `json:"vnet"`
	// Public is a CIDR of a public subnet to create (used for bastion).
	// +optional
	Public *CIDR `json:"public,omitempty"`
	// Workers is a CIDR of a worker subnet (private) to create (used for the VMs).
	Workers CIDR `json:"workers"`
}

// AzureVNet indicates whether to use an existing VNet or create a new one.
type AzureVNet struct {
	// Name is the AWS VNet name of an existing VNet.
	// +optional
	Name string `json:"name,omitempty"`
	// CIDR is a CIDR range for a new VNet.
	// +optional
	CIDR CIDR `json:"cidr,omitempty"`
}

// AzureWorker is the definition of a worker group.
type AzureWorker struct {
	Worker
	// VolumeType is the type of the root volumes.
	VolumeType string `json:"volumeType"`
	// VolumeSize is the size of the root volume.
	VolumeSize string `json:"volumeSize"`
}

// GCPCloud contains the Shoot specification for GCP.
type GCPCloud struct {
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks GCPNetworks `json:"networks"`
	// Workers is a list of worker groups.
	Workers []GCPWorker `json:"workers"`
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string `json:"zones"`
}

// GCPNetworks holds information about the Kubernetes and infrastructure networks.
type GCPNetworks struct {
	K8SNetworks
	// VPC indicates whether to use an existing VPC or create a new one.
	// +optional
	VPC *GCPVPC `json:"vpc,omitempty"`
	// Workers is a list of CIDRs of worker subnets (private) to create (used for the VMs).
	Workers []CIDR `json:"workers"`
}

// GCPVPC indicates whether to use an existing VPC or create a new one.
type GCPVPC struct {
	// Name is the name of an existing GCP VPC.
	Name string `json:"name"`
}

// GCPWorker is the definition of a worker group.
type GCPWorker struct {
	Worker
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
	// Networks holds information about the Kubernetes and infrastructure networks.
	Networks OpenStackNetworks `json:"networks"`
	// Workers is a list of worker groups.
	Workers []OpenStackWorker `json:"workers"`
	// Zones is a list of availability zones to deploy the Shoot cluster to.
	Zones []string `json:"zones"`
}

// OpenStackNetworks holds information about the Kubernetes and infrastructure networks.
type OpenStackNetworks struct {
	K8SNetworks
	// Router indicates whether to use an existing router or create a new one.
	// +optional
	Router *OpenStackRouter `json:"router,omitempty"`
	// Workers is a list of CIDRs of worker subnets (private) to create (used for the VMs).
	Workers []CIDR `json:"workers"`
}

// OpenStackRouter indicates whether to use an existing router or create a new one.
type OpenStackRouter struct {
	// ID is the router id of an existing OpenStack router.
	ID string `json:"id"`
}

// OpenStackWorker is the definition of a worker group.
type OpenStackWorker struct {
	Worker
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
}

// Addons is a collection of configuration for specific addons which are managed by the Gardener.
type Addons struct {
	// Kube2IAM holds configuration settings for the kube2iam addon (only AWS).
	// +optional
	Kube2IAM Kube2IAM `json:"kube2iam,omitempty"`
	// Heapster holds configuration settings for the heapster addon.
	// +optional
	Heapster Heapster `json:"heapster,omitempty"`
	// KubernetesDashboard holds configuration settings for the kubernetes dashboard addon.
	// +optional
	KubernetesDashboard KubernetesDashboard `json:"kubernetes-dashboard,omitempty"`
	// ClusterAutoscaler holds configuration settings for the cluster autoscaler addon.
	// +optional
	ClusterAutoscaler ClusterAutoscaler `json:"cluster-autoscaler,omitempty"`
	// NginxIngress holds configuration settings for the nginx-ingress addon.
	// +optional
	NginxIngress NginxIngress `json:"nginx-ingress,omitempty"`
	// Monocular holds configuration settings for the monocular addon.
	// +optional
	Monocular Monocular `json:"monocular,omitempty"`
	// KubeLego holds configuration settings for the kube-lego addon.
	// +optional
	KubeLego KubeLego `json:"kube-lego,omitempty"`
}

// Addon also enabling or disabling a specific addon and is used to derive from.
type Addon struct {
	// Enabled indicates whether the addon is enabled or not.
	// +optional
	Enabled bool `json:"enabled"`
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
	Mail string `json:"email,omitempty"`
}

// Kube2IAM describes configuration values for the kube2iam addon.
type Kube2IAM struct {
	Addon
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

// Backup holds information about the backup interval and maximum.
type Backup struct {
	// IntervalInSecond defines the interval in seconds how often a backup is taken from etcd.
	IntervalInSecond int `json:"intervalInSecond"`
	// Maximum indicates how many backups should be kept at maximum.
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
}

// DNSProvider is a string alias.
type DNSProvider string

const (
	// DNSUnmanaged is a constant for the 'unmanaged' DNS provider.
	DNSUnmanaged DNSProvider = "unmanaged"
	// DNSAWSRoute53 is a constant for the 'aws-route53' DNS provider.
	DNSAWSRoute53 DNSProvider = "aws-route53"
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
)

// CIDR is a string alias.
type CIDR string

// Kubernetes contains the version and configuration variables for the Shoot control plane.
type Kubernetes struct {
	// AllowPrivilegedContainers indicates whether privileged containers are allowed in the Shoot (default: true).
	// +optional
	AllowPrivilegedContainers *bool `json:"allowPrivilegedContainers,omitempty"`
	// KubeAPIServer contains configuration settings for the kube-apiserver.
	// +optional
	KubeAPIServer KubeAPIServerConfig `json:"kubeAPIServer,omitempty"`
	// KubeControllerManager contains configuration settings for the kube-controller-manager.
	// +optional
	KubeControllerManager KubeControllerManagerConfig `json:"kubeControllerManager,omitempty"`
	// KubeScheduler contains configuration settings for the kube-scheduler.
	// +optional
	KubeScheduler KubeSchedulerConfig `json:"kubeScheduler,omitempty"`
	// KubeProxy contains configuration settings for the kube-proxy.
	// +optional
	KubeProxy KubeProxyConfig `json:"kubeProxy,omitempty"`
	// Kubelet contains configuration settings for the kubelet.
	// +optional
	Kubelet KubeletConfig `json:"kubelet,omitempty"`
	// Version is the semantic Kubernetes version to use for the Shoot cluster.
	Version string `json:"version"`
}

// KubernetesConfig contains common configuration fields for the control plane components.
type KubernetesConfig struct {
	// FeatureGates contains information about enabled feature gates.
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// KubeAPIServerConfig contains configuration settings for the kube-apiserver.
type KubeAPIServerConfig struct {
	KubernetesConfig
	// RuntimeConfig contains information about enabled or disabled APIs.
	// +optional
	RuntimeConfig map[string]bool `json:"runtimeConfig,omitempty"`
	// OIDCConfig contains configuration settings for the OIDC provider.
	// +optional
	OIDCConfig *OIDCConfig `json:"oidcConfig,omitempty"`
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
	// The OpenID claim to use as the user name. Note that claims other than the default ('sub') is not guaranteed to be unique and immutable. This flag is experimental, please see the authentication documentation for further details. (default "sub")
	// +optional
	UsernameClaim *string `json:"usernameClaim,omitempty"`
	// If provided, all usernames will be prefixed with this value. If not provided, username claims other than 'email' are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value '-'.
	// +optional
	UsernamePrefix *string `json:"usernamePrefix,omitempty"`
}

// KubeControllerManagerConfig contains configuration settings for the kube-controller-manager.
type KubeControllerManagerConfig struct {
	KubernetesConfig
}

// KubeSchedulerConfig contains configuration settings for the kube-scheduler.
type KubeSchedulerConfig struct {
	KubernetesConfig
}

// KubeProxyConfig contains configuration settings for the kube-proxy.
type KubeProxyConfig struct {
	KubernetesConfig
}

// KubeletConfig contains configuration settings for the kubelet.
type KubeletConfig struct {
	KubernetesConfig
}

const (
	// DefaultPodNetworkCIDR is a constant for the default pod network CIDR of a Shoot cluster.
	DefaultPodNetworkCIDR = CIDR("100.96.0.0/11")
	// DefaultServiceNetworkCIDR is a constant for the default service network CIDR of a Shoot cluster.
	DefaultServiceNetworkCIDR = CIDR("100.64.0.0/13")
	// DefaultETCDBackupIntervalSeconds is a constant for the default interval to take backups of a Shoot cluster (24 hours).
	DefaultETCDBackupIntervalSeconds = 60 * 60 * 24
	// DefaultETCDBackupMaximum is a constant for the default number of etcd backups to keep for a Shoot cluster.
	DefaultETCDBackupMaximum = 7
)

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

// LastOperation indicates the type and the state of the last operation, along with a description
// message and a progress indicator.
type LastOperation struct {
	// A human readable message indicating details about the last operation.
	Description string `json:"description"`
	// Last time the operation state transitioned from one to another.
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	// The progress in percentage (0-100) of the last operation.
	Progress int `json:"progress"`
	// Status of the last operation, one of Processing, Succeeded, Error, Failed.
	State ShootLastOperationState `json:"state"`
	// Type of the last operation, one of Create, Reconcile, Update, Delete.
	Type ShootLastOperationType `json:"type"`
}

// ShootLastOperationType is a string alias.
type ShootLastOperationType string

const (
	// ShootLastOperationTypeCreate indicates a 'create' operation.
	ShootLastOperationTypeCreate ShootLastOperationType = "Create"
	// ShootLastOperationTypeReconcile indicates a 'reconcile' operation.
	ShootLastOperationTypeReconcile ShootLastOperationType = "Reconcile"
	// ShootLastOperationTypeUpdate indicates an 'update' operation.
	ShootLastOperationTypeUpdate ShootLastOperationType = "Update"
	// ShootLastOperationTypeDelete indicates a 'delete' operation.
	ShootLastOperationTypeDelete ShootLastOperationType = "Delete"
)

// ShootLastOperationState is a string alias.
type ShootLastOperationState string

const (
	// ShootLastOperationStateProcessing indicates that an operation is ongoing.
	ShootLastOperationStateProcessing ShootLastOperationState = "Processing"
	// ShootLastOperationStateSucceeded indicates that an operation has completed successfully.
	ShootLastOperationStateSucceeded ShootLastOperationState = "Succeeded"
	// ShootLastOperationStateError indicates that an operation is completed with errors and will be retried.
	ShootLastOperationStateError ShootLastOperationState = "Error"
	// ShootLastOperationStateFailed indicates that an operation is completed with errors and won't be retried.
	ShootLastOperationStateFailed ShootLastOperationState = "Failed"
)

// LastError indicates the last occurred error for an operation on a Shoot cluster.
type LastError struct {
	// A human readable message indicating details about the last error.
	Description string `json:"description"`
	// Well-defined error codes of the last error(s).
	// +optional
	Codes []ErrorCode `json:"codes,omitempty"`
}

// ErrorCode is a string alias.
type ErrorCode string

const (
	// ErrorInfraUnauthorized indicates that the last error occurred due to invalid cloud provider credentials.
	ErrorInfraUnauthorized ErrorCode = "ERR_INFRA_UNAUTHORIZED"
	// ErrorInfraInsufficientPrivileges indicates that the last error occurred due to insufficient cloud provider privileges.
	ErrorInfraInsufficientPrivileges ErrorCode = "ERR_INFRA_INSUFFICIENT_PRIVILEGES"
	// ErrorInfraQuotaExceeded indicates that the last error occurred due to cloud provider quota limits.
	ErrorInfraQuotaExceeded ErrorCode = "ERR_INFRA_QUOTA_EXCEEDED"
	// ErrorInfraDependencies indicates that the last error occurred due to dependent objects on the cloud provider level.
	ErrorInfraDependencies ErrorCode = "ERR_INFRA_DEPENDENCIES"
)

const (
	// ShootEventReconciling indicates that the a Reconcile operation started.
	ShootEventReconciling = "ReconcilingShoot"
	// ShootEventReconciled indicates that the a Reconcile operation was successful.
	ShootEventReconciled = "ReconciledShoot"
	// ShootEventReconcileError indicates that the a Reconcile operation failed.
	ShootEventReconcileError = "ReconcileError"
	// ShootEventDeleting indicates that the a Delete operation started.
	ShootEventDeleting = "DeletingShoot"
	// ShootEventDeleted indicates that the a Delete operation was successful.
	ShootEventDeleted = "DeletedShoot"
	// ShootEventDeleteError indicates that the a Delete operation failed.
	ShootEventDeleteError = "DeleteError"
)

const (
	// GardenerName is the value in a Shoot's `.metadata.finalizers[]` array on which the GardenerName will react
	// when performing a delete request on a Shoot resource.
	GardenerName = "gardener"
)

// Condition holds the information about the state of a resource.
type Condition struct {
	// Type of the Shoot condition.
	Type ConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// The reason for the condition's last transition.
	Reason string `json:"reason"`
	// A human readable message indicating details about the transition.
	Message string `json:"message"`
}

// ConditionType is a string alias.
type ConditionType string

const (
	// SeedAvailable is a constant for a condition type indicating the Seed cluster availability.
	SeedAvailable ConditionType = "Available"
	// ShootControlPlaneHealthy is a constant for a condition type indicating the control plane health.
	ShootControlPlaneHealthy ConditionType = "ControlPlaneHealthy"
	// ShootEveryNodeReady is a constant for a condition type indicating the node health.
	ShootEveryNodeReady ConditionType = "EveryNodeReady"
	// ShootSystemComponentsHealthy is a constant for a condition type indicating the system components health.
	ShootSystemComponentsHealthy ConditionType = "SystemComponentsHealthy"
	// ConditionCheckError is a constant for indicating that a condition could not be checked.
	ConditionCheckError = "ConditionCheckError"
)
