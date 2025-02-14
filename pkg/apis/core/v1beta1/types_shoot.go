// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"time"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Shoot represents a Shoot cluster created and managed by Gardener.
type Shoot struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Specification of the Shoot cluster.
	// If the object's deletion timestamp is set, this field is immutable.
	// +optional
	Spec ShootSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Most recently observed status of the Shoot cluster.
	// +optional
	Status ShootStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootList is a list of Shoot objects.
type ShootList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of Shoots.
	Items []Shoot `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ShootTemplate is a template for creating a Shoot object.
type ShootTemplate struct {
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Specification of the desired behavior of the Shoot.
	// +optional
	Spec ShootSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// ShootSpec is the specification of a Shoot.
type ShootSpec struct {
	// Addons contains information about enabled/disabled addons and their configuration.
	// +optional
	Addons *Addons `json:"addons,omitempty" protobuf:"bytes,1,opt,name=addons"`
	// CloudProfileName is a name of a CloudProfile object.
	// Deprecated: This field will be removed in a future version of Gardener. Use `CloudProfile` instead.
	// Until removed, this field is synced with the `CloudProfile` field.
	// +optional
	CloudProfileName *string `json:"cloudProfileName,omitempty" protobuf:"bytes,2,opt,name=cloudProfileName"`
	// DNS contains information about the DNS settings of the Shoot.
	// +optional
	DNS *DNS `json:"dns,omitempty" protobuf:"bytes,3,opt,name=dns"`
	// Extensions contain type and provider information for Shoot extensions.
	// +optional
	Extensions []Extension `json:"extensions,omitempty" protobuf:"bytes,4,rep,name=extensions"`
	// Hibernation contains information whether the Shoot is suspended or not.
	// +optional
	Hibernation *Hibernation `json:"hibernation,omitempty" protobuf:"bytes,5,opt,name=hibernation"`
	// Kubernetes contains the version and configuration settings of the control plane components.
	Kubernetes Kubernetes `json:"kubernetes" protobuf:"bytes,6,opt,name=kubernetes"`
	// Networking contains information about cluster networking such as CNI Plugin type, CIDRs, ...etc.
	// +optional
	Networking *Networking `json:"networking,omitempty" protobuf:"bytes,7,opt,name=networking"`
	// Maintenance contains information about the time window for maintenance operations and which
	// operations should be performed.
	// +optional
	Maintenance *Maintenance `json:"maintenance,omitempty" protobuf:"bytes,8,opt,name=maintenance"`
	// Monitoring contains information about custom monitoring configurations for the shoot.
	// +optional
	Monitoring *Monitoring `json:"monitoring,omitempty" protobuf:"bytes,9,opt,name=monitoring"`
	// Provider contains all provider-specific and provider-relevant information.
	Provider Provider `json:"provider" protobuf:"bytes,10,opt,name=provider"`
	// Purpose is the purpose class for this cluster.
	// +optional
	Purpose *ShootPurpose `json:"purpose,omitempty" protobuf:"bytes,11,opt,name=purpose,casttype=ShootPurpose"`
	// Region is a name of a region. This field is immutable.
	Region string `json:"region" protobuf:"bytes,12,opt,name=region"`
	// SecretBindingName is the name of a SecretBinding that has a reference to the provider secret.
	// The credentials inside the provider secret will be used to create the shoot in the respective account.
	// The field is mutually exclusive with CredentialsBindingName.
	// This field is immutable.
	// +optional
	SecretBindingName *string `json:"secretBindingName,omitempty" protobuf:"bytes,13,opt,name=secretBindingName"`
	// SeedName is the name of the seed cluster that runs the control plane of the Shoot.
	// +optional
	SeedName *string `json:"seedName,omitempty" protobuf:"bytes,14,opt,name=seedName"`
	// SeedSelector is an optional selector which must match a seed's labels for the shoot to be scheduled on that seed.
	// +optional
	SeedSelector *SeedSelector `json:"seedSelector,omitempty" protobuf:"bytes,15,opt,name=seedSelector"`
	// Resources holds a list of named resource references that can be referred to in extension configs by their names.
	// +optional
	Resources []NamedResourceReference `json:"resources,omitempty" protobuf:"bytes,16,rep,name=resources"`
	// Tolerations contains the tolerations for taints on seed clusters.
	// +patchMergeKey=key
	// +patchStrategy=merge
	// +optional
	Tolerations []Toleration `json:"tolerations,omitempty" patchStrategy:"merge" patchMergeKey:"key" protobuf:"bytes,17,rep,name=tolerations"`
	// ExposureClassName is the optional name of an exposure class to apply a control plane endpoint exposure strategy.
	// This field is immutable.
	// +optional
	ExposureClassName *string `json:"exposureClassName,omitempty" protobuf:"bytes,18,opt,name=exposureClassName"`
	// SystemComponents contains the settings of system components in the control or data plane of the Shoot cluster.
	// +optional
	SystemComponents *SystemComponents `json:"systemComponents,omitempty" protobuf:"bytes,19,opt,name=systemComponents"`
	// ControlPlane contains general settings for the control plane of the shoot.
	// +optional
	ControlPlane *ControlPlane `json:"controlPlane,omitempty" protobuf:"bytes,20,opt,name=controlPlane"`
	// SchedulerName is the name of the responsible scheduler which schedules the shoot.
	// If not specified, the default scheduler takes over.
	// This field is immutable.
	// +optional
	SchedulerName *string `json:"schedulerName,omitempty" protobuf:"bytes,21,opt,name=schedulerName"`
	// CloudProfile contains a reference to a CloudProfile or a NamespacedCloudProfile.
	// +optional
	CloudProfile *CloudProfileReference `json:"cloudProfile,omitempty" protobuf:"bytes,22,opt,name=cloudProfile"`
	// CredentialsBindingName is the name of a CredentialsBinding that has a reference to the provider credentials.
	// The credentials will be used to create the shoot in the respective account. The field is mutually exclusive with SecretBindingName.
	// +optional
	CredentialsBindingName *string `json:"credentialsBindingName,omitempty" protobuf:"bytes,23,opt,name=credentialsBindingName"`
	// AccessRestrictions describe a list of access restrictions for this shoot cluster.
	// +optional
	AccessRestrictions []AccessRestrictionWithOptions `json:"accessRestrictions,omitempty" protobuf:"bytes,24,rep,name=accessRestrictions"`
}

// ShootStatus holds the most recently observed status of the Shoot cluster.
type ShootStatus struct {
	// Conditions represents the latest available observations of a Shoots's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// Constraints represents conditions of a Shoot's current state that constraint some operations on it.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Constraints []Condition `json:"constraints,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=constraints"`
	// Gardener holds information about the Gardener which last acted on the Shoot.
	Gardener Gardener `json:"gardener" protobuf:"bytes,3,opt,name=gardener"`
	// IsHibernated indicates whether the Shoot is currently hibernated.
	IsHibernated bool `json:"hibernated" protobuf:"varint,4,opt,name=hibernated"`
	// LastOperation holds information about the last operation on the Shoot.
	// +optional
	LastOperation *LastOperation `json:"lastOperation,omitempty" protobuf:"bytes,5,opt,name=lastOperation"`
	// LastErrors holds information about the last occurred error(s) during an operation.
	// +optional
	LastErrors []LastError `json:"lastErrors,omitempty" protobuf:"bytes,6,rep,name=lastErrors"`
	// ObservedGeneration is the most recent generation observed for this Shoot. It corresponds to the
	// Shoot's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,7,opt,name=observedGeneration"`
	// RetryCycleStartTime is the start time of the last retry cycle (used to determine how often an operation
	// must be retried until we give up).
	// +optional
	RetryCycleStartTime *metav1.Time `json:"retryCycleStartTime,omitempty" protobuf:"bytes,8,opt,name=retryCycleStartTime"`
	// SeedName is the name of the seed cluster that runs the control plane of the Shoot. This value is only written
	// after a successful create/reconcile operation. It will be used when control planes are moved between Seeds.
	// +optional
	SeedName *string `json:"seedName,omitempty" protobuf:"bytes,9,opt,name=seedName"`
	// TechnicalID is a unique technical ID for this Shoot. It is used for the infrastructure resources, and
	// basically everything that is related to this particular Shoot. For regular shoot clusters, this is also the name
	// of the namespace in the seed cluster running the shoot's control plane. This field is immutable.
	TechnicalID string `json:"technicalID" protobuf:"bytes,10,opt,name=technicalID"`
	// UID is a unique identifier for the Shoot cluster to avoid portability between Kubernetes clusters.
	// It is used to compute unique hashes. This field is immutable.
	UID types.UID `json:"uid" protobuf:"bytes,11,opt,name=uid,casttype=k8s.io/apimachinery/pkg/types.UID"`
	// ClusterIdentity is the identity of the Shoot cluster. This field is immutable.
	// +optional
	ClusterIdentity *string `json:"clusterIdentity,omitempty" protobuf:"bytes,12,opt,name=clusterIdentity"`
	// List of addresses that are relevant to the shoot.
	// These include the Kube API server address and also the service account issuer.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	AdvertisedAddresses []ShootAdvertisedAddress `json:"advertisedAddresses,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,13,rep,name=advertisedAddresses"`
	// MigrationStartTime is the time when a migration to a different seed was initiated.
	// +optional
	MigrationStartTime *metav1.Time `json:"migrationStartTime,omitempty" protobuf:"bytes,14,opt,name=migrationStartTime"`
	// Credentials contains information about the shoot credentials.
	// +optional
	Credentials *ShootCredentials `json:"credentials,omitempty" protobuf:"bytes,15,opt,name=credentials"`
	// LastHibernationTriggerTime indicates the last time when the hibernation controller
	// managed to change the hibernation settings of the cluster
	// +optional
	LastHibernationTriggerTime *metav1.Time `json:"lastHibernationTriggerTime,omitempty" protobuf:"bytes,16,opt,name=lastHibernationTriggerTime"`
	// LastMaintenance holds information about the last maintenance operations on the Shoot.
	// +optional
	LastMaintenance *LastMaintenance `json:"lastMaintenance,omitempty" protobuf:"bytes,17,opt,name=lastMaintenance"`
	// EncryptedResources is the list of resources in the Shoot which are currently encrypted.
	// Secrets are encrypted by default and are not part of the list.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/security/etcd_encryption_config.md for more details.
	// +optional
	EncryptedResources []string `json:"encryptedResources,omitempty" protobuf:"bytes,18,rep,name=encryptedResources"`
	// Networking contains information about cluster networking such as CIDRs.
	// +optional
	Networking *NetworkingStatus `json:"networking,omitempty" protobuf:"bytes,19,opt,name=networking"`
}

// LastMaintenance holds information about a maintenance operation on the Shoot.
type LastMaintenance struct {
	// A human-readable message containing details about the operations performed in the last maintenance.
	Description string `json:"description" protobuf:"bytes,1,opt,name=description"`
	// TriggeredTime is the time when maintenance was triggered.
	TriggeredTime metav1.Time `json:"triggeredTime" protobuf:"bytes,2,opt,name=triggeredTime"`
	// Status of the last maintenance operation, one of Processing, Succeeded, Error.
	State LastOperationState `json:"state" protobuf:"bytes,3,opt,name=state,casttype=LastOperationState"`
	// FailureReason holds the information about the last maintenance operation failure reason.
	// +optional
	FailureReason *string `json:"failureReason,omitempty" protobuf:"bytes,4,opt,name=failureReason"`
}

// NetworkingStatus contains information about cluster networking such as CIDRs.
type NetworkingStatus struct {
	// Pods are the CIDRs of the pod network.
	// +optional
	Pods []string `json:"pods,omitempty" protobuf:"bytes,1,rep,name=pods"`
	// Nodes are the CIDRs of the node network.
	// +optional
	Nodes []string `json:"nodes,omitempty" protobuf:"bytes,2,rep,name=nodes"`
	// Services are the CIDRs of the service network.
	// +optional
	Services []string `json:"services,omitempty" protobuf:"bytes,3,rep,name=services"`
	// EgressCIDRs is a list of CIDRs used by the shoot as the source IP for egress traffic as reported by the used
	// Infrastructure extension controller. For certain environments the egress IPs may not be stable in which case the
	// extension controller may opt to not populate this field.
	// +optional
	EgressCIDRs []string `json:"egressCIDRs,omitempty" protobuf:"bytes,4,rep,name=egressCIDRs"`
}

// ShootCredentials contains information about the shoot credentials.
type ShootCredentials struct {
	// Rotation contains information about the credential rotations.
	// +optional
	Rotation *ShootCredentialsRotation `json:"rotation,omitempty" protobuf:"bytes,1,opt,name=rotation"`
}

// ShootCredentialsRotation contains information about the rotation of credentials.
type ShootCredentialsRotation struct {
	// CertificateAuthorities contains information about the certificate authority credential rotation.
	// +optional
	CertificateAuthorities *CARotation `json:"certificateAuthorities,omitempty" protobuf:"bytes,1,opt,name=certificateAuthorities"`
	// Kubeconfig contains information about the kubeconfig credential rotation.
	// +optional
	//
	// Deprecated: This field is deprecated and will be removed in gardener v1.120
	Kubeconfig *ShootKubeconfigRotation `json:"kubeconfig,omitempty" protobuf:"bytes,2,opt,name=kubeconfig"`
	// SSHKeypair contains information about the ssh-keypair credential rotation.
	// +optional
	SSHKeypair *ShootSSHKeypairRotation `json:"sshKeypair,omitempty" protobuf:"bytes,3,opt,name=sshKeypair"`
	// Observability contains information about the observability credential rotation.
	// +optional
	Observability *ObservabilityRotation `json:"observability,omitempty" protobuf:"bytes,4,opt,name=observability"`
	// ServiceAccountKey contains information about the service account key credential rotation.
	// +optional
	ServiceAccountKey *ServiceAccountKeyRotation `json:"serviceAccountKey,omitempty" protobuf:"bytes,5,opt,name=serviceAccountKey"`
	// ETCDEncryptionKey contains information about the ETCD encryption key credential rotation.
	// +optional
	ETCDEncryptionKey *ETCDEncryptionKeyRotation `json:"etcdEncryptionKey,omitempty" protobuf:"bytes,6,opt,name=etcdEncryptionKey"`
}

// CARotation contains information about the certificate authority credential rotation.
type CARotation struct {
	// Phase describes the phase of the certificate authority credential rotation.
	Phase CredentialsRotationPhase `json:"phase" protobuf:"bytes,1,opt,name=phase"`
	// LastCompletionTime is the most recent time when the certificate authority credential rotation was successfully
	// completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
	// LastInitiationTime is the most recent time when the certificate authority credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,3,opt,name=lastInitiationTime"`
	// LastInitiationFinishedTime is the recent time when the certificate authority credential rotation initiation was
	// completed.
	// +optional
	LastInitiationFinishedTime *metav1.Time `json:"lastInitiationFinishedTime,omitempty" protobuf:"bytes,4,opt,name=lastInitiationFinishedTime"`
	// LastCompletionTriggeredTime is the recent time when the certificate authority credential rotation completion was
	// triggered.
	// +optional
	LastCompletionTriggeredTime *metav1.Time `json:"lastCompletionTriggeredTime,omitempty" protobuf:"bytes,5,opt,name=lastCompletionTriggeredTime"`
	// PendingWorkersRollouts contains the name of a worker pool and the initiation time of their last rollout due to
	// credentials rotation.
	// +optional
	PendingWorkersRollouts []PendingWorkersRollout `json:"pendingWorkersRollouts,omitempty" protobuf:"bytes,6,rep,name=pendingWorkersRollouts"`
}

// ShootKubeconfigRotation contains information about the kubeconfig credential rotation.
type ShootKubeconfigRotation struct {
	// LastInitiationTime is the most recent time when the kubeconfig credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,1,opt,name=lastInitiationTime"`
	// LastCompletionTime is the most recent time when the kubeconfig credential rotation was successfully completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
}

// ShootSSHKeypairRotation contains information about the ssh-keypair credential rotation.
type ShootSSHKeypairRotation struct {
	// LastInitiationTime is the most recent time when the ssh-keypair credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,1,opt,name=lastInitiationTime"`
	// LastCompletionTime is the most recent time when the ssh-keypair credential rotation was successfully completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
}

// ObservabilityRotation contains information about the observability credential rotation.
type ObservabilityRotation struct {
	// LastInitiationTime is the most recent time when the observability credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,1,opt,name=lastInitiationTime"`
	// LastCompletionTime is the most recent time when the observability credential rotation was successfully completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
}

// ServiceAccountKeyRotation contains information about the service account key credential rotation.
type ServiceAccountKeyRotation struct {
	// Phase describes the phase of the service account key credential rotation.
	Phase CredentialsRotationPhase `json:"phase" protobuf:"bytes,1,opt,name=phase"`
	// LastCompletionTime is the most recent time when the service account key credential rotation was successfully
	// completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
	// LastInitiationTime is the most recent time when the service account key credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,3,opt,name=lastInitiationTime"`
	// LastInitiationFinishedTime is the recent time when the service account key credential rotation initiation was
	// completed.
	// +optional
	LastInitiationFinishedTime *metav1.Time `json:"lastInitiationFinishedTime,omitempty" protobuf:"bytes,4,opt,name=lastInitiationFinishedTime"`
	// LastCompletionTriggeredTime is the recent time when the service account key credential rotation completion was
	// triggered.
	// +optional
	LastCompletionTriggeredTime *metav1.Time `json:"lastCompletionTriggeredTime,omitempty" protobuf:"bytes,5,opt,name=lastCompletionTriggeredTime"`
	// PendingWorkersRollouts contains the name of a worker pool and the initiation time of their last rollout due to
	// credentials rotation.
	// +optional
	PendingWorkersRollouts []PendingWorkersRollout `json:"pendingWorkersRollouts,omitempty" protobuf:"bytes,6,rep,name=pendingWorkersRollouts"`
}

// ETCDEncryptionKeyRotation contains information about the ETCD encryption key credential rotation.
type ETCDEncryptionKeyRotation struct {
	// Phase describes the phase of the ETCD encryption key credential rotation.
	Phase CredentialsRotationPhase `json:"phase" protobuf:"bytes,1,opt,name=phase"`
	// LastCompletionTime is the most recent time when the ETCD encryption key credential rotation was successfully
	// completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
	// LastInitiationTime is the most recent time when the ETCD encryption key credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,3,opt,name=lastInitiationTime"`
	// LastInitiationFinishedTime is the recent time when the ETCD encryption key credential rotation initiation was
	// completed.
	// +optional
	LastInitiationFinishedTime *metav1.Time `json:"lastInitiationFinishedTime,omitempty" protobuf:"bytes,4,opt,name=lastInitiationFinishedTime"`
	// LastCompletionTriggeredTime is the recent time when the ETCD encryption key credential rotation completion was
	// triggered.
	// +optional
	LastCompletionTriggeredTime *metav1.Time `json:"lastCompletionTriggeredTime,omitempty" protobuf:"bytes,5,opt,name=lastCompletionTriggeredTime"`
}

// CredentialsRotationPhase is a string alias.
type CredentialsRotationPhase string

const (
	// RotationPreparing is a constant for the credentials rotation phase describing that the procedure is being prepared.
	RotationPreparing CredentialsRotationPhase = "Preparing"
	// RotationPreparingWithoutWorkersRollout is a constant for the credentials rotation phase describing that the
	// procedure is being prepared without triggering a worker pool rollout.
	RotationPreparingWithoutWorkersRollout CredentialsRotationPhase = "PreparingWithoutWorkersRollout"
	// RotationWaitingForWorkersRollout is a constant for the credentials rotation phase describing that the procedure
	// was prepared but is still waiting for the workers to roll out.
	RotationWaitingForWorkersRollout CredentialsRotationPhase = "WaitingForWorkersRollout"
	// RotationPrepared is a constant for the credentials rotation phase describing that the procedure was prepared.
	RotationPrepared CredentialsRotationPhase = "Prepared"
	// RotationCompleting is a constant for the credentials rotation phase describing that the procedure is being
	// completed.
	RotationCompleting CredentialsRotationPhase = "Completing"
	// RotationCompleted is a constant for the credentials rotation phase describing that the procedure was completed.
	RotationCompleted CredentialsRotationPhase = "Completed"
)

// PendingWorkersRollout contains the name of a worker pool and the initiation time of their last rollout due to
// credentials rotation.
type PendingWorkersRollout struct {
	// Name is the name of a worker pool.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// LastInitiationTime is the most recent time when the credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,2,opt,name=lastInitiationTime"`
}

// ShootAdvertisedAddress contains information for the shoot's Kube API server.
type ShootAdvertisedAddress struct {
	// Name of the advertised address. e.g. external
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// The URL of the API Server. e.g. https://api.foo.bar or https://1.2.3.4
	URL string `json:"url" protobuf:"bytes,2,opt,name=url"`
}

// Addons is a collection of configuration for specific addons which are managed by the Gardener.
type Addons struct {
	// KubernetesDashboard holds configuration settings for the kubernetes dashboard addon.
	// +optional
	KubernetesDashboard *KubernetesDashboard `json:"kubernetesDashboard,omitempty" protobuf:"bytes,1,opt,name=kubernetesDashboard"`
	// NginxIngress holds configuration settings for the nginx-ingress addon.
	// +optional
	NginxIngress *NginxIngress `json:"nginxIngress,omitempty" protobuf:"bytes,2,opt,name=nginxIngress"`
}

// Addon allows enabling or disabling a specific addon and is used to derive from.
type Addon struct {
	// Enabled indicates whether the addon is enabled or not.
	Enabled bool `json:"enabled" protobuf:"varint,1,opt,name=enabled"`
}

// KubernetesDashboard describes configuration values for the kubernetes-dashboard addon.
type KubernetesDashboard struct {
	Addon `json:",inline" protobuf:"bytes,2,opt,name=addon"`
	// AuthenticationMode defines the authentication mode for the kubernetes-dashboard.
	// +optional
	AuthenticationMode *string `json:"authenticationMode,omitempty" protobuf:"bytes,1,opt,name=authenticationMode"`
}

const (
	// KubernetesDashboardAuthModeToken uses token-based mode for auth.
	KubernetesDashboardAuthModeToken = "token"
)

// NginxIngress describes configuration values for the nginx-ingress addon.
type NginxIngress struct {
	Addon `json:",inline" protobuf:"bytes,1,opt,name=addon"`
	// LoadBalancerSourceRanges is list of allowed IP sources for NginxIngress
	// +optional
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty" protobuf:"bytes,2,rep,name=loadBalancerSourceRanges"`
	// Config contains custom configuration for the nginx-ingress-controller configuration.
	// See https://github.com/kubernetes/ingress-nginx/blob/master/docs/user-guide/nginx-configuration/configmap.md#configuration-options
	// +optional
	Config map[string]string `json:"config,omitempty" protobuf:"bytes,3,rep,name=config"`
	// ExternalTrafficPolicy controls the `.spec.externalTrafficPolicy` value of the load balancer `Service`
	// exposing the nginx-ingress. Defaults to `Cluster`.
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty" protobuf:"bytes,4,opt,name=externalTrafficPolicy,casttype=k8s.io/api/core/v1.ServiceExternalTrafficPolicy"`
}

// ControlPlane holds information about the general settings for the control plane of a shoot.
type ControlPlane struct {
	// HighAvailability holds the configuration settings for high availability of the
	// control plane of a shoot.
	// +optional
	HighAvailability *HighAvailability `json:"highAvailability,omitempty" protobuf:"bytes,1,name=highAvailability"`
}

// DNS holds information about the provider, the hosted zone id and the domain.
type DNS struct {
	// Domain is the external available domain of the Shoot cluster. This domain will be written into the
	// kubeconfig that is handed out to end-users. This field is immutable.
	// +optional
	Domain *string `json:"domain,omitempty" protobuf:"bytes,1,opt,name=domain"`
	// Providers is a list of DNS providers that shall be enabled for this shoot cluster. Only relevant if
	// not a default domain is used.
	//
	// Deprecated: Configuring multiple DNS providers is deprecated and will be forbidden in a future release.
	// Please use the DNS extension provider config (e.g. shoot-dns-service) for additional providers.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Providers []DNSProvider `json:"providers,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=providers"`
}

// TODO(timuthy): Rework the 'DNSProvider' struct and deprecated fields in the scope of https://github.com/gardener/gardener/issues/9176.

// DNSProvider contains information about a DNS provider.
type DNSProvider struct {
	// Domains contains information about which domains shall be included/excluded for this provider.
	//
	// Deprecated: This field is deprecated and will be removed in a future release.
	// Please use the DNS extension provider config (e.g. shoot-dns-service) for additional configuration.
	// +optional
	Domains *DNSIncludeExclude `json:"domains,omitempty" protobuf:"bytes,1,opt,name=domains"`
	// Primary indicates that this DNSProvider is used for shoot related domains.
	//
	// Deprecated: This field is deprecated and will be removed in a future release.
	// Please use the DNS extension provider config (e.g. shoot-dns-service) for additional and non-primary providers.
	// +optional
	Primary *bool `json:"primary,omitempty" protobuf:"varint,2,opt,name=primary"`
	// SecretName is a name of a secret containing credentials for the stated domain and the
	// provider. When not specified, the Gardener will use the cloud provider credentials referenced
	// by the Shoot and try to find respective credentials there (primary provider only). Specifying this field may override
	// this behavior, i.e. forcing the Gardener to only look into the given secret.
	// +optional
	SecretName *string `json:"secretName,omitempty" protobuf:"bytes,3,opt,name=secretName"`
	// Type is the DNS provider type.
	// +optional
	Type *string `json:"type,omitempty" protobuf:"bytes,4,opt,name=type"`

	// Zones contains information about which hosted zones shall be included/excluded for this provider.
	//
	// Deprecated: This field is deprecated and will be removed in a future release.
	// Please use the DNS extension provider config (e.g. shoot-dns-service) for additional configuration.
	// +optional
	Zones *DNSIncludeExclude `json:"zones,omitempty" protobuf:"bytes,5,opt,name=zones"`
}

// DNSIncludeExclude contains information about which domains shall be included/excluded.
type DNSIncludeExclude struct {
	// Include is a list of domains that shall be included.
	// +optional
	Include []string `json:"include,omitempty" protobuf:"bytes,1,rep,name=include"`
	// Exclude is a list of domains that shall be excluded.
	// +optional
	Exclude []string `json:"exclude,omitempty" protobuf:"bytes,2,rep,name=exclude"`
}

// DefaultDomain is the default value in the Shoot's '.spec.dns.domain' when '.spec.dns.provider' is 'unmanaged'
const DefaultDomain = "cluster.local"

// Extension contains type and provider information for Shoot extensions.
type Extension struct {
	// Type is the type of the extension resource.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// ProviderConfig is the configuration passed to extension resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
	// Disabled allows to disable extensions that were marked as 'globally enabled' by Gardener administrators.
	// +optional
	Disabled *bool `json:"disabled,omitempty" protobuf:"varint,3,opt,name=disabled"`
}

// NamedResourceReference is a named reference to a resource.
type NamedResourceReference struct {
	// Name of the resource reference.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// ResourceRef is a reference to a resource.
	ResourceRef autoscalingv1.CrossVersionObjectReference `json:"resourceRef" protobuf:"bytes,2,opt,name=resourceRef"`
}

// Hibernation contains information whether the Shoot is suspended or not.
type Hibernation struct {
	// Enabled specifies whether the Shoot needs to be hibernated or not. If it is true, the Shoot's desired state is to be hibernated.
	// If it is false or nil, the Shoot's desired state is to be awakened.
	// +optional
	Enabled *bool `json:"enabled,omitempty" protobuf:"varint,1,opt,name=enabled"`
	// Schedules determine the hibernation schedules.
	// +optional
	Schedules []HibernationSchedule `json:"schedules,omitempty" protobuf:"bytes,2,rep,name=schedules"`
}

// HibernationSchedule determines the hibernation schedule of a Shoot.
// A Shoot will be regularly hibernated at each start time and will be woken up at each end time.
// Start or End can be omitted, though at least one of each has to be specified.
type HibernationSchedule struct {
	// Start is a Cron spec at which time a Shoot will be hibernated.
	// +optional
	Start *string `json:"start,omitempty" protobuf:"bytes,1,opt,name=start"`
	// End is a Cron spec at which time a Shoot will be woken up.
	// +optional
	End *string `json:"end,omitempty" protobuf:"bytes,2,opt,name=end"`
	// Location is the time location in which both start and shall be evaluated.
	// +optional
	Location *string `json:"location,omitempty" protobuf:"bytes,3,opt,name=location"`
}

// Kubernetes contains the version and configuration variables for the Shoot control plane.
type Kubernetes struct {
	// AllowPrivilegedContainers is tombstoned to show why 1 is reserved protobuf tag.
	// AllowPrivilegedContainers *bool `json:"allowPrivilegedContainers,omitempty" protobuf:"varint,1,opt,name=allowPrivilegedContainers"`

	// ClusterAutoscaler contains the configuration flags for the Kubernetes cluster autoscaler.
	// +optional
	ClusterAutoscaler *ClusterAutoscaler `json:"clusterAutoscaler,omitempty" protobuf:"bytes,2,opt,name=clusterAutoscaler"`
	// KubeAPIServer contains configuration settings for the kube-apiserver.
	// +optional
	KubeAPIServer *KubeAPIServerConfig `json:"kubeAPIServer,omitempty" protobuf:"bytes,3,opt,name=kubeAPIServer"`
	// KubeControllerManager contains configuration settings for the kube-controller-manager.
	// +optional
	KubeControllerManager *KubeControllerManagerConfig `json:"kubeControllerManager,omitempty" protobuf:"bytes,4,opt,name=kubeControllerManager"`
	// KubeScheduler contains configuration settings for the kube-scheduler.
	// +optional
	KubeScheduler *KubeSchedulerConfig `json:"kubeScheduler,omitempty" protobuf:"bytes,5,opt,name=kubeScheduler"`
	// KubeProxy contains configuration settings for the kube-proxy.
	// +optional
	KubeProxy *KubeProxyConfig `json:"kubeProxy,omitempty" protobuf:"bytes,6,opt,name=kubeProxy"`
	// Kubelet contains configuration settings for the kubelet.
	// +optional
	Kubelet *KubeletConfig `json:"kubelet,omitempty" protobuf:"bytes,7,opt,name=kubelet"`
	// Note: Even though 'Version' is an optional field for users, we deliberately chose to not make it a pointer
	// because the field is guaranteed to be not-empty after the admission plugin processed the shoot object.
	// Thus, pointer handling for this field is not beneficial and would make things more cumbersome.

	// Version is the semantic Kubernetes version to use for the Shoot cluster.
	// Defaults to the highest supported minor and patch version given in the referenced cloud profile.
	// The version can be omitted completely or partially specified, e.g. `<major>.<minor>`.
	// +optional
	Version string `json:"version,omitempty" protobuf:"bytes,8,opt,name=version"`
	// VerticalPodAutoscaler contains the configuration flags for the Kubernetes vertical pod autoscaler.
	// +optional
	VerticalPodAutoscaler *VerticalPodAutoscaler `json:"verticalPodAutoscaler,omitempty" protobuf:"bytes,9,opt,name=verticalPodAutoscaler"`
	// EnableStaticTokenKubeconfig indicates whether static token kubeconfig secret will be created for the Shoot cluster.
	// Setting this field to true is not supported.
	// +optional
	//
	// Deprecated: This field is deprecated and will be removed in gardener v1.120
	EnableStaticTokenKubeconfig *bool `json:"enableStaticTokenKubeconfig,omitempty" protobuf:"varint,10,opt,name=enableStaticTokenKubeconfig"`
	// ETCD contains configuration for etcds of the shoot cluster.
	// +optional
	ETCD *ETCD `json:"etcd,omitempty" protobuf:"bytes,11,opt,name=etcd"`
}

// ETCD contains configuration for etcds of the shoot cluster.
type ETCD struct {
	// Main contains configuration for the main etcd.
	// +optional
	Main *ETCDConfig `json:"main,omitempty" protobuf:"bytes,1,opt,name=main"`
	// Events contains configuration for the events etcd.
	// +optional
	Events *ETCDConfig `json:"events,omitempty" protobuf:"bytes,2,opt,name=events"`
}

// ETCDConfig contains etcd configuration.
type ETCDConfig struct {
	// Autoscaling contains auto-scaling configuration options for etcd.
	// +optional
	Autoscaling *ControlPlaneAutoscaling `json:"autoscaling,omitempty" protobuf:"bytes,1,opt,name=autoscaling"`
}

// ClusterAutoscaler contains the configuration flags for the Kubernetes cluster autoscaler.
type ClusterAutoscaler struct {
	// ScaleDownDelayAfterAdd defines how long after scale up that scale down evaluation resumes (default: 1 hour).
	// +optional
	ScaleDownDelayAfterAdd *metav1.Duration `json:"scaleDownDelayAfterAdd,omitempty" protobuf:"bytes,1,opt,name=scaleDownDelayAfterAdd"`
	// ScaleDownDelayAfterDelete how long after node deletion that scale down evaluation resumes, defaults to scanInterval (default: 0 secs).
	// +optional
	ScaleDownDelayAfterDelete *metav1.Duration `json:"scaleDownDelayAfterDelete,omitempty" protobuf:"bytes,2,opt,name=scaleDownDelayAfterDelete"`
	// ScaleDownDelayAfterFailure how long after scale down failure that scale down evaluation resumes (default: 3 mins).
	// +optional
	ScaleDownDelayAfterFailure *metav1.Duration `json:"scaleDownDelayAfterFailure,omitempty" protobuf:"bytes,3,opt,name=scaleDownDelayAfterFailure"`
	// ScaleDownUnneededTime defines how long a node should be unneeded before it is eligible for scale down (default: 30 mins).
	// +optional
	ScaleDownUnneededTime *metav1.Duration `json:"scaleDownUnneededTime,omitempty" protobuf:"bytes,4,opt,name=scaleDownUnneededTime"`
	// ScaleDownUtilizationThreshold defines the threshold in fraction (0.0 - 1.0) under which a node is being removed (default: 0.5).
	// +optional
	ScaleDownUtilizationThreshold *float64 `json:"scaleDownUtilizationThreshold,omitempty" protobuf:"fixed64,5,opt,name=scaleDownUtilizationThreshold"`
	// ScanInterval how often cluster is reevaluated for scale up or down (default: 10 secs).
	// +optional
	ScanInterval *metav1.Duration `json:"scanInterval,omitempty" protobuf:"bytes,6,opt,name=scanInterval"`
	// Expander defines the algorithm to use during scale up (default: least-waste).
	// See: https://github.com/gardener/autoscaler/blob/machine-controller-manager-provider/cluster-autoscaler/FAQ.md#what-are-expanders.
	// +optional
	Expander *ExpanderMode `json:"expander,omitempty" protobuf:"bytes,7,opt,name=expander"`
	// MaxNodeProvisionTime defines how long CA waits for node to be provisioned (default: 20 mins).
	// +optional
	MaxNodeProvisionTime *metav1.Duration `json:"maxNodeProvisionTime,omitempty" protobuf:"bytes,8,opt,name=maxNodeProvisionTime"`
	// MaxGracefulTerminationSeconds is the number of seconds CA waits for pod termination when trying to scale down a node (default: 600).
	// +optional
	MaxGracefulTerminationSeconds *int32 `json:"maxGracefulTerminationSeconds,omitempty" protobuf:"varint,9,opt,name=maxGracefulTerminationSeconds"`
	// IgnoreTaints specifies a list of taint keys to ignore in node templates when considering to scale a node group.
	// Deprecated: Ignore taints are deprecated as of K8S 1.29 and treated as startup taints
	// +optional
	IgnoreTaints []string `json:"ignoreTaints,omitempty" protobuf:"bytes,10,opt,name=ignoreTaints"`

	// NewPodScaleUpDelay specifies how long CA should ignore newly created pods before they have to be considered for scale-up (default: 0s).
	// +optional
	NewPodScaleUpDelay *metav1.Duration `json:"newPodScaleUpDelay,omitempty" protobuf:"bytes,11,opt,name=newPodScaleUpDelay"`
	// MaxEmptyBulkDelete specifies the maximum number of empty nodes that can be deleted at the same time (default: 10).
	// +optional
	MaxEmptyBulkDelete *int32 `json:"maxEmptyBulkDelete,omitempty" protobuf:"varint,12,opt,name=maxEmptyBulkDelete"`
	// IgnoreDaemonsetsUtilization allows CA to ignore DaemonSet pods when calculating resource utilization for scaling down (default: false).
	// +optional
	IgnoreDaemonsetsUtilization *bool `json:"ignoreDaemonsetsUtilization,omitempty" protobuf:"varint,13,opt,name=ignoreDaemonsetsUtilization"`
	// Verbosity allows CA to modify its log level (default: 2).
	// +optional
	Verbosity *int32 `json:"verbosity,omitempty" protobuf:"varint,14,opt,name=verbosity"`
	// StartupTaints specifies a list of taint keys to ignore in node templates when considering to scale a node group.
	// Cluster Autoscaler treats nodes tainted with startup taints as unready, but taken into account during scale up logic, assuming they will become ready shortly.
	// +optional
	StartupTaints []string `json:"startupTaints,omitempty" protobuf:"bytes,15,opt,name=startupTaints"`
	// StatusTaints specifies a list of taint keys to ignore in node templates when considering to scale a node group.
	// Cluster Autoscaler internally treats nodes tainted with status taints as ready, but filtered out during scale up logic.
	// +optional
	StatusTaints []string `json:"statusTaints,omitempty" protobuf:"bytes,16,opt,name=statusTaints"`
}

// ExpanderMode is type used for Expander values
type ExpanderMode string

const (
	// ClusterAutoscalerExpanderLeastWaste selects the node group that will have the least idle CPU (if tied, unused memory) after scale-up.
	// This is useful when you have different classes of nodes, for example, high CPU or high memory nodes, and
	// only want to expand those when there are pending pods that need a lot of those resources.
	// This is the default value.
	ClusterAutoscalerExpanderLeastWaste ExpanderMode = "least-waste"
	// ClusterAutoscalerExpanderMostPods selects the node group that would be able to schedule the most pods when scaling up.
	// This is useful when you are using nodeSelector to make sure certain pods land on certain nodes.
	// Note that this won't cause the autoscaler to select bigger nodes vs. smaller, as it can add multiple smaller nodes at once.
	ClusterAutoscalerExpanderMostPods ExpanderMode = "most-pods"
	// ClusterAutoscalerExpanderPriority selects the node group that has the highest priority assigned by the user. For configurations,
	// See: https://github.com/gardener/autoscaler/blob/machine-controller-manager-provider/cluster-autoscaler/expander/priority/readme.md
	ClusterAutoscalerExpanderPriority ExpanderMode = "priority"
	// ClusterAutoscalerExpanderRandom should be used when you don't have a particular need
	// for the node groups to scale differently.
	ClusterAutoscalerExpanderRandom ExpanderMode = "random"
)

// VerticalPodAutoscaler contains the configuration flags for the Kubernetes vertical pod autoscaler.
type VerticalPodAutoscaler struct {
	// Enabled specifies whether the Kubernetes VPA shall be enabled for the shoot cluster.
	Enabled bool `json:"enabled" protobuf:"varint,1,opt,name=enabled"`
	// EvictAfterOOMThreshold defines the threshold that will lead to pod eviction in case it OOMed in less than the given
	// threshold since its start and if it has only one container (default: 10m0s).
	// +optional
	EvictAfterOOMThreshold *metav1.Duration `json:"evictAfterOOMThreshold,omitempty" protobuf:"bytes,2,opt,name=evictAfterOOMThreshold"`
	// EvictionRateBurst defines the burst of pods that can be evicted (default: 1)
	// +optional
	EvictionRateBurst *int32 `json:"evictionRateBurst,omitempty" protobuf:"varint,3,opt,name=evictionRateBurst"`
	// EvictionRateLimit defines the number of pods that can be evicted per second. A rate limit set to 0 or -1 will
	// disable the rate limiter (default: -1).
	// +optional
	EvictionRateLimit *float64 `json:"evictionRateLimit,omitempty" protobuf:"fixed64,4,opt,name=evictionRateLimit"`
	// EvictionTolerance defines the fraction of replica count that can be evicted for update in case more than one
	// pod can be evicted (default: 0.5).
	// +optional
	EvictionTolerance *float64 `json:"evictionTolerance,omitempty" protobuf:"fixed64,5,opt,name=evictionTolerance"`
	// RecommendationMarginFraction is the fraction of usage added as the safety margin to the recommended request
	// (default: 0.15).
	// +optional
	RecommendationMarginFraction *float64 `json:"recommendationMarginFraction,omitempty" protobuf:"fixed64,6,opt,name=recommendationMarginFraction"`
	// UpdaterInterval is the interval how often the updater should run (default: 1m0s).
	// +optional
	UpdaterInterval *metav1.Duration `json:"updaterInterval,omitempty" protobuf:"bytes,7,opt,name=updaterInterval"`
	// RecommenderInterval is the interval how often metrics should be fetched (default: 1m0s).
	// +optional
	RecommenderInterval *metav1.Duration `json:"recommenderInterval,omitempty" protobuf:"bytes,8,opt,name=recommenderInterval"`
	// TargetCPUPercentile is the usage percentile that will be used as a base for CPU target recommendation.
	// Doesn't affect CPU lower bound, CPU upper bound nor memory recommendations.
	// (default: 0.9)
	// +optional
	TargetCPUPercentile *float64 `json:"targetCPUPercentile,omitempty" protobuf:"fixed64,9,opt,name=targetCPUPercentile"`
	// RecommendationLowerBoundCPUPercentile is the usage percentile that will be used for the lower bound on CPU recommendation.
	// (default: 0.5)
	// +optional
	RecommendationLowerBoundCPUPercentile *float64 `json:"recommendationLowerBoundCPUPercentile,omitempty" protobuf:"fixed64,10,opt,name=recommendationLowerBoundCPUPercentile"`
	// RecommendationUpperBoundCPUPercentile is the usage percentile that will be used for the upper bound on CPU recommendation.
	// (default: 0.95)
	// +optional
	RecommendationUpperBoundCPUPercentile *float64 `json:"recommendationUpperBoundCPUPercentile,omitempty" protobuf:"fixed64,11,opt,name=recommendationUpperBoundCPUPercentile"`
	// TargetMemoryPercentile is the usage percentile that will be used as a base for memory target recommendation.
	// Doesn't affect memory lower bound nor memory upper bound.
	// (default: 0.9)
	// +optional
	TargetMemoryPercentile *float64 `json:"targetMemoryPercentile,omitempty" protobuf:"fixed64,12,opt,name=targetMemoryPercentile"`
	// RecommendationLowerBoundMemoryPercentile is the usage percentile that will be used for the lower bound on memory recommendation.
	// (default: 0.5)
	// +optional
	RecommendationLowerBoundMemoryPercentile *float64 `json:"recommendationLowerBoundMemoryPercentile,omitempty" protobuf:"fixed64,13,opt,name=recommendationLowerBoundMemoryPercentile"`
	// RecommendationUpperBoundMemoryPercentile is the usage percentile that will be used for the upper bound on memory recommendation.
	// (default: 0.95)
	// +optional
	RecommendationUpperBoundMemoryPercentile *float64 `json:"recommendationUpperBoundMemoryPercentile,omitempty" protobuf:"fixed64,14,opt,name=recommendationUpperBoundMemoryPercentile"`
	// CPUHistogramDecayHalfLife is the amount of time it takes a historical CPU usage sample to lose half of its weight.
	// (default: 24h)
	// +optional
	CPUHistogramDecayHalfLife *metav1.Duration `json:"cpuHistogramDecayHalfLife,omitempty" protobuf:"bytes,15,opt,name=cpuHistogramDecayHalfLife"`
	// MemoryHistogramDecayHalfLife is the amount of time it takes a historical memory usage sample to lose half of its weight.
	// (default: 24h)
	// +optional
	MemoryHistogramDecayHalfLife *metav1.Duration `json:"memoryHistogramDecayHalfLife,omitempty" protobuf:"bytes,16,opt,name=memoryHistogramDecayHalfLife"`
	// MemoryAggregationInterval is the length of a single interval, for which the peak memory usage is computed.
	// (default: 24h)
	// +optional
	MemoryAggregationInterval *metav1.Duration `json:"memoryAggregationInterval,omitempty" protobuf:"bytes,17,opt,name=memoryAggregationInterval"`
	// MemoryAggregationIntervalCount is the number of consecutive memory-aggregation-intervals which make up the
	// MemoryAggregationWindowLength which in turn is the period for memory usage aggregation by VPA. In other words,
	// `MemoryAggregationWindowLength = memory-aggregation-interval * memory-aggregation-interval-count`.
	// (default: 8)
	// +optional
	MemoryAggregationIntervalCount *int64 `json:"memoryAggregationIntervalCount,omitempty" protobuf:"varint,18,opt,name=memoryAggregationIntervalCount"`
}

const (
	// DefaultEvictionRateBurst is the default value for the EvictionRateBurst field in the VPA configuration.
	DefaultEvictionRateBurst int32 = 1
	// DefaultEvictionRateLimit is the default value for the EvictionRateLimit field in the VPA configuration.
	DefaultEvictionRateLimit float64 = -1
	// DefaultEvictionTolerance is the default value for the EvictionTolerance field in the VPA configuration.
	DefaultEvictionTolerance = 0.5
	// DefaultRecommendationMarginFraction is the default value for the RecommendationMarginFraction field in the VPA configuration.
	DefaultRecommendationMarginFraction = 0.15
	// DefaultTargetCPUPercentile is the default value for the TargetCPUPercentile field in the VPA configuration.
	DefaultTargetCPUPercentile = 0.9
	// DefaultRecommendationLowerBoundCPUPercentile is the default value for the RecommendationLowerBoundCPUPercentile field in the VPA configuration.
	DefaultRecommendationLowerBoundCPUPercentile = 0.5
	// DefaultRecommendationUpperBoundCPUPercentile is the default value for the RecommendationUpperBoundCPUPercentile field in the VPA configuration.
	DefaultRecommendationUpperBoundCPUPercentile = 0.95
	// DefaultTargetMemoryPercentile is the default value for the TargetMemoryPercentile field in the VPA configuration.
	DefaultTargetMemoryPercentile = 0.9
	// DefaultRecommendationLowerBoundMemoryPercentile is the default value for the RecommendationLowerBoundMemoryPercentile field in the VPA configuration.
	DefaultRecommendationLowerBoundMemoryPercentile = 0.5
	// DefaultRecommendationUpperBoundMemoryPercentile is the default value for the RecommendationUpperBoundMemoryPercentile field in the VPA configuration.
	DefaultRecommendationUpperBoundMemoryPercentile = 0.95
)

var (
	// DefaultEvictAfterOOMThreshold is the default value for the EvictAfterOOMThreshold field in the VPA configuration.
	DefaultEvictAfterOOMThreshold = metav1.Duration{Duration: 10 * time.Minute}
	// DefaultUpdaterInterval is the default value for the UpdaterInterval field in the VPA configuration.
	DefaultUpdaterInterval = metav1.Duration{Duration: time.Minute}
	// DefaultRecommenderInterval is the default value for the RecommenderInterval field in the VPA configuration.
	DefaultRecommenderInterval = metav1.Duration{Duration: time.Minute}
	// DefaultCPUHistogramDecayHalfLife is the default value for the CPUHistogramDecayHalfLife field in the VPA configuration.
	DefaultCPUHistogramDecayHalfLife = metav1.Duration{Duration: 24 * time.Hour}
	// DefaultMemoryHistogramDecayHalfLife is the default value for the MemoryHistogramDecayHalfLife field in the VPA configuration.
	DefaultMemoryHistogramDecayHalfLife = metav1.Duration{Duration: 24 * time.Hour}
	// DefaultMemoryAggregationInterval is the default value for the MemoryAggregationInterval field in the VPA configuration.
	DefaultMemoryAggregationInterval = metav1.Duration{Duration: 24 * time.Hour}
	// DefaultMemoryAggregationIntervalCount is the default value for the MemoryAggregationIntervalCount field in the VPA configuration.
	DefaultMemoryAggregationIntervalCount = int64(8)
)

// KubernetesConfig contains common configuration fields for the control plane components.
type KubernetesConfig struct {
	// FeatureGates contains information about enabled feature gates.
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty" protobuf:"bytes,1,rep,name=featureGates"`
}

// KubeAPIServerConfig contains configuration settings for the kube-apiserver.
type KubeAPIServerConfig struct {
	KubernetesConfig `json:",inline" protobuf:"bytes,1,opt,name=kubernetesConfig"`
	// AdmissionPlugins contains the list of user-defined admission plugins (additional to those managed by Gardener), and, if desired, the corresponding
	// configuration.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	AdmissionPlugins []AdmissionPlugin `json:"admissionPlugins,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,2,rep,name=admissionPlugins"`
	// APIAudiences are the identifiers of the API. The service account token authenticator will
	// validate that tokens used against the API are bound to at least one of these audiences.
	// Defaults to ["kubernetes"].
	// +optional
	APIAudiences []string `json:"apiAudiences,omitempty" protobuf:"bytes,3,rep,name=apiAudiences"`
	// AuditConfig contains configuration settings for the audit of the kube-apiserver.
	// +optional
	AuditConfig *AuditConfig `json:"auditConfig,omitempty" protobuf:"bytes,4,opt,name=auditConfig"`

	// EnableBasicAuthentication is tombstoned to show why 5 is reserved protobuf tag.
	// EnableBasicAuthentication *bool `json:"enableBasicAuthentication,omitempty" protobuf:"varint,5,opt,name=enableBasicAuthentication"`

	// OIDCConfig contains configuration settings for the OIDC provider.
	//
	// Deprecated: This field is deprecated and will be forbidden starting from Kubernetes 1.32.
	// Please configure and use structured authentication instead of oidc flags.
	// For more information check https://github.com/gardener/gardener/issues/9858
	// TODO(AleksandarSavchev): Drop this field after support for Kubernetes 1.31 is dropped.
	// +optional
	OIDCConfig *OIDCConfig `json:"oidcConfig,omitempty" protobuf:"bytes,6,opt,name=oidcConfig"`
	// RuntimeConfig contains information about enabled or disabled APIs.
	// +optional
	RuntimeConfig map[string]bool `json:"runtimeConfig,omitempty" protobuf:"bytes,7,rep,name=runtimeConfig"`
	// ServiceAccountConfig contains configuration settings for the service account handling
	// of the kube-apiserver.
	// +optional
	ServiceAccountConfig *ServiceAccountConfig `json:"serviceAccountConfig,omitempty" protobuf:"bytes,8,opt,name=serviceAccountConfig"`
	// WatchCacheSizes contains configuration of the API server's watch cache sizes.
	// Configuring these flags might be useful for large-scale Shoot clusters with a lot of parallel update requests
	// and a lot of watching controllers (e.g. large ManagedSeed clusters). When the API server's watch cache's
	// capacity is too small to cope with the amount of update requests and watchers for a particular resource, it
	// might happen that controller watches are permanently stopped with `too old resource version` errors.
	// Starting from kubernetes v1.19, the API server's watch cache size is adapted dynamically and setting the watch
	// cache size flags will have no effect, except when setting it to 0 (which disables the watch cache).
	// +optional
	WatchCacheSizes *WatchCacheSizes `json:"watchCacheSizes,omitempty" protobuf:"bytes,9,opt,name=watchCacheSizes"`
	// Requests contains configuration for request-specific settings for the kube-apiserver.
	// +optional
	Requests *APIServerRequests `json:"requests,omitempty" protobuf:"bytes,10,opt,name=requests"`
	// EnableAnonymousAuthentication defines whether anonymous requests to the secure port
	// of the API server should be allowed (flag `--anonymous-auth`).
	// See: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/
	// +optional
	EnableAnonymousAuthentication *bool `json:"enableAnonymousAuthentication,omitempty" protobuf:"varint,11,opt,name=enableAnonymousAuthentication"`
	// EventTTL controls the amount of time to retain events.
	// Defaults to 1h.
	// +optional
	EventTTL *metav1.Duration `json:"eventTTL,omitempty" protobuf:"bytes,12,opt,name=eventTTL"`
	// Logging contains configuration for the log level and HTTP access logs.
	// +optional
	Logging *APIServerLogging `json:"logging,omitempty" protobuf:"bytes,13,opt,name=logging"`
	// DefaultNotReadyTolerationSeconds indicates the tolerationSeconds of the toleration for notReady:NoExecute
	// that is added by default to every pod that does not already have such a toleration (flag `--default-not-ready-toleration-seconds`).
	// The field has effect only when the `DefaultTolerationSeconds` admission plugin is enabled.
	// Defaults to 300.
	// +optional
	DefaultNotReadyTolerationSeconds *int64 `json:"defaultNotReadyTolerationSeconds,omitempty" protobuf:"varint,14,opt,name=defaultNotReadyTolerationSeconds"`
	// DefaultUnreachableTolerationSeconds indicates the tolerationSeconds of the toleration for unreachable:NoExecute
	// that is added by default to every pod that does not already have such a toleration (flag `--default-unreachable-toleration-seconds`).
	// The field has effect only when the `DefaultTolerationSeconds` admission plugin is enabled.
	// Defaults to 300.
	// +optional
	DefaultUnreachableTolerationSeconds *int64 `json:"defaultUnreachableTolerationSeconds,omitempty" protobuf:"varint,15,opt,name=defaultUnreachableTolerationSeconds"`
	// EncryptionConfig contains customizable encryption configuration of the Kube API server.
	// +optional
	EncryptionConfig *EncryptionConfig `json:"encryptionConfig,omitempty" protobuf:"bytes,16,opt,name=encryptionConfig"`
	// StructuredAuthentication contains configuration settings for structured authentication for the kube-apiserver.
	// This field is only available for Kubernetes v1.30 or later.
	// +optional
	StructuredAuthentication *StructuredAuthentication `json:"structuredAuthentication,omitempty" protobuf:"bytes,17,opt,name=structuredAuthentication"`
	// StructuredAuthorization contains configuration settings for structured authorization for the kube-apiserver.
	// This field is only available for Kubernetes v1.30 or later.
	// +optional
	StructuredAuthorization *StructuredAuthorization `json:"structuredAuthorization,omitempty" protobuf:"bytes,18,opt,name=structuredAuthorization"`
	// Autoscaling contains auto-scaling configuration options for the kube-apiserver.
	// +optional
	Autoscaling *ControlPlaneAutoscaling `json:"autoscaling,omitempty" protobuf:"bytes,19,opt,name=autoscaling"`
}

// ControlPlaneAutoscaling contains auto-scaling configuration options for control-plane components.
type ControlPlaneAutoscaling struct {
	// MinAllowed configures the minimum allowed resource requests.
	// Configuration of minAllowed resources is an advanced feature that can help clusters to overcome scale-up delays.
	// Default values are not applied to this field.
	// +optional
	MinAllowed corev1.ResourceList `json:"minAllowed,omitempty" protobuf:"bytes,1,rep,name=minAllowed,casttype=k8s.io/api/core/v1.ResourceList,castkey=k8s.io/api/core/v1.ResourceName"`
}

// APIServerLogging contains configuration for the logs level and http access logs
type APIServerLogging struct {
	// Verbosity is the kube-apiserver log verbosity level
	// Defaults to 2.
	// +optional
	Verbosity *int32 `json:"verbosity,omitempty" protobuf:"varint,1,opt,name=verbosity"`
	// HTTPAccessVerbosity is the kube-apiserver access logs level
	// +optional
	HTTPAccessVerbosity *int32 `json:"httpAccessVerbosity,omitempty" protobuf:"varint,2,opt,name=httpAccessVerbosity"`
}

// APIServerRequests contains configuration for request-specific settings for the kube-apiserver.
type APIServerRequests struct {
	// MaxNonMutatingInflight is the maximum number of non-mutating requests in flight at a given time. When the server
	// exceeds this, it rejects requests.
	// +optional
	MaxNonMutatingInflight *int32 `json:"maxNonMutatingInflight,omitempty" protobuf:"bytes,1,name=maxNonMutatingInflight"`
	// MaxMutatingInflight is the maximum number of mutating requests in flight at a given time. When the server
	// exceeds this, it rejects requests.
	// +optional
	MaxMutatingInflight *int32 `json:"maxMutatingInflight,omitempty" protobuf:"bytes,2,name=maxMutatingInflight"`
}

// EncryptionConfig contains customizable encryption configuration of the API server.
type EncryptionConfig struct {
	// Resources contains the list of resources that shall be encrypted in addition to secrets.
	// Each item is a Kubernetes resource name in plural (resource or resource.group) that should be encrypted.
	// Wildcards are not supported for now.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/security/etcd_encryption_config.md for more details.
	Resources []string `json:"resources" protobuf:"bytes,1,rep,name=resources"`
}

// ServiceAccountConfig is the kube-apiserver configuration for service accounts.
type ServiceAccountConfig struct {
	// Issuer is the identifier of the service account token issuer. The issuer will assert this
	// identifier in "iss" claim of issued tokens. This value is used to generate new service account tokens.
	// This value is a string or URI. Defaults to URI of the API server.
	// +optional
	Issuer *string `json:"issuer,omitempty" protobuf:"bytes,1,opt,name=issuer"`

	// SigningKeySecret is tombstoned to show why 2 is reserved protobuf tag.
	// SigningKeySecret *corev1.LocalObjectReference `json:"signingKeySecretName,omitempty" protobuf:"bytes,2,opt,name=signingKeySecretName"`

	// ExtendTokenExpiration turns on projected service account expiration extension during token generation, which
	// helps safe transition from legacy token to bound service account token feature. If this flag is enabled,
	// admission injected tokens would be extended up to 1 year to prevent unexpected failure during transition,
	// ignoring value of service-account-max-token-expiration.
	// +optional
	ExtendTokenExpiration *bool `json:"extendTokenExpiration,omitempty" protobuf:"bytes,3,opt,name=extendTokenExpiration"`
	// MaxTokenExpiration is the maximum validity duration of a token created by the service account token issuer. If an
	// otherwise valid TokenRequest with a validity duration larger than this value is requested, a token will be issued
	// with a validity duration of this value.
	// This field must be within [30d,90d].
	// +optional
	MaxTokenExpiration *metav1.Duration `json:"maxTokenExpiration,omitempty" protobuf:"bytes,4,opt,name=maxTokenExpiration"`
	// AcceptedIssuers is an additional set of issuers that are used to determine which service account tokens are accepted.
	// These values are not used to generate new service account tokens. Only useful when service account tokens are also
	// issued by another external system or a change of the current issuer that is used for generating tokens is being performed.
	// +optional
	AcceptedIssuers []string `json:"acceptedIssuers,omitempty" protobuf:"bytes,5,opt,name=acceptedIssuers"`
}

// AuditConfig contains settings for audit of the api server
type AuditConfig struct {
	// AuditPolicy contains configuration settings for audit policy of the kube-apiserver.
	// +optional
	AuditPolicy *AuditPolicy `json:"auditPolicy,omitempty" protobuf:"bytes,1,opt,name=auditPolicy"`
}

// AuditPolicy contains audit policy for kube-apiserver
type AuditPolicy struct {
	// ConfigMapRef is a reference to a ConfigMap object in the same namespace,
	// which contains the audit policy for the kube-apiserver.
	// +optional
	ConfigMapRef *corev1.ObjectReference `json:"configMapRef,omitempty" protobuf:"bytes,1,opt,name=configMapRef"`
}

// StructuredAuthentication contains authentication config for kube-apiserver.
type StructuredAuthentication struct {
	// ConfigMapName is the name of the ConfigMap in the project namespace which contains AuthenticationConfiguration
	// for the kube-apiserver.
	ConfigMapName string `json:"configMapName" protobuf:"bytes,1,opt,name=configMapName"`
}

// StructuredAuthorization contains authorization config for kube-apiserver.
type StructuredAuthorization struct {
	// ConfigMapName is the name of the ConfigMap in the project namespace which contains AuthorizationConfiguration for
	// the kube-apiserver.
	ConfigMapName string `json:"configMapName" protobuf:"bytes,1,opt,name=configMapName"`
	// Kubeconfigs is a list of references for kubeconfigs for the authorization webhooks.
	Kubeconfigs []AuthorizerKubeconfigReference `json:"kubeconfigs" protobuf:"bytes,2,rep,name=kubeconfigs"`
}

// AuthorizerKubeconfigReference is a reference for a kubeconfig for a authorization webhook.
type AuthorizerKubeconfigReference struct {
	// AuthorizerName is the name of a webhook authorizer.
	AuthorizerName string `json:"authorizerName" protobuf:"bytes,1,opt,name=authorizerName"`
	// SecretName is the name of a secret containing the kubeconfig.
	SecretName string `json:"secretName" protobuf:"bytes,2,opt,name=secretName"`
}

// OIDCConfig contains configuration settings for the OIDC provider.
// Note: Descriptions were taken from the Kubernetes documentation.
type OIDCConfig struct {
	// If set, the OpenID server's certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host's root CA set will be used.
	// +optional
	CABundle *string `json:"caBundle,omitempty" protobuf:"bytes,1,opt,name=caBundle"`
	// ClientAuthentication can optionally contain client configuration used for kubeconfig generation.
	//
	// Deprecated: This field has no implemented use and will be forbidden starting from Kubernetes 1.31.
	// It's use was planned for genereting OIDC kubeconfig https://github.com/gardener/gardener/issues/1433
	// TODO(AleksandarSavchev): Drop this field after support for Kubernetes 1.30 is dropped.
	// +optional
	ClientAuthentication *OpenIDConnectClientAuthentication `json:"clientAuthentication,omitempty" protobuf:"bytes,2,opt,name=clientAuthentication"`
	// The client ID for the OpenID Connect client, must be set.
	// +optional
	ClientID *string `json:"clientID,omitempty" protobuf:"bytes,3,opt,name=clientID"`
	// If provided, the name of a custom OpenID Connect claim for specifying user groups. The claim value is expected to be a string or array of strings. This flag is experimental, please see the authentication documentation for further details.
	// +optional
	GroupsClaim *string `json:"groupsClaim,omitempty" protobuf:"bytes,4,opt,name=groupsClaim"`
	// If provided, all groups will be prefixed with this value to prevent conflicts with other authentication strategies.
	// +optional
	GroupsPrefix *string `json:"groupsPrefix,omitempty" protobuf:"bytes,5,opt,name=groupsPrefix"`
	// The URL of the OpenID issuer, only HTTPS scheme will be accepted. Used to verify the OIDC JSON Web Token (JWT).
	// +optional
	IssuerURL *string `json:"issuerURL,omitempty" protobuf:"bytes,6,opt,name=issuerURL"`
	// key=value pairs that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value.
	// +optional
	RequiredClaims map[string]string `json:"requiredClaims,omitempty" protobuf:"bytes,7,rep,name=requiredClaims"`
	// List of allowed JOSE asymmetric signing algorithms. JWTs with a 'alg' header value not in this list will be rejected. Values are defined by RFC 7518 https://tools.ietf.org/html/rfc7518#section-3.1
	// +optional
	SigningAlgs []string `json:"signingAlgs,omitempty" protobuf:"bytes,8,rep,name=signingAlgs"`
	// The OpenID claim to use as the user name. Note that claims other than the default ('sub') is not guaranteed to be unique and immutable. This flag is experimental, please see the authentication documentation for further details. (default "sub")
	// +optional
	UsernameClaim *string `json:"usernameClaim,omitempty" protobuf:"bytes,9,opt,name=usernameClaim"`
	// If provided, all usernames will be prefixed with this value. If not provided, username claims other than 'email' are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value '-'.
	// +optional
	UsernamePrefix *string `json:"usernamePrefix,omitempty" protobuf:"bytes,10,opt,name=usernamePrefix"`
}

// OpenIDConnectClientAuthentication contains configuration for OIDC clients.
type OpenIDConnectClientAuthentication struct {
	// Extra configuration added to kubeconfig's auth-provider.
	// Must not be any of idp-issuer-url, client-id, client-secret, idp-certificate-authority, idp-certificate-authority-data, id-token or refresh-token
	// +optional
	ExtraConfig map[string]string `json:"extraConfig,omitempty" protobuf:"bytes,1,rep,name=extraConfig"`
	// The client Secret for the OpenID Connect client.
	// +optional
	Secret *string `json:"secret,omitempty" protobuf:"bytes,2,opt,name=secret"`
}

// AdmissionPlugin contains information about a specific admission plugin and its corresponding configuration.
type AdmissionPlugin struct {
	// Name is the name of the plugin.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Config is the configuration of the plugin.
	// +optional
	Config *runtime.RawExtension `json:"config,omitempty" protobuf:"bytes,2,opt,name=config"`
	// Disabled specifies whether this plugin should be disabled.
	// +optional
	Disabled *bool `json:"disabled,omitempty" protobuf:"varint,3,opt,name=disabled"`
	// KubeconfigSecretName specifies the name of a secret containing the kubeconfig for this admission plugin.
	// +optional
	KubeconfigSecretName *string `json:"kubeconfigSecretName,omitempty" protobuf:"bytes,4,opt,name=kubeconfigSecretName"`
}

// WatchCacheSizes contains configuration of the API server's watch cache sizes.
type WatchCacheSizes struct {
	// Default configures the default watch cache size of the kube-apiserver
	// (flag `--default-watch-cache-size`, defaults to 100).
	// See: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/
	// +optional
	Default *int32 `json:"default,omitempty" protobuf:"varint,1,opt,name=default"`
	// Resources configures the watch cache size of the kube-apiserver per resource
	// (flag `--watch-cache-sizes`).
	// See: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/
	// +optional
	Resources []ResourceWatchCacheSize `json:"resources,omitempty" protobuf:"bytes,2,rep,name=resources"`
}

// ResourceWatchCacheSize contains configuration of the API server's watch cache size for one specific resource.
type ResourceWatchCacheSize struct {
	// APIGroup is the API group of the resource for which the watch cache size should be configured.
	// An unset value is used to specify the legacy core API (e.g. for `secrets`).
	// +optional
	APIGroup *string `json:"apiGroup,omitempty" protobuf:"bytes,1,opt,name=apiGroup"`
	// Resource is the name of the resource for which the watch cache size should be configured
	// (in lowercase plural form, e.g. `secrets`).
	Resource string `json:"resource" protobuf:"bytes,2,opt,name=resource"`
	// CacheSize specifies the watch cache size that should be configured for the specified resource.
	CacheSize int32 `json:"size" protobuf:"varint,3,opt,name=size"`
}

// KubeControllerManagerConfig contains configuration settings for the kube-controller-manager.
type KubeControllerManagerConfig struct {
	KubernetesConfig `json:",inline" protobuf:"bytes,1,opt,name=kubernetesConfig"`
	// HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
	// +optional
	HorizontalPodAutoscalerConfig *HorizontalPodAutoscalerConfig `json:"horizontalPodAutoscaler,omitempty" protobuf:"bytes,2,opt,name=horizontalPodAutoscaler"`
	// NodeCIDRMaskSize defines the mask size for node cidr in cluster (default is 24). This field is immutable.
	// +optional
	NodeCIDRMaskSize *int32 `json:"nodeCIDRMaskSize,omitempty" protobuf:"varint,3,opt,name=nodeCIDRMaskSize"`
	// PodEvictionTimeout defines the grace period for deleting pods on failed nodes. Defaults to 2m.
	// +optional
	//
	// Deprecated: The corresponding kube-controller-manager flag `--pod-eviction-timeout` is deprecated
	// in favor of the kube-apiserver flags `--default-not-ready-toleration-seconds` and `--default-unreachable-toleration-seconds`.
	// The `--pod-eviction-timeout` flag does not have effect when the taint based eviction is enabled. The taint
	// based eviction is beta (enabled by default) since Kubernetes 1.13 and GA since Kubernetes 1.18. Hence,
	// instead of setting this field, set the `spec.kubernetes.kubeAPIServer.defaultNotReadyTolerationSeconds` and
	// `spec.kubernetes.kubeAPIServer.defaultUnreachableTolerationSeconds`. This field will be removed in gardener v1.120.
	PodEvictionTimeout *metav1.Duration `json:"podEvictionTimeout,omitempty" protobuf:"bytes,4,opt,name=podEvictionTimeout"`
	// NodeMonitorGracePeriod defines the grace period before an unresponsive node is marked unhealthy.
	// +optional
	NodeMonitorGracePeriod *metav1.Duration `json:"nodeMonitorGracePeriod,omitempty" protobuf:"bytes,5,opt,name=nodeMonitorGracePeriod"`
}

// HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
// Note: Descriptions were taken from the Kubernetes documentation.
type HorizontalPodAutoscalerConfig struct {
	// The period after which a ready pod transition is considered to be the first.
	// +optional
	CPUInitializationPeriod *metav1.Duration `json:"cpuInitializationPeriod,omitempty" protobuf:"bytes,1,opt,name=cpuInitializationPeriod"`
	// The configurable window at which the controller will choose the highest recommendation for autoscaling.
	// +optional
	DownscaleStabilization *metav1.Duration `json:"downscaleStabilization,omitempty" protobuf:"bytes,3,opt,name=downscaleStabilization"`
	// The configurable period at which the horizontal pod autoscaler considers a Pod “not yet ready” given that it’s unready and it has  transitioned to unready during that time.
	// +optional
	InitialReadinessDelay *metav1.Duration `json:"initialReadinessDelay,omitempty" protobuf:"bytes,4,opt,name=initialReadinessDelay"`
	// The period for syncing the number of pods in horizontal pod autoscaler.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty" protobuf:"bytes,5,opt,name=syncPeriod"`
	// The minimum change (from 1.0) in the desired-to-actual metrics ratio for the horizontal pod autoscaler to consider scaling.
	// +optional
	Tolerance *float64 `json:"tolerance,omitempty" protobuf:"fixed64,6,opt,name=tolerance"`
}

const (
	// DefaultHPASyncPeriod is a constant for the default HPA sync period for a Shoot cluster.
	DefaultHPASyncPeriod = 30 * time.Second
	// DefaultHPATolerance is a constant for the default HPA tolerance for a Shoot cluster.
	DefaultHPATolerance = 0.1
	// DefaultDownscaleStabilization is the default HPA downscale stabilization window for a Shoot cluster
	DefaultDownscaleStabilization = 5 * time.Minute
	// DefaultInitialReadinessDelay is for the default HPA  ReadinessDelay value in the Shoot cluster
	DefaultInitialReadinessDelay = 30 * time.Second
	// DefaultCPUInitializationPeriod is the for the default value of the CPUInitializationPeriod in the Shoot cluster
	DefaultCPUInitializationPeriod = 5 * time.Minute
)

// KubeSchedulerConfig contains configuration settings for the kube-scheduler.
type KubeSchedulerConfig struct {
	KubernetesConfig `json:",inline" protobuf:"bytes,1,opt,name=kubernetesConfig"`
	// KubeMaxPDVols allows to configure the `KUBE_MAX_PD_VOLS` environment variable for the kube-scheduler.
	// Please find more information here: https://kubernetes.io/docs/concepts/storage/storage-limits/#custom-limits
	// Note that using this field is considered alpha-/experimental-level and is on your own risk. You should be aware
	// of all the side-effects and consequences when changing it.
	// +optional
	KubeMaxPDVols *string `json:"kubeMaxPDVols,omitempty" protobuf:"bytes,2,opt,name=kubeMaxPDVols"`
	// Profile configures the scheduling profile for the cluster.
	// If not specified, the used profile is "balanced" (provides the default kube-scheduler behavior).
	// +optional
	Profile *SchedulingProfile `json:"profile,omitempty" protobuf:"bytes,3,opt,name=profile,casttype=SchedulingProfile"`
}

// SchedulingProfile is a string alias used for scheduling profile values.
type SchedulingProfile string

const (
	// SchedulingProfileBalanced is a scheduling profile that attempts to spread Pods evenly across Nodes
	// to obtain a more balanced resource usage. This profile provides the default kube-scheduler behavior.
	SchedulingProfileBalanced SchedulingProfile = "balanced"
	// SchedulingProfileBinPacking is a scheduling profile that scores Nodes based on the allocation of resources.
	// It prioritizes Nodes with most allocated resources. This leads the Node count in the cluster to be minimized and
	// the Node resource utilization to be increased.
	SchedulingProfileBinPacking SchedulingProfile = "bin-packing"
)

// KubeProxyConfig contains configuration settings for the kube-proxy.
type KubeProxyConfig struct {
	KubernetesConfig `json:",inline" protobuf:"bytes,1,opt,name=kubernetesConfig"`
	// Mode specifies which proxy mode to use.
	// defaults to IPTables.
	// +optional
	Mode *ProxyMode `json:"mode,omitempty" protobuf:"bytes,2,opt,name=mode,casttype=ProxyMode"`
	// Enabled indicates whether kube-proxy should be deployed or not.
	// Depending on the networking extensions switching kube-proxy off might be rejected. Consulting the respective documentation of the used networking extension is recommended before using this field.
	// defaults to true if not specified.
	// +optional
	Enabled *bool `json:"enabled,omitempty" protobuf:"varint,3,opt,name=enabled"`
}

// ProxyMode available in Linux platform: 'userspace' (older, going to be EOL), 'iptables'
// (newer, faster), 'ipvs' (newest, better in performance and scalability).
// As of now only 'iptables' and 'ipvs' is supported by Gardener.
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
	KubernetesConfig `json:",inline" protobuf:"bytes,1,opt,name=kubernetesConfig"`
	// CPUCFSQuota allows you to disable/enable CPU throttling for Pods.
	// +optional
	CPUCFSQuota *bool `json:"cpuCFSQuota,omitempty" protobuf:"varint,2,opt,name=cpuCFSQuota"`
	// CPUManagerPolicy allows to set alternative CPU management policies (default: none).
	// +optional
	CPUManagerPolicy *string `json:"cpuManagerPolicy,omitempty" protobuf:"bytes,3,opt,name=cpuManagerPolicy"`
	// EvictionHard describes a set of eviction thresholds (e.g. memory.available<1Gi) that if met would trigger a Pod eviction.
	// +optional
	// Default:
	//   memory.available:   "100Mi/1Gi/5%"
	//   nodefs.available:   "5%"
	//   nodefs.inodesFree:  "5%"
	//   imagefs.available:  "5%"
	//   imagefs.inodesFree: "5%"
	EvictionHard *KubeletConfigEviction `json:"evictionHard,omitempty" protobuf:"bytes,4,opt,name=evictionHard"`
	// EvictionMaxPodGracePeriod describes the maximum allowed grace period (in seconds) to use when terminating pods in response to a soft eviction threshold being met.
	// +optional
	// Default: 90
	EvictionMaxPodGracePeriod *int32 `json:"evictionMaxPodGracePeriod,omitempty" protobuf:"varint,5,opt,name=evictionMaxPodGracePeriod"`
	// EvictionMinimumReclaim configures the amount of resources below the configured eviction threshold that the kubelet attempts to reclaim whenever the kubelet observes resource pressure.
	// +optional
	// Default: 0 for each resource
	EvictionMinimumReclaim *KubeletConfigEvictionMinimumReclaim `json:"evictionMinimumReclaim,omitempty" protobuf:"bytes,6,opt,name=evictionMinimumReclaim"`
	// EvictionPressureTransitionPeriod is the duration for which the kubelet has to wait before transitioning out of an eviction pressure condition.
	// +optional
	// Default: 4m0s
	EvictionPressureTransitionPeriod *metav1.Duration `json:"evictionPressureTransitionPeriod,omitempty" protobuf:"bytes,7,opt,name=evictionPressureTransitionPeriod"`
	// EvictionSoft describes a set of eviction thresholds (e.g. memory.available<1.5Gi) that if met over a corresponding grace period would trigger a Pod eviction.
	// +optional
	// Default:
	//   memory.available:   "200Mi/1.5Gi/10%"
	//   nodefs.available:   "10%"
	//   nodefs.inodesFree:  "10%"
	//   imagefs.available:  "10%"
	//   imagefs.inodesFree: "10%"
	EvictionSoft *KubeletConfigEviction `json:"evictionSoft,omitempty" protobuf:"bytes,8,opt,name=evictionSoft"`
	// EvictionSoftGracePeriod describes a set of eviction grace periods (e.g. memory.available=1m30s) that correspond to how long a soft eviction threshold must hold before triggering a Pod eviction.
	// +optional
	// Default:
	//   memory.available:   1m30s
	//   nodefs.available:   1m30s
	//   nodefs.inodesFree:  1m30s
	//   imagefs.available:  1m30s
	//   imagefs.inodesFree: 1m30s
	EvictionSoftGracePeriod *KubeletConfigEvictionSoftGracePeriod `json:"evictionSoftGracePeriod,omitempty" protobuf:"bytes,9,opt,name=evictionSoftGracePeriod"`
	// MaxPods is the maximum number of Pods that are allowed by the Kubelet.
	// +optional
	// Default: 110
	MaxPods *int32 `json:"maxPods,omitempty" protobuf:"varint,10,opt,name=maxPods"`
	// PodPIDsLimit is the maximum number of process IDs per pod allowed by the kubelet.
	// +optional
	PodPIDsLimit *int64 `json:"podPidsLimit,omitempty" protobuf:"varint,11,opt,name=podPidsLimit"`

	// ImagePullProgressDeadline is tombstoned to show why 12 is reserved protobuf tag.
	// ImagePullProgressDeadline *metav1.Duration `json:"imagePullProgressDeadline,omitempty" protobuf:"bytes,12,opt,name=imagePullProgressDeadline"`

	// FailSwapOn makes the Kubelet fail to start if swap is enabled on the node. (default true).
	// +optional
	FailSwapOn *bool `json:"failSwapOn,omitempty" protobuf:"varint,13,opt,name=failSwapOn"`
	// KubeReserved is the configuration for resources reserved for kubernetes node components (mainly kubelet and container runtime).
	// When updating these values, be aware that cgroup resizes may not succeed on active worker nodes. Look for the NodeAllocatableEnforced event to determine if the configuration was applied.
	// +optional
	// Default: cpu=80m,memory=1Gi,pid=20k
	KubeReserved *KubeletConfigReserved `json:"kubeReserved,omitempty" protobuf:"bytes,14,opt,name=kubeReserved"`
	// SystemReserved is the configuration for resources reserved for system processes not managed by kubernetes (e.g. journald).
	// When updating these values, be aware that cgroup resizes may not succeed on active worker nodes. Look for the NodeAllocatableEnforced event to determine if the configuration was applied.
	//
	// Deprecated: Separately configuring resource reservations for system processes is deprecated in Gardener and will be forbidden starting from Kubernetes 1.31.
	// Please merge existing resource reservations into the kubeReserved field.
	// TODO(MichaelEischer): Drop this field after support for Kubernetes 1.30 is dropped.
	// +optional
	SystemReserved *KubeletConfigReserved `json:"systemReserved,omitempty" protobuf:"bytes,15,opt,name=systemReserved"`
	// ImageGCHighThresholdPercent describes the percent of the disk usage which triggers image garbage collection.
	// +optional
	// Default: 50
	ImageGCHighThresholdPercent *int32 `json:"imageGCHighThresholdPercent,omitempty" protobuf:"bytes,16,opt,name=imageGCHighThresholdPercent"`
	// ImageGCLowThresholdPercent describes the percent of the disk to which garbage collection attempts to free.
	// +optional
	// Default: 40
	ImageGCLowThresholdPercent *int32 `json:"imageGCLowThresholdPercent,omitempty" protobuf:"bytes,17,opt,name=imageGCLowThresholdPercent"`
	// SerializeImagePulls describes whether the images are pulled one at a time.
	// +optional
	// Default: true
	SerializeImagePulls *bool `json:"serializeImagePulls,omitempty" protobuf:"varint,18,opt,name=serializeImagePulls"`
	// RegistryPullQPS is the limit of registry pulls per second. The value must not be a negative number.
	// Setting it to 0 means no limit.
	// Default: 5
	// +optional
	RegistryPullQPS *int32 `json:"registryPullQPS,omitempty" protobuf:"varint,19,opt,name=registryPullQPS"`
	// RegistryBurst is the maximum size of bursty pulls, temporarily allows pulls to burst to this number,
	// while still not exceeding registryPullQPS. The value must not be a negative number.
	// Only used if registryPullQPS is greater than 0.
	// Default: 10
	// +optional
	RegistryBurst *int32 `json:"registryBurst,omitempty" protobuf:"varint,20,opt,name=registryBurst"`
	// SeccompDefault enables the use of `RuntimeDefault` as the default seccomp profile for all workloads.
	// +optional
	SeccompDefault *bool `json:"seccompDefault,omitempty" protobuf:"varint,21,opt,name=seccompDefault"`
	// A quantity defines the maximum size of the container log file before it is rotated. For example: "5Mi" or "256Ki".
	// +optional
	// Default: 100Mi
	ContainerLogMaxSize *resource.Quantity `json:"containerLogMaxSize,omitempty" protobuf:"bytes,22,opt,name=containerLogMaxSize"`
	// Maximum number of container log files that can be present for a container.
	// +optional
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty" protobuf:"bytes,23,opt,name=containerLogMaxFiles"`
	// ProtectKernelDefaults ensures that the kernel tunables are equal to the kubelet defaults.
	// Defaults to true.
	// +optional
	ProtectKernelDefaults *bool `json:"protectKernelDefaults,omitempty" protobuf:"varint,24,opt,name=protectKernelDefaults"`
	// StreamingConnectionIdleTimeout is the maximum time a streaming connection can be idle before the connection is automatically closed.
	// This field cannot be set lower than "30s" or greater than "4h".
	// Default: "5m".
	// +optional
	StreamingConnectionIdleTimeout *metav1.Duration `json:"streamingConnectionIdleTimeout,omitempty" protobuf:"bytes,25,opt,name=streamingConnectionIdleTimeout"`
	// MemorySwap configures swap memory available to container workloads.
	// +optional
	MemorySwap *MemorySwapConfiguration `json:"memorySwap,omitempty" protobuf:"bytes,26,opt,name=memorySwap"`
}

// KubeletConfigEviction contains kubelet eviction thresholds supporting either a resource.Quantity or a percentage based value.
type KubeletConfigEviction struct {
	// MemoryAvailable is the threshold for the free memory on the host server.
	// +optional
	MemoryAvailable *string `json:"memoryAvailable,omitempty" protobuf:"bytes,1,opt,name=memoryAvailable"`
	// ImageFSAvailable is the threshold for the free disk space in the imagefs filesystem (docker images and container writable layers).
	// +optional
	ImageFSAvailable *string `json:"imageFSAvailable,omitempty" protobuf:"bytes,2,opt,name=imageFSAvailable"`
	// ImageFSInodesFree is the threshold for the available inodes in the imagefs filesystem.
	// +optional
	ImageFSInodesFree *string `json:"imageFSInodesFree,omitempty" protobuf:"bytes,3,opt,name=imageFSInodesFree"`
	// NodeFSAvailable is the threshold for the free disk space in the nodefs filesystem (docker volumes, logs, etc).
	// +optional
	NodeFSAvailable *string `json:"nodeFSAvailable,omitempty" protobuf:"bytes,4,opt,name=nodeFSAvailable"`
	// NodeFSInodesFree is the threshold for the available inodes in the nodefs filesystem.
	// +optional
	NodeFSInodesFree *string `json:"nodeFSInodesFree,omitempty" protobuf:"bytes,5,opt,name=nodeFSInodesFree"`
}

// KubeletConfigEvictionMinimumReclaim contains configuration for the kubelet eviction minimum reclaim.
type KubeletConfigEvictionMinimumReclaim struct {
	// MemoryAvailable is the threshold for the memory reclaim on the host server.
	// +optional
	MemoryAvailable *resource.Quantity `json:"memoryAvailable,omitempty" protobuf:"bytes,1,opt,name=memoryAvailable"`
	// ImageFSAvailable is the threshold for the disk space reclaim in the imagefs filesystem (docker images and container writable layers).
	// +optional
	ImageFSAvailable *resource.Quantity `json:"imageFSAvailable,omitempty" protobuf:"bytes,2,opt,name=imageFSAvailable"`
	// ImageFSInodesFree is the threshold for the inodes reclaim in the imagefs filesystem.
	// +optional
	ImageFSInodesFree *resource.Quantity `json:"imageFSInodesFree,omitempty" protobuf:"bytes,3,opt,name=imageFSInodesFree"`
	// NodeFSAvailable is the threshold for the disk space reclaim in the nodefs filesystem (docker volumes, logs, etc).
	// +optional
	NodeFSAvailable *resource.Quantity `json:"nodeFSAvailable,omitempty" protobuf:"bytes,4,opt,name=nodeFSAvailable"`
	// NodeFSInodesFree is the threshold for the inodes reclaim in the nodefs filesystem.
	// +optional
	NodeFSInodesFree *resource.Quantity `json:"nodeFSInodesFree,omitempty" protobuf:"bytes,5,opt,name=nodeFSInodesFree"`
}

// KubeletConfigEvictionSoftGracePeriod contains grace periods for kubelet eviction thresholds.
type KubeletConfigEvictionSoftGracePeriod struct {
	// MemoryAvailable is the grace period for the MemoryAvailable eviction threshold.
	// +optional
	MemoryAvailable *metav1.Duration `json:"memoryAvailable,omitempty" protobuf:"bytes,1,opt,name=memoryAvailable"`
	// ImageFSAvailable is the grace period for the ImageFSAvailable eviction threshold.
	// +optional
	ImageFSAvailable *metav1.Duration `json:"imageFSAvailable,omitempty" protobuf:"bytes,2,opt,name=imageFSAvailable"`
	// ImageFSInodesFree is the grace period for the ImageFSInodesFree eviction threshold.
	// +optional
	ImageFSInodesFree *metav1.Duration `json:"imageFSInodesFree,omitempty" protobuf:"bytes,3,opt,name=imageFSInodesFree"`
	// NodeFSAvailable is the grace period for the NodeFSAvailable eviction threshold.
	// +optional
	NodeFSAvailable *metav1.Duration `json:"nodeFSAvailable,omitempty" protobuf:"bytes,4,opt,name=nodeFSAvailable"`
	// NodeFSInodesFree is the grace period for the NodeFSInodesFree eviction threshold.
	// +optional
	NodeFSInodesFree *metav1.Duration `json:"nodeFSInodesFree,omitempty" protobuf:"bytes,5,opt,name=nodeFSInodesFree"`
}

// KubeletConfigReserved contains reserved resources for daemons
type KubeletConfigReserved struct {
	// CPU is the reserved cpu.
	// +optional
	CPU *resource.Quantity `json:"cpu,omitempty" protobuf:"bytes,1,opt,name=cpu"`
	// Memory is the reserved memory.
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty" protobuf:"bytes,2,opt,name=memory"`
	// EphemeralStorage is the reserved ephemeral-storage.
	// +optional
	EphemeralStorage *resource.Quantity `json:"ephemeralStorage,omitempty" protobuf:"bytes,3,opt,name=ephemeralStorage"`
	// PID is the reserved process-ids.
	// +optional
	PID *resource.Quantity `json:"pid,omitempty" protobuf:"bytes,4,opt,name=pid"`
}

// SwapBehavior configures swap memory available to container workloads
type SwapBehavior string

const (
	// NoSwap is a constant for the kubelet's swap behavior restricting Kubernetes workloads to not use swap.
	// Only available for Kubernetes versions >= v1.30.
	NoSwap SwapBehavior = "NoSwap"
	// LimitedSwap is a constant for the kubelet's swap behavior limiting the amount of swap usable for Kubernetes workloads. Workloads on the node not managed by Kubernetes can still swap.
	// - cgroupsv1 host: Kubernetes workloads can use any combination of memory and swap, up to the pod's memory limit
	// - cgroupsv2 host: swap is managed independently from memory. Kubernetes workloads cannot use swap memory.
	LimitedSwap SwapBehavior = "LimitedSwap"
	// UnlimitedSwap is a constant for the kubelet's swap behavior enabling Kubernetes workloads to use as much swap memory as required, up to the system limit (not limited by pod or container memory limits).
	// Only available for Kubernetes versions < v1.30.
	UnlimitedSwap SwapBehavior = "UnlimitedSwap"
)

// MemorySwapConfiguration contains kubelet swap configuration
// For more information, please see KEP: 2400-node-swap
type MemorySwapConfiguration struct {
	// SwapBehavior configures swap memory available to container workloads. May be one of {"LimitedSwap", "UnlimitedSwap"}
	// defaults to: LimitedSwap
	// +optional
	SwapBehavior *SwapBehavior `json:"swapBehavior,omitempty" protobuf:"bytes,1,opt,name=swapBehavior"`
}

// Networking defines networking parameters for the shoot cluster.
type Networking struct {
	// Type identifies the type of the networking plugin. This field is immutable.
	// +optional
	Type *string `json:"type,omitempty" protobuf:"bytes,1,opt,name=type"`
	// ProviderConfig is the configuration passed to network resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
	// Pods is the CIDR of the pod network. This field is immutable.
	// +optional
	Pods *string `json:"pods,omitempty" protobuf:"bytes,3,opt,name=pods"`
	// Nodes is the CIDR of the entire node network.
	// This field is mutable.
	// +optional
	Nodes *string `json:"nodes,omitempty" protobuf:"bytes,4,opt,name=nodes"`
	// Services is the CIDR of the service network. This field is immutable.
	// +optional
	Services *string `json:"services,omitempty" protobuf:"bytes,5,opt,name=services"`
	// IPFamilies specifies the IP protocol versions to use for shoot networking. This field is immutable.
	// See https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md.
	// Defaults to ["IPv4"].
	// +optional
	IPFamilies []IPFamily `json:"ipFamilies,omitempty" protobuf:"bytes,6,rep,name=ipFamilies,casttype=IPFamily"`
}

const (
	// DefaultPodNetworkCIDR is a constant for the default pod network CIDR of a Shoot cluster.
	DefaultPodNetworkCIDR = "100.96.0.0/11"
	// DefaultServiceNetworkCIDR is a constant for the default service network CIDR of a Shoot cluster.
	DefaultServiceNetworkCIDR = "100.64.0.0/13"
)

const (
	// MaintenanceTimeWindowDurationMinimum is the minimum duration for a maintenance time window.
	MaintenanceTimeWindowDurationMinimum = 30 * time.Minute
	// MaintenanceTimeWindowDurationMaximum is the maximum duration for a maintenance time window.
	MaintenanceTimeWindowDurationMaximum = 6 * time.Hour
)

// Maintenance contains information about the time window for maintenance operations and which
// operations should be performed.
type Maintenance struct {
	// AutoUpdate contains information about which constraints should be automatically updated.
	// +optional
	AutoUpdate *MaintenanceAutoUpdate `json:"autoUpdate,omitempty" protobuf:"bytes,1,opt,name=autoUpdate"`
	// TimeWindow contains information about the time window for maintenance operations.
	// +optional
	TimeWindow *MaintenanceTimeWindow `json:"timeWindow,omitempty" protobuf:"bytes,2,opt,name=timeWindow"`
	// ConfineSpecUpdateRollout prevents that changes/updates to the shoot specification will be rolled out immediately.
	// Instead, they are rolled out during the shoot's maintenance time window. There is one exception that will trigger
	// an immediate roll out which is changes to the Spec.Hibernation.Enabled field.
	// +optional
	ConfineSpecUpdateRollout *bool `json:"confineSpecUpdateRollout,omitempty" protobuf:"varint,3,opt,name=confineSpecUpdateRollout"`
}

// MaintenanceAutoUpdate contains information about which constraints should be automatically updated.
type MaintenanceAutoUpdate struct {
	// KubernetesVersion indicates whether the patch Kubernetes version may be automatically updated (default: true).
	KubernetesVersion bool `json:"kubernetesVersion" protobuf:"varint,1,opt,name=kubernetesVersion"`
	// MachineImageVersion indicates whether the machine image version may be automatically updated (default: true).
	// +optional
	MachineImageVersion *bool `json:"machineImageVersion,omitempty" protobuf:"varint,2,opt,name=machineImageVersion"`
}

// MaintenanceTimeWindow contains information about the time window for maintenance operations.
type MaintenanceTimeWindow struct {
	// Begin is the beginning of the time window in the format HHMMSS+ZONE, e.g. "220000+0100".
	// If not present, a random value will be computed.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`([0-1][0-9]|2[0-3])[0-5][0-9][0-5][0-9]\+[0-1][0-4]00`
	Begin string `json:"begin" protobuf:"bytes,1,opt,name=begin"`
	// End is the end of the time window in the format HHMMSS+ZONE, e.g. "220000+0100".
	// If not present, the value will be computed based on the "Begin" value.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`([0-1][0-9]|2[0-3])[0-5][0-9][0-5][0-9]\+[0-1][0-4]00`
	End string `json:"end" protobuf:"bytes,2,opt,name=end"`
}

// Monitoring contains information about the monitoring configuration for the shoot.
type Monitoring struct {
	// Alerting contains information about the alerting configuration for the shoot cluster.
	// +optional
	Alerting *Alerting `json:"alerting,omitempty" protobuf:"bytes,1,opt,name=alerting"`
}

// Alerting contains information about how alerting will be done (i.e. who will receive alerts and how).
type Alerting struct {
	// MonitoringEmailReceivers is a list of recipients for alerts
	// +optional
	EmailReceivers []string `json:"emailReceivers,omitempty" protobuf:"bytes,1,rep,name=emailReceivers"`
}

// Provider contains provider-specific information that are handed-over to the provider-specific
// extension controller.
type Provider struct {
	// Type is the type of the provider. This field is immutable.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// ControlPlaneConfig contains the provider-specific control plane config blob. Please look up the concrete
	// definition in the documentation of your provider extension.
	// +optional
	ControlPlaneConfig *runtime.RawExtension `json:"controlPlaneConfig,omitempty" protobuf:"bytes,2,opt,name=controlPlaneConfig"`
	// InfrastructureConfig contains the provider-specific infrastructure config blob. Please look up the concrete
	// definition in the documentation of your provider extension.
	// +optional
	InfrastructureConfig *runtime.RawExtension `json:"infrastructureConfig,omitempty" protobuf:"bytes,3,opt,name=infrastructureConfig"`
	// Workers is a list of worker groups.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	Workers []Worker `json:"workers,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,4,rep,name=workers"`
	// WorkersSettings contains settings for all workers.
	// +optional
	WorkersSettings *WorkersSettings `json:"workersSettings,omitempty" protobuf:"bytes,5,opt,name=workersSettings"`
}

// Worker is the base definition of a worker group.
type Worker struct {
	// Annotations is a map of key/value pairs for annotations for all the `Node` objects in this worker pool.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,1,rep,name=annotations"`
	// CABundle is a certificate bundle which will be installed onto every machine of this worker pool.
	// +optional
	CABundle *string `json:"caBundle,omitempty" protobuf:"bytes,2,opt,name=caBundle"`
	// CRI contains configurations of CRI support of every machine in the worker pool.
	// Defaults to a CRI with name `containerd`.
	// +optional
	CRI *CRI `json:"cri,omitempty" protobuf:"bytes,3,opt,name=cri"`
	// Kubernetes contains configuration for Kubernetes components related to this worker pool.
	// +optional
	Kubernetes *WorkerKubernetes `json:"kubernetes,omitempty" protobuf:"bytes,4,opt,name=kubernetes"`
	// Labels is a map of key/value pairs for labels for all the `Node` objects in this worker pool.
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,5,rep,name=labels"`
	// Name is the name of the worker group.
	Name string `json:"name" protobuf:"bytes,6,opt,name=name"`
	// Machine contains information about the machine type and image.
	Machine Machine `json:"machine" protobuf:"bytes,7,opt,name=machine"`
	// Maximum is the maximum number of machines to create.
	// This value is divided by the number of configured zones for a fair distribution.
	Maximum int32 `json:"maximum" protobuf:"varint,8,opt,name=maximum"`
	// Minimum is the minimum number of machines to create.
	// This value is divided by the number of configured zones for a fair distribution.
	Minimum int32 `json:"minimum" protobuf:"varint,9,opt,name=minimum"`
	// MaxSurge is maximum number of machines that are created during an update.
	// This value is divided by the number of configured zones for a fair distribution.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty" protobuf:"bytes,10,opt,name=maxSurge"`
	// MaxUnavailable is the maximum number of machines that can be unavailable during an update.
	// This value is divided by the number of configured zones for a fair distribution.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty" protobuf:"bytes,11,opt,name=maxUnavailable"`
	// ProviderConfig is the provider-specific configuration for this worker pool.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,12,opt,name=providerConfig"`
	// Taints is a list of taints for all the `Node` objects in this worker pool.
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty" protobuf:"bytes,13,rep,name=taints"`
	// Volume contains information about the volume type and size.
	// +optional
	Volume *Volume `json:"volume,omitempty" protobuf:"bytes,14,opt,name=volume"`
	// DataVolumes contains a list of additional worker volumes.
	// +optional
	DataVolumes []DataVolume `json:"dataVolumes,omitempty" protobuf:"bytes,15,rep,name=dataVolumes"`
	// KubeletDataVolumeName contains the name of a dataVolume that should be used for storing kubelet state.
	// +optional
	KubeletDataVolumeName *string `json:"kubeletDataVolumeName,omitempty" protobuf:"bytes,16,opt,name=kubeletDataVolumeName"`
	// Zones is a list of availability zones that are used to evenly distribute this worker pool. Optional
	// as not every provider may support availability zones.
	// +optional
	Zones []string `json:"zones,omitempty" protobuf:"bytes,17,rep,name=zones"`
	// SystemComponents contains configuration for system components related to this worker pool
	// +optional
	SystemComponents *WorkerSystemComponents `json:"systemComponents,omitempty" protobuf:"bytes,18,opt,name=systemComponents"`
	// MachineControllerManagerSettings contains configurations for different worker-pools. Eg. MachineDrainTimeout, MachineHealthTimeout.
	// +optional
	MachineControllerManagerSettings *MachineControllerManagerSettings `json:"machineControllerManager,omitempty" protobuf:"bytes,19,opt,name=machineControllerManager"`
	// Sysctls is a map of kernel settings to apply on all machines in this worker pool.
	// +optional
	Sysctls map[string]string `json:"sysctls,omitempty" protobuf:"bytes,20,rep,name=sysctls"`
	// ClusterAutoscaler contains the cluster autoscaler configurations for the worker pool.
	// +optional
	ClusterAutoscaler *ClusterAutoscalerOptions `json:"clusterAutoscaler,omitempty" protobuf:"bytes,21,opt,name=clusterAutoscaler"`
	// Priority (or weight) is the importance by which this worker group will be scaled by cluster autoscaling.
	// +optional
	Priority *int32 `json:"priority,omitempty" protobuf:"varint,22,opt,name=priority"`
	// UpdateStrategy specifies the machine update strategy for the worker pool.
	// +optional
	UpdateStrategy *MachineUpdateStrategy `json:"updateStrategy,omitempty" protobuf:"bytes,23,opt,name=updateStrategy,casttype=MachineUpdateStrategy"`
	// ControlPlane specifies that the shoot cluster control plane components should be running in this worker pool.
	// This is only relevant for autonomous shoot clusters.
	// +optional
	ControlPlane *WorkerControlPlane `json:"controlPlane,omitempty" protobuf:"bytes,24,opt,name=controlPlane"`
}

// WorkerControlPlane specifies that the shoot cluster control plane components should be running in this worker pool.
type WorkerControlPlane struct{}

// MachineUpdateStrategy specifies the machine update strategy for the worker pool.
type MachineUpdateStrategy string

const (
	// AutoRollingUpdate represents a machine update strategy where nodes are replaced during the update process.
	// This approach involves draining the existing node, deleting it, and creating a new node to replace it.
	AutoRollingUpdate MachineUpdateStrategy = "AutoRollingUpdate"
	// AutoInPlaceUpdate represents a machine update strategy where updates are applied directly to the existing nodes without replacing them.
	// In this approach, nodes are selected automatically by the machine-controller-manager.
	AutoInPlaceUpdate MachineUpdateStrategy = "AutoInPlaceUpdate"
	// ManualInPlaceUpdate represents a machine update strategy where updates are applied directly to the existing nodes without replacing them.
	// In this approach, nodes are selected manually by the user.
	ManualInPlaceUpdate MachineUpdateStrategy = "ManualInPlaceUpdate"
)

// ClusterAutoscalerOptions contains the cluster autoscaler configurations for a worker pool.
type ClusterAutoscalerOptions struct {
	// ScaleDownUtilizationThreshold defines the threshold in fraction (0.0 - 1.0) under which a node is being removed.
	// +optional
	ScaleDownUtilizationThreshold *float64 `json:"scaleDownUtilizationThreshold,omitempty" protobuf:"fixed64,1,opt,name=scaleDownUtilizationThreshold"`
	// ScaleDownGpuUtilizationThreshold defines the threshold in fraction (0.0 - 1.0) of gpu resources under which a node is being removed.
	// +optional
	ScaleDownGpuUtilizationThreshold *float64 `json:"scaleDownGpuUtilizationThreshold,omitempty" protobuf:"fixed64,2,opt,name=scaleDownGpuUtilizationThreshold"`
	// ScaleDownUnneededTime defines how long a node should be unneeded before it is eligible for scale down.
	// +optional
	ScaleDownUnneededTime *metav1.Duration `json:"scaleDownUnneededTime,omitempty" protobuf:"bytes,3,opt,name=scaleDownUnneededTime"`
	// ScaleDownUnreadyTime defines how long an unready node should be unneeded before it is eligible for scale down.
	// +optional
	ScaleDownUnreadyTime *metav1.Duration `json:"scaleDownUnreadyTime,omitempty" protobuf:"bytes,4,opt,name=scaleDownUnreadyTime"`
	// MaxNodeProvisionTime defines how long CA waits for node to be provisioned.
	// +optional
	MaxNodeProvisionTime *metav1.Duration `json:"maxNodeProvisionTime,omitempty" protobuf:"bytes,5,opt,name=maxNodeProvisionTime"`
}

// MachineControllerManagerSettings contains configurations for different worker-pools. Eg. MachineDrainTimeout, MachineHealthTimeout.
type MachineControllerManagerSettings struct {
	// MachineDrainTimeout is the period after which machine is forcefully deleted.
	// +optional
	MachineDrainTimeout *metav1.Duration `json:"machineDrainTimeout,omitempty" protobuf:"bytes,1,name=machineDrainTimeout"`
	// MachineHealthTimeout is the period after which machine is declared failed.
	// +optional
	MachineHealthTimeout *metav1.Duration `json:"machineHealthTimeout,omitempty" protobuf:"bytes,2,name=machineHealthTimeout"`
	// MachineCreationTimeout is the period after which creation of the machine is declared failed.
	// +optional
	MachineCreationTimeout *metav1.Duration `json:"machineCreationTimeout,omitempty" protobuf:"bytes,3,name=machineCreationTimeout"`
	// MaxEvictRetries are the number of eviction retries on a pod after which drain is declared failed, and forceful deletion is triggered.
	// +optional
	MaxEvictRetries *int32 `json:"maxEvictRetries,omitempty" protobuf:"bytes,4,name=maxEvictRetries"`
	// NodeConditions are the set of conditions if set to true for the period of MachineHealthTimeout, machine will be declared failed.
	// +optional
	NodeConditions []string `json:"nodeConditions,omitempty" protobuf:"bytes,5,name=nodeConditions"`
}

// WorkerSystemComponents contains configuration for system components related to this worker pool
type WorkerSystemComponents struct {
	// Allow determines whether the pool should be allowed to host system components or not (defaults to true)
	Allow bool `json:"allow" protobuf:"bytes,1,name=allow"`
}

// WorkerKubernetes contains configuration for Kubernetes components related to this worker pool.
type WorkerKubernetes struct {
	// Kubelet contains configuration settings for all kubelets of this worker pool.
	// If set, all `spec.kubernetes.kubelet` settings will be overwritten for this worker pool (no merge of settings).
	// +optional
	Kubelet *KubeletConfig `json:"kubelet,omitempty" protobuf:"bytes,1,opt,name=kubelet"`
	// Version is the semantic Kubernetes version to use for the Kubelet in this Worker Group.
	// If not specified the kubelet version is derived from the global shoot cluster kubernetes version.
	// version must be equal or lower than the version of the shoot kubernetes version.
	// Only one minor version difference to other worker groups and global kubernetes version is allowed.
	// +optional
	Version *string `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
}

// Machine contains information about the machine type and image.
type Machine struct {
	// Type is the machine type of the worker group.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// Image holds information about the machine image to use for all nodes of this pool. It will default to the
	// latest version of the first image stated in the referenced CloudProfile if no value has been provided.
	// +optional
	Image *ShootMachineImage `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	// Architecture is CPU architecture of machines in this worker pool.
	// +optional
	Architecture *string `json:"architecture,omitempty" protobuf:"bytes,3,opt,name=architecture"`
}

// ShootMachineImage defines the name and the version of the shoot's machine image in any environment. Has to be
// defined in the respective CloudProfile.
type ShootMachineImage struct {
	// Name is the name of the image.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// ProviderConfig is the shoot's individual configuration passed to an extension resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
	// Version is the version of the shoot's image.
	// If version is not provided, it will be defaulted to the latest version from the CloudProfile.
	// +optional
	Version *string `json:"version,omitempty" protobuf:"bytes,3,opt,name=version"`
}

// Volume contains information about the volume type, size, and encryption.
type Volume struct {
	// Name of the volume to make it referenceable.
	// +optional
	Name *string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Type is the type of the volume.
	// +optional
	Type *string `json:"type,omitempty" protobuf:"bytes,2,opt,name=type"`
	// VolumeSize is the size of the volume.
	VolumeSize string `json:"size" protobuf:"bytes,3,opt,name=size"`
	// Encrypted determines if the volume should be encrypted.
	// +optional
	Encrypted *bool `json:"encrypted,omitempty" protobuf:"varint,4,opt,name=encrypted"`
}

// DataVolume contains information about a data volume.
type DataVolume struct {
	// Name of the volume to make it referenceable.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Type is the type of the volume.
	// +optional
	Type *string `json:"type,omitempty" protobuf:"bytes,2,opt,name=type"`
	// VolumeSize is the size of the volume.
	VolumeSize string `json:"size" protobuf:"bytes,3,opt,name=size"`
	// Encrypted determines if the volume should be encrypted.
	// +optional
	Encrypted *bool `json:"encrypted,omitempty" protobuf:"varint,4,opt,name=encrypted"`
}

// CRI contains information about the Container Runtimes.
type CRI struct {
	// The name of the CRI library. Supported values are `containerd`.
	Name CRIName `json:"name" protobuf:"bytes,1,opt,name=name,casttype=CRIName"`
	// ContainerRuntimes is the list of the required container runtimes supported for a worker pool.
	// +optional
	ContainerRuntimes []ContainerRuntime `json:"containerRuntimes,omitempty" protobuf:"bytes,2,rep,name=containerRuntimes"`
}

// CRIName is a type alias for the CRI name string.
type CRIName string

const (
	// CRINameContainerD is a constant for ContainerD CRI name.
	CRINameContainerD CRIName = "containerd"
)

// ContainerRuntime contains information about worker's available container runtime
type ContainerRuntime struct {
	// Type is the type of the Container Runtime.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`

	// ProviderConfig is the configuration passed to container runtime resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
}

// WorkersSettings contains settings for all workers.
type WorkersSettings struct {
	// SSHAccess contains settings regarding ssh access to the worker nodes.
	// +optional
	SSHAccess *SSHAccess `json:"sshAccess,omitempty" protobuf:"bytes,1,opt,name=sshAccess"`
}

// SSHAccess contains settings regarding ssh access to the worker nodes.
type SSHAccess struct {
	// Enabled indicates whether the SSH access to the worker nodes is ensured to be enabled or disabled in systemd.
	// Defaults to true.
	Enabled bool `json:"enabled" protobuf:"varint,1,opt,name=enabled"`
}

var (
	// DefaultWorkerMaxSurge is the default value for Worker MaxSurge.
	DefaultWorkerMaxSurge = intstr.FromInt32(1)
	// DefaultWorkerMaxUnavailable is the default value for Worker MaxUnavailable.
	DefaultWorkerMaxUnavailable = intstr.FromInt32(0)
	// DefaultWorkerSystemComponentsAllow is the default value for Worker AllowSystemComponents
	DefaultWorkerSystemComponentsAllow = true
)

// SystemComponents contains the settings of system components in the control or data plane of the Shoot cluster.
type SystemComponents struct {
	// CoreDNS contains the settings of the Core DNS components running in the data plane of the Shoot cluster.
	// +optional
	CoreDNS *CoreDNS `json:"coreDNS,omitempty" protobuf:"bytes,1,opt,name=coreDNS"`
	// NodeLocalDNS contains the settings of the node local DNS components running in the data plane of the Shoot cluster.
	// +optional
	NodeLocalDNS *NodeLocalDNS `json:"nodeLocalDNS,omitempty" protobuf:"bytes,2,opt,name=nodeLocalDNS"`
}

// CoreDNS contains the settings of the Core DNS components running in the data plane of the Shoot cluster.
type CoreDNS struct {
	// Autoscaling contains the settings related to autoscaling of the Core DNS components running in the data plane of the Shoot cluster.
	// +optional
	Autoscaling *CoreDNSAutoscaling `json:"autoscaling,omitempty" protobuf:"bytes,1,opt,name=autoscaling"`
	// Rewriting contains the setting related to rewriting of requests, which are obviously incorrect due to the unnecessary application of the search path.
	// +optional
	Rewriting *CoreDNSRewriting `json:"rewriting,omitempty" protobuf:"bytes,2,opt,name=rewriting"`
}

// CoreDNSAutoscaling contains the settings related to autoscaling of the Core DNS components running in the data plane of the Shoot cluster.
type CoreDNSAutoscaling struct {
	// The mode of the autoscaling to be used for the Core DNS components running in the data plane of the Shoot cluster.
	// Supported values are `horizontal` and `cluster-proportional`.
	Mode CoreDNSAutoscalingMode `json:"mode" protobuf:"bytes,1,opt,name=mode"`
}

// CoreDNSAutoscalingMode is a type alias for the Core DNS autoscaling mode string.
type CoreDNSAutoscalingMode string

const (
	// CoreDNSAutoscalingModeHorizontal is a constant for horizontal Core DNS autoscaling mode.
	CoreDNSAutoscalingModeHorizontal CoreDNSAutoscalingMode = "horizontal"
	// CoreDNSAutoscalingModeClusterProportional is a constant for cluster-proportional Core DNS autoscaling mode.
	CoreDNSAutoscalingModeClusterProportional CoreDNSAutoscalingMode = "cluster-proportional"
)

// CoreDNSRewriting contains the setting related to rewriting requests, which are obviously incorrect due to the unnecessary application of the search path.
type CoreDNSRewriting struct {
	// CommonSuffixes are expected to be the suffix of a fully qualified domain name. Each suffix should contain at least one or two dots ('.') to prevent accidental clashes.
	// +optional
	CommonSuffixes []string `json:"commonSuffixes,omitempty" protobuf:"bytes,1,rep,name=commonSuffixes"`
}

// NodeLocalDNS contains the settings of the node local DNS components running in the data plane of the Shoot cluster.
type NodeLocalDNS struct {
	// Enabled indicates whether node local DNS is enabled or not.
	Enabled bool `json:"enabled" protobuf:"varint,1,opt,name=enabled"`
	// ForceTCPToClusterDNS indicates whether the connection from the node local DNS to the cluster DNS (Core DNS) will be forced to TCP or not.
	// Default, if unspecified, is to enforce TCP.
	// +optional
	ForceTCPToClusterDNS *bool `json:"forceTCPToClusterDNS,omitempty" protobuf:"varint,2,opt,name=forceTCPToClusterDNS"`
	// ForceTCPToUpstreamDNS indicates whether the connection from the node local DNS to the upstream DNS (infrastructure DNS) will be forced to TCP or not.
	// Default, if unspecified, is to enforce TCP.
	// +optional
	ForceTCPToUpstreamDNS *bool `json:"forceTCPToUpstreamDNS,omitempty" protobuf:"varint,3,opt,name=forceTCPToUpstreamDNS"`
	// DisableForwardToUpstreamDNS indicates whether requests from node local DNS to upstream DNS should be disabled.
	// Default, if unspecified, is to forward requests for external domains to upstream DNS
	// +optional
	DisableForwardToUpstreamDNS *bool `json:"disableForwardToUpstreamDNS,omitempty" protobuf:"varint,4,opt,name=disableForwardToUpstreamDNS"`
}

const (
	// ShootMaintenanceFailed indicates that a shoot maintenance operation failed.
	ShootMaintenanceFailed = "MaintenanceFailed"
	// ShootEventImageVersionMaintenance indicates that a maintenance operation regarding the image version has been performed.
	ShootEventImageVersionMaintenance = "MachineImageVersionMaintenance"
	// ShootEventK8sVersionMaintenance indicates that a maintenance operation regarding the K8s version has been performed.
	ShootEventK8sVersionMaintenance = "KubernetesVersionMaintenance"
	// ShootEventHibernationEnabled indicates that hibernation started.
	ShootEventHibernationEnabled = "Hibernated"
	// ShootEventHibernationDisabled indicates that hibernation ended.
	ShootEventHibernationDisabled = "WokenUp"
	// ShootEventSchedulingSuccessful indicates that a scheduling decision was taken successfully.
	ShootEventSchedulingSuccessful = "SchedulingSuccessful"
	// ShootEventSchedulingFailed indicates that a scheduling decision failed.
	ShootEventSchedulingFailed = "SchedulingFailed"
)

const (
	// ShootAPIServerAvailable is a constant for a condition type indicating that the Shoot cluster's API server is available.
	ShootAPIServerAvailable ConditionType = "APIServerAvailable"
	// ShootControlPlaneHealthy is a constant for a condition type indicating the health of core control plane components.
	ShootControlPlaneHealthy ConditionType = "ControlPlaneHealthy"
	// ShootObservabilityComponentsHealthy is a constant for a condition type indicating the health of observability components.
	ShootObservabilityComponentsHealthy ConditionType = v1beta1constants.ObservabilityComponentsHealthy
	// ShootEveryNodeReady is a constant for a condition type indicating the node health.
	ShootEveryNodeReady ConditionType = "EveryNodeReady"
	// ShootSystemComponentsHealthy is a constant for a condition type indicating the system components health.
	ShootSystemComponentsHealthy ConditionType = "SystemComponentsHealthy"
	// ShootHibernationPossible is a constant for a condition type indicating whether the Shoot can be hibernated.
	ShootHibernationPossible ConditionType = "HibernationPossible"
	// ShootMaintenancePreconditionsSatisfied is a constant for a condition type indicating whether all preconditions
	// for a shoot maintenance operation are satisfied.
	ShootMaintenancePreconditionsSatisfied ConditionType = "MaintenancePreconditionsSatisfied"
	// ShootCACertificateValiditiesAcceptable is a constant for a condition type indicating that the validities of all
	// CA certificates is long enough.
	ShootCACertificateValiditiesAcceptable ConditionType = "CACertificateValiditiesAcceptable"
	// ShootCRDsWithProblematicConversionWebhooks is a constant for a condition type indicating that the Shoot cluster has
	// CRDs with conversion webhooks and multiple stored versions which can break the reconciliation flow of the cluster.
	ShootCRDsWithProblematicConversionWebhooks ConditionType = "CRDsWithProblematicConversionWebhooks"
	// ShootAPIServerProxyUsesHTTPProxy is a constant for a constraint type indicating that the Shoot cluster uses
	// the new HTTP proxy connection method for in-cluster API server traffic (See https://github.com/gardener/gardener/blob/master/docs/proposals/30-apiserver-proxy.md)
	ShootAPIServerProxyUsesHTTPProxy ConditionType = "APIServerProxyUsesHTTPProxy"
	// ShootReadyForMigration is a constant for a condition type indicating whether the Shoot can be migrated.
	ShootReadyForMigration ConditionType = "ReadyForMigration"
)

// ShootPurpose is a type alias for string.
type ShootPurpose string

const (
	// ShootPurposeEvaluation is a constant for the evaluation purpose.
	ShootPurposeEvaluation ShootPurpose = "evaluation"
	// ShootPurposeTesting is a constant for the testing purpose.
	ShootPurposeTesting ShootPurpose = "testing"
	// ShootPurposeDevelopment is a constant for the development purpose.
	ShootPurposeDevelopment ShootPurpose = "development"
	// ShootPurposeProduction is a constant for the production purpose.
	ShootPurposeProduction ShootPurpose = "production"
	// ShootPurposeInfrastructure is a constant for the infrastructure purpose.
	ShootPurposeInfrastructure ShootPurpose = "infrastructure"
)
