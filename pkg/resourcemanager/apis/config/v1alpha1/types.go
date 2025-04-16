// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceManagerConfiguration defines the configuration for the gardener-resource-manager.
type ResourceManagerConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// SourceClientConnection specifies the client connection settings for the proxy server
	// to use when communicating with the source apiserver.
	// +optional
	SourceClientConnection ClientConnection `json:"sourceClientConnection"`
	// TargetClientConnection specifies the client connection settings for the proxy server
	// to use when communicating with the target apiserver.
	// +optional
	TargetClientConnection *ClientConnection `json:"targetClientConnection,omitempty"`
	// LeaderElection defines the configuration of leader election client.
	LeaderElection componentbaseconfigv1alpha1.LeaderElectionConfiguration `json:"leaderElection"`
	// Server defines the configuration of the HTTP server.
	Server ServerConfiguration `json:"server"`
	// Debugging holds configuration for Debugging related features.
	// +optional
	Debugging *componentbaseconfigv1alpha1.DebuggingConfiguration `json:"debugging,omitempty"`
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string `json:"logLevel"`
	// LogFormat is the output format for the logs. Must be one of [text,json].
	LogFormat string `json:"logFormat"`
	// Controllers defines the configuration of the controllers.
	Controllers ResourceManagerControllerConfiguration `json:"controllers"`
	// Webhooks defines the configuration of the webhooks.
	Webhooks ResourceManagerWebhookConfiguration `json:"webhooks"`
}

// ClientConnection specifies the client connection settings to use when communicating with an API server.
type ClientConnection struct {
	componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:",inline"`
	// Namespaces in which the ManagedResources should be observed (defaults to "all namespaces").
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`
	// CacheResyncPeriod specifies the duration how often the cache for the cluster is resynced.
	// +optional
	CacheResyncPeriod *metav1.Duration `json:"cacheResyncPeriod,omitempty"`
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
	// Webhooks is the configuration for the HTTPS webhook server.
	Webhooks HTTPSServer `json:"webhooks"`
	// HealthProbes is the configuration for serving the healthz and readyz endpoints.
	// +optional
	HealthProbes *Server `json:"healthProbes,omitempty"`
	// Metrics is the configuration for serving the metrics endpoint.
	// +optional
	Metrics *Server `json:"metrics,omitempty"`
}

// Server contains information for HTTP(S) server configuration.
type Server struct {
	// BindAddress is the IP address on which to listen for the specified port.
	BindAddress string `json:"bindAddress"`
	// Port is the port on which to serve unsecured, unauthenticated access.
	Port int `json:"port"`
}

// HTTPSServer is the configuration for the HTTPSServer server.
type HTTPSServer struct {
	// Server is the configuration for the bind address and the port.
	Server `json:",inline"`
	// TLSServer contains information about the TLS configuration for an HTTPS server.
	TLS TLSServer `json:"tls"`
}

// TLSServer contains information about the TLS configuration for an HTTPS server.
type TLSServer struct {
	// ServerCertDir is the path to a directory containing the server's TLS certificate and key (the files must be
	// named tls.crt and tls.key respectively).
	ServerCertDir string `json:"serverCertDir"`
}

// ResourceManagerControllerConfiguration defines the configuration of the controllers.
type ResourceManagerControllerConfiguration struct {
	// ClusterID is the ID of the source cluster.
	// +optional
	ClusterID *string `json:"clusterID,omitempty"`
	// ResourceClass is the name of the class in ManagedResources to filter for.
	// +optional
	ResourceClass *string `json:"resourceClass,omitempty"`

	// GarbageCollector is the configuration for the garbage-collector controller.
	GarbageCollector GarbageCollectorControllerConfig `json:"garbageCollector"`
	// Health is the configuration for the health controller.
	Health HealthControllerConfig `json:"health"`
	// CSRApprover is the configuration for the csr-approver controller.
	CSRApprover CSRApproverControllerConfig `json:"csrApprover"`
	// ManagedResource is the configuration for the managed resource controller.
	ManagedResource ManagedResourceControllerConfig `json:"managedResource"`
	// NetworkPolicy is the configuration for the networkpolicy controller.
	NetworkPolicy NetworkPolicyControllerConfig `json:"networkPolicy"`
	// NodeCriticalComponents is the configuration for the node critical components controller.
	NodeCriticalComponents NodeCriticalComponentsControllerConfig `json:"nodeCriticalComponents"`
	// NodeAgentReconciliationDelay is the configuration for the node-agent reconciliation delay controller.
	NodeAgentReconciliationDelay NodeAgentReconciliationDelayControllerConfig `json:"nodeAgentReconciliationDelay"`
	// TokenRequestor is the configuration for the token-requestor controller.
	TokenRequestor TokenRequestorControllerConfig `json:"tokenRequestor"`
}

// CSRApproverControllerConfig is the configuration for the csr-approver controller.
type CSRApproverControllerConfig struct {
	// Enabled defines whether this controller is enabled.
	Enabled bool `json:"enabled"`
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// MachineNamespace is the namespace in the source cluster in which the Machine objects are stored.
	MachineNamespace string `json:"machineNamespace"`
}

// GarbageCollectorControllerConfig is the configuration for the garbage-collector controller.
type GarbageCollectorControllerConfig struct {
	// Enabled defines whether this controller is enabled.
	Enabled bool `json:"enabled"`
	// SyncPeriod is the duration how often the controller performs its reconciliation.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
}

// HealthControllerConfig is the configuration for the health controller.
type HealthControllerConfig struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the controller performs its reconciliation.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
}

// ManagedResourceControllerConfig is the configuration for the managed resource controller.
type ManagedResourceControllerConfig struct {
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// SyncPeriod is the duration how often the controller performs its reconciliation.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`
	// AlwaysUpdate specifies whether resources will only be updated if their desired state differs from the actual
	// state. If true, an update request will be sent in each reconciliation independent of this condition.
	// +optional
	AlwaysUpdate *bool `json:"alwaysUpdate,omitempty"`
	// ManagedByLabelValue is the value that is used for labeling all resources managed by the controller. The labels
	// will have key `resources.gardener.cloud/managed-by`.
	// Default: gardener
	// +optional
	ManagedByLabelValue *string `json:"managedByLabelValue,omitempty"`
}

// NetworkPolicyControllerConfig is the configuration for the networkpolicy controller.
type NetworkPolicyControllerConfig struct {
	// Enabled defines whether this controller is enabled.
	Enabled bool `json:"enabled"`
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// NamespaceSelectors is a list of label selectors for namespaces in which the controller shall reconcile Service
	// objects. An empty list means all namespaces.
	// +optional
	NamespaceSelectors []metav1.LabelSelector `json:"namespaceSelectors,omitempty"`
	// IngressControllerSelector contains the pod selector and namespace for an ingress controller. If provided, this
	// NetworkPolicy controller watches Ingress resources and automatically creates NetworkPolicy resources allowing
	// the respective ingress/egress traffic for the backends exposed by the Ingresses.
	// +optional
	IngressControllerSelector *IngressControllerSelector `json:"ingressControllerSelector,omitempty"`
}

// IngressControllerSelector contains the pod selector and namespace for an ingress controller.
type IngressControllerSelector struct {
	// Namespace is the name of the namespace in which the ingress controller runs.
	Namespace string `json:"namespace"`
	// PodSelector is the selector for the ingress controller pods.
	PodSelector metav1.LabelSelector `json:"podSelector"`
}

// TokenRequestorControllerConfig is the configuration for the token-requestor controller.
type TokenRequestorControllerConfig struct {
	// Enabled defines whether this controller is enabled.
	Enabled bool `json:"enabled"`
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
}

// NodeCriticalComponentsControllerConfig is the configuration for the node critical components controller.
type NodeCriticalComponentsControllerConfig struct {
	// Enabled defines whether this controller is enabled.
	Enabled bool `json:"enabled"`
	// ConcurrentSyncs is the number of concurrent worker routines for this controller.
	// +optional
	ConcurrentSyncs *int `json:"concurrentSyncs,omitempty"`
	// Backoff is the duration to use as backoff when Nodes have non-ready node-critical pods (defaults to 10s).
	// +optional
	Backoff *metav1.Duration `json:"backoff,omitempty"`
}

// NodeAgentReconciliationDelayControllerConfig is the configuration for the node-agent reconciliation delay controller.
type NodeAgentReconciliationDelayControllerConfig struct {
	// Enabled defines whether this controller is enabled.
	Enabled bool `json:"enabled"`
	// MinDelay is the minimum duration to use for delays (default: 0s).
	// +optional
	MinDelay *metav1.Duration `json:"minDelay,omitempty"`
	// MaxDelay is the maximum duration to use for delays (default: 5m).
	// +optional
	MaxDelay *metav1.Duration `json:"maxDelay,omitempty"`
}

// ResourceManagerWebhookConfiguration defines the configuration of the webhooks.
type ResourceManagerWebhookConfiguration struct {
	// CRDDeletionProtection is the configuration for the crd-deletion-protection webhook.
	CRDDeletionProtection CRDDeletionProtection `json:"crdDeletionProtection"`
	// EndpointSliceHints is the configuration for the endpoint-slice-hints webhook.
	EndpointSliceHints EndpointSliceHintsWebhookConfig `json:"endpointSliceHints"`
	// ExtensionValidation is the configuration for the extension-validation webhook.
	ExtensionValidation ExtensionValidation `json:"extensionValidation"`
	// HighAvailabilityConfig is the configuration for the high-availability-config webhook.
	HighAvailabilityConfig HighAvailabilityConfigWebhookConfig `json:"highAvailabilityConfig"`
	// KubernetesServiceHost is the configuration for the kubernetes-service-host webhook.
	KubernetesServiceHost KubernetesServiceHostWebhookConfig `json:"kubernetesServiceHost"`
	// SystemComponentsConfig is the configuration for the system-components-config webhook.
	SystemComponentsConfig SystemComponentsConfigWebhookConfig `json:"systemComponentsConfig"`
	// PodSchedulerName is the configuration for the pod-scheduler-name webhook.
	PodSchedulerName PodSchedulerNameWebhookConfig `json:"podSchedulerName"`
	// PodTopologySpreadConstraints is the configuration for the pod-topology-spread-constraints webhook.
	PodTopologySpreadConstraints PodTopologySpreadConstraintsWebhookConfig `json:"podTopologySpreadConstraints"`
	// ProjectedTokenMount is the configuration for the projected-token-mount webhook.
	ProjectedTokenMount ProjectedTokenMountWebhookConfig `json:"projectedTokenMount"`
	// NodeAgentAuthorizer is the configuration for the node-agent-authorizer webhook.
	NodeAgentAuthorizer NodeAgentAuthorizerWebhookConfig `json:"nodeAgentAuthorizer"`
	// SeccompProfile is the configuration for the seccomp-profile webhook.
	SeccompProfile SeccompProfileWebhookConfig `json:"seccompProfile"`
}

// CRDDeletionProtection is the configuration for the crd-deletion-protection webhook.
type CRDDeletionProtection struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
}

// EndpointSliceHintsWebhookConfig is the configuration for the endpoint-slice-hints webhook.
type EndpointSliceHintsWebhookConfig struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
}

// ExtensionValidation is the configuration for the extension-validation webhook.
type ExtensionValidation struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
}

// HighAvailabilityConfigWebhookConfig is the configuration for the high-availability-config webhook.
type HighAvailabilityConfigWebhookConfig struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
	// DefaultNotReadyTolerationSeconds specifies the seconds for the `node.kubernetes.io/not-ready` toleration that
	// should be added to pods not already tolerating this taint.
	// +optional
	DefaultNotReadyTolerationSeconds *int64 `json:"defaultNotReadyTolerationSeconds,omitempty"`
	// DefaultUnreachableTolerationSeconds specifies the seconds for the `node.kubernetes.io/unreachable` toleration that
	// should be added to pods not already tolerating this taint.
	// +optional
	DefaultUnreachableTolerationSeconds *int64 `json:"defaultUnreachableTolerationSeconds,omitempty"`
}

// KubernetesServiceHostWebhookConfig is the configuration for the kubernetes-service-host webhook.
type KubernetesServiceHostWebhookConfig struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
	// Host is the FQDN of the API server.
	Host string `json:"host"`
}

// SystemComponentsConfigWebhookConfig is the configuration for the system-components-config webhook.
type SystemComponentsConfigWebhookConfig struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
	// NodeSelector is the selector used to retrieve nodes that run system components.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// PodNodeSelector is the node selector that should be added to pods.
	// +optional
	PodNodeSelector map[string]string `json:"podNodeSelector,omitempty"`
	// PodTolerations are the tolerations that should be added to pods.
	// +optional
	PodTolerations []corev1.Toleration `json:"podTolerations,omitempty"`
}

// PodSchedulerNameWebhookConfig is the configuration for the pod-scheduler-name webhook.
type PodSchedulerNameWebhookConfig struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
	// SchedulerName is the name of the scheduler that should be written into the .spec.schedulerName of pod resources.
	// +optional
	SchedulerName *string `json:"schedulerName,omitempty"`
}

// PodTopologySpreadConstraintsWebhookConfig is the configuration for the pod-topology-spread-constraints webhook.
type PodTopologySpreadConstraintsWebhookConfig struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
}

// ProjectedTokenMountWebhookConfig is the configuration for the projected-token-mount webhook.
type ProjectedTokenMountWebhookConfig struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
	// ExpirationSeconds is the number of seconds until mounted projected service account tokens expire.
	// +optional
	ExpirationSeconds *int64 `json:"expirationSeconds,omitempty"`
}

// NodeAgentAuthorizerWebhookConfig is the configuration for the node-agent-authorizer webhook.
type NodeAgentAuthorizerWebhookConfig struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
	// MachineNamespace is the namespace in the source cluster in which the Machine objects are stored.
	MachineNamespace string `json:"machineNamespace"`
	// AuthorizeWithSelectors defines whether authorization is allowed to use field selectors.
	// +optional
	AuthorizeWithSelectors *bool `json:"authorizeWithSelectors,omitempty"`
}

// SeccompProfileWebhookConfig is the configuration for the seccomp-profile webhook.
type SeccompProfileWebhookConfig struct {
	// Enabled defines whether this webhook is enabled.
	Enabled bool `json:"enabled"`
}

const (
	// DefaultResourceClass is used as resource class if no class is specified on the command line.
	DefaultResourceClass = "resources"
	// AllResourceClass is used as resource class when all values for resource classes should be covered.
	AllResourceClass = "*"
)
