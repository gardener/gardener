// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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

	createArchitecturesMap func(A) map[string]map[string]B
	// imageVersionArchitectures will be calculated lazily on first access of each key.
	imageVersionsArchitectures map[string]map[string]map[string]B
}

// GetImage returns the image with the given name.
func (vc *ImagesContext[A, B]) GetImage(imageName string) (A, bool) {
	o, exists := vc.Images[imageName]
	return o, exists
}

// GetImageVersionAnyArchitecture returns if an image with the given name, version an at least one architecture exists.
func (vc *ImagesContext[A, B]) GetImageVersionAnyArchitecture(imageName string, version string) (B, bool) {
	imageArchitectures := vc.getImageArchitectures(imageName, version)
	for k := range imageArchitectures {
		return imageArchitectures[k], true
	}
	var empty B
	return empty, false
}

// GetImageVersion returns the image with the given name, version and architecture.
func (vc *ImagesContext[A, B]) GetImageVersion(imageName string, version string, architecture string) (B, bool) {
	o, exists := vc.getImageArchitectures(imageName, version)[architecture]
	return o, exists
}

func (vc *ImagesContext[A, B]) getImageArchitectures(imageName string, version string) map[string]B {
	if architectures, exists := vc.imageVersionsArchitectures[imageName][version]; exists {
		return architectures
	}
	vc.imageVersionsArchitectures[imageName] = vc.createArchitecturesMap(vc.Images[imageName])
	return vc.imageVersionsArchitectures[imageName][version]
}

// NewImagesContext creates a new generic ImagesContext.
func NewImagesContext[A any, B any](images map[string]A,
	createArchitecturesMap func(A) map[string]map[string]B) *ImagesContext[A, B] {
	return &ImagesContext[A, B]{
		Images:                     images,
		createArchitecturesMap:     createArchitecturesMap,
		imageVersionsArchitectures: make(map[string]map[string]map[string]B),
	}
}

// NewCoreImagesContext creates a new ImagesContext for core.MachineImage.
func NewCoreImagesContext(profileImages []core.MachineImage) *ImagesContext[core.MachineImage, core.MachineImageVersion] {
	return NewImagesContext(
		utils.CreateMapFromSlice(profileImages, func(mi core.MachineImage) string { return mi.Name }),
		func(mi core.MachineImage) map[string]map[string]core.MachineImageVersion {
			mapped := make(map[string]map[string]core.MachineImageVersion)
			for _, value := range mi.Versions {
				mapped[value.Version] = make(map[string]core.MachineImageVersion)
				for _, arch := range value.Architectures {
					mapped[value.Version][arch] = value
				}
				if len(value.Architectures) == 0 {
					mapped[value.Version][""] = value
				}
			}
			return mapped
		},
	)
}

// NewV1beta1ImagesContext creates a new ImagesContext for gardencorev1beta1.MachineImage.
func NewV1beta1ImagesContext(parentImages []gardencorev1beta1.MachineImage) *ImagesContext[gardencorev1beta1.MachineImage, gardencorev1beta1.MachineImageVersion] {
	return NewImagesContext(
		utils.CreateMapFromSlice(parentImages, func(mi gardencorev1beta1.MachineImage) string { return mi.Name }),
		func(mi gardencorev1beta1.MachineImage) map[string]map[string]gardencorev1beta1.MachineImageVersion {
			mapped := make(map[string]map[string]gardencorev1beta1.MachineImageVersion)
			for _, value := range mi.Versions {
				mapped[value.Version] = make(map[string]gardencorev1beta1.MachineImageVersion)
				for _, arch := range value.Architectures {
					mapped[value.Version][arch] = value
				}
				if len(value.Architectures) == 0 {
					mapped[value.Version][""] = value
				}
			}
			return mapped
		},
	)
}
