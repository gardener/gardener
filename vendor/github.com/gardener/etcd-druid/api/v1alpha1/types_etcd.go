// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// TODO Remove unused constants
const (
	// GarbageCollectionPolicyExponential defines the exponential policy for garbage collecting old backups
	GarbageCollectionPolicyExponential = "Exponential"
	// GarbageCollectionPolicyLimitBased defines the limit based policy for garbage collecting old backups
	GarbageCollectionPolicyLimitBased = "LimitBased"

	// Basic is a constant for metrics level basic.
	Basic MetricsLevel = "basic"
	// Extensive is a constant for metrics level extensive.
	Extensive MetricsLevel = "extensive"

	// GzipCompression is constant for gzip compression policy.
	GzipCompression CompressionPolicy = "gzip"
	// LzwCompression is constant for lzw compression policy.
	LzwCompression CompressionPolicy = "lzw"
	// ZlibCompression is constant for zlib compression policy.
	ZlibCompression CompressionPolicy = "zlib"

	// DefaultCompression is constant for default compression policy(only if compression is enabled).
	DefaultCompression CompressionPolicy = GzipCompression
	// DefaultCompressionEnabled is constant to define whether to compress the snapshots or not.
	DefaultCompressionEnabled = false

	// Periodic is a constant to set auto-compaction-mode 'periodic' for duration based retention.
	Periodic CompactionMode = "periodic"
	// Revision is a constant to set auto-compaction-mode 'revision' for revision number based retention.
	Revision CompactionMode = "revision"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.labelSelector
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Quorate",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="All Members Ready",type=string,JSONPath=`.status.conditions[?(@.type=="AllMembersReady")].status`
// +kubebuilder:printcolumn:name="Backup Ready",type=string,JSONPath=`.status.conditions[?(@.type=="BackupReady")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Cluster Size",type=integer,JSONPath=`.spec.replicas`,priority=1
// +kubebuilder:printcolumn:name="Current Replicas",type=integer,JSONPath=`.status.currentReplicas`,priority=1
// +kubebuilder:printcolumn:name="Ready Replicas",type=integer,JSONPath=`.status.readyReplicas`,priority=1

// Etcd is the Schema for the etcds API
type Etcd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdSpec   `json:"spec,omitempty"`
	Status EtcdStatus `json:"status,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true

// EtcdList contains a list of Etcd
type EtcdList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Etcd `json:"items"`
}

// MetricsLevel defines the level 'basic' or 'extensive'.
// +kubebuilder:validation:Enum=basic;extensive
type MetricsLevel string

// GarbageCollectionPolicy defines the type of policy for snapshot garbage collection.
// +kubebuilder:validation:Enum=Exponential;LimitBased
type GarbageCollectionPolicy string

// CompressionPolicy defines the type of policy for compression of snapshots.
// +kubebuilder:validation:Enum=gzip;lzw;zlib
type CompressionPolicy string

// CompactionMode defines the auto-compaction-mode: 'periodic' or 'revision'.
// 'periodic' for duration based retention and 'revision' for revision number based retention.
// +kubebuilder:validation:Enum=periodic;revision
type CompactionMode string

// TLSConfig hold the TLS configuration details.
type TLSConfig struct {
	// +required
	TLSCASecretRef SecretReference `json:"tlsCASecretRef"`
	// +required
	ServerTLSSecretRef corev1.SecretReference `json:"serverTLSSecretRef"`
	// +optional
	ClientTLSSecretRef corev1.SecretReference `json:"clientTLSSecretRef"`
}

// SecretReference defines a reference to a secret.
type SecretReference struct {
	corev1.SecretReference `json:",inline"`
	// DataKey is the name of the key in the data map containing the credentials.
	// +optional
	DataKey *string `json:"dataKey,omitempty"`
}

// CompressionSpec defines parameters related to compression of Snapshots(full as well as delta).
type CompressionSpec struct {
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// +optional
	Policy *CompressionPolicy `json:"policy,omitempty"`
}

// LeaderElectionSpec defines parameters related to the LeaderElection configuration.
type LeaderElectionSpec struct {
	// ReelectionPeriod defines the Period after which leadership status of corresponding etcd is checked.
	// +optional
	ReelectionPeriod *metav1.Duration `json:"reelectionPeriod,omitempty"`
	// EtcdConnectionTimeout defines the timeout duration for etcd client connection during leader election.
	// +optional
	EtcdConnectionTimeout *metav1.Duration `json:"etcdConnectionTimeout,omitempty"`
}

// BackupSpec defines parameters associated with the full and delta snapshots of etcd.
type BackupSpec struct {
	// Port define the port on which etcd-backup-restore server will be exposed.
	// +optional
	Port *int32 `json:"port,omitempty"`
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`
	// Image defines the etcd container image and tag
	// +optional
	Image *string `json:"image,omitempty"`
	// Store defines the specification of object store provider for storing backups.
	// +optional
	Store *StoreSpec `json:"store,omitempty"`
	// Resources defines compute Resources required by backup-restore container.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// CompactionResources defines compute Resources required by compaction job.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	CompactionResources *corev1.ResourceRequirements `json:"compactionResources,omitempty"`
	// FullSnapshotSchedule defines the cron standard schedule for full snapshots.
	// +optional
	FullSnapshotSchedule *string `json:"fullSnapshotSchedule,omitempty"`
	// GarbageCollectionPolicy defines the policy for garbage collecting old backups
	// +optional
	GarbageCollectionPolicy *GarbageCollectionPolicy `json:"garbageCollectionPolicy,omitempty"`
	// GarbageCollectionPeriod defines the period for garbage collecting old backups
	// +optional
	GarbageCollectionPeriod *metav1.Duration `json:"garbageCollectionPeriod,omitempty"`
	// DeltaSnapshotPeriod defines the period after which delta snapshots will be taken
	// +optional
	DeltaSnapshotPeriod *metav1.Duration `json:"deltaSnapshotPeriod,omitempty"`
	// DeltaSnapshotMemoryLimit defines the memory limit after which delta snapshots will be taken
	// +optional
	DeltaSnapshotMemoryLimit *resource.Quantity `json:"deltaSnapshotMemoryLimit,omitempty"`
	// SnapshotCompression defines the specification for compression of Snapshots.
	// +optional
	SnapshotCompression *CompressionSpec `json:"compression,omitempty"`
	// EnableProfiling defines if profiling should be enabled for the etcd-backup-restore-sidecar
	// +optional
	EnableProfiling *bool `json:"enableProfiling,omitempty"`
	// EtcdSnapshotTimeout defines the timeout duration for etcd FullSnapshot operation
	// +optional
	EtcdSnapshotTimeout *metav1.Duration `json:"etcdSnapshotTimeout,omitempty"`
	// LeaderElection defines parameters related to the LeaderElection configuration.
	// +optional
	LeaderElection *LeaderElectionSpec `json:"leaderElection,omitempty"`
}

// EtcdConfig defines parameters associated etcd deployed
type EtcdConfig struct {
	// Quota defines the etcd DB quota.
	// +optional
	Quota *resource.Quantity `json:"quota,omitempty"`
	// DefragmentationSchedule defines the cron standard schedule for defragmentation of etcd.
	// +optional
	DefragmentationSchedule *string `json:"defragmentationSchedule,omitempty"`
	// +optional
	ServerPort *int32 `json:"serverPort,omitempty"`
	// +optional
	ClientPort *int32 `json:"clientPort,omitempty"`
	// Image defines the etcd container image and tag
	// +optional
	Image *string `json:"image,omitempty"`
	// +optional
	AuthSecretRef *corev1.SecretReference `json:"authSecretRef,omitempty"`
	// Metrics defines the level of detail for exported metrics of etcd, specify 'extensive' to include histogram metrics.
	// +optional
	Metrics *MetricsLevel `json:"metrics,omitempty"`
	// Resources defines the compute Resources required by etcd container.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// ClientUrlTLS contains the ca, server TLS and client TLS secrets for client communication to ETCD cluster
	// +optional
	ClientUrlTLS *TLSConfig `json:"clientUrlTls,omitempty"`
	// PeerUrlTLS contains the ca and server TLS secrets for peer communication within ETCD cluster
	// Currently, PeerUrlTLS does not require client TLS secrets for gardener implementation of ETCD cluster.
	// +optional
	PeerUrlTLS *TLSConfig `json:"peerUrlTls,omitempty"`
	// EtcdDefragTimeout defines the timeout duration for etcd defrag call
	// +optional
	EtcdDefragTimeout *metav1.Duration `json:"etcdDefragTimeout,omitempty"`
	// HeartbeatDuration defines the duration for members to send heartbeats. The default value is 10s.
	// +optional
	HeartbeatDuration *metav1.Duration `json:"heartbeatDuration,omitempty"`
	// ClientService defines the parameters of the client service that a user can specify
	// +optional
	ClientService *ClientService `json:"clientService,omitempty"`
}

// ClientService defines the parameters of the client service that a user can specify
type ClientService struct {
	// Annotations specify the annotations that should be added to the client service
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels specify the labels that should be added to the client service
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// SharedConfig defines parameters shared and used by Etcd as well as backup-restore sidecar.
type SharedConfig struct {
	// AutoCompactionMode defines the auto-compaction-mode:'periodic' mode or 'revision' mode for etcd and embedded-Etcd of backup-restore sidecar.
	// +optional
	AutoCompactionMode *CompactionMode `json:"autoCompactionMode,omitempty"`
	//AutoCompactionRetention defines the auto-compaction-retention length for etcd as well as for embedded-Etcd of backup-restore sidecar.
	// +optional
	AutoCompactionRetention *string `json:"autoCompactionRetention,omitempty"`
}

// SchedulingConstraints defines the different scheduling constraints that must be applied to the
// pod spec in the etcd statefulset.
// Currently supported constraints are Affinity and TopologySpreadConstraints.
type SchedulingConstraints struct {
	// Affinity defines the various affinity and anti-affinity rules for a pod
	// that are honoured by the kube-scheduler.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// TopologySpreadConstraints describes how a group of pods ought to spread across topology domains,
	// that are honoured by the kube-scheduler.
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// EtcdSpec defines the desired state of Etcd
type EtcdSpec struct {
	// selector is a label query over pods that should match the replica count.
	// It must match the pod template's labels.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector *metav1.LabelSelector `json:"selector"`
	// +required
	Labels map[string]string `json:"labels"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +required
	Etcd EtcdConfig `json:"etcd"`
	// +required
	Backup BackupSpec `json:"backup"`
	// +optional
	Common SharedConfig `json:"sharedConfig,omitempty"`
	// +optional
	SchedulingConstraints SchedulingConstraints `json:"schedulingConstraints,omitempty"`
	// +required
	Replicas int32 `json:"replicas"`
	// PriorityClassName is the name of a priority class that shall be used for the etcd pods.
	// +optional
	PriorityClassName *string `json:"priorityClassName,omitempty"`
	// StorageClass defines the name of the StorageClass required by the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1
	// +optional
	StorageClass *string `json:"storageClass,omitempty"`
	// StorageCapacity defines the size of persistent volume.
	// +optional
	StorageCapacity *resource.Quantity `json:"storageCapacity,omitempty"`
	// VolumeClaimTemplate defines the volume claim template to be created
	// +optional
	VolumeClaimTemplate *string `json:"volumeClaimTemplate,omitempty"`
}

// CrossVersionObjectReference contains enough information to let you identify the referred resource.
type CrossVersionObjectReference struct {
	// Kind of the referent
	// +required
	Kind string `json:"kind,omitempty"`
	// Name of the referent
	// +required
	Name string `json:"name,omitempty"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

const (
	// ConditionTypeReady is a constant for a condition type indicating that the etcd cluster is ready.
	ConditionTypeReady ConditionType = "Ready"
	// ConditionTypeAllMembersReady is a constant for a condition type indicating that all members of the etcd cluster are ready.
	ConditionTypeAllMembersReady ConditionType = "AllMembersReady"
	// ConditionTypeBackupReady is a constant for a condition type indicating that the etcd backup is ready.
	ConditionTypeBackupReady ConditionType = "BackupReady"
)

// EtcdMemberConditionStatus is the status of an etcd cluster member.
type EtcdMemberConditionStatus string

const (
	// EtcdMemberStatusReady means a etcd member is ready.
	EtcdMemberStatusReady EtcdMemberConditionStatus = "Ready"
	// EtcdMemberStatusNotReady means a etcd member is not ready.
	EtcdMemberStatusNotReady EtcdMemberConditionStatus = "NotReady"
	// EtcdMemberStatusUnknown means the status of an etcd member is unknown.
	EtcdMemberStatusUnknown EtcdMemberConditionStatus = "Unknown"
)

// EtcdRole is the role of an etcd cluster member.
type EtcdRole string

const (
	// EtcdRoleLeader describes the etcd role `Leader`.
	EtcdRoleLeader EtcdRole = "Leader"
	// EtcdRoleMember describes the etcd role `Member`.
	EtcdRoleMember EtcdRole = "Member"
)

// EtcdMemberStatus holds information about a etcd cluster membership.
type EtcdMemberStatus struct {
	// Name is the name of the etcd member. It is the name of the backing `Pod`.
	Name string `json:"name"`
	// ID is the ID of the etcd member.
	// +optional
	ID *string `json:"id,omitempty"`
	// Role is the role in the etcd cluster, either `Leader` or `Member`.
	// +optional
	Role *EtcdRole `json:"role,omitempty"`
	// Status of the condition, one of True, False, Unknown.
	Status EtcdMemberConditionStatus `json:"status"`
	// The reason for the condition's last transition.
	Reason string `json:"reason"`
	// LastTransitionTime is the last time the condition's status changed.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// EtcdStatus defines the observed state of Etcd.
type EtcdStatus struct {
	// ObservedGeneration is the most recent generation observed for this resource.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
	// +optional
	Etcd *CrossVersionObjectReference `json:"etcd,omitempty"`
	// Conditions represents the latest available observations of an etcd's current state.
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
	// ServiceName is the name of the etcd service.
	// Deprecated: this field will be removed in the future.
	// +optional
	ServiceName *string `json:"serviceName,omitempty"`
	// LastError represents the last occurred error.
	// +optional
	LastError *string `json:"lastError,omitempty"`
	// Cluster size is the current size of the etcd cluster.
	// Deprecated: this field will not be populated with any value and will be removed in the future.
	// +optional
	ClusterSize *int32 `json:"clusterSize,omitempty"`
	// CurrentReplicas is the current replica count for the etcd cluster.
	// +optional
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`
	// Replicas is the replica count of the etcd resource.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
	// ReadyReplicas is the count of replicas being ready in the etcd cluster.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// Ready is `true` if all etcd replicas are ready.
	// +optional
	Ready *bool `json:"ready,omitempty"`
	// UpdatedReplicas is the count of updated replicas in the etcd cluster.
	// Deprecated: this field will be removed in the future.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
	// LabelSelector is a label query over pods that should match the replica count.
	// It must match the pod template's labels.
	// Deprecated: this field will be removed in the future.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	// Members represents the members of the etcd cluster
	// +optional
	Members []EtcdMemberStatus `json:"members,omitempty"`
	// PeerUrlTLSEnabled captures the state of peer url TLS being enabled for the etcd member(s)
	// +optional
	PeerUrlTLSEnabled *bool `json:"peerUrlTLSEnabled,omitempty"`
}

// GetPeerServiceName returns the peer service name for the Etcd cluster reachable by members within the Etcd cluster.
func (e *Etcd) GetPeerServiceName() string {
	return fmt.Sprintf("%s-peer", e.Name)
}

// GetClientServiceName returns the client service name for the Etcd cluster reachable by external clients.
func (e *Etcd) GetClientServiceName() string {
	return fmt.Sprintf("%s-client", e.Name)
}

// GetServiceAccountName returns the service account name for the Etcd.
func (e *Etcd) GetServiceAccountName() string {
	return e.Name
}

// GetConfigmapName returns the name of the configmap for the Etcd.
func (e *Etcd) GetConfigmapName() string {
	return fmt.Sprintf("etcd-bootstrap-%s", string(e.UID[:6]))
}

// GetCompactionJobName returns the compaction job name for the Etcd.
func (e *Etcd) GetCompactionJobName() string {
	return fmt.Sprintf("%s-compact-job", string(e.UID[:6]))
}

// GetOrdinalPodName returns the Etcd pod name based on the ordinal.
func (e *Etcd) GetOrdinalPodName(ordinal int) string {
	return fmt.Sprintf("%s-%d", e.Name, ordinal)
}

// GetDeltaSnapshotLeaseName returns the name of the delta snapshot lease for the Etcd.
func (e *Etcd) GetDeltaSnapshotLeaseName() string {
	return fmt.Sprintf("%s-delta-snap", e.Name)
}

// GetFullSnapshotLeaseName returns the name of the full snapshot lease for the Etcd.
func (e *Etcd) GetFullSnapshotLeaseName() string {
	return fmt.Sprintf("%s-full-snap", e.Name)
}

// GetDefaultLabels returns the default labels for etcd.
func (e *Etcd) GetDefaultLabels() map[string]string {
	return map[string]string{
		"name":     "etcd",
		"instance": e.Name,
	}
}

// GetAsOwnerReference returns an OwnerReference object that represents the current Etcd instance.
func (e *Etcd) GetAsOwnerReference() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         GroupVersion.String(),
		Kind:               "Etcd",
		Name:               e.Name,
		UID:                e.UID,
		Controller:         pointer.Bool(true),
		BlockOwnerDeletion: pointer.Bool(true),
	}
}

// GetRoleName returns the role name for the Etcd
func (e *Etcd) GetRoleName() string {
	return fmt.Sprintf("%s:etcd:%s", GroupVersion.Group, e.Name)
}

// GetRoleBindingName returns the rolebinding name for the Etcd
func (e *Etcd) GetRoleBindingName() string {
	return fmt.Sprintf("%s:etcd:%s", GroupVersion.Group, e.Name)
}
