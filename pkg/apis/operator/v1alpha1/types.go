// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	// Networking defines the networking configuration of the runtime cluster.
	Networking RuntimeNetworking `json:"networking"`
	// Provider defines the provider-specific information for this cluster.
	Provider Provider `json:"provider"`
	// Settings contains certain settings for this cluster.
	// +optional
	Settings *Settings `json:"settings,omitempty"`
}

// RuntimeNetworking defines the networking configuration of the runtime cluster.
type RuntimeNetworking struct {
	// Nodes is the CIDR of the node network. This field is immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +optional
	Nodes *string `json:"nodes,omitempty"`
	// Pods is the CIDR of the pod network. This field is immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Pods string `json:"pods"`
	// Services is the CIDR of the service network. This field is immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Services string `json:"services"`
	// BlockCIDRs is a list of network addresses that should be blocked.
	// +optional
	BlockCIDRs []string `json:"blockCIDRs,omitempty"`
}

// Provider defines the provider-specific information for this cluster.
type Provider struct {
	// Zones is the list of availability zones the cluster is deployed to.
	// +optional
	Zones []string `json:"zones,omitempty"`
}

// Settings contains certain settings for this cluster.
type Settings struct {
	// LoadBalancerServices controls certain settings for services of type load balancer that are created in the runtime
	// cluster.
	// +optional
	LoadBalancerServices *SettingLoadBalancerServices `json:"loadBalancerServices,omitempty"`
	// VerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
	// cluster.
	// +optional
	VerticalPodAutoscaler *SettingVerticalPodAutoscaler `json:"verticalPodAutoscaler,omitempty"`
	// TopologyAwareRouting controls certain settings for topology-aware traffic routing in the cluster.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/topology_aware_routing.md.
	// +optional
	TopologyAwareRouting *SettingTopologyAwareRouting `json:"topologyAwareRouting,omitempty"`
}

// SettingLoadBalancerServices controls certain settings for services of type load balancer that are created in the
// runtime cluster.
type SettingLoadBalancerServices struct {
	// Annotations is a map of annotations that will be injected/merged into every load balancer service object.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
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

// SettingTopologyAwareRouting controls certain settings for topology-aware traffic routing in the cluster.
// See https://github.com/gardener/gardener/blob/master/docs/usage/topology_aware_routing.md.
type SettingTopologyAwareRouting struct {
	// Enabled controls whether certain Services deployed in the cluster should be topology-aware.
	// These Services are virtual-garden-etcd-main-client, virtual-garden-etcd-events-client and virtual-garden-kube-apiserver.
	// Additionally, other components that are deployed to the runtime cluster via other means can read this field and
	// according to its value enable/disable topology-aware routing for their Services.
	Enabled bool `json:"enabled"`
}

// VirtualCluster contains configuration for the virtual cluster.
type VirtualCluster struct {
	// ControlPlane holds information about the general settings for the control plane of the virtual cluster.
	// +optional
	ControlPlane *ControlPlane `json:"controlPlane,omitempty"`
	// DNS holds information about DNS settings.
	DNS DNS `json:"dns"`
	// ETCD contains configuration for the etcds of the virtual garden cluster.
	// +optional
	ETCD *ETCD `json:"etcd,omitempty"`
	// Kubernetes contains the version and configuration options for the Kubernetes components of the virtual garden
	// cluster.
	Kubernetes Kubernetes `json:"kubernetes"`
	// Maintenance contains information about the time window for maintenance operations.
	Maintenance Maintenance `json:"maintenance"`
	// Networking contains information about cluster networking such as CIDRs, etc.
	Networking Networking `json:"networking"`
}

// DNS holds information about DNS settings.
type DNS struct {
	// Deprecated: This field is deprecated and will be removed soon. Please use `Domains` instead.
	// TODO(timuthy): Drop this after v1.74 has been released.
	// +optional
	Domain *string `json:"domain,omitempty"`
	// Domains are the external domains of the virtual garden cluster.
	// The first given domain in this list is immutable.
	// +optional
	Domains []string `json:"domains,omitempty"`
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
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Provider is immutable"
	Provider string `json:"provider"`
	// BucketName is the name of the backup bucket.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="BucketName is immutable"
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

// ControlPlane holds information about the general settings for the control plane of the virtual garden cluster.
type ControlPlane struct {
	// HighAvailability holds the configuration settings for high availability settings.
	// +optional
	HighAvailability *HighAvailability `json:"highAvailability,omitempty"`
}

// HighAvailability specifies the configuration settings for high availability for a resource.
type HighAvailability struct{}

// Kubernetes contains the version and configuration options for the Kubernetes components of the virtual garden
// cluster.
type Kubernetes struct {
	// KubeAPIServer contains configuration settings for the kube-apiserver.
	// +optional
	KubeAPIServer *KubeAPIServerConfig `json:"kubeAPIServer,omitempty"`
	// KubeControllerManager contains configuration settings for the kube-controller-manager.
	// +optional
	KubeControllerManager *KubeControllerManagerConfig `json:"kubeControllerManager,omitempty"`
	// Version is the semantic Kubernetes version to use for the virtual garden cluster.
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`
}

// KubeAPIServerConfig contains configuration settings for the kube-apiserver.
type KubeAPIServerConfig struct {
	// KubeAPIServerConfig contains all configuration values not specific to the virtual garden cluster.
	// +optional
	*gardencorev1beta1.KubeAPIServerConfig `json:",inline"`
	// AuditWebhook contains settings related to an audit webhook configuration.
	// +optional
	AuditWebhook *AuditWebhook `json:"auditWebhook,omitempty"`
	// Authentication contains settings related to authentication.
	// +optional
	Authentication *Authentication `json:"authentication,omitempty"`
	// Authorization contains settings related to authorization.
	// +optional
	Authorization *Authorization `json:"authorization,omitempty"`
	// ResourcesToStoreInETCDEvents contains a list of resources which should be stored in etcd-events instead of
	// etcd-main. The 'events' resource is always stored in etcd-events. Note that adding or removing resources from
	// this list will not migrate them automatically from the etcd-main to etcd-events or vice versa.
	// +optional
	ResourcesToStoreInETCDEvents []GroupResource `json:"resourcesToStoreInETCDEvents,omitempty"`
	// SNI contains configuration options for the TLS SNI settings.
	// +optional
	SNI *SNI `json:"sni,omitempty"`
}

// AuditWebhook contains settings related to an audit webhook configuration.
type AuditWebhook struct {
	// BatchMaxSize is the maximum size of a batch.
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	// +optional
	BatchMaxSize *int32 `json:"batchMaxSize,omitempty"`
	// KubeconfigSecretName specifies the name of a secret containing the kubeconfig for this webhook.
	// +kubebuilder:validation:MinLength=1
	KubeconfigSecretName string `json:"kubeconfigSecretName"`
	// Version is the API version to send and expect from the webhook.
	// +kubebuilder:default=audit.k8s.io/v1
	// +kubebuilder:validation:Enum=audit.k8s.io/v1
	// +optional
	Version *string `json:"version,omitempty"`
}

// Authentication contains settings related to authentication.
type Authentication struct {
	// Webhook contains settings related to an authentication webhook configuration.
	// +optional
	Webhook *AuthenticationWebhook `json:"webhook,omitempty"`
}

// AuthenticationWebhook contains settings related to an authentication webhook configuration.
type AuthenticationWebhook struct {
	// CacheTTL is the duration to cache responses from the webhook authenticator.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	// +optional
	CacheTTL *metav1.Duration `json:"cacheTTL,omitempty"`
	// KubeconfigSecretName specifies the name of a secret containing the kubeconfig for this webhook.
	// +kubebuilder:validation:MinLength=1
	KubeconfigSecretName string `json:"kubeconfigSecretName"`
	// Version is the API version to send and expect from the webhook.
	// +kubebuilder:default=v1beta1
	// +kubebuilder:validation:Enum=v1alpha1;v1beta1;v1
	// +optional
	Version *string `json:"version,omitempty"`
}

// Authorization contains settings related to authorization.
type Authorization struct {
	// Webhook contains settings related to an authorization webhook configuration.
	// +optional
	Webhook *AuthorizationWebhook `json:"webhook,omitempty"`
}

// AuthorizationWebhook contains settings related to an authorization webhook configuration.
type AuthorizationWebhook struct {
	// CacheAuthorizedTTL is the duration to cache 'authorized' responses from the webhook authorizer.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	// +optional
	CacheAuthorizedTTL *metav1.Duration `json:"cacheAuthorizedTTL,omitempty"`
	// CacheUnauthorizedTTL is the duration to cache 'unauthorized' responses from the webhook authorizer.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	// +optional
	CacheUnauthorizedTTL *metav1.Duration `json:"cacheUnauthorizedTTL,omitempty"`
	// KubeconfigSecretName specifies the name of a secret containing the kubeconfig for this webhook.
	// +kubebuilder:validation:MinLength=1
	KubeconfigSecretName string `json:"kubeconfigSecretName"`
	// Version is the API version to send and expect from the webhook.
	// +kubebuilder:default=v1beta1
	// +kubebuilder:validation:Enum=v1beta1;v1
	// +optional
	Version *string `json:"version,omitempty"`
}

// GroupResource contains a list of resources which should be stored in etcd-events instead of etcd-main.
type GroupResource struct {
	// Group is the API group name.
	// +kubebuilder:validation:MinLength=1
	Group string `json:"group"`
	// Resource is the resource name.
	// +kubebuilder:validation:MinLength=1
	Resource string `json:"resource"`
}

// SNI contains configuration options for the TLS SNI settings.
type SNI struct {
	// SecretName is the name of a secret containing the TLS certificate and private key.
	// +kubebuilder:validation:MinLength=1
	SecretName string `json:"secretName"`
	// DomainPatterns is a list of fully qualified domain names, possibly with prefixed wildcard segments. The domain
	// patterns also allow IP addresses, but IPs should only be used if the apiserver has visibility to the IP address
	// requested by a client. If no domain patterns are provided, the names of the certificate are extracted.
	// Non-wildcard matches trump over wildcard matches, explicit domain patterns trump over extracted names.
	// +optional
	DomainPatterns []string `json:"domainPatterns,omitempty"`
}

// Networking defines networking parameters for the virtual garden cluster.
type Networking struct {
	// Services is the CIDR of the service network. This field is immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Services string `json:"services"`
}

// KubeControllerManagerConfig contains configuration settings for the kube-controller-manager.
type KubeControllerManagerConfig struct {
	// KubeControllerManagerConfig contains all configuration values not specific to the virtual garden cluster.
	// +optional
	*gardencorev1beta1.KubeControllerManagerConfig `json:",inline"`
	// CertificateSigningDuration is the maximum length of duration signed certificates will be given. Individual CSRs
	// may request shorter certs by setting `spec.expirationSeconds`.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	// +kubebuilder:default=`48h`
	// +optional
	CertificateSigningDuration *metav1.Duration `json:"certificateSigningDuration,omitempty"`
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
	// ServiceAccountKey contains information about the service account key credential rotation.
	// +optional
	ServiceAccountKey *gardencorev1beta1.ServiceAccountKeyRotation `json:"serviceAccountKey,omitempty"`
	// ETCDEncryptionKey contains information about the ETCD encryption key credential rotation.
	// +optional
	ETCDEncryptionKey *gardencorev1beta1.ETCDEncryptionKeyRotation `json:"etcdEncryptionKey,omitempty"`
}

const (
	// GardenReconciled is a constant for a condition type indicating that the garden has been reconciled.
	GardenReconciled gardencorev1beta1.ConditionType = "Reconciled"
)

// AvailableOperationAnnotations is the set of available operation annotations for Garden resources.
var AvailableOperationAnnotations = sets.New(
	v1beta1constants.GardenerOperationReconcile,
	v1beta1constants.OperationRotateCAStart,
	v1beta1constants.OperationRotateCAComplete,
	v1beta1constants.OperationRotateServiceAccountKeyStart,
	v1beta1constants.OperationRotateServiceAccountKeyComplete,
	v1beta1constants.OperationRotateETCDEncryptionKeyStart,
	v1beta1constants.OperationRotateETCDEncryptionKeyComplete,
	v1beta1constants.OperationRotateCredentialsStart,
	v1beta1constants.OperationRotateCredentialsComplete,
)
