// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package core

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
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta

	// Spec defines the offered component versions and the update policy.
	Spec ExtensionProfileSpec
	// Status contains the computed classification of the offered versions.
	Status ExtensionProfileStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtensionProfileList is a collection of ExtensionProfiles.
type ExtensionProfileList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta

	// Items is the list of ExtensionProfiles.
	Items []ExtensionProfile
}

// ExtensionProfileSpec is the specification of an ExtensionProfile.
type ExtensionProfileSpec struct {
	// UpdateStrategy is the default auto-update policy for this extension type. It is applied to a
	// component that does not define its own UpdateStrategy.
	UpdateStrategy ExtensionUpdateStrategy
	// Components is the list of named components offered by this extension, each versioned independently.
	Components []ExtensionProfileComponent
}

// ExtensionUpdateStrategy defines the auto-update policy for an extension or one of its components.
type ExtensionUpdateStrategy struct {
	// Default is the update strategy applied when a shoot does not request an explicit strategy.
	// Possible values are: patch, minor, major.
	Default UpdateStrategy
	// Supported is the allow-list of update strategies this extension implements. Admission rejects any
	// shoot (or default) requesting a strategy not listed here. Default must be contained in Supported.
	Supported []UpdateStrategy
}

// ExtensionProfileComponent is a named component of an extension with an independently versioned lifecycle.
type ExtensionProfileComponent struct {
	// Name is the name of the component.
	Name string
	// UpdateStrategy overrides the profile-level update strategy for this component.
	UpdateStrategy *ExtensionUpdateStrategy
	// Versions is the list of offered versions of this component.
	Versions []ExtensionVersion
}

// ExtensionVersion is a single offered version of a component with its lifecycle and compatibility envelope.
type ExtensionVersion struct {
	// Version is the version identifier. Must be a valid semantic version.
	Version string
	// Lifecycle defines the lifecycle stages for this version (reusing the classification semantics of
	// CloudProfile versions, see GEP-0032).
	Lifecycle []LifecycleStage
	// Compatibility is an optional compatibility envelope evaluated against the shoot on admission.
	Compatibility *ExtensionCompatibility
	// ProviderConfig is optional, opaque, extension-owned per-version metadata. It is not written to the
	// Shoot spec; gardenlet carries it into the seed Extension resource when the version is resolved.
	ProviderConfig *runtime.RawExtension
}

// ExtensionCompatibility is a compatibility envelope for an offered version.
type ExtensionCompatibility struct {
	// Validations is a list of CEL rules, all of which must evaluate to true against the shoot. The CEL
	// environment binds the variable `shoot` to the Shoot being admitted.
	Validations []ExtensionCompatibilityValidation
}

// ExtensionCompatibilityValidation is a single CEL compatibility rule.
type ExtensionCompatibilityValidation struct {
	// Rule is a CEL expression evaluated against the shoot, bound to the variable `shoot`. It must return a boolean.
	Rule string
	// Message is the message surfaced when the rule evaluates to false.
	Message string
}

// ExtensionProfileStatus is the computed status of an ExtensionProfile.
type ExtensionProfileStatus struct {
	// Components mirrors spec.components and carries the computed classification of each offered version.
	Components []ExtensionProfileComponentStatus
}

// ExtensionProfileComponentStatus is the computed status for a single component.
type ExtensionProfileComponentStatus struct {
	// Name is the name of the component.
	Name string
	// Versions is the list of offered versions with their currently computed classification.
	Versions []ExpirableVersionStatus
}
