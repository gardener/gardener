// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Project holds certain properties about a Gardener project.
type Project struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the project properties.
	// +optional
	Spec ProjectSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Most recently observed status of the Project.
	// +optional
	Status ProjectStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectList is a collection of Projects.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is the list of Projects.
	Items []Project `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ProjectSpec is the specification of a Project.
type ProjectSpec struct {
	// CreatedBy is a subject representing a user name, an email address, or any other identifier of a user
	// who created the project. This field is immutable.
	// +optional
	CreatedBy *rbacv1.Subject `json:"createdBy,omitempty" protobuf:"bytes,1,opt,name=createdBy"`
	// Description is a human-readable description of what the project is used for.
	// Only letters, digits and certain punctuation characters are allowed for this field.
	// +optional
	Description *string `json:"description,omitempty" protobuf:"bytes,2,opt,name=description"`
	// Owner is a subject representing a user name, an email address, or any other identifier of a user owning
	// the project.
	// IMPORTANT: Be aware that this field will be removed in the `v1` version of this API in favor of the `owner`
	// role. The only way to change the owner will be by moving the `owner` role. In this API version the only way
	// to change the owner is to use this field.
	// +optional
	// TODO: Remove this field in favor of the `owner` role in `v1`.
	Owner *rbacv1.Subject `json:"owner,omitempty" protobuf:"bytes,3,opt,name=owner"`
	// Purpose is a human-readable explanation of the project's purpose.
	// Only letters, digits and certain punctuation characters are allowed for this field.
	// +optional
	Purpose *string `json:"purpose,omitempty" protobuf:"bytes,4,opt,name=purpose"`
	// Members is a list of subjects representing a user name, an email address, or any other identifier of a user,
	// group, or service account that has a certain role.
	// +optional
	Members []ProjectMember `json:"members,omitempty" protobuf:"bytes,5,rep,name=members"`
	// Namespace is the name of the namespace that has been created for the Project object.
	// A nil value means that Gardener will determine the name of the namespace.
	// If set, its value must be prefixed with `garden-`.
	// This field is immutable.
	// +optional
	Namespace *string `json:"namespace,omitempty" protobuf:"bytes,6,opt,name=namespace"`
	// Tolerations contains the tolerations for taints on seed clusters.
	// +optional
	Tolerations *ProjectTolerations `json:"tolerations,omitempty" protobuf:"bytes,7,opt,name=tolerations"`
	// DualApprovalForDeletion contains configuration for the dual approval concept for resource deletion.
	// +optional
	DualApprovalForDeletion []DualApprovalForDeletion `json:"dualApprovalForDeletion,omitempty" protobuf:"bytes,8,opt,name=dualApprovalForDeletion"`
}

// ProjectStatus holds the most recently observed status of the project.
type ProjectStatus struct {
	// ObservedGeneration is the most recent generation observed for this project.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,1,opt,name=observedGeneration"`
	// Phase is the current phase of the project.
	Phase ProjectPhase `json:"phase,omitempty" protobuf:"bytes,2,opt,name=phase,casttype=ProjectPhase"`
	// StaleSinceTimestamp contains the timestamp when the project was first discovered to be stale/unused.
	// +optional
	StaleSinceTimestamp *metav1.Time `json:"staleSinceTimestamp,omitempty" protobuf:"bytes,3,opt,name=staleSinceTimestamp"`
	// StaleAutoDeleteTimestamp contains the timestamp when the project will be garbage-collected/automatically deleted
	// because it's stale/unused.
	// +optional
	StaleAutoDeleteTimestamp *metav1.Time `json:"staleAutoDeleteTimestamp,omitempty" protobuf:"bytes,4,opt,name=staleAutoDeleteTimestamp"`
	// LastActivityTimestamp contains the timestamp from the last activity performed in this project.
	// +optional
	LastActivityTimestamp *metav1.Time `json:"lastActivityTimestamp,omitempty" protobuf:"bytes,5,opt,name=lastActivityTimestamp"`
}

// ProjectMember is a member of a project.
type ProjectMember struct {
	// Subject is representing a user name, an email address, or any other identifier of a user, group, or service
	// account that has a certain role.
	rbacv1.Subject `json:",inline" protobuf:"bytes,1,opt,name=subject"`

	// Role represents the role of this member.
	// IMPORTANT: Be aware that this field will be removed in the `v1` version of this API in favor of the `roles`
	// list.
	// TODO: Remove this field in favor of the `roles` list in `v1`.
	Role string `json:"role" protobuf:"bytes,2,opt,name=role"`
	// Roles represents the list of roles of this member.
	// +optional
	Roles []string `json:"roles,omitempty" protobuf:"bytes,3,rep,name=roles"`
}

// ProjectTolerations contains the tolerations for taints on seed clusters.
type ProjectTolerations struct {
	// Defaults contains a list of tolerations that are added to the shoots in this project by default.
	// +patchMergeKey=key
	// +patchStrategy=merge
	// +optional
	Defaults []Toleration `json:"defaults,omitempty" patchStrategy:"merge" patchMergeKey:"key" protobuf:"bytes,1,rep,name=defaults"`
	// Whitelist contains a list of tolerations that are allowed to be added to the shoots in this project. Please note
	// that this list may only be added by users having the `spec-tolerations-whitelist` verb for project resources.
	// +patchMergeKey=key
	// +patchStrategy=merge
	// +optional
	Whitelist []Toleration `json:"whitelist,omitempty" patchStrategy:"merge" patchMergeKey:"key" protobuf:"bytes,2,rep,name=whitelist"`
}

// Toleration is a toleration for a seed taint.
type Toleration struct {
	// Key is the toleration key to be applied to a project or shoot.
	Key string `json:"key" protobuf:"bytes,1,opt,name=key"`
	// Value is the toleration value corresponding to the toleration key.
	// +optional
	Value *string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
}

// DualApprovalForDeletion contains configuration for the dual approval concept for resource deletion.
type DualApprovalForDeletion struct {
	// Resource is the name of the resource this applies to.
	Resource string `json:"resource" protobuf:"bytes,1,opt,name=resource"`
	// Selector is the label selector for the resources.
	Selector metav1.LabelSelector `json:"selector" protobuf:"bytes,2,opt,name=selector"`
	// IncludeServiceAccounts specifies whether the concept also applies when deletion is triggered by ServiceAccounts.
	// Defaults to true.
	// +optional
	IncludeServiceAccounts *bool `json:"includeServiceAccounts,omitempty" protobuf:"varint,3,opt,name=includeServiceAccounts"`
}

const (
	// ProjectMemberAdmin is a const for a role that provides full admin access.
	ProjectMemberAdmin = "admin"
	// ProjectMemberOwner is a const for a role that provides full owner access.
	ProjectMemberOwner = "owner"
	// ProjectMemberUserAccessManager is a const for a role that provides permissions to manage human user(s, (groups)).
	ProjectMemberUserAccessManager = "uam"
	// ProjectMemberServiceAccountManager is a const for a role that provides permissions to manage service accounts and request tokens for them.
	ProjectMemberServiceAccountManager = "serviceaccountmanager"
	// ProjectMemberViewer is a const for a role that provides limited permissions to only view some resources.
	ProjectMemberViewer = "viewer"
	// ProjectMemberExtensionPrefix is a prefix for custom roles that are not known by Gardener.
	ProjectMemberExtensionPrefix = "extension:"
)

// ProjectPhase is a label for the condition of a project at the current time.
type ProjectPhase string

const (
	// ProjectPending indicates that the project reconciliation is pending.
	ProjectPending ProjectPhase = "Pending"
	// ProjectReady indicates that the project reconciliation was successful.
	ProjectReady ProjectPhase = "Ready"
	// ProjectFailed indicates that the project reconciliation failed.
	ProjectFailed ProjectPhase = "Failed"
	// ProjectTerminating indicates that the project is in termination process.
	ProjectTerminating ProjectPhase = "Terminating"

	// ProjectEventNamespaceReconcileFailed indicates that the namespace reconciliation has failed.
	ProjectEventNamespaceReconcileFailed = "NamespaceReconcileFailed"
	// ProjectEventNamespaceReconcileSuccessful indicates that the namespace reconciliation has succeeded.
	ProjectEventNamespaceReconcileSuccessful = "NamespaceReconcileSuccessful"
	// ProjectEventNamespaceNotEmpty indicates that the namespace cannot be released because it is not empty.
	ProjectEventNamespaceNotEmpty = "NamespaceNotEmpty"
	// ProjectEventNamespaceDeletionFailed indicates that the namespace deletion failed.
	ProjectEventNamespaceDeletionFailed = "NamespaceDeletionFailed"
	// ProjectEventNamespaceMarkedForDeletion indicates that the namespace has been successfully marked for deletion.
	ProjectEventNamespaceMarkedForDeletion = "NamespaceMarkedForDeletion"
)
