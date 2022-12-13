// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,shortName="grdn"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Reconciled",type=string,JSONPath=`.status.conditions[?(@.type=="Reconciled")].status`,description="Indicates whether the garden has been reconciled."
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="creation timestamp"

// Garden describes a list of gardens.
type Garden struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec contains the specification of this garden.
	Spec GardenSpec `json:"spec,omitempty"`
	// Status contains the status of this garden.
	Status GardenStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GardenList is a list of Garden resources.
type GardenList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of Garden.
	Items []Garden `json:"items"`
}

// GardenSpec contains the specification of a garden environment.
type GardenSpec struct {
	// RuntimeCluster contains configuration for the runtime cluster.
	RuntimeCluster RuntimeCluster `json:"runtimeCluster"`
	// VirtualCluster contains configuration for the virtual cluster.
	VirtualCluster VirtualCluster `json:"virtualCluster"`
}

// RuntimeCluster contains configuration for the runtime cluster.
type RuntimeCluster struct {
	// Provider defines the provider-specific information for this cluster.
	Provider Provider `json:"provider"`
	// Settings contains certain settings for this cluster.
	// +optional
	Settings *Settings `json:"settings,omitempty"`
}

// Provider defines the provider-specific information for this cluster.
type Provider struct {
	// Zones is the list of availability zones the cluster is deployed to.
	// +optional
	Zones []string `json:"zones,omitempty"`
}

// Settings contains certain settings for this cluster.
type Settings struct {
	// VerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
	// cluster.
	// +optional
	VerticalPodAutoscaler *SettingVerticalPodAutoscaler `json:"verticalPodAutoscaler,omitempty"`
}

// SettingVerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
// seed.
type SettingVerticalPodAutoscaler struct {
	// Enabled controls whether the VPA components shall be deployed into this cluster. It is true by default because
	// the operator (and Gardener) heavily rely on a VPA being deployed. You should only disable this if your runtime
	// cluster already has another, manually/custom managed VPA deployment. If this is not the case, but you still
	// disable it, then reconciliation will fail.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// VirtualCluster contains configuration for the virtual cluster.
type VirtualCluster struct {
	// ETCD contains configuration for the etcds of the virtual garden cluster.
	// +optional
	ETCD *ETCD `json:"etcd,omitempty"`
	// Maintenance contains information about the time window for maintenance operations.
	Maintenance Maintenance `json:"maintenance"`
}

// ETCD contains configuration for the etcds of the virtual garden cluster.
type ETCD struct {
	// Main contains configuration for the main etcd.
	// +optional
	Main *ETCDMain `json:"main,omitempty"`
	// Events contains configuration for the events etcd.
	// +optional
	Events *ETCDEvents `json:"events,omitempty"`
}

// ETCDMain contains configuration for the main etcd.
type ETCDMain struct {
	// Backup contains the object store configuration for backups for the virtual garden etcd.
	// +optional
	Backup *Backup `json:"backup,omitempty"`
	// Storage contains storage configuration.
	// +optional
	Storage *Storage `json:"storage,omitempty"`
}

// ETCDEvents contains configuration for the events etcd.
type ETCDEvents struct {
	// Storage contains storage configuration.
	// +optional
	Storage *Storage `json:"storage,omitempty"`
}

// Storage contains storage configuration.
type Storage struct {
	// Capacity is the storage capacity for the volumes.
	// +kubebuilder:default=`10Gi`
	// +optional
	Capacity *resource.Quantity `json:"capacity,omitempty"`
	// ClassName is the name of a storage class.
	// +optional
	ClassName *string `json:"className,omitempty"`
}

// Backup contains the object store configuration for backups for the virtual garden etcd.
type Backup struct {
	// Provider is a provider name. This field is immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Provider string `json:"provider"`
	// BucketName is the name of the backup bucket.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	BucketName string `json:"bucketName"`
	// SecretRef is a reference to a Secret object containing the cloud provider credentials for the object store where
	// backups should be stored. It should have enough privileges to manipulate the objects as well as buckets.
	SecretRef corev1.SecretReference `json:"secretRef"`
}

// Maintenance contains information about the time window for maintenance operations.
type Maintenance struct {
	// TimeWindow contains information about the time window for maintenance operations.
	TimeWindow gardencorev1beta1.MaintenanceTimeWindow `json:"timeWindow"`
}

// GardenStatus is the status of a garden environment.
type GardenStatus struct {
	// Gardener holds information about the Gardener which last acted on the Garden.
	// +optional
	Gardener *gardencorev1beta1.Gardener `json:"gardener,omitempty"`
	// Conditions is a list of conditions.
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Credentials contains information about the virtual garden cluster credentials.
	// +optional
	Credentials *Credentials `json:"credentials,omitempty"`
}

// Credentials contains information about the virtual garden cluster credentials.
type Credentials struct {
	// Rotation contains information about the credential rotations.
	// +optional
	Rotation *CredentialsRotation `json:"rotation,omitempty"`
}

// CredentialsRotation contains information about the rotation of credentials.
type CredentialsRotation struct {
	// CertificateAuthorities contains information about the certificate authority credential rotation.
	// +optional
	CertificateAuthorities *gardencorev1beta1.CARotation `json:"certificateAuthorities,omitempty"`
}

const (
	// GardenReconciled is a constant for a condition type indicating that the garden has been reconciled.
	GardenReconciled gardencorev1beta1.ConditionType = "Reconciled"
)

// AvailableOperationAnnotations is the set of available operation annotations for Garden resources.
var AvailableOperationAnnotations = sets.NewString(
	v1beta1constants.GardenerOperationReconcile,
	v1beta1constants.OperationRotateCAStart,
	v1beta1constants.OperationRotateCAComplete,
	v1beta1constants.OperationRotateCredentialsStart,
	v1beta1constants.OperationRotateCredentialsComplete,
)
