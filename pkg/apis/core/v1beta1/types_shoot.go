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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

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
	// +optional
	Addons *Addons `json:"addons,omitempty"`
	// CloudProfileName is a name of a CloudProfile object.
	CloudProfileName string `json:"cloudProfileName"`
	// DNS contains information about the DNS settings of the Shoot.
	// +optional
	DNS *DNS `json:"dns,omitempty"`
	// Extensions contain type and provider information for Shoot extensions.
	// +optional
	Extensions []Extension `json:"extensions,omitempty"`
	// Hibernation contains information whether the Shoot is suspended or not.
	// +optional
	Hibernation *Hibernation `json:"hibernation,omitempty"`
	// Kubernetes contains the version and configuration settings of the control plane components.
	Kubernetes Kubernetes `json:"kubernetes"`
	// Networking contains information about cluster networking such as CNI Plugin type, CIDRs, ...etc.
	Networking Networking `json:"networking"`
	// Maintenance contains information about the time window for maintenance operations and which
	// operations should be performed.
	// +optional
	Maintenance *Maintenance `json:"maintenance,omitempty"`
	// Monitoring contains information about custom monitoring configurations for the shoot.
	// +optional
	Monitoring *Monitoring `json:"monitoring,omitempty"`
	// Provider contains all provider-specific and provider-relevant information.
	Provider Provider `json:"provider"`
	// Purpose is the purpose class for this cluster.
	// +optional
	Purpose *ShootPurpose `json:"purpose,omitempty"`
	// Region is a name of a region.
	Region string `json:"region"`
	// SecretBindingName is the name of the a SecretBinding that has a reference to the provider secret.
	// The credentials inside the provider secret will be used to create the shoot in the respective account.
	SecretBindingName string `json:"secretBindingName"`
	// SeedName is the name of the seed cluster that runs the control plane of the Shoot.
	// +optional
	SeedName *string `json:"seedName,omitempty"`
}

// ShootStatus holds the most recently observed status of the Shoot cluster.
type ShootStatus struct {
	// Conditions represents the latest available observations of a Shoots's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	// Constraints represents conditions of a Shoot's current state that constraint some operations on it.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Constraints []Condition `json:"constraints,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	// Gardener holds information about the Gardener which last acted on the Shoot.
	Gardener Gardener `json:"gardener"`
	// IsHibernated indicates whether the Shoot is currently hibernated.
	IsHibernated bool `json:"hibernated"`
	// LastOperation holds information about the last operation on the Shoot.
	// +optional
	LastOperation *LastOperation `json:"lastOperation,omitempty"`
	// LastErrors holds information about the last occurred error(s) during an operation.
	// +optional
	LastErrors []LastError `json:"lastErrors,omitempty"`
	// ObservedGeneration is the most recent generation observed for this Shoot. It corresponds to the
	// Shoot's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// RetryCycleStartTime is the start time of the last retry cycle (used to determine how often an operation
	// must be retried until we give up).
	// +optional
	RetryCycleStartTime *metav1.Time `json:"retryCycleStartTime,omitempty"`
	// SeedName is the name of the seed cluster that runs the control plane of the Shoot. This value is only written
	// after a successful create/reconcile operation. It will be used when control planes are moved between Seeds.
	// +optional
	SeedName *string `json:"seedName,omitempty"`
	// TechnicalID is the name that is used for creating the Seed namespace, the infrastructure resources, and
	// basically everything that is related to this particular Shoot.
	TechnicalID string `json:"technicalID"`
	// UID is a unique identifier for the Shoot cluster to avoid portability between Kubernetes clusters.
	// It is used to compute unique hashes.
	UID types.UID `json:"uid"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Addons relevant types                                                                        //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Addons is a collection of configuration for specific addons which are managed by the Gardener.
type Addons struct {
	// KubernetesDashboard holds configuration settings for the kubernetes dashboard addon.
	// +optional
	KubernetesDashboard *KubernetesDashboard `json:"kubernetesDashboard,omitempty"`
	// NginxIngress holds configuration settings for the nginx-ingress addon.
	// +optional
	NginxIngress *NginxIngress `json:"nginxIngress,omitempty"`
}

// Addon allows enabling or disabling a specific addon and is used to derive from.
type Addon struct {
	// Enabled indicates whether the addon is enabled or not.
	Enabled bool `json:"enabled"`
}

// KubernetesDashboard describes configuration values for the kubernetes-dashboard addon.
type KubernetesDashboard struct {
	Addon `json:",inline"`
	// AuthenticationMode defines the authentication mode for the kubernetes-dashboard.
	// +optional
	AuthenticationMode *string `json:"authenticationMode,omitempty"`
}

const (
	// KubernetesDashboardAuthModeBasic uses basic authentication mode for auth.
	KubernetesDashboardAuthModeBasic = "basic"
	// KubernetesDashboardAuthModeToken uses token-based mode for auth.
	KubernetesDashboardAuthModeToken = "token"
)

// NginxIngress describes configuration values for the nginx-ingress addon.
type NginxIngress struct {
	Addon `json:",inline"`
	// LoadBalancerSourceRanges is list of whitelist IP sources for NginxIngress
	// +optional
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`
	// Config contains custom configuration for the nginx-ingress-controller configuration.
	// See https://github.com/kubernetes/ingress-nginx/blob/master/docs/user-guide/nginx-configuration/configmap.md#configuration-options
	// +optional
	Config map[string]string `json:"config,omitempty"`
	// ExternalTrafficPolicy controls the `.spec.externalTrafficPolicy` value of the load balancer `Service`
	// exposing the nginx-ingress. Defaults to `Cluster`.
	// +optional
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy,omitempty"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// DNS relevant types                                                                           //
//////////////////////////////////////////////////////////////////////////////////////////////////

// DNS holds information about the provider, the hosted zone id and the domain.
type DNS struct {
	// Domain is the external available domain of the Shoot cluster. This domain will be written into the
	// kubeconfig that is handed out to end-users.
	// +optional
	Domain *string `json:"domain,omitempty"`
	// Providers is a list of DNS providers that shall be enabled for this shoot cluster. Only relevant if
	// not a default domain is used.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Providers []DNSProvider `json:"providers,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// DNSProvider contains information about a DNS provider.
type DNSProvider struct {
	// Domains contains information about which domains shall be included/excluded for this provider.
	// +optional
	Domains *DNSIncludeExclude `json:"domains,omitempty"`
	// SecretName is a name of a secret containing credentials for the stated domain and the
	// provider. When not specified, the Gardener will use the cloud provider credentials referenced
	// by the Shoot and try to find respective credentials there. Specifying this field may override
	// this behavior, i.e. forcing the Gardener to only look into the given secret.
	// +optional
	SecretName *string `json:"secretName,omitempty"`
	// Type is the DNS provider type for the Shoot. Only relevant if not the default domain is used for
	// this shoot.
	// +optional
	Type *string `json:"type,omitempty"`
	// Zones contains information about which hosted zones shall be included/excluded for this provider.
	// +optional
	Zones *DNSIncludeExclude `json:"zones,omitempty"`
}

type DNSIncludeExclude struct {
	// Include is a list of resources that shall be included.
	// +optional
	Include []string `json:"include,omitempty"`
	// Exclude is a list of resources that shall be excluded.
	// +optional
	Exclude []string `json:"exclude,omitempty"`
}

// DefaultDomain is the default value in the Shoot's '.spec.dns.domain' when '.spec.dns.provider' is 'unmanaged'
const DefaultDomain = "cluster.local"

//////////////////////////////////////////////////////////////////////////////////////////////////
// Extension relevant types                                                                     //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Extension contains type and provider information for Shoot extensions.
type Extension struct {
	// Type is the type of the extension resource.
	Type string `json:"type"`
	// ProviderConfig is the configuration passed to extension resource.
	// +optional
	ProviderConfig *ProviderConfig `json:"providerConfig,omitempty"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Hibernation relevant types                                                                   //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Hibernation contains information whether the Shoot is suspended or not.
type Hibernation struct {
	// Enabled specifies whether the Shoot needs to be hibernated or not. If it is true, the Shoot's desired state is to be hibernated.
	// If it is false or nil, the Shoot's desired state is to be awaken.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
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
	// Location is the time location in which both start and and shall be evaluated.
	// +optional
	Location *string `json:"location,omitempty"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Kubernetes relevant types                                                                    //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Kubernetes contains the version and configuration variables for the Shoot control plane.
type Kubernetes struct {
	// AllowPrivilegedContainers indicates whether privileged containers are allowed in the Shoot (default: true).
	// +optional
	AllowPrivilegedContainers *bool `json:"allowPrivilegedContainers,omitempty"`
	// ClusterAutoscaler contains the configration flags for the Kubernetes cluster autoscaler.
	// +optional
	ClusterAutoscaler *ClusterAutoscaler `json:"clusterAutoscaler,omitempty"`
	// KubeAPIServer contains configuration settings for the kube-apiserver.
	// +optional
	KubeAPIServer *KubeAPIServerConfig `json:"kubeAPIServer,omitempty"`
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

// ClusterAutoscaler contains the configration flags for the Kubernetes cluster autoscaler.
type ClusterAutoscaler struct {
	// ScaleDownDelayAfterAdd defines how long after scale up that scale down evaluation resumes (default: 10 mins).
	// +optional
	ScaleDownDelayAfterAdd *metav1.Duration `json:"scaleDownDelayAfterAdd,omitempty"`
	// ScaleDownDelayAfterDelete how long after node deletion that scale down evaluation resumes, defaults to scanInterval (defaults to ScanInterval).
	// +optional
	ScaleDownDelayAfterDelete *metav1.Duration `json:"scaleDownDelayAfterDelete,omitempty"`
	// ScaleDownDelayAfterFailure how long after scale down failure that scale down evaluation resumes (default: 3 mins).
	// +optional
	ScaleDownDelayAfterFailure *metav1.Duration `json:"scaleDownDelayAfterFailure,omitempty"`
	// ScaleDownUnneededTime defines how long a node should be unneeded before it is eligible for scale down (default: 10 mins).
	// +optional
	ScaleDownUnneededTime *metav1.Duration `json:"scaleDownUnneededTime,omitempty"`
	// ScaleDownUtilizationThreshold defines the threshold in % under which a node is being removed
	// +optional
	ScaleDownUtilizationThreshold *float64 `json:"scaleDownUtilizationThreshold,omitempty"`
	// ScanInterval how often cluster is reevaluated for scale up or down (default: 10 secs).
	// +optional
	ScanInterval *metav1.Duration `json:"scanInterval,omitempty"`
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
	// AdmissionPlugins contains the list of user-defined admission plugins (additional to those managed by Gardener), and, if desired, the corresponding
	// configuration.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	AdmissionPlugins []AdmissionPlugin `json:"admissionPlugins,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
	// APIAudiences are the identifiers of the API. The service account token authenticator will
	// validate that tokens used against the API are bound to at least one of these audiences.
	// If `serviceAccountConfig.issuer` is configured and this is not, this defaults to a single
	// element list containing the issuer URL.
	// +optional
	APIAudiences []string `json:"apiAudiences,omitempty"`
	// AuditConfig contains configuration settings for the audit of the kube-apiserver.
	// +optional
	AuditConfig *AuditConfig `json:"auditConfig,omitempty"`
	// EnableBasicAuthentication defines whether basic authentication should be enabled for this cluster or not.
	// +optional
	EnableBasicAuthentication *bool `json:"enableBasicAuthentication,omitempty"`
	// OIDCConfig contains configuration settings for the OIDC provider.
	// +optional
	OIDCConfig *OIDCConfig `json:"oidcConfig,omitempty"`
	// RuntimeConfig contains information about enabled or disabled APIs.
	// +optional
	RuntimeConfig map[string]bool `json:"runtimeConfig,omitempty"`
	// ServiceAccountConfig contains configuration settings for the service account handling
	// of the kube-apiserver.
	// +optional
	ServiceAccountConfig *ServiceAccountConfig `json:"serviceAccountConfig,omitempty"`
}

// ServiceAccountConfig is the kube-apiserver configuration for service accounts.
type ServiceAccountConfig struct {
	// Issuer is the identifier of the service account token issuer. The issuer will assert this
	// identifier in "iss" claim of issued tokens. This value is a string or URI.
	// +optional
	Issuer *string `json:"issuer,omitempty"`
	// SigningKeySecret is a reference to a secret that contains the current private key of the
	// service account token issuer. The issuer will sign issued ID tokens with this private key.
	// (Requires the 'TokenRequest' feature gate.)
	// +optional
	SigningKeySecret *corev1.LocalObjectReference `json:"signingKeySecretName,omitempty"`
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
	ConfigMapRef *corev1.ObjectReference `json:"configMapRef,omitempty"`
}

// OIDCConfig contains configuration settings for the OIDC provider.
// Note: Descriptions were taken from the Kubernetes documentation.
type OIDCConfig struct {
	// If set, the OpenID server's certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host's root CA set will be used.
	// +optional
	CABundle *string `json:"caBundle,omitempty"`
	// ClientAuthentication can optionally contain client configuration used for kubeconfig generation.
	// +optional
	ClientAuthentication *OpenIDConnectClientAuthentication `json:"clientAuthentication,omitempty"`
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

// OpenIDConnectClientAuthentication contains configuration for OIDC clients.
type OpenIDConnectClientAuthentication struct {
	// Extra configuration added to kubeconfig's auth-provider.
	// Must not be any of idp-issuer-url, client-id, client-secret, idp-certificate-authority, idp-certificate-authority-data, id-token or refresh-token
	// +optional
	ExtraConfig map[string]string `json:"extraConfig,omitempty"`
	// The client Secret for the OpenID Connect client.
	// +optional
	Secret *string `json:"secret,omitempty"`
}

// AdmissionPlugin contains information about a specific admission plugin and its corresponding configuration.
type AdmissionPlugin struct {
	// Name is the name of the plugin.
	Name string `json:"name"`
	// Config is the configuration of the plugin.
	// +optional
	Config *ProviderConfig `json:"config,omitempty"`
}

// KubeControllerManagerConfig contains configuration settings for the kube-controller-manager.
type KubeControllerManagerConfig struct {
	KubernetesConfig `json:",inline"`
	// HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
	// +optional
	HorizontalPodAutoscalerConfig *HorizontalPodAutoscalerConfig `json:"horizontalPodAutoscaler,omitempty"`
	// NodeCIDRMaskSize defines the mask size for node cidr in cluster (default is 24)
	// +optional
	NodeCIDRMaskSize *int32 `json:"nodeCIDRMaskSize,omitempty"`
}

// HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
// Note: Descriptions were taken from the Kubernetes documentation.
type HorizontalPodAutoscalerConfig struct {
	// The period after which a ready pod transition is considered to be the first.
	// +optional
	CPUInitializationPeriod *metav1.Duration `json:"cpuInitializationPeriod,omitempty"`
	// The period since last downscale, before another downscale can be performed in horizontal pod autoscaler.
	// +optional
	DownscaleDelay *metav1.Duration `json:"downscaleDelay,omitempty"`
	// The configurable window at which the controller will choose the highest recommendation for autoscaling.
	// +optional
	DownscaleStabilization *metav1.Duration `json:"downscaleStabilization,omitempty"`
	// The configurable period at which the horizontal pod autoscaler considers a Pod “not yet ready” given that it’s unready and it has  transitioned to unready during that time.
	// +optional
	InitialReadinessDelay *metav1.Duration `json:"initialReadinessDelay,omitempty"`
	// The period for syncing the number of pods in horizontal pod autoscaler.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// The minimum change (from 1.0) in the desired-to-actual metrics ratio for the horizontal pod autoscaler to consider scaling.
	// +optional
	Tolerance *float64 `json:"tolerance,omitempty"`
	// The period since last upscale, before another upscale can be performed in horizontal pod autoscaler.
	// +optional
	UpscaleDelay *metav1.Duration `json:"upscaleDelay,omitempty"`
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
	KubernetesConfig `json:",inline"`
	// CPUCFSQuota allows you to disable/enable CPU throttling for Pods.
	// +optional
	CPUCFSQuota *bool `json:"cpuCFSQuota,omitempty"`
	// CPUManagerPolicy allows to set alternative CPU management policies (default: none).
	// +optional
	CPUManagerPolicy *string `json:"cpuManagerPolicy,omitempty"`
	// EvictionHard describes a set of eviction thresholds (e.g. memory.available<1Gi) that if met would trigger a Pod eviction.
	// +optional
	// Default:
	//   memory.available:   "100Mi/1Gi/5%"
	//   nodefs.available:   "5%"
	//   nodefs.inodesFree:  "5%"
	//   imagefs.available:  "5%"
	//   imagefs.inodesFree: "5%"
	EvictionHard *KubeletConfigEviction `json:"evictionHard,omitempty"`
	// EvictionMaxPodGracePeriod describes the maximum allowed grace period (in seconds) to use when terminating pods in response to a soft eviction threshold being met.
	// +optional
	// Default: 90
	EvictionMaxPodGracePeriod *int32 `json:"evictionMaxPodGracePeriod,omitempty"`
	// EvictionMinimumReclaim configures the amount of resources below the configured eviction threshold that the kubelet attempts to reclaim whenever the kubelet observes resource pressure.
	// +optional
	// Default: 0 for each resource
	EvictionMinimumReclaim *KubeletConfigEvictionMinimumReclaim `json:"evictionMinimumReclaim,omitempty"`
	// EvictionPressureTransitionPeriod is the duration for which the kubelet has to wait before transitioning out of an eviction pressure condition.
	// +optional
	// Default: 4m0s
	EvictionPressureTransitionPeriod *metav1.Duration `json:"evictionPressureTransitionPeriod,omitempty"`
	// EvictionSoft describes a set of eviction thresholds (e.g. memory.available<1.5Gi) that if met over a corresponding grace period would trigger a Pod eviction.
	// +optional
	// Default:
	//   memory.available:   "200Mi/1.5Gi/10%"
	//   nodefs.available:   "10%"
	//   nodefs.inodesFree:  "10%"
	//   imagefs.available:  "10%"
	//   imagefs.inodesFree: "10%"
	EvictionSoft *KubeletConfigEviction `json:"evictionSoft,omitempty"`
	// EvictionSoftGracePeriod describes a set of eviction grace periods (e.g. memory.available=1m30s) that correspond to how long a soft eviction threshold must hold before triggering a Pod eviction.
	// +optional
	// Default:
	//   memory.available:   1m30s
	//   nodefs.available:   1m30s
	//   nodefs.inodesFree:  1m30s
	//   imagefs.available:  1m30s
	//   imagefs.inodesFree: 1m30s
	EvictionSoftGracePeriod *KubeletConfigEvictionSoftGracePeriod `json:"evictionSoftGracePeriod,omitempty"`
	// MaxPods is the maximum number of Pods that are allowed by the Kubelet.
	// +optional
	// Default: 110
	MaxPods *int32 `json:"maxPods,omitempty"`
	// PodPIDsLimit is the maximum number of process IDs per pod allowed by the kubelet.
	// +optional
	PodPIDsLimit *int64 `json:"podPidsLimit,omitempty"`
}

// KubeletConfigEviction contains kubelet eviction thresholds supporting either a resource.Quantity or a percentage based value.
type KubeletConfigEviction struct {
	// MemoryAvailable is the threshold for the free memory on the host server.
	// +optional
	MemoryAvailable *string `json:"memoryAvailable,omitempty"`
	// ImageFSAvailable is the threshold for the free disk space in the imagefs filesystem (docker images and container writable layers).
	// +optional
	ImageFSAvailable *string `json:"imageFSAvailable,omitempty"`
	// ImageFSInodesFree is the threshold for the available inodes in the imagefs filesystem.
	// +optional
	ImageFSInodesFree *string `json:"imageFSInodesFree,omitempty"`
	// NodeFSAvailable is the threshold for the free disk space in the nodefs filesystem (docker volumes, logs, etc).
	// +optional
	NodeFSAvailable *string `json:"nodeFSAvailable,omitempty"`
	// NodeFSInodesFree is the threshold for the available inodes in the nodefs filesystem.
	// +optional
	NodeFSInodesFree *string `json:"nodeFSInodesFree,omitempty"`
}

// KubeletConfigEvictionMinimumReclaim contains configuration for the kubelet eviction minimum reclaim.
type KubeletConfigEvictionMinimumReclaim struct {
	// MemoryAvailable is the threshold for the memory reclaim on the host server.
	// +optional
	MemoryAvailable *resource.Quantity `json:"memoryAvailable,omitempty"`
	// ImageFSAvailable is the threshold for the disk space reclaim in the imagefs filesystem (docker images and container writable layers).
	// +optional
	ImageFSAvailable *resource.Quantity `json:"imageFSAvailable,omitempty"`
	// ImageFSInodesFree is the threshold for the inodes reclaim in the imagefs filesystem.
	// +optional
	ImageFSInodesFree *resource.Quantity `json:"imageFSInodesFree,omitempty"`
	// NodeFSAvailable is the threshold for the disk space reclaim in the nodefs filesystem (docker volumes, logs, etc).
	// +optional
	NodeFSAvailable *resource.Quantity `json:"nodeFSAvailable,omitempty"`
	// NodeFSInodesFree is the threshold for the inodes reclaim in the nodefs filesystem.
	// +optional
	NodeFSInodesFree *resource.Quantity `json:"nodeFSInodesFree,omitempty"`
}

// KubeletConfigEvictionSoftGracePeriod contains grace periods for kubelet eviction thresholds.
type KubeletConfigEvictionSoftGracePeriod struct {
	// MemoryAvailable is the grace period for the MemoryAvailable eviction threshold.
	// +optional
	MemoryAvailable *metav1.Duration `json:"memoryAvailable,omitempty"`
	// ImageFSAvailable is the grace period for the ImageFSAvailable eviction threshold.
	// +optional
	ImageFSAvailable *metav1.Duration `json:"imageFSAvailable,omitempty"`
	// ImageFSInodesFree is the grace period for the ImageFSInodesFree eviction threshold.
	// +optional
	ImageFSInodesFree *metav1.Duration `json:"imageFSInodesFree,omitempty"`
	// NodeFSAvailable is the grace period for the NodeFSAvailable eviction threshold.
	// +optional
	NodeFSAvailable *metav1.Duration `json:"nodeFSAvailable,omitempty"`
	// NodeFSInodesFree is the grace period for the NodeFSInodesFree eviction threshold.
	// +optional
	NodeFSInodesFree *metav1.Duration `json:"nodeFSInodesFree,omitempty"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Networking relevant types                                                                    //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Networking defines networking parameters for the shoot cluster.
type Networking struct {
	// Type identifies the type of the networking plugin.
	Type string `json:"type"`
	// ProviderConfig is the configuration passed to network resource.
	// +optional
	ProviderConfig *ProviderConfig `json:"providerConfig,omitempty"`
	// Pods is the CIDR of the pod network.
	// +optional
	Pods *string `json:"pods,omitempty"`
	// Nodes is the CIDR of the entire node network.
	// +optional
	Nodes *string `json:"nodes,omitempty"`
	// Services is the CIDR of the service network.
	// +optional
	Services *string `json:"services,omitempty"`
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
	// KubernetesVersion indicates whether the patch Kubernetes version may be automatically updated (default: true).
	KubernetesVersion bool `json:"kubernetesVersion"`
	// MachineImageVersion indicates whether the machine image version may be automatically updated (default: true).
	MachineImageVersion bool `json:"machineImageVersion"`
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

//////////////////////////////////////////////////////////////////////////////////////////////////
// Monitoring relevant types                                                                    //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Monitoring contains information about the monitoring configuration for the shoot.
type Monitoring struct {
	// Alerting contains information about the alerting configuration for the shoot cluster.
	// +optional
	Alerting *Alerting `json:"alerting,omitempty"`
}

// Alerting contains information about how alerting will be done (i.e. who will receive alerts and how).
type Alerting struct {
	// MonitoringEmailReceivers is a list of recipients for alerts
	// +optional
	EmailReceivers []string `json:"emailReceivers,omitempty"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Provider relevant types                                                                      //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Provider contains provider-specific information that are handed-over to the provider-specific
// extension controller.
type Provider struct {
	// Type is the type of the provider.
	Type string `json:"type"`
	// ControlPlaneConfig contains the provider-specific control plane config blob. Please look up the concrete
	// definition in the documentation of your provider extension.
	// +optional
	ControlPlaneConfig *ProviderConfig `json:"controlPlaneConfig,omitempty"`
	// InfrastructureConfig contains the provider-specific infrastructure config blob. Please look up the concrete
	// definition in the documentation of your provider extension.
	// +optional
	InfrastructureConfig *ProviderConfig `json:"infrastructureConfig,omitempty"`
	// Workers is a list of worker groups.
	// +patchMergeKey=name
	// +patchStrategy=merge
	Workers []Worker `json:"workers" patchStrategy:"merge" patchMergeKey:"name"`
}

// Worker is the base definition of a worker group.
type Worker struct {
	// Annotations is a map of key/value pairs for annotations for all the `Node` objects in this worker pool.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// CABundle is a certificate bundle which will be installed onto every machine of this worker pool.
	// +optional
	CABundle *string `json:"caBundle,omitempty"`
	// Kubernetes contains configuration for Kubernetes components related to this worker pool.
	// +optional
	Kubernetes *WorkerKubernetes `json:"kubernetes,omitempty"`
	// Labels is a map of key/value pairs for labels for all the `Node` objects in this worker pool.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Name is the name of the worker group.
	Name string `json:"name"`
	// Machine contains information about the machine type and image.
	Machine Machine `json:"machine"`
	// Maximum is the maximum number of VMs to create.
	Maximum int32 `json:"maximum"`
	// Minimum is the minimum number of VMs to create.
	Minimum int32 `json:"minimum"`
	// MaxSurge is maximum number of VMs that are created during an update.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`
	// MaxUnavailable is the maximum number of VMs that can be unavailable during an update.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
	// ProviderConfig is the provider-specific configuration for this worker pool.
	// +optional
	ProviderConfig *ProviderConfig `json:"providerConfig,omitempty"`
	// Taints is a list of taints for all the `Node` objects in this worker pool.
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`
	// Volume contains information about the volume type and size.
	// +optional
	Volume *Volume `json:"volume,omitempty"`
	// Zones is a list of availability zones that are used to evenly distribute this worker pool. Optional
	// as not every provider may support availability zones.
	// +optional
	Zones []string `json:"zones,omitempty"`
}

// WorkerKubernetes contains configuration for Kubernetes components related to this worker pool.
type WorkerKubernetes struct {
	// Kubelet contains configuration settings for all kubelets of this worker pool.
	// +optional
	Kubelet *KubeletConfig `json:"kubelet,omitempty"`
}

// Machine contains information about the machine type and image.
type Machine struct {
	// Type is the machine type of the worker group.
	Type string `json:"type"`
	// Image holds information about the machine image to use for all nodes of this pool. It will default to the
	// latest version of the first image stated in the referenced CloudProfile if no value has been provided.
	// +optional
	Image *ShootMachineImage `json:"image,omitempty"`
}

// ShootMachineImage defines the name and the version of the shoot's machine image in any environment. Has to be
// defined in the respective CloudProfile.
type ShootMachineImage struct {
	// Name is the name of the image.
	Name string `json:"name"`
	// ProviderConfig is the shoot's individual configuration passed to an extension resource.
	// +optional
	ProviderConfig *ProviderConfig `json:"providerConfig,omitempty"`
	// Version is the version of the shoot's image.
	Version string `json:"version"`
}

// Volume contains information about the volume type and size.
type Volume struct {
	// Type is the machine type of the worker group.
	// +optional
	Type *string `json:"type,omitempty"`
	// Size is the size of the root volume.
	Size string `json:"size"`
}

var (
	// DefaultWorkerMaxSurge is the default value for Worker MaxSurge.
	DefaultWorkerMaxSurge = intstr.FromInt(1)
	// DefaultWorkerMaxUnavailable is the default value for Worker MaxUnavailable.
	DefaultWorkerMaxUnavailable = intstr.FromInt(0)
)

//////////////////////////////////////////////////////////////////////////////////////////////////
// Other/miscellaneous constants and types                                                      //
//////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// ShootEventMaintenanceDone indicates that a maintenance operation has been performed.
	ShootEventMaintenanceDone = "MaintenanceDone"
	// ShootEventMaintenanceError indicates that a maintenance operation has failed.
	ShootEventMaintenanceError = "MaintenanceError"

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
