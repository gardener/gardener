// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileConfig contains provider-specific configuration that is embedded into Gardener's `CloudProfile`
// resource.
type CloudProfileConfig struct {
	metav1.TypeMeta `json:",inline"`

	// MachineImages is the list of machine images that are understood by the controller. It maps
	// logical names and versions to provider-specific identifiers.
	MachineImages []MachineImages `json:"machineImages"`
}

// MachineImages is a mapping from logical names and versions to provider-specific identifiers.
type MachineImages struct {
	// Name is the logical name of the machine image.
	Name string `json:"name"`
	// Versions contains versions and a provider-specific identifier.
	Versions []MachineImageVersion `json:"versions"`
}

// MachineImageVersion contains a version and a provider-specific identifier.
type MachineImageVersion struct {
	// Version is the version of the image.
	Version string `json:"version"`
	// Image is the image for the machine image.
	Image string `json:"image,omitempty"`
	// CapabilityFlavors contains provider-specific image identifiers of this version with their capabilities.
	CapabilityFlavors []MachineImageFlavor `json:"capabilityFlavors"`
}

// MachineImageFlavor is a provider-specific image identifier with its supported capabilities.
type MachineImageFlavor struct {
	// Image is the image for the machine image.
	Image string `json:"image"`
	// Capabilities that are supported by the identifier in this set.
	Capabilities gardencorev1beta1.Capabilities `json:"capabilities,omitempty"`
}
