// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,shortName="grdn"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="K8S Version",type=string,JSONPath=`.spec.virtualCluster.kubernetes.version`,description="Kubernetes version of virtual cluster."
// +kubebuilder:printcolumn:name="Gardener Version",type=string,JSONPath=`.status.gardener.version`,description="Version of the Gardener components."
// +kubebuilder:printcolumn:name="Last Operation",type=string,JSONPath=`.status.lastOperation.state`,description="Status of the last operation"
// +kubebuilder:printcolumn:name="Runtime",type=string,JSONPath=`.status.conditions[?(@.type=="RuntimeComponentsHealthy")].status`,description="Indicates whether the components related to the runtime cluster are healthy."
// +kubebuilder:printcolumn:name="Virtual",type=string,JSONPath=`.status.conditions[?(@.type=="VirtualComponentsHealthy")].status`,description="Indicates whether the components related to the virtual cluster are healthy."
// +kubebuilder:printcolumn:name="API Server",type=string,JSONPath=`.status.conditions[?(@.type=="VirtualGardenAPIServerAvailable")].status`,description="Indicates whether the API server of the virtual cluster is available."
// +kubebuilder:printcolumn:name="Observability",type=string,JSONPath=`.status.conditions[?(@.type=="ObservabilityComponentsHealthy")].status`,description="Indicates whether the observability components related to the runtime cluster are healthy."
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
	// DNS contains specifications of DNS providers.
	// +optional
	DNS *DNSManagement `json:"dns,omitempty"`
	// Extensions contain type and provider information for Garden extensions.
	// +optional
	Extensions []GardenExtension `json:"extensions,omitempty"`
	// RuntimeCluster contains configuration for the runtime cluster.
	RuntimeCluster RuntimeCluster `json:"runtimeCluster"`
	// VirtualCluster contains configuration for the virtual cluster.
	VirtualCluster VirtualCluster `json:"virtualCluster"`
}

// DNSManagement contains specifications of DNS providers.
type DNSManagement struct {
	// Providers is a list of DNS providers.
	// +kubebuilder:validation:MinItems=1
	Providers []DNSProvider `json:"providers"`
}

// DNSProvider contains the configuration for a DNS provider.
type DNSProvider struct {
	// Name is the name of the DNS provider.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Type is the type of the DNS provider.
	// +kubebuilder:validation:MinLength=1
	Type string `json:"type"`
	// Config is the provider-specific configuration passed to DNSRecord resources.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty"`
	// SecretRef is a reference to a Secret object containing the DNS provider credentials.
	SecretRef corev1.LocalObjectReference `json:"secretRef"`
}

// RuntimeCluster contains configuration for the runtime cluster.
type RuntimeCluster struct {
	// Ingress configures Ingress specific settings for the Garden cluster.
	Ingress Ingress `json:"ingress"`
	// Networking defines the networking configuration of the runtime cluster.
	Networking RuntimeNetworking `json:"networking"`
	// Provider defines the provider-specific information for this cluster.
	Provider Provider `json:"provider"`
	// Settings contains certain settings for this cluster.
	// +optional
	Settings *Settings `json:"settings,omitempty"`
	// Volume contains settings for persistent volumes created in the runtime cluster.
	// +optional
	Volume *Volume `json:"volume,omitempty"`
}

// Ingress configures the Ingress specific settings of the runtime cluster.
type Ingress struct {
	// Domains specify the ingress domains of the cluster pointing to the ingress controller endpoint. They will be used
	// to construct ingress URLs for system applications running in runtime cluster.
	// +kubebuilder:validation:MinItems=1
	Domains []DNSDomain `json:"domains,omitempty"`
	// Controller configures a Gardener managed Ingress Controller listening on the ingressDomain.
	Controller gardencorev1beta1.IngressController `json:"controller"`
}

// DNSDomain defines a DNS domain with optional provider.
type DNSDomain struct {
	// Name is the domain name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Provider is the name of the DNS provider as declared in the '.spec.dns.providers' section.
	// It is only optional, if the `.spec.dns` section is not provided at all.
	// +optional
	Provider *string `json:"provider,omitempty"`
}

// RuntimeNetworking defines the networking configuration of the runtime cluster.
type RuntimeNetworking struct {
	// Nodes are the CIDRs of the node network. Elements can be appended to this list, but not removed.
	// +optional
	Nodes []string `json:"nodes,omitempty"`
	// Pods are the CIDRs of the pod network. Elements can be appended to this list, but not removed.
	// +kubebuilder:validation:MinItems=1
	Pods []string `json:"pods"`
	// Services are the CIDRs of the service network. Elements can be appended to this list, but not removed.
	// +kubebuilder:validation:MinItems=1
	Services []string `json:"services"`
	// BlockCIDRs is a list of network addresses that should be blocked.
	// +optional
	BlockCIDRs []string `json:"blockCIDRs,omitempty"`
}

// Provider defines the provider-specific information for this cluster.
type Provider struct {
	// Region is the region the cluster is deployed to.
	// +optional
	Region *string `json:"region,omitempty"`
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
	// See https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md.
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
// See https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md.
type SettingTopologyAwareRouting struct {
	// Enabled controls whether certain Services deployed in the cluster should be topology-aware.
	// These Services are virtual-garden-etcd-main-client, virtual-garden-etcd-events-client and virtual-garden-kube-apiserver.
	// Additionally, other components that are deployed to the runtime cluster via other means can read this field and
	// according to its value enable/disable topology-aware routing for their Services.
	Enabled bool `json:"enabled"`
}

// Volume contains settings for persistent volumes created in the runtime cluster.
type Volume struct {
	// MinimumSize defines the minimum size that should be used for PVCs in the runtime cluster.
	// +optional
	MinimumSize *resource.Quantity `json:"minimumSize,omitempty"`
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
	// Gardener contains the configuration options for the Gardener control plane components.
	Gardener Gardener `json:"gardener"`
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
	// Domains are the external domains of the virtual garden cluster.
	// The first given domain in this list is immutable.
	// +kubebuilder:validation:MinItems=1
	Domains []DNSDomain `json:"domains,omitempty"`
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
	// Autoscaling contains auto-scaling configuration options for etcd.
	// +optional
	Autoscaling *gardencorev1beta1.ControlPlaneAutoscaling `json:"autoscaling,omitempty"`
	// Backup contains the object store configuration for backups for the virtual garden etcd.
	// +optional
	Backup *Backup `json:"backup,omitempty"`
	// Storage contains storage configuration.
	// +optional
	Storage *Storage `json:"storage,omitempty"`
}

// ETCDEvents contains configuration for the events etcd.
type ETCDEvents struct {
	// Autoscaling contains auto-scaling configuration options for etcd.
	// +optional
	Autoscaling *gardencorev1beta1.ControlPlaneAutoscaling `json:"autoscaling,omitempty"`
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
	// BucketName is the name of the backup bucket. If not provided, gardener-operator attempts to manage a new bucket.
	// In this case, the cloud provider credentials provided in the SecretRef must have enough privileges for creating
	// and deleting buckets.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="BucketName is immutable"
	// +optional
	BucketName *string `json:"bucketName,omitempty"`
	// ProviderConfig is the provider-specific configuration passed to BackupBucket resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty"`
	// Region is a region name. If undefined, the provider region is used. This field is immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Region is immutable"
	// +optional
	Region *string `json:"region,omitempty"`
	// SecretRef is a reference to a Secret object containing the cloud provider credentials for the object store where
	// backups should be stored. It should have enough privileges to manipulate the objects as well as buckets.
	SecretRef corev1.LocalObjectReference `json:"secretRef"`
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
	// If not configured, Gardener falls back to a secret labelled with 'gardener.cloud/role=garden-cert'.
	// +kubebuilder:validation:MinLength=1
	// +optional
	SecretName *string `json:"secretName,omitempty"`
	// DomainPatterns is a list of fully qualified domain names, possibly with prefixed wildcard segments. The domain
	// patterns also allow IP addresses, but IPs should only be used if the apiserver has visibility to the IP address
	// requested by a client. If no domain patterns are provided, the names of the certificate are extracted.
	// Non-wildcard matches trump over wildcard matches, explicit domain patterns trump over extracted names.
	// +optional
	DomainPatterns []string `json:"domainPatterns,omitempty"`
}

// Networking defines networking parameters for the virtual garden cluster.
type Networking struct {
	// Services are the CIDRs of the service network. Elements can be appended to this list, but not removed.
	// +kubebuilder:validation:MinItems=1
	Services []string `json:"services"`
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

// Gardener contains the configuration settings for the Gardener components.
type Gardener struct {
	// ClusterIdentity is the identity of the garden cluster. This field is immutable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	ClusterIdentity string `json:"clusterIdentity"`
	// APIServer contains configuration settings for the gardener-apiserver.
	// +optional
	APIServer *GardenerAPIServerConfig `json:"gardenerAPIServer,omitempty"`
	// AdmissionController contains configuration settings for the gardener-admission-controller.
	// +optional
	AdmissionController *GardenerAdmissionControllerConfig `json:"gardenerAdmissionController,omitempty"`
	// ControllerManager contains configuration settings for the gardener-controller-manager.
	// +optional
	ControllerManager *GardenerControllerManagerConfig `json:"gardenerControllerManager,omitempty"`
	// Scheduler contains configuration settings for the gardener-scheduler.
	// +optional
	Scheduler *GardenerSchedulerConfig `json:"gardenerScheduler,omitempty"`
	// Dashboard contains configuration settings for the gardener-dashboard.
	// +optional
	Dashboard *GardenerDashboardConfig `json:"gardenerDashboard,omitempty"`
	// DiscoveryServer contains configuration settings for the gardener-discovery-server.
	// +optional
	DiscoveryServer *GardenerDiscoveryServerConfig `json:"gardenerDiscoveryServer,omitempty"`
}

// GardenerAPIServerConfig contains configuration settings for the gardener-apiserver.
type GardenerAPIServerConfig struct {
	gardencorev1beta1.KubernetesConfig `json:",inline"`
	// AdmissionPlugins contains the list of user-defined admission plugins (additional to those managed by Gardener),
	// and, if desired, the corresponding configuration.
	// +optional
	AdmissionPlugins []gardencorev1beta1.AdmissionPlugin `json:"admissionPlugins,omitempty"`
	// AuditConfig contains configuration settings for the audit of the kube-apiserver.
	// +optional
	AuditConfig *gardencorev1beta1.AuditConfig `json:"auditConfig,omitempty"`
	// AuditWebhook contains settings related to an audit webhook configuration.
	// +optional
	AuditWebhook *AuditWebhook `json:"auditWebhook,omitempty"`
	// Logging contains configuration for the log level and HTTP access logs.
	// +optional
	Logging *gardencorev1beta1.APIServerLogging `json:"logging,omitempty"`
	// Requests contains configuration for request-specific settings for the kube-apiserver.
	// +optional
	Requests *gardencorev1beta1.APIServerRequests `json:"requests,omitempty"`
	// WatchCacheSizes contains configuration of the API server's watch cache sizes.
	// Configuring these flags might be useful for large-scale Garden clusters with a lot of parallel update requests
	// and a lot of watching controllers (e.g. large ManagedSeed clusters). When the API server's watch cache's
	// capacity is too small to cope with the amount of update requests and watchers for a particular resource, it
	// might happen that controller watches are permanently stopped with `too old resource version` errors.
	// Starting from kubernetes v1.19, the API server's watch cache size is adapted dynamically and setting the watch
	// cache size flags will have no effect, except when setting it to 0 (which disables the watch cache).
	// +optional
	WatchCacheSizes *gardencorev1beta1.WatchCacheSizes `json:"watchCacheSizes,omitempty"`
	// EncryptionConfig contains customizable encryption configuration of the Gardener API server.
	// +optional
	EncryptionConfig *gardencorev1beta1.EncryptionConfig `json:"encryptionConfig,omitempty"`
	// GoAwayChance can be used to prevent HTTP/2 clients from getting stuck on a single apiserver, randomly close a
	// connection (GOAWAY). The client's other in-flight requests won't be affected, and the client will reconnect,
	// likely landing on a different apiserver after going through the load balancer again. This field sets the fraction
	// of requests that will be sent a GOAWAY. Min is 0 (off), Max is 0.02 (1/50 requests); 0.001 (1/1000) is a
	// recommended starting point.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=0.02
	// +optional
	GoAwayChance *float64 `json:"goAwayChance,omitempty"`
	// ShootAdminKubeconfigMaxExpiration is the maximum validity duration of a credential requested to a Shoot by an AdminKubeconfigRequest.
	// If an otherwise valid AdminKubeconfigRequest with a validity duration larger than this value is requested,
	// a credential will be issued with a validity duration of this value.
	// +optional
	ShootAdminKubeconfigMaxExpiration *metav1.Duration `json:"shootAdminKubeconfigMaxExpiration,omitempty"`
}

// GardenerAdmissionControllerConfig contains configuration settings for the gardener-admission-controller.
type GardenerAdmissionControllerConfig struct {
	// LogLevel is the configured log level for the gardener-admission-controller. Must be one of [info,debug,error].
	// Defaults to info.
	// +kubebuilder:validation:Enum=info;debug;error
	// +kubebuilder:default=info
	// +optional
	LogLevel *string `json:"logLevel,omitempty"`
	// ResourceAdmissionConfiguration is the configuration for resource size restrictions for arbitrary Group-Version-Kinds.
	// +optional
	ResourceAdmissionConfiguration *ResourceAdmissionConfiguration `json:"resourceAdmissionConfiguration,omitempty"`
}

// ResourceAdmissionConfiguration contains settings about arbitrary kinds and the size each resource should have at most.
type ResourceAdmissionConfiguration struct {
	// Limits contains configuration for resources which are subjected to size limitations.
	Limits []ResourceLimit `json:"limits"`
	// UnrestrictedSubjects contains references to users, groups, or service accounts which aren't subjected to any resource size limit.
	// +optional
	UnrestrictedSubjects []rbacv1.Subject `json:"unrestrictedSubjects,omitempty"`
	// OperationMode specifies the mode the webhooks operates in. Allowed values are "block" and "log". Defaults to "block".
	// +optional
	OperationMode *ResourceAdmissionWebhookMode `json:"operationMode,omitempty"`
}

// ResourceAdmissionWebhookMode is an alias type for the resource admission webhook mode.
type ResourceAdmissionWebhookMode string

// ResourceLimit contains settings about a kind and the size each resource should have at most.
type ResourceLimit struct {
	// APIGroups is the name of the APIGroup that contains the limited resource. WildcardAll represents all groups.
	// +optional
	APIGroups []string `json:"apiGroups,omitempty"`
	// APIVersions is the version of the resource. WildcardAll represents all versions.
	// +optional
	APIVersions []string `json:"apiVersions,omitempty"`
	// Resources is the name of the resource this rule applies to. WildcardAll represents all resources.
	Resources []string `json:"resources"`
	// Size specifies the imposed limit.
	Size resource.Quantity `json:"size"`
}

// GardenerControllerManagerConfig contains configuration settings for the gardener-controller-manager.
type GardenerControllerManagerConfig struct {
	gardencorev1beta1.KubernetesConfig `json:",inline"`
	// DefaultProjectQuotas is the default configuration matching projects are set up with if a quota is not already
	// specified.
	// +optional
	DefaultProjectQuotas []ProjectQuotaConfiguration `json:"defaultProjectQuotas,omitempty"`
	// LogLevel is the configured log level for the gardener-controller-manager. Must be one of [info,debug,error].
	// Defaults to info.
	// +kubebuilder:validation:Enum=info;debug;error
	// +kubebuilder:default=info
	// +optional
	LogLevel *string `json:"logLevel,omitempty"`
}

// ProjectQuotaConfiguration defines quota configurations.
type ProjectQuotaConfiguration struct {
	// Config is the corev1.ResourceQuota specification used for the project set-up.
	Config corev1.ResourceQuota `json:"config"`
	// ProjectSelector is an optional setting to select the projects considered for quotas.
	// Defaults to empty LabelSelector, which matches all projects.
	// +optional
	ProjectSelector *metav1.LabelSelector `json:"projectSelector,omitempty"`
}

// GardenerSchedulerConfig contains configuration settings for the gardener-scheduler.
type GardenerSchedulerConfig struct {
	gardencorev1beta1.KubernetesConfig `json:",inline"`
	// LogLevel is the configured log level for the gardener-scheduler. Must be one of [info,debug,error].
	// Defaults to info.
	// +kubebuilder:validation:Enum=info;debug;error
	// +kubebuilder:default=info
	// +optional
	LogLevel *string `json:"logLevel,omitempty"`
}

// GardenerDashboardConfig contains configuration settings for the gardener-dashboard.
type GardenerDashboardConfig struct {
	// EnableTokenLogin specifies whether it is possible to log into the dashboard with a JWT token. If disabled, OIDC
	// must be configured.
	// +kubebuilder:default=true
	// +optional
	EnableTokenLogin *bool `json:"enableTokenLogin,omitempty"`
	// FrontendConfigMapRef is the reference to a ConfigMap in the garden namespace containing the frontend
	// configuration.
	// +optional
	FrontendConfigMapRef *corev1.LocalObjectReference `json:"frontendConfigMapRef,omitempty"`
	// AssetsConfigMapRef is the reference to a ConfigMap in the garden namespace containing the assets (logos/icons).
	// +optional
	AssetsConfigMapRef *corev1.LocalObjectReference `json:"assetsConfigMapRef,omitempty"`
	// GitHub contains configuration for the GitHub ticketing feature.
	// +optional
	GitHub *DashboardGitHub `json:"gitHub,omitempty"`
	// LogLevel is the configured log level. Must be one of [trace,debug,info,warn,error].
	// Defaults to info.
	// +kubebuilder:validation:Enum=trace;debug;info;warn;error
	// +kubebuilder:default=info
	// +optional
	LogLevel *string `json:"logLevel,omitempty"`
	// OIDCConfig contains configuration for the OIDC provider. This field must be provided when EnableTokenLogin is false.
	// +optional
	OIDCConfig *DashboardOIDC `json:"oidcConfig,omitempty"`
	// Terminal contains configuration for the terminal settings.
	// +optional
	Terminal *DashboardTerminal `json:"terminal,omitempty"`
	// Ingress contains configuration for the ingress settings.
	// +optional
	Ingress *DashboardIngress `json:"ingress,omitempty"`
}

// DashboardGitHub contains configuration for the GitHub ticketing feature.
type DashboardGitHub struct {
	// APIURL is the URL to the GitHub API.
	// +kubebuilder:default=`https://api.github.com`
	// +kubebuilder:validation:MinLength=1
	APIURL string `json:"apiURL"`
	// Organisation is the name of the GitHub organisation.
	// +kubebuilder:validation:MinLength=1
	Organisation string `json:"organisation"`
	// Repository is the name of the GitHub repository.
	// +kubebuilder:validation:MinLength=1
	Repository string `json:"repository"`
	// SecretRef is the reference to a secret in the garden namespace containing the GitHub credentials.
	SecretRef corev1.LocalObjectReference `json:"secretRef"`
	// PollInterval is the interval of how often the GitHub API is polled for issue updates. This field is used as a
	// fallback mechanism to ensure state synchronization, even when there is a GitHub webhook configuration. If a
	// webhook event is missed or not successfully delivered, the polling will help catch up on any missed updates.
	// If this field is not provided and there is no 'webhookSecret' key in the referenced secret, it will be
	// implicitly defaulted to `15m`.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	// +optional
	PollInterval *metav1.Duration `json:"pollInterval,omitempty"`
}

// DashboardOIDC contains configuration for the OIDC settings.
type DashboardOIDC struct {
	// ClientIDPublic is the public client ID.
	// Falls back to the API server's OIDC client ID configuration if not set here.
	// +kubebuilder:validation:MinLength=1
	// +optional
	ClientIDPublic *string `json:"clientIDPublic,omitempty"`
	// The URL of the OpenID issuer, only HTTPS scheme will be accepted. Used to verify the OIDC JSON Web Token (JWT).
	// Falls back to the API server's OIDC issuer URL configuration if not set here.
	// +kubebuilder:validation:MinLength=1
	// +optional
	IssuerURL *string `json:"issuerURL,omitempty"`
	// SessionLifetime is the maximum duration of a session.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	// +optional
	SessionLifetime *metav1.Duration `json:"sessionLifetime,omitempty"`
	// AdditionalScopes is the list of additional OIDC scopes.
	// +optional
	AdditionalScopes []string `json:"additionalScopes,omitempty"`
	// SecretRef is the reference to a secret in the garden namespace containing the OIDC client ID and secret for the dashboard.
	SecretRef corev1.LocalObjectReference `json:"secretRef"`
	// CertificateAuthoritySecretRef is the reference to a secret in the garden namespace containing a custom CA certificate under the "ca.crt" key
	// +optional
	CertificateAuthoritySecretRef *corev1.LocalObjectReference `json:"certificateAuthoritySecretRef,omitempty"`
}

// DashboardTerminal contains configuration for the terminal settings.
type DashboardTerminal struct {
	// Container contains configuration for the dashboard terminal container.
	Container DashboardTerminalContainer `json:"container"`
	// AllowedHosts should consist of permitted hostnames (without the scheme) for terminal connections.
	// It is important to consider that the usage of wildcards follows the rules defined by the content security policy.
	// '*.seed.local.gardener.cloud', or '*.other-seeds.local.gardener.cloud'. For more information, see
	// https://github.com/gardener/dashboard/blob/master/docs/operations/webterminals.md#allowlist-for-hosts.
	// +optional
	AllowedHosts []string `json:"allowedHosts,omitempty"`
}

// DashboardTerminalContainer contains configuration for the dashboard terminal container.
type DashboardTerminalContainer struct {
	// Image is the container image for the dashboard terminal container.
	Image string `json:"image"`
	// Description is a description for the dashboard terminal container with hints for the user.
	// +optional
	Description *string `json:"description,omitempty"`
}

// DashboardIngress contains configuration for the dashboard ingress resource.
type DashboardIngress struct {
	// Enabled controls whether the Dashboard Ingress resource will be deployed to the cluster.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// GardenerDiscoveryServerConfig contains configuration settings for the gardener-discovery-server.
type GardenerDiscoveryServerConfig struct{}

const (
	// ClusterTypeGarden enables the resource only for the garden cluster.
	ClusterTypeGarden gardencorev1beta1.ClusterType = "garden"
)

// GardenExtension contains type and provider information for Garden extensions.
type GardenExtension struct {
	// Type is the type of the extension resource.
	// +kubebuilder:validation:MinLength=1
	Type string `json:"type"`
	// ProviderConfig is the configuration passed to extension resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty"`
}

// GardenStatus is the status of a garden environment.
type GardenStatus struct {
	// Gardener holds information about the Gardener which last acted on the Garden.
	// +optional
	Gardener *gardencorev1beta1.Gardener `json:"gardener,omitempty"`
	// Conditions is a list of conditions.
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty"`
	// LastOperation holds information about the last operation on the Garden.
	// +optional
	LastOperation *gardencorev1beta1.LastOperation `json:"lastOperation,omitempty"`
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Credentials contains information about the virtual garden cluster credentials.
	// +optional
	Credentials *Credentials `json:"credentials,omitempty"`
	// EncryptedResources is the list of resources which are currently encrypted in the virtual garden by the virtual kube-apiserver.
	// Resources which are encrypted by default will not appear here.
	// See https://github.com/gardener/gardener/blob/master/docs/concepts/operator.md#etcd-encryption-config for more details.
	// +optional
	EncryptedResources []string `json:"encryptedResources,omitempty"`
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
	// Observability contains information about the observability credential rotation.
	// +optional
	Observability *gardencorev1beta1.ObservabilityRotation `json:"observability,omitempty"`
	// WorkloadIdentityKey contains information about the workload identity key credential rotation.
	// +optional
	WorkloadIdentityKey *WorkloadIdentityKeyRotation `json:"workloadIdentityKey,omitempty"`
}

// WorkloadIdentityKeyRotation contains information about the workload identity key credential rotation.
type WorkloadIdentityKeyRotation struct {
	// Phase describes the phase of the workload identity key credential rotation.
	Phase gardencorev1beta1.CredentialsRotationPhase `json:"phase"`
	// LastCompletionTime is the most recent time when the workload identity key credential rotation was successfully
	// completed.
	// +optional
	LastCompletionTime *metav1.Time `json:"lastCompletionTime,omitempty"`
	// LastInitiationTime is the most recent time when the workload identity key credential rotation was initiated.
	// +optional
	LastInitiationTime *metav1.Time `json:"lastInitiationTime,omitempty"`
	// LastInitiationFinishedTime is the recent time when the workload identity key credential rotation initiation was
	// completed.
	// +optional
	LastInitiationFinishedTime *metav1.Time `json:"lastInitiationFinishedTime,omitempty"`
	// LastCompletionTriggeredTime is the recent time when the workload identity key credential rotation completion was
	// triggered.
	// +optional
	LastCompletionTriggeredTime *metav1.Time `json:"lastCompletionTriggeredTime,omitempty"`
}

const (
	// RuntimeComponentsHealthy is a constant for a condition type indicating the runtime components health.
	RuntimeComponentsHealthy gardencorev1beta1.ConditionType = "RuntimeComponentsHealthy"
	// VirtualComponentsHealthy is a constant for a condition type indicating the virtual garden components health.
	VirtualComponentsHealthy gardencorev1beta1.ConditionType = "VirtualComponentsHealthy"
	// VirtualGardenAPIServerAvailable is a constant for a condition type indicating that the virtual garden's API server is available.
	VirtualGardenAPIServerAvailable gardencorev1beta1.ConditionType = "VirtualGardenAPIServerAvailable"
	// ObservabilityComponentsHealthy is a constant for a condition type indicating the health of observability components.
	ObservabilityComponentsHealthy gardencorev1beta1.ConditionType = v1beta1constants.ObservabilityComponentsHealthy
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
	v1beta1constants.OperationRotateObservabilityCredentials,
	v1beta1constants.OperationRotateCredentialsStart,
	v1beta1constants.OperationRotateCredentialsComplete,
	OperationRotateWorkloadIdentityKeyStart,
	OperationRotateWorkloadIdentityKeyComplete,
)

// FinalizerName is the name of the finalizer used by gardener-operator.
const FinalizerName = "gardener.cloud/operator"
