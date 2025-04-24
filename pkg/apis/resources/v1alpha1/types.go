// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

const (
	// Ignore is an annotation that dictates whether a resources should be ignored during
	// reconciliation.
	Ignore = "resources.gardener.cloud/ignore"
	// SkipHealthCheck is an annotation that dictates whether a resource should be ignored during health check.
	SkipHealthCheck = "resources.gardener.cloud/skip-health-check"
	// DeleteOnInvalidUpdate is a constant for an annotation on a resource managed by a ManagedResource. If set to
	// true then the controller will delete the object in case it faces an "Invalid" response during an update operation.
	DeleteOnInvalidUpdate = "resources.gardener.cloud/delete-on-invalid-update"
	// KeepObject is a constant for an annotation on a resource managed by a ManagedResource. If set to
	// true then the controller will not delete the object in case it is removed from the ManagedResource or the
	// ManagedResource itself is deleted.
	KeepObject = "resources.gardener.cloud/keep-object"
	// Mode is a constant for an annotation on a resource managed by a ManagedResource. It indicates the
	// mode that should be used to reconcile the resource.
	Mode = "resources.gardener.cloud/mode"
	// ModeIgnore is a constant for the value of the mode annotation describing an ignore mode.
	// Reconciliation in ignore mode removes the resource from the ManagedResource status and does not
	// perform any action on the cluster.
	ModeIgnore = "Ignore"
	// PreserveReplicas is a constant for an annotation on a resource managed by a ManagedResource. If set to
	// true then the controller will keep the `spec.replicas` field's value during updates to the resource.
	PreserveReplicas = "resources.gardener.cloud/preserve-replicas"
	// PreserveResources is a constant for an annotation on a resource managed by a ManagedResource. If set to
	// true then the controller will keep the resource requests and limits in Pod templates (e.g. in a
	// DeploymentSpec) during updates to the resource. This applies for all containers.
	PreserveResources = "resources.gardener.cloud/preserve-resources"
	// OriginAnnotation is a constant for an annotation on a resource managed by a ManagedResource.
	// It is set by the ManagedResource controller to the key of the owning ManagedResource, optionally prefixed with the
	// clusterID.
	OriginAnnotation = "resources.gardener.cloud/origin"
	// FinalizeDeletionAfter is an annotation on an object part of a ManagedResource that whose value states the
	// duration after which a deletion should be finalized (i.e., removal of `.metadata.finalizers[]`).
	FinalizeDeletionAfter = "resources.gardener.cloud/finalize-deletion-after"
	// BrotliCompressionSuffix is the common suffix used for Brotli compression.
	BrotliCompressionSuffix = ".br"
	// CompressedDataKey is the name of a data key containing Brotli compressed YAML manifests.
	CompressedDataKey = "data.yaml" + BrotliCompressionSuffix

	// ManagedBy is a constant for a label on an object managed by a ManagedResource.
	// It is set by the ManagedResource controller depending on its configuration. By default it is set to "gardener".
	ManagedBy = "resources.gardener.cloud/managed-by"

	// GardenerManager is a constant for the default value of the 'ManagedBy' label.
	GardenerManager = "gardener"

	// TokenRequestorTargetSecretName is a constant for an annotation on a Secret which indicates that the token requestor
	// shall sync the token to a secret in the target cluster with the given name.
	TokenRequestorTargetSecretName = "token-requestor.resources.gardener.cloud/target-secret-name"
	// TokenRequestorTargetSecretNamespace is a constant for an annotation on a Secret which indicates that the token
	// requestor shall sync the token to a secret in the target cluster with the given namespace.
	TokenRequestorTargetSecretNamespace = "token-requestor.resources.gardener.cloud/target-secret-namespace"

	// ResourceManagerPurpose is a constant for the key in a label describing the purpose of the respective object
	// reconciled by the resource manager.
	ResourceManagerPurpose = "resources.gardener.cloud/purpose"
	// LabelPurposeTokenRequest is a constant for a label value indicating that this secret should be reconciled by the
	// token-requestor.
	LabelPurposeTokenRequest = "token-requestor"
	// ResourceManagerClass is a constant for the key in a label describing the class of the respective object. This can
	// be used to differentiate between multiple instances of the same controller (e.g., token-requestor).
	ResourceManagerClass = "resources.gardener.cloud/class"
	// ResourceManagerClassGarden is a constant for the 'garden' class.
	ResourceManagerClassGarden = "garden"
	// ResourceManagerClassShoot is a constant for the 'shoot' class.
	ResourceManagerClassShoot = "shoot"

	// ServiceAccountName is the key of an annotation of a secret whose value contains the service account name.
	ServiceAccountName = "serviceaccount.resources.gardener.cloud/name"
	// ServiceAccountNamespace is the key of an annotation of a secret whose value contains the service account
	// namespace.
	ServiceAccountNamespace = "serviceaccount.resources.gardener.cloud/namespace"
	// ServiceAccountLabels is the key of an annotation of a secret whose value contains the service account
	// labels.
	ServiceAccountLabels = "serviceaccount.resources.gardener.cloud/labels"
	// ServiceAccountTokenExpirationDuration is the key of an annotation of a secret whose value contains the expiration
	// duration of the token created.
	ServiceAccountTokenExpirationDuration = "serviceaccount.resources.gardener.cloud/token-expiration-duration"
	// ServiceAccountTokenRenewTimestamp is the key of an annotation of a secret whose value contains the timestamp when
	// the token needs to be renewed.
	ServiceAccountTokenRenewTimestamp = "serviceaccount.resources.gardener.cloud/token-renew-timestamp"
	// ServiceAccountInjectCABundle instructs the Token Requester to also write the CA bundle.
	ServiceAccountInjectCABundle = "serviceaccount.resources.gardener.cloud/inject-ca-bundle"

	// DataKeyToken is the data key whose value contains a service account token.
	DataKeyToken = "token"
	// DataKeyCABundle is the data key where the ca bundle is stored.
	DataKeyCABundle = "bundle.crt"
	// DataKeyKubeconfig is the data key whose value contains a kubeconfig with a service account token.
	DataKeyKubeconfig = "kubeconfig"

	// ProjectedTokenSkip is a constant for a label on a Pod which indicates that this Pod should not be considered for
	// an automatic mount of a projected ServiceAccount token.
	ProjectedTokenSkip = "projected-token-mount.resources.gardener.cloud/skip"
	// ProjectedTokenExpirationSeconds is a constant for an annotation on a Pod which overwrites the default token expiration
	// seconds for the automatic mount of a projected ServiceAccount token.
	ProjectedTokenExpirationSeconds = "projected-token-mount.resources.gardener.cloud/expiration-seconds"
	// ProjectedTokenFileMode is a constant for an annotation on a Pod which overwrites the default file mode for the automatic mount of a
	// projected ServiceAccount token.
	ProjectedTokenFileMode = "projected-token-mount.resources.gardener.cloud/file-mode"

	// HighAvailabilityConfigConsider is a constant for a label on a Namespace which indicates that the workload
	// resources in this namespace should be considered by the HA config webhook.
	HighAvailabilityConfigConsider = "high-availability-config.resources.gardener.cloud/consider"
	// HighAvailabilityConfigSkip is a constant for a label on a resource which indicates that this resource should not
	// be considered by the HA config webhook.
	HighAvailabilityConfigSkip = "high-availability-config.resources.gardener.cloud/skip"
	// HighAvailabilityConfigFailureToleranceType is a constant for a label on a Namespace which describes the HA
	// failure tolerance type.
	HighAvailabilityConfigFailureToleranceType = "high-availability-config.resources.gardener.cloud/failure-tolerance-type"
	// HighAvailabilityConfigZones is a constant for an annotation on a Namespace which describes the availability
	// zones are used.
	HighAvailabilityConfigZones = "high-availability-config.resources.gardener.cloud/zones"
	// HighAvailabilityConfigZonePinning is a constant for an annotation on a Namespace which enables pinning of
	// workload to the specified zones.
	HighAvailabilityConfigZonePinning = "high-availability-config.resources.gardener.cloud/zone-pinning"
	// HighAvailabilityConfigType is a constant for a label on a resource which describes which component type it is.
	HighAvailabilityConfigType = "high-availability-config.resources.gardener.cloud/type"
	// HighAvailabilityConfigHostSpread is a constant for an annotation on a resource which enforces a topology spread
	// constraint across hosts.
	HighAvailabilityConfigHostSpread = "high-availability-config.resources.gardener.cloud/host-spread"
	// HighAvailabilityConfigTypeController is a constant for a label value on a resource describing it's a controller.
	HighAvailabilityConfigTypeController = "controller"
	// HighAvailabilityConfigTypeServer is a constant for a label value on a resource describing it's a (webhook)
	// server.
	HighAvailabilityConfigTypeServer = "server"
	// HighAvailabilityConfigReplicas is a constant for an annotation on a resource which overwrites the desired replica
	// count.
	HighAvailabilityConfigReplicas = "high-availability-config.resources.gardener.cloud/replicas"

	// SeccompProfileSkip is a constant for a label on a Pod which indicates that this Pod should not be considered for
	// defaulting of its seccomp profile.
	SeccompProfileSkip = "seccompprofile.resources.gardener.cloud/skip"

	// KubernetesServiceHostInject is a constant for a label on a Pod or a Namespace which indicates that all pods in
	// this namespace (or the specific pod) should not be considered for injection of the KUBERNETES_SERVICE_HOST
	// environment variable.
	KubernetesServiceHostInject = "apiserver-proxy.networking.gardener.cloud/inject"

	// SystemComponentsConfigSkip is a constant for a label on a Pod which indicates that this Pod should not be considered for
	// adding default node selector and tolerations.
	SystemComponentsConfigSkip = "system-components-config.resources.gardener.cloud/skip"

	// PodTopologySpreadConstraintsSkip is a constant for a label on a Pod which indicates that this Pod should not be considered for
	// adding the pod-template-hash selector to the topology spread constraint.
	PodTopologySpreadConstraintsSkip = "topology-spread-constraints.resources.gardener.cloud/skip"

	// EndpointSliceHintsConsider is a constant for a label on an Service which indicates that the EndpointSlices of the
	// Service should be considered by the EndpointSlice hints webhook. This label is added to the Service object, Kubernetes
	// maintains the Service label as EndpointSlice label. Finally, the EndpointSlice hints webhook mutates EndpointSlice resources
	// containing this label.
	EndpointSliceHintsConsider = "endpoint-slice-hints.resources.gardener.cloud/consider"

	// NetworkingNamespaceSelectors is a constant for an annotation on a Service which contains a list of namespace
	// selectors. By default, NetworkPolicy resources are only created in the Service's namespace. If any selector is
	// present, NetworkPolicy resources are also created in all namespaces matching any of the provided selectors.
	NetworkingNamespaceSelectors = "networking.resources.gardener.cloud/namespace-selectors"
	// NetworkingPodLabelSelectorNamespaceAlias is a constant for an annotation on a Service which describes the label
	// that can be used to define an alias for the namespace name in the default pod label selector. This is helpful for
	// scenarios where the target service can exist n-times in multiple namespaces and a component needs to talk to all
	// of them but doesn't know the namespace names upfront.
	NetworkingPodLabelSelectorNamespaceAlias = "networking.resources.gardener.cloud/pod-label-selector-namespace-alias"
	// NetworkingFromWorldToPorts is a constant for an annotation on a Service which contains a list of ports to which
	// ingress traffic from everywhere shall be allowed.
	NetworkingFromWorldToPorts = "networking.resources.gardener.cloud/from-world-to-ports"
	// NetworkPolicyFromPolicyAnnotationPrefix is a constant for an annotation key prefix on a Service which contains
	// the label selector alias which is used by pods initiating the communication to this Service. The annotation key
	// must be suffixed with NetworkPolicyFromPolicyAnnotationSuffix, and the annotations value must be a list of
	// container ports (not service ports).
	NetworkPolicyFromPolicyAnnotationPrefix = "networking.resources.gardener.cloud/from-"
	// NetworkPolicyFromPolicyAnnotationSuffix is a constant for an annotation key suffix on a Service which contains
	// the label selector alias which is used by pods initiating the communication to this Service. The annotation key
	// must be prefixed with NetworkPolicyFromPolicyAnnotationPrefix, and the annotations value must be a list of
	// container ports (not service ports).
	NetworkPolicyFromPolicyAnnotationSuffix = "-allowed-ports"
	// NetworkingServiceName is a constant for a label on a NetworkPolicy which contains the name of the Service is has
	// been created for.
	NetworkingServiceName = "networking.resources.gardener.cloud/service-name"
	// NetworkingServiceNamespace is a constant for a label on a NetworkPolicy which contains the namespace of the
	// Service is has been created for.
	NetworkingServiceNamespace = "networking.resources.gardener.cloud/service-namespace"
)

// +kubebuilder:resource:shortName="mr"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Class",type=string,JSONPath=`.spec.class`,description="The class identifies which resource manager is responsible for this ManagedResource."
// +kubebuilder:printcolumn:name="Applied",type=string,JSONPath=`.status.conditions[?(@.type=="ResourcesApplied")].status`,description=" Indicates whether all resources have been applied."
// +kubebuilder:printcolumn:name="Healthy",type=string,JSONPath=`.status.conditions[?(@.type=="ResourcesHealthy")].status`,description="Indicates whether all resources are healthy."
// +kubebuilder:printcolumn:name="Progressing",type=string,JSONPath=`.status.conditions[?(@.type=="ResourcesProgressing")].status`,description="Indicates whether some resources are still progressing to be rolled out."
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="creation timestamp"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedResource describes a list of managed resources.
type ManagedResource struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec contains the specification of this managed resource.
	Spec ManagedResourceSpec `json:"spec,omitempty"`
	// Status contains the status of this managed resource.
	Status ManagedResourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedResourceList is a list of ManagedResource resources.
type ManagedResourceList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of ManagedResource.
	Items []ManagedResource `json:"items"`
}

// ManagedResourceSpec contains the specification of this managed resource.
type ManagedResourceSpec struct {
	// Class holds the resource class used to control the responsibility for multiple resource manager instances
	// +optional
	Class *string `json:"class,omitempty"`
	// SecretRefs is a list of secret references.
	SecretRefs []corev1.LocalObjectReference `json:"secretRefs"`
	// InjectLabels injects the provided labels into every resource that is part of the referenced secrets.
	// +optional
	InjectLabels map[string]string `json:"injectLabels,omitempty"`
	// ForceOverwriteLabels specifies that all existing labels should be overwritten. Defaults to false.
	// +optional
	ForceOverwriteLabels *bool `json:"forceOverwriteLabels,omitempty"`
	// ForceOverwriteAnnotations specifies that all existing annotations should be overwritten. Defaults to false.
	// +optional
	ForceOverwriteAnnotations *bool `json:"forceOverwriteAnnotations,omitempty"`
	// KeepObjects specifies whether the objects should be kept although the managed resource has already been deleted.
	// Defaults to false.
	// +optional
	KeepObjects *bool `json:"keepObjects,omitempty"`
	// Equivalences specifies possible group/kind equivalences for objects.
	// +optional
	Equivalences [][]metav1.GroupKind `json:"equivalences,omitempty"`
	// DeletePersistentVolumeClaims specifies if PersistentVolumeClaims created by StatefulSets, which are managed by this
	// resource, should also be deleted when the corresponding StatefulSet is deleted (defaults to false).
	// +optional
	DeletePersistentVolumeClaims *bool `json:"deletePersistentVolumeClaims,omitempty"`
}

// ManagedResourceStatus is the status of a managed resource.
type ManagedResourceStatus struct {
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Resources is a list of objects that have been created.
	// +optional
	Resources []ObjectReference `json:"resources,omitempty"`
	// SecretsDataChecksum is the checksum of referenced secrets data.
	// +optional
	SecretsDataChecksum *string `json:"secretsDataChecksum,omitempty"`
}

// ObjectReference is a reference to another object.
type ObjectReference struct {
	corev1.ObjectReference `json:",inline"`
	// Labels is a map of labels that were used during last update of the resource.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations is a map of annotations that were used during last update of the resource.
	Annotations map[string]string `json:"annotations,omitempty"`
}

const (
	// ResourcesApplied is a condition type that indicates whether all resources are applied to the target cluster.
	ResourcesApplied gardencorev1beta1.ConditionType = "ResourcesApplied"
	// ResourcesHealthy is a condition type that indicates whether all resources are present and healthy.
	ResourcesHealthy gardencorev1beta1.ConditionType = "ResourcesHealthy"
	// ResourcesProgressing is a condition type that indicates whether some resources are still progressing to be rolled out.
	ResourcesProgressing gardencorev1beta1.ConditionType = "ResourcesProgressing"
)

// These are well-known reasons for Conditions.
const (
	// ConditionApplySucceeded indicates that the `ResourcesApplied` condition is `True`,
	// because all resources have been applied successfully.
	ConditionApplySucceeded = "ApplySucceeded"
	// ConditionApplyFailed indicates that the `ResourcesApplied` condition is `False`,
	// because applying the resources failed.
	ConditionApplyFailed = "ApplyFailed"
	// ConditionDecodingFailed indicates that the `ResourcesApplied` condition is `False`,
	// because decoding the resources of the ManagedResource failed.
	ConditionDecodingFailed = "DecodingFailed"
	// ConditionApplyProgressing indicates that the `ResourcesApplied` condition is `Progressing`,
	// because the resources are currently being reconciled.
	ConditionApplyProgressing = "ApplyProgressing"
	// ConditionDeletionFailed indicates that the `ResourcesApplied` condition is `False`,
	// because deleting the resources failed.
	ConditionDeletionFailed = "DeletionFailed"
	// ConditionDeletionPending indicates that the `ResourcesApplied` condition is `Progressing`,
	// because the deletion of some resources is still pending.
	ConditionDeletionPending = "DeletionPending"
	// ReleaseOfOrphanedResourcesFailed indicates that the `ResourcesApplied` condition is `False`,
	// because the release of orphaned resources failed.
	ReleaseOfOrphanedResourcesFailed = "ReleaseOfOrphanedResourcesFailed"
	// ConditionManagedResourceIgnored indicates that the ManagedResource's conditions are not checked,
	// because the ManagedResource is marked to be ignored.
	ConditionManagedResourceIgnored = "ManagedResourceIgnored"
	// ConditionChecksPending indicates that the `ResourcesProgressing` condition is `Unknown`,
	// because the condition checks have not been completely executed yet for the current set of resources.
	ConditionChecksPending = "ChecksPending"
)
