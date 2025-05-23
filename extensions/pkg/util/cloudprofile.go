// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
)

// ImagesContext is a helper struct to consume cloud profile images and their versions.
type ImagesContext[A any, B any] struct {
	Images map[string]A

	createVersionsMap func(A) map[string]B
	// imageVersions will be calculated lazily on first access of each key.
	imageVersions map[string]map[string]B
}

// GetImage returns the image with the given name.
func (vc *ImagesContext[A, B]) GetImage(imageName string) (A, bool) {
	o, exists := vc.Images[imageName]
	return o, exists
}

// GetImageVersion returns the image version with the given name and version.
func (vc *ImagesContext[A, B]) GetImageVersion(imageName string, version string) (B, bool) {
	o, exists := vc.getImageVersions(imageName)[version]
	return o, exists
}

func (vc *ImagesContext[A, B]) getImageVersions(imageName string) map[string]B {
	if versions, exists := vc.imageVersions[imageName]; exists {
		return versions
	}
	vc.imageVersions[imageName] = vc.createVersionsMap(vc.Images[imageName])
	return vc.imageVersions[imageName]
}

// NewImagesContext creates a new generic ImagesContext.
func NewImagesContext[A any, B any](images map[string]A, createVersionsMap func(A) map[string]B) *ImagesContext[A, B] {
	return &ImagesContext[A, B]{
		Images:            images,
		createVersionsMap: createVersionsMap,
		imageVersions:     make(map[string]map[string]B),
	}
}

// NewCoreImagesContext creates a new ImagesContext for core.MachineImage.
func NewCoreImagesContext(profileImages []core.MachineImage) *ImagesContext[core.MachineImage, core.MachineImageVersion] {
	return NewImagesContext(
		utils.CreateMapFromSlice(profileImages, func(mi core.MachineImage) string { return mi.Name }),
		func(mi core.MachineImage) map[string]core.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v core.MachineImageVersion) string { return v.Version })
		},
	)
}

// NewV1beta1ImagesContext creates a new ImagesContext for gardencorev1beta1.MachineImage.
func NewV1beta1ImagesContext(parentImages []gardencorev1beta1.MachineImage) *ImagesContext[gardencorev1beta1.MachineImage, gardencorev1beta1.MachineImageVersion] {
	return NewImagesContext(
		utils.CreateMapFromSlice(parentImages, func(mi gardencorev1beta1.MachineImage) string { return mi.Name }),
		func(mi gardencorev1beta1.MachineImage) map[string]gardencorev1beta1.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v gardencorev1beta1.MachineImageVersion) string { return v.Version })
		},
	)
}
