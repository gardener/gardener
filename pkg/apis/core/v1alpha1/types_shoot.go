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
	"time"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +genclient
// +genclient:method=CreateAdminKubeconfigRequest,verb=create,subresource=adminkubeconfig,input=github.com/gardener/gardener/pkg/apis/authentication/v1alpha1.AdminKubeconfigRequest,result=github.com/gardener/gardener/pkg/apis/authentication/v1alpha1.AdminKubeconfigRequest
// +genclient:method=UpdateBinding,verb=update,subresource=binding
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

// ShootSpec is the specification of a Shoot.
type ShootSpec struct {
	// Addons contains information about enabled/disabled addons and their configuration.
	// +optional
	Addons *Addons `json:"addons,omitempty" protobuf:"bytes,1,opt,name=addons"`
	// CloudProfileName is a name of a CloudProfile object. This field is immutable.
	CloudProfileName string `json:"cloudProfileName" protobuf:"bytes,2,opt,name=cloudProfileName"`
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
	Networking Networking `json:"networking" protobuf:"bytes,7,opt,name=networking"`
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
	Purpose *ShootPurpose `json:"purpose,omitempty" protobuf:"bytes,11,opt,name=purpose"`
	// Region is a name of a region. This field is immutable.
	Region string `json:"region" protobuf:"bytes,12,opt,name=region"`
	// SecretBindingName is the name of the a SecretBinding that has a reference to the provider secret.
	// The credentials inside the provider secret will be used to create the shoot in the respective account.
	// This field is immutable.
	SecretBindingName string `json:"secretBindingName" protobuf:"bytes,13,opt,name=secretBindingName"`
	// SeedName is the name of the seed cluster that runs the control plane of the Shoot.
	// This field is immutable when the SeedChange feature gate is disabled.
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
}

// ShootStatus holds the most recently observed status of the Shoot cluster.
type ShootStatus struct {
	// Conditions represents the latest available observations of a Shoots's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// Constraints represents conditions of a Shoot's current state that constraint some operations on it.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Constraints []Condition `json:"constraints,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=constraints"`
	// Gardener holds information about the Gardener which last acted on the Shoot.
	Gardener Gardener `json:"gardener" protobuf:"bytes,3,opt,name=gardener"`
	// IsHibernated indicates whether the Shoot is currently hibernated.
	IsHibernated bool `json:"hibernated" protobuf:"varint,4,opt,name=hibernated"`
	// LastOperation holds information about the last operation on the Shoot.
	// +optional
	LastOperation *LastOperation `json:"lastOperation,omitempty" protobuf:"bytes,5,opt,name=lastOperation"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *LastError `json:"lastError,omitempty" protobuf:"bytes,6,opt,name=lastError"`
	// LastErrors holds information about the last occurred error(s) during an operation.
	// +optional
	LastErrors []LastError `json:"lastErrors,omitempty" protobuf:"bytes,7,rep,name=lastErrors"`
	// ObservedGeneration is the most recent generation observed for this Shoot. It corresponds to the
	// Shoot's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,8,opt,name=observedGeneration"`
	// RetryCycleStartTime is the start time of the last retry cycle (used to determine how often an operation
	// must be retried until we give up).
	// +optional
	RetryCycleStartTime *metav1.Time `json:"retryCycleStartTime,omitempty" protobuf:"bytes,9,opt,name=retryCycleStartTime"`
	// Seed is the name of the seed cluster that runs the control plane of the Shoot. This value is only written
	// after a successful create/reconcile operation. It will be used when control planes are moved between Seeds.
	// +optional
	Seed *string `json:"seed,omitempty" protobuf:"bytes,10,opt,name=seed"`
	// TechnicalID is the name that is used for creating the Seed namespace, the infrastructure resources, and
	// basically everything that is related to this particular Shoot. This field is immutable.
	TechnicalID string `json:"technicalID" protobuf:"bytes,11,opt,name=technicalID"`
	// UID is a unique identifier for the Shoot cluster to avoid portability between Kubernetes clusters.
	// It is used to compute unique hashes. This field is immutable.
	UID types.UID `json:"uid" protobuf:"bytes,12,opt,name=uid"`
	// ClusterIdentity is the identity of the Shoot cluster. This field is immutable.
	// +optional
	ClusterIdentity *string `json:"clusterIdentity,omitempty" protobuf:"bytes,13,opt,name=clusterIdentity"`
	// List of addresses on which the Kube API server can be reached.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	AdvertisedAddresses []ShootAdvertisedAddress `json:"advertisedAddresses,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,14,rep,name=advertisedAddresses"`
	// MigrationStartTime is the time when a migration to a different seed was initiated.
	// +optional
	MigrationStartTime *metav1.Time `json:"migrationStartTime,omitempty" protobuf:"bytes,15,opt,name=migrationStartTime"`
	// Credentials contains information about the shoot credentials.
	// +optional
	Credentials *ShootCredentials `json:"credentials,omitempty" protobuf:"bytes,16,opt,name=credentials"`
	// LastHibernationTriggerTime indicates the last time when the hibernation controller
	// managed to change the hibernation settings of the cluster
	// +optional
	LastHibernationTriggerTime *metav1.Time `json:"lastHibernationTriggerTime,omitempty" protobuf:"bytes,17,opt,name=lastHibernationTriggerTime"`
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
	CertificateAuthorities *ShootCARotation `json:"certificateAuthorities,omitempty" protobuf:"bytes,1,opt,name=certificateAuthorities"`
	// Kubeconfig contains information about the kubeconfig credential rotation.
	// +optional
	Kubeconfig *ShootKubeconfigRotation `json:"kubeconfig,omitempty" protobuf:"bytes,2,opt,name=kubeconfig"`
	// SSHKeypair contains information about the ssh-keypair credential rotation.
	// +optional
	SSHKeypair *ShootSSHKeypairRotation `json:"sshKeypair,omitempty" protobuf:"bytes,3,opt,name=sshKeypair"`
	// Observability contains information about the observability credential rotation.
	// +optional
	Observability *ShootObservabilityRotation `json:"observability,omitempty" protobuf:"bytes,4,opt,name=observability"`
	// ServiceAccountKey contains information about the service account key credential rotation.
	// +optional
	ServiceAccountKey *ShootServiceAccountKeyRotation `json:"serviceAccountKey,omitempty" protobuf:"bytes,5,opt,name=serviceAccountKey"`
	// ETCDEncryptionKey contains information about the ETCD encryption key credential rotation.
	// +optional
	ETCDEncryptionKey *ShootETCDEncryptionKeyRotation `json:"etcdEncryptionKey,omitempty" protobuf:"bytes,6,opt,name=etcdEncryptionKey"`
}

// ShootCARotation contains information about the certificate authority credential rotation.
type ShootCARotation struct {
	// Phase describes the phase of the certificate authority credential rotation.
	Phase ShootCredentialsRotationPhase `json:"phase" protobuf:"bytes,1,opt,name=phase"`
	// LastInitiationTime is the most recent time when the certificate authority credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,3,opt,name=lastInitiationTime"`
	// LastCompletionTime is the most recent time when the certificate authority credential rotation was successfully
	// completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
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
	// LastInitiationTime is the most recent time when the certificate authority credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,1,opt,name=lastInitiationTime"`
	// LastCompletionTime is the most recent time when the ssh-keypair credential rotation was successfully completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
}

// ShootObservabilityRotation contains information about the observability credential rotation.
type ShootObservabilityRotation struct {
	// LastInitiationTime is the most recent time when the observability credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,1,opt,name=lastInitiationTime"`
	// LastCompletionTime is the most recent time when the observability credential rotation was successfully completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
}

// ShootServiceAccountKeyRotation contains information about the service account key credential rotation.
type ShootServiceAccountKeyRotation struct {
	// Phase describes the phase of the service account key credential rotation.
	Phase ShootCredentialsRotationPhase `json:"phase" protobuf:"bytes,1,opt,name=phase"`
	// LastInitiationTime is the most recent time when the service account key credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,3,opt,name=lastInitiationTime"`
	// LastCompletionTime is the most recent time when the service account key credential rotation was successfully
	// completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
}

// ShootETCDEncryptionKeyRotation contains information about the ETCD encryption key credential rotation.
type ShootETCDEncryptionKeyRotation struct {
	// Phase describes the phase of the ETCD encryption key credential rotation.
	Phase ShootCredentialsRotationPhase `json:"phase" protobuf:"bytes,1,opt,name=phase"`
	// LastInitiationTime is the most recent time when the ETCD encryption key credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty" protobuf:"bytes,3,opt,name=lastInitiationTime"`
	// LastCompletionTime is the most recent time when the ETCD encryption key credential rotation was successfully
	// completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty" protobuf:"bytes,2,opt,name=lastCompletionTime"`
}

// ShootCredentialsRotationPhase is a string alias.
type ShootCredentialsRotationPhase string

const (
	// RotationPreparing is a constant for the credentials rotation phase describing that the procedure is being prepared.
	RotationPreparing ShootCredentialsRotationPhase = "Preparing"
	// RotationPrepared is a constant for the credentials rotation phase describing that the procedure was prepared.
	RotationPrepared ShootCredentialsRotationPhase = "Prepared"
	// RotationCompleting is a constant for the credentials rotation phase describing that the procedure is being
	// completed.
	RotationCompleting ShootCredentialsRotationPhase = "Completing"
	// RotationCompleted is a constant for the credentials rotation phase describing that the procedure was completed.
	RotationCompleted ShootCredentialsRotationPhase = "Completed"
)

// ShootAdvertisedAddress contains information for the shoot's Kube API server.
type ShootAdvertisedAddress struct {
	// Name of the advertised address. e.g. external
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// The URL of the API Server. e.g. https://api.foo.bar or https://1.2.3.4
	URL string `json:"url" protobuf:"bytes,2,opt,name=url"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Addons relevant types                                                                        //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Addons is a collection of configuration for specific addons which are managed by the Gardener.
type Addons struct {
	// KubernetesDashboard holds configuration settings for the kubernetes dashboard addon.
	// +optional
	KubernetesDashboard *KubernetesDashboard `json:"kubernetes-dashboard,omitempty" protobuf:"bytes,1,opt,name=kubernetesDashboard"`
	// NginxIngress holds configuration settings for the nginx-ingress addon.
	// +optional
	NginxIngress *NginxIngress `json:"nginx-ingress,omitempty" protobuf:"bytes,2,opt,name=nginxIngress"`
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
	// KubernetesDashboardAuthModeBasic uses basic authentication mode for auth.
	KubernetesDashboardAuthModeBasic = "basic"
	// KubernetesDashboardAuthModeToken uses token-based mode for auth.
	KubernetesDashboardAuthModeToken = "token"
)

// NginxIngress describes configuration values for the nginx-ingress addon.
type NginxIngress struct {
	Addon `json:",inline" protobuf:"bytes,4,opt,name=addon"`
	// LoadBalancerSourceRanges is list of allowed IP sources for NginxIngress
	// +optional
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty" protobuf:"bytes,1,rep,name=loadBalancerSourceRanges"`
	// Config contains custom configuration for the nginx-ingress-controller configuration.
	// See https://github.com/kubernetes/ingress-nginx/blob/master/docs/user-guide/nginx-configuration/configmap.md#configuration-options
	// +optional
	Config map[string]string `json:"config,omitempty" protobuf:"bytes,2,rep,name=config"`
	// ExternalTrafficPolicy controls the `.spec.externalTrafficPolicy` value of the load balancer `Service`
	// exposing the nginx-ingress. Defaults to `Cluster`.
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy,omitempty" protobuf:"bytes,3,opt,name=externalTrafficPolicy,casttype=k8s.io/api/core/v1.ServiceExternalTrafficPolicyType"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// ControlPlane relevant types                                                             //
//////////////////////////////////////////////////////////////////////////////////////////////////

// ControlPlane holds information about the general settings for the control plane of a shoot.
type ControlPlane struct {
	// HighAvailability holds the configuration settings for high availability of the
	// control plane of a shoot.
	// +optional
	HighAvailability *HighAvailability `json:"highAvailability,omitempty" protobuf:"bytes,1,name=highAvailability"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// DNS relevant types                                                                           //
//////////////////////////////////////////////////////////////////////////////////////////////////

// DNS holds information about the provider, the hosted zone id and the domain.
type DNS struct {
	// Domain is the external available domain of the Shoot cluster. This domain will be written into the
	// kubeconfig that is handed out to end-users. This field is immutable.
	// +optional
	Domain *string `json:"domain,omitempty" protobuf:"bytes,1,opt,name=domain"`
	// Providers is a list of DNS providers that shall be enabled for this shoot cluster. Only relevant if
	// not a default domain is used.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Providers []DNSProvider `json:"providers,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=providers"`
}

// DNSProvider contains information about a DNS provider.
type DNSProvider struct {
	// Domains contains information about which domains shall be included/excluded for this provider.
	// +optional
	Domains *DNSIncludeExclude `json:"domains,omitempty" protobuf:"bytes,1,opt,name=domains"`
	// Primary indicates that this DNSProvider is used for shoot related domains.
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

//////////////////////////////////////////////////////////////////////////////////////////////////
// Extension relevant types                                                                     //
//////////////////////////////////////////////////////////////////////////////////////////////////

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

//////////////////////////////////////////////////////////////////////////////////////////////////
// NamedResourceReference relevant types                                                        //
//////////////////////////////////////////////////////////////////////////////////////////////////

// NamedResourceReference is a named reference to a resource.
type NamedResourceReference struct {
	// Name of the resource reference.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// ResourceRef is a reference to a resource.
	ResourceRef autoscalingv1.CrossVersionObjectReference `json:"resourceRef" protobuf:"bytes,2,opt,name=resourceRef"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Hibernation relevant types                                                                   //
//////////////////////////////////////////////////////////////////////////////////////////////////

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
	// Location is the time location in which both start and and shall be evaluated.
	// +optional
	Location *string `json:"location,omitempty" protobuf:"bytes,3,opt,name=location"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Kubernetes relevant types                                                                    //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Kubernetes contains the version and configuration variables for the Shoot control plane.
type Kubernetes struct {
	// AllowPrivilegedContainers indicates whether privileged containers are allowed in the Shoot.
	// Defaults to true for Kubernetes versions below v1.25. Unusable for Kubernetes versions v1.25 and higher.
	// +optional
	AllowPrivilegedContainers *bool `json:"allowPrivilegedContainers,omitempty" protobuf:"varint,1,opt,name=allowPrivilegedContainers"`
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
	// Version is the semantic Kubernetes version to use for the Shoot cluster.
	Version string `json:"version" protobuf:"bytes,8,opt,name=version"`
	// VerticalPodAutoscaler contains the configuration flags for the Kubernetes vertical pod autoscaler.
	// +optional
	VerticalPodAutoscaler *VerticalPodAutoscaler `json:"verticalPodAutoscaler,omitempty" protobuf:"bytes,9,opt,name=verticalPodAutoscaler"`
	// EnableStaticTokenKubeconfig indicates whether static token kubeconfig secret should be present in garden cluster
	// (default: true).
	// +optional
	EnableStaticTokenKubeconfig *bool `json:"enableStaticTokenKubeconfig,omitempty" protobuf:"varint,10,opt,name=enableStaticTokenKubeconfig"`
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
	// ScaleDownUtilizationThreshold defines the threshold in decimal (0.0 - 1.0) under which a node is being removed (default: 0.5).
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
	// +optional
	IgnoreTaints []string `json:"ignoreTaints,omitempty" protobuf:"bytes,10,opt,name=ignoreTaints"`
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
)

var (
	// DefaultEvictAfterOOMThreshold is the default value for the EvictAfterOOMThreshold field in the VPA configuration.
	DefaultEvictAfterOOMThreshold = metav1.Duration{Duration: 10 * time.Minute}
	// DefaultUpdaterInterval is the default value for the UpdaterInterval field in the VPA configuration.
	DefaultUpdaterInterval = metav1.Duration{Duration: time.Minute}
	// DefaultRecommenderInterval is the default value for the RecommenderInterval field in the VPA configuration.
	DefaultRecommenderInterval = metav1.Duration{Duration: time.Minute}
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
	// EnableBasicAuthentication defines whether basic authentication should be enabled for this cluster or not.
	// +optional
	EnableBasicAuthentication *bool `json:"enableBasicAuthentication,omitempty" protobuf:"varint,5,opt,name=enableBasicAuthentication"`
	// OIDCConfig contains configuration settings for the OIDC provider.
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
	Requests *KubeAPIServerRequests `json:"requests,omitempty" protobuf:"bytes,10,opt,name=requests"`
	// EnableAnonymousAuthentication defines whether anonymous requests to the secure port
	// of the API server should be allowed (flag `--anonymous-auth`).
	// See: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/
	// +optional
	EnableAnonymousAuthentication *bool `json:"enableAnonymousAuthentication,omitempty" protobuf:"varint,11,opt,name=enableAnonymousAuthentication"`
	// EventTTL controls the amount of time to retain events.
	// Defaults to 1h.
	// +optional
	EventTTL *metav1.Duration `json:"eventTTL,omitempty" protobuf:"bytes,12,opt,name=eventTTL"`
}

// KubeAPIServerRequests contains configuration for request-specific settings for the kube-apiserver.
type KubeAPIServerRequests struct {
	// MaxNonMutatingInflight is the maximum number of non-mutating requests in flight at a given time. When the server
	// exceeds this, it rejects requests.
	// +optional
	MaxNonMutatingInflight *int32 `json:"maxNonMutatingInflight,omitempty" protobuf:"bytes,1,name=maxNonMutatingInflight"`
	// MaxMutatingInflight is the maximum number of mutating requests in flight at a given time. When the server
	// exceeds this, it rejects requests.
	// +optional
	MaxMutatingInflight *int32 `json:"maxMutatingInflight,omitempty" protobuf:"bytes,2,name=maxMutatingInflight"`
}

// ServiceAccountConfig is the kube-apiserver configuration for service accounts.
type ServiceAccountConfig struct {
	// Issuer is the identifier of the service account token issuer. The issuer will assert this
	// identifier in "iss" claim of issued tokens. This value is used to generate new service account tokens.
	// This value is a string or URI. Defaults to URI of the API server.
	// +optional
	Issuer *string `json:"issuer,omitempty" protobuf:"bytes,1,opt,name=issuer"`
	// SigningKeySecret is a reference to a secret that contains an optional private key of the
	// service account token issuer. The issuer will sign issued ID tokens with this private key.
	// Only useful if service account tokens are also issued by another external system.
	// Deprecated: This field is deprecated and will be removed in a future version of Gardener. Do not use it.
	// +optional
	SigningKeySecret *corev1.LocalObjectReference `json:"signingKeySecretName,omitempty" protobuf:"bytes,2,opt,name=signingKeySecretName"`
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
	// This field is only available for Kubernetes v1.22 or later.
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

// OIDCConfig contains configuration settings for the OIDC provider.
// Note: Descriptions were taken from the Kubernetes documentation.
type OIDCConfig struct {
	// If set, the OpenID server's certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host's root CA set will be used.
	// +optional
	CABundle *string `json:"caBundle,omitempty" protobuf:"bytes,1,opt,name=caBundle"`
	// ClientAuthentication can optionally contain client configuration used for kubeconfig generation.
	// +optional
	ClientAuthentication *OpenIDConnectClientAuthentication `json:"clientAuthentication,omitempty" protobuf:"bytes,2,opt,name=clientAuthentication"`
	// The client ID for the OpenID Connect client, must be set if oidc-issuer-url is set.
	// +optional
	ClientID *string `json:"clientID,omitempty" protobuf:"bytes,3,opt,name=clientID"`
	// If provided, the name of a custom OpenID Connect claim for specifying user groups. The claim value is expected to be a string or array of strings. This flag is experimental, please see the authentication documentation for further details.
	// +optional
	GroupsClaim *string `json:"groupsClaim,omitempty" protobuf:"bytes,4,opt,name=groupsClaim"`
	// If provided, all groups will be prefixed with this value to prevent conflicts with other authentication strategies.
	// +optional
	GroupsPrefix *string `json:"groupsPrefix,omitempty" protobuf:"bytes,5,opt,name=groupsPrefix"`
	// The URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT).
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
	// ImagePullProgressDeadline describes the time limit under which if no pulling progress is made, the image pulling will be cancelled.
	// +optional
	// Default: 1m
	ImagePullProgressDeadline *metav1.Duration `json:"imagePullProgressDeadline,omitempty" protobuf:"bytes,12,opt,name=imagePullProgressDeadline"`
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
	// This requires the corresponding SeccompDefault feature gate to be enabled as well.
	// This field is only available for Kubernetes v1.25 or later.
	// +optional
	SeccompDefault *bool `json:"seccompDefault,omitempty" protobuf:"varint,21,opt,name=seccompDefault"`
	// ContainerLogMaxSize defines the maximum size of the container log file before it is rotated. For example: "5Mi" or "256Ki".
	// Default: 100Mi
	// +optional
	ContainerLogMaxSize *resource.Quantity `json:"containerLogMaxSize,omitempty" protobuf:"bytes,22,opt,name=containerLogMaxSize"`
	// ContainerLogMaxFiles is the maximum number of container log files that can be present for a container.
	// +optional
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty" protobuf:"bytes,23,opt,name=containerLogMaxFiles"`
	// ProtectKernelDefaults ensures that the kernel tunables are equal to the kubelet defaults.
	// Defaults to true for Kubernetes v1.26 or later.
	// +optional
	ProtectKernelDefaults *bool `json:"protectKernelDefaults,omitempty" protobuf:"varint,24,opt,name=protectKernelDefaults"`
	// StreamingConnectionIdleTimeout is the maximum time a streaming connection can be idle before the connection is automatically closed.
	// This field cannot be set lower than "30s" or greater than "4h".
	// Default:
	//  "4h" for Kubernetes < v1.26.
	//  "5m" for Kubernetes >= v1.26.
	// +optional
	StreamingConnectionIdleTimeout *metav1.Duration `json:"streamingConnectionIdleTimeout,omitempty" protobuf:"bytes,25,opt,name=streamingConnectionIdleTimeout"`
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

//////////////////////////////////////////////////////////////////////////////////////////////////
// Networking relevant types                                                                    //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Networking defines networking parameters for the shoot cluster.
type Networking struct {
	// Type identifies the type of the networking plugin. This field is immutable.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// ProviderConfig is the configuration passed to network resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
	// Pods is the CIDR of the pod network. This field is immutable.
	// +optional
	Pods *string `json:"pods,omitempty" protobuf:"bytes,3,opt,name=pods"`
	// Nodes is the CIDR of the entire node network. This field is immutable.
	// +optional
	Nodes *string `json:"nodes,omitempty" protobuf:"bytes,4,opt,name=nodes"`
	// Services is the CIDR of the service network. This field is immutable.
	// +optional
	Services *string `json:"services,omitempty" protobuf:"bytes,5,opt,name=services"`
}

const (
	// DefaultPodNetworkCIDR is a constant for the default pod network CIDR of a Shoot cluster.
	DefaultPodNetworkCIDR = "100.96.0.0/11"
	// DefaultServiceNetworkCIDR is a constant for the default service network CIDR of a Shoot cluster.
	DefaultServiceNetworkCIDR = "100.64.0.0/13"
)

//////////////////////////////////////////////////////////////////////////////////////////////////
// Maintenance relevant types                                                                   //
//////////////////////////////////////////////////////////////////////////////////////////////////

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
	MachineImageVersion bool `json:"machineImageVersion" protobuf:"varint,2,opt,name=machineImageVersion"`
}

// MaintenanceTimeWindow contains information about the time window for maintenance operations.
type MaintenanceTimeWindow struct {
	// Begin is the beginning of the time window in the format HHMMSS+ZONE, e.g. "220000+0100".
	// If not present, a random value will be computed.
	Begin string `json:"begin" protobuf:"bytes,1,opt,name=begin"`
	// End is the end of the time window in the format HHMMSS+ZONE, e.g. "220000+0100".
	// If not present, the value will be computed based on the "Begin" value.
	End string `json:"end" protobuf:"bytes,2,opt,name=end"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Monitoring relevant types                                                                    //
//////////////////////////////////////////////////////////////////////////////////////////////////

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

//////////////////////////////////////////////////////////////////////////////////////////////////
// Provider relevant types                                                                      //
//////////////////////////////////////////////////////////////////////////////////////////////////

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
	Workers []Worker `json:"workers" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,4,rep,name=workers"`
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
	// Defaults to a CRI with name `containerd` when the Kubernetes version of the `Shoot` is >= 1.22.
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
	// Maximum is the maximum number of VMs to create.
	Maximum int32 `json:"maximum" protobuf:"varint,8,opt,name=maximum"`
	// Minimum is the minimum number of VMs to create.
	Minimum int32 `json:"minimum" protobuf:"varint,9,opt,name=minimum"`
	// MaxSurge is maximum number of VMs that are created during an update.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty" protobuf:"bytes,10,opt,name=maxSurge"`
	// MaxUnavailable is the maximum number of VMs that can be unavailable during an update.
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
	// Architecture is the CPU architecture of the machines in this worker pool.
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

// Volume contains information about the volume type and size.
type Volume struct {
	// Name of the volume to make it referencable.
	// +optional
	Name *string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Type is the type of the volume.
	// +optional
	Type *string `json:"type,omitempty" protobuf:"bytes,2,opt,name=type"`
	// Size is the size of the volume.
	VolumeSize string `json:"size" protobuf:"bytes,3,opt,name=size"`
	// Encrypted determines if the volume should be encrypted.
	// +optional
	Encrypted *bool `json:"encrypted,omitempty" protobuf:"varint,4,opt,name=primary"`
}

// DataVolume contains information about a data volume.
type DataVolume struct {
	// Name of the volume to make it referencable.
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
	// The name of the CRI library. Supported values are `docker` and `containerd`.
	Name CRIName `json:"name" protobuf:"bytes,1,opt,name=name"`
	// ContainerRuntimes is the list of the required container runtimes supported for a worker pool.
	// +optional
	ContainerRuntimes []ContainerRuntime `json:"containerRuntimes,omitempty" protobuf:"bytes,2,rep,name=containerRuntimes"`
}

// CRIName is a type alias for the CRI name string.
type CRIName string

const (
	// CRINameContainerD is a constant for ContainerD CRI name.
	CRINameContainerD CRIName = "containerd"
	// CRINameDocker is a constant for Docker CRI name.
	CRINameDocker CRIName = "docker"
)

// ContainerRuntime contains information about worker's available container runtime
type ContainerRuntime struct {
	// Type is the type of the Container Runtime.
	Type string `json:"type" protobuf:"bytes,1,opt,name=name"`
	// ProviderConfig is the configuration passed to the ContainerRuntime resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
}

var (
	// DefaultWorkerMaxSurge is the default value for Worker MaxSurge.
	DefaultWorkerMaxSurge = intstr.FromInt(1)
	// DefaultWorkerMaxUnavailable is the default value for Worker MaxUnavailable.
	DefaultWorkerMaxUnavailable = intstr.FromInt(0)
	// DefaultWorkerSystemComponentsAllow is the default value for Worker AllowSystemComponents
	DefaultWorkerSystemComponentsAllow = true
)

//////////////////////////////////////////////////////////////////////////////////////////////////
// System components relevant types                                                             //
//////////////////////////////////////////////////////////////////////////////////////////////////

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
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Other/miscellaneous constants and types                                                      //
//////////////////////////////////////////////////////////////////////////////////////////////////

const (
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
	// ShootControlPlaneHealthy is a constant for a condition type indicating the control plane health.
	ShootControlPlaneHealthy ConditionType = "ControlPlaneHealthy"
	// ShootEveryNodeReady is a constant for a condition type indicating the node health.
	ShootEveryNodeReady ConditionType = "EveryNodeReady"
	// ShootSystemComponentsHealthy is a constant for a condition type indicating the system components health.
	ShootSystemComponentsHealthy ConditionType = "SystemComponentsHealthy"
	// ShootHibernationPossible is a constant for a condition type indicating whether the Shoot can be hibernated.
	ShootHibernationPossible ConditionType = "HibernationPossible"
	// ShootMaintenancePreconditionsSatisfied is a constant for a condition type indicating whether all preconditions
	// for a shoot maintenance operation are satisfied.
	ShootMaintenancePreconditionsSatisfied ConditionType = "MaintenancePreconditionsSatisfied"
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
