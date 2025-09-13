// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileConfig contains provider-specific configuration that is embedded into Gardener's `CloudProfile`
// resource.
type CloudProfileConfig struct {
	metav1.TypeMeta

	// MachineImages is the list of machine images that are understood by the controller. It maps
	// logical names and versions to provider-specific identifiers.
	MachineImages []MachineImages
}

// MachineImages is a mapping from logical names and versions to provider-specific identifiers.
type MachineImages struct {
	// Name is the logical name of the machine image.
	Name string
	// Versions contains versions and a provider-specific identifier.
	Versions []MachineImageVersion
}

// MachineImageVersion contains a version and a provider-specific identifier.
type MachineImageVersion struct {
	// Version is the version of the image.
	Version string
	// Image is the image for the machine image.
	Image string
	// CapabilityFlavors contains provider-specific image identifiers of this version with their capabilities.
	CapabilityFlavors []MachineImageFlavor
}

// MachineImageFlavor is a provider-specific image identifier with its supported capabilities.
type MachineImageFlavor struct {
	// Image is the image for the machine image.
	Image string
	// Capabilities that are supported by the identifier in this set.
	Capabilities gardencorev1beta1.Capabilities
}

// GetCapabilities returns the Capabilities of a MachineImageFlavor
func (cs MachineImageFlavor) GetCapabilities() gardencorev1beta1.Capabilities {
	return cs.Capabilities
}
