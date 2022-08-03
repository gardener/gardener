// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// ManagedBy is a constant for a label on an object managed by a ManagedResource.
	// It is set by the ManagedResource controller depending on its configuration. By default it is set to "gardener".
	ManagedBy = "resources.gardener.cloud/managed-by"

	// StaticTokenSkip is a constant for a label on a ServiceAccount which indicates that this ServiceAccount should not
	// be considered by this controller.
	StaticTokenSkip = "token-invalidator.resources.gardener.cloud/skip"
	// StaticTokenConsider is a constant for a label on a Secret which indicates that this Secret should be considered
	// for the invalidation of the static ServiceAccount token.
	StaticTokenConsider = "token-invalidator.resources.gardener.cloud/consider"

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
	// LabelPurposeTokenInvalidation is a constant for a label value indicating that this secret should be considered by
	// the token-invalidator.
	LabelPurposeTokenInvalidation = "token-invalidator"

	// ServiceAccountName is the key of an annotation of a secret whose value contains the service account name.
	ServiceAccountName = "serviceaccount.resources.gardener.cloud/name"
	// ServiceAccountNamespace is the key of an annotation of a secret whose value contains the service account
	// namespace.
	ServiceAccountNamespace = "serviceaccount.resources.gardener.cloud/namespace"
	// ServiceAccountTokenExpirationDuration is the key of an annotation of a secret whose value contains the expiration
	// duration of the token created.
	ServiceAccountTokenExpirationDuration = "serviceaccount.resources.gardener.cloud/token-expiration-duration"
	// ServiceAccountTokenRenewTimestamp is the key of an annotation of a secret whose value contains the timestamp when
	// the token needs to be renewed.
	ServiceAccountTokenRenewTimestamp = "serviceaccount.resources.gardener.cloud/token-renew-timestamp"

	// DataKeyToken is the data key whose value contains a service account token.
	DataKeyToken = "token"
	// DataKeyKubeconfig is the data key whose value contains a kubeconfig with a service account token.
	DataKeyKubeconfig = "kubeconfig"

	// ProjectedTokenSkip is a constant for a label on a Pod which indicates that this Pod should not be considered for
	// an automatic mount of a projected ServiceAccount token.
	ProjectedTokenSkip = "projected-token-mount.resources.gardener.cloud/skip"
	// ProjectedTokenExpirationSeconds is a constant for an annotation on a Pod which overwrites the default token expiration
	// seconds for the automatic mount of a projected ServiceAccount token.
	ProjectedTokenExpirationSeconds = "projected-token-mount.resources.gardener.cloud/expiration-seconds"

	// SeccompProfileSkip is a constant for a label on a Pod which indicates that this Pod should not be considered for
	// defaulting of its seccomp profile.
	SeccompProfileSkip = "seccompprofile.resources.gardener.cloud/skip"
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
