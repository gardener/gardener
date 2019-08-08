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

package v1alpha1

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
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec contains the specification of this installation.
	Spec SeedSpec `json:"spec,omitempty"`
	// Status contains the status of this installation.
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
	// Backup holds the object store configuration for the backups of shoot (currently only etcd).
	// If it is not specified, then there won't be any backups taken for shoots associated with this seed.
	// If backup field is present in seed, then backups of the etcd from shoot control plane will be stored
	// under the configured object store.
	// +optional
	Backup *SeedBackup `json:"backup,omitempty"`
	// BlockCIDRs is a list of network addresses tha should be blocked for shoot control plane components running
	// in the seed cluster.
	// +optional
	BlockCIDRs []CIDR `json:"blockCIDRs,omitempty"`
	// DNS contains DNS-relevant information about this seed cluster.
	DNS SeedDNS `json:"dns"`
	// Networks defines the pod, service and worker network of the Seed cluster.
	Networks SeedNetworks `json:"networks"`
	// Provider defines the provider type and region for this Seed cluster.
	Provider SeedProvider `json:"provider"`
	// SecretRef is a reference to a Secret object containing the Kubeconfig and the cloud provider credentials for
	// the account the Seed cluster has been deployed to.
	SecretRef corev1.SecretReference `json:"secretRef"`
	// Taints describes taints on the seed.
	// +optional
	Taints []SeedTaint `json:"taints,omitempty"`
	// Volume contains settings for persistentvolumes created in the seed cluster.
	// +optional
	Volume *SeedVolume `json:"volume,omitempty"`
}

// SeedStatus is the status of a Seed.
type SeedStatus struct {
	// Gardener holds information about the Gardener which last acted on the Shoot.
	// +optional
	Gardener Gardener `json:"gardener,omitempty"`
	// Conditions represents the latest available observations of a Seed's current state.
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
	// ObservedGeneration is the most recent generation observed for this Seed. It corresponds to the
	// Seed's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// SeedBackup contains the object store configuration for backups for shoot (currently only etcd).
type SeedBackup struct {
	// Provider is a provider name.
	Provider string `json:"provider"`
	// Region is a region name.
	Region *string `json:"region,omitempty"`
	// SecretRef is a reference to a Secret object containing the cloud provider credentials for
	// the object store where backups should be stored. It should have enough privileges to manipulate
	// the objects as well as buckets.
	SecretRef corev1.SecretReference `json:"secretRef"`
}

// SeedDNS contains DNS-relevant information about this seed cluster.
type SeedDNS struct {
	// IngressDomain is the domain of the Seed cluster pointing to the ingress controller endpoint. It will be used
	// to construct ingress URLs for system applications running in Shoot clusters.
	IngressDomain string `json:"ingressDomain"`
}

// SeedNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
type SeedNetworks struct {
	// Nodes is the CIDR of the node network.
	Nodes CIDR `json:"nodes"`
	// Pods is the CIDR of the pod network.
	Pods CIDR `json:"pods"`
	// Services is the CIDR of the service network.
	Services CIDR `json:"services"`
}

// SeedProvider defines the provider type and region for this Seed cluster.
type SeedProvider struct {
	// Type is the name of the provider.
	Type string `json:"type"`
	// Region is a name of a region.
	Region string `json:"region"`
}

// SeedTaint describes a taint on a seed.
type SeedTaint struct {
	// Key is the taint key to be applied to a seed.
	Key string `json:"key"`
	// Value is the taint value corresponding to the taint key.
	// +optional
	Value *string `json:"value,omitempty"`
}

const (
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
	// +optional
	MinimumSize *resource.Quantity `json:"minimumSize,omitempty"`
	// Providers is a list of storage class provisioner types for the seed.
	// +optional
	Providers []SeedVolumeProvider `json:"providers,omitempty"`
}

// SeedVolumeProvider is a storage class provisioner type.
type SeedVolumeProvider struct {
	// Purpose is the purpose of this provider.
	Purpose string `json:"purpose"`
	// Name is the name of the storage class provisioner type.
	Name string `json:"name"`
}

const (
	// SeedAvailable is a constant for a condition type indicating the Seed cluster availability.
	SeedAvailable ConditionType = "Available"
)
