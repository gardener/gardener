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
	// SecretRef is a reference to a Secret object containing the Kubeconfig and the cloud provider credentials for
	// the account the Seed cluster has been deployed to.
	SecretRef *corev1.SecretReference
	// Taints describes taints on the seed.
	Taints []SeedTaint
	// Volume contains settings for persistentvolumes created in the seed cluster.
	Volume *SeedVolume
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
}

// SeedBackup contains the object store configuration for backups for shoot (currently only etcd).
type SeedBackup struct {
	// Provider is a provider name.
	Provider string
	// ProviderConfig is the configuration passed to BackupBucket resource.
	ProviderConfig *ProviderConfig
	// Region is a region name.
	Region *string
	// SecretRef is a reference to a Secret object containing the cloud provider credentials for
	// the object store where backups should be stored. It should have enough privileges to manipulate
	// the objects as well as buckets.
	SecretRef corev1.SecretReference
}

// SeedDNS contains DNS-relevant information about this seed cluster.
type SeedDNS struct {
	// IngressDomain is the domain of the Seed cluster pointing to the ingress controller endpoint. It will be used
	// to construct ingress URLs for system applications running in Shoot clusters.
	IngressDomain string
}

// SeedNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
type SeedNetworks struct {
	// Nodes is the CIDR of the node network.
	Nodes *string
	// Pods is the CIDR of the pod network.
	Pods string
	// Services is the CIDR of the service network.
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

// SeedProvider defines the provider type and region for this Seed cluster.
type SeedProvider struct {
	// Type is the name of the provider.
	Type string
	// Region is a name of a region.
	Region string
}

// SeedTaint describes a taint on a seed.
type SeedTaint struct {
	// Key is the taint key to be applied to a seed.
	Key string
	// Value is the taint value corresponding to the taint key.
	Value *string
}

const (
	// SeedTaintDisableCapacityReservation is a constant for a taint key on a seed that marks it for disabling
	// excess capacity reservation. This can be useful for seed clusters which only host shooted seeds to reduce
	// costs.
	SeedTaintDisableCapacityReservation = "seed.gardener.cloud/disable-capacity-reservation"
	// SeedTaintDisableDNS is a constant for a taint key on a seed that marks it for disabling DNS. All shoots
	// using this seed won't get any DNS providers, DNS records, and no DNS extension controller is required to
	// be installed here. This is useful for environment where DNS is not required.
	SeedTaintDisableDNS = "seed.gardener.cloud/disable-dns"
	// SeedTaintProtected is a constant for a taint key on a seed that marks it as protected. Protected seeds
	// may only be used by shoots in the `garden` namespace.
	SeedTaintProtected = "seed.gardener.cloud/protected"
	// SeedTaintInvisible is a constant for a taint key on a seed that marks it as invisible. Invisible seeds
	// are not considered by the gardener-scheduler.
	SeedTaintInvisible = "seed.gardener.cloud/invisible"
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
	// SeedBootstrapped is a constant for a condition type indicating that the seed cluster has been
	// bootstrapped.
	SeedBootstrapped ConditionType = "Bootstrapped"
	// SeedExtensionsReady is a constant for a condition type indicating that the extensions are ready.
	SeedExtensionsReady ConditionType = "ExtensionsReady"
	// SeedGardenletReady is a constant for a condition type indicating that the Gardenlet is ready.
	SeedGardenletReady ConditionType = "GardenletReady"
)
