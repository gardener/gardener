// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtensionProfile describes the offered component versions and their lifecycle for an
// extension-managed component. One ExtensionProfile exists per participating extension type;
// the binding to the extension type is the object's name (metadata.name equals the extension type).
type ExtensionProfile struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the offered component versions and the update policy.
	// +optional
	Spec ExtensionProfileSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Status contains the computed classification of the offered versions.
	// +optional
	Status ExtensionProfileStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtensionProfileList is a collection of ExtensionProfiles.
type ExtensionProfileList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is the list of ExtensionProfiles.
	Items []ExtensionProfile `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ExtensionProfileSpec is the specification of an ExtensionProfile.
type ExtensionProfileSpec struct {
	// UpdateStrategy is the default auto-update policy for this extension type. It is applied to a
	// component that does not define its own updateStrategy.
	UpdateStrategy ExtensionUpdateStrategy `json:"updateStrategy" protobuf:"bytes,1,opt,name=updateStrategy"`
	// Components is the list of named components offered by this extension, each versioned independently.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Components []ExtensionProfileComponent `json:"components,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,2,rep,name=components"`
}

// ExtensionUpdateStrategy defines the auto-update policy for an extension or one of its components.
type ExtensionUpdateStrategy struct {
	// Default is the update strategy applied when a shoot does not request an explicit strategy. Required
	// at the profile level; at the component level it may be omitted and is then inherited from the profile.
	// Possible values are: patch, minor, major.
	// +optional
	Default UpdateStrategy `json:"default,omitempty" protobuf:"bytes,1,opt,name=default,casttype=MachineImageUpdateStrategy"`
	// Supported is the allow-list of update strategies this extension implements. Admission rejects any
	// shoot (or default) requesting a strategy not listed here. Default must be contained in Supported.
	Supported []UpdateStrategy `json:"supported" protobuf:"bytes,2,rep,name=supported,casttype=MachineImageUpdateStrategy"`
}

// ExtensionProfileComponent is a named component of an extension with an independently versioned lifecycle.
type ExtensionProfileComponent struct {
	// Name is the name of the component.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// UpdateStrategy overrides the profile-level update strategy for this component.
	// +optional
	UpdateStrategy *ExtensionUpdateStrategy `json:"updateStrategy,omitempty" protobuf:"bytes,2,opt,name=updateStrategy"`
	// Versions is the list of offered versions of this component.
	// +optional
	// +patchMergeKey=version
	// +patchStrategy=merge
	Versions []ExtensionVersion `json:"versions,omitempty" patchStrategy:"merge" patchMergeKey:"version" protobuf:"bytes,3,rep,name=versions"`
}

// ExtensionVersion is a single offered version of a component with its lifecycle and compatibility envelope.
type ExtensionVersion struct {
	// Version is the version identifier. Must be a valid semantic version.
	Version string `json:"version" protobuf:"bytes,1,opt,name=version"`
	// Lifecycle defines the lifecycle stages for this version (reusing the classification semantics of
	// CloudProfile versions, see GEP-0032). At least one stage must be provided.
	Lifecycle []LifecycleStage `json:"lifecycle" protobuf:"bytes,2,rep,name=lifecycle"`
	// Compatibility is an optional compatibility envelope evaluated against the shoot on admission.
	// +optional
	Compatibility *ExtensionCompatibility `json:"compatibility,omitempty" protobuf:"bytes,3,opt,name=compatibility"`
	// ProviderConfig is optional, opaque, extension-owned per-version metadata. It is not written to the
	// Shoot spec; gardenlet carries it into the seed Extension resource when the version is resolved.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,4,opt,name=providerConfig"`
}

// ExtensionCompatibility is a compatibility envelope for an offered version.
type ExtensionCompatibility struct {
	// Validations is a list of CEL rules, all of which must evaluate to true against the shoot. The CEL
	// environment binds the variable `shoot` to the Shoot being admitted.
	// +optional
	Validations []ExtensionCompatibilityValidation `json:"validations,omitempty" protobuf:"bytes,1,rep,name=validations"`
}

// ExtensionCompatibilityValidation is a single CEL compatibility rule.
type ExtensionCompatibilityValidation struct {
	// Rule is a CEL expression evaluated against the shoot, bound to the variable `shoot`. It must return a boolean.
	Rule string `json:"rule" protobuf:"bytes,1,opt,name=rule"`
	// Message is the message surfaced when the rule evaluates to false.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`
}

// ExtensionProfileStatus is the computed status of an ExtensionProfile.
type ExtensionProfileStatus struct {
	// Components mirrors spec.components and carries the computed classification of each offered version.
	// +optional
	Components []ExtensionProfileComponentStatus `json:"components,omitempty" protobuf:"bytes,1,rep,name=components"`
}

// ExtensionProfileComponentStatus is the computed status for a single component.
type ExtensionProfileComponentStatus struct {
	// Name is the name of the component.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Versions is the list of offered versions with their currently computed classification.
	// +optional
	Versions []ExpirableVersionStatus `json:"versions,omitempty" protobuf:"bytes,2,rep,name=versions"`
}
