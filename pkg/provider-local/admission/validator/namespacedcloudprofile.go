// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/utils"
)

// NewNamespacedCloudProfileValidator returns a new instance of a NamespacedCloudProfile validator.
func NewNamespacedCloudProfileValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &namespacedCloudProfile{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type namespacedCloudProfile struct {
	client  client.Client
	decoder runtime.Decoder
}

// Validate validates the given NamespacedCloudProfile objects.
func (p *namespacedCloudProfile) Validate(ctx context.Context, new, _ client.Object) error {
	profile, ok := new.(*core.NamespacedCloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", new)
	}

	if profile.DeletionTimestamp != nil {
		return nil
	}

	cloudProfileConfig := &api.CloudProfileConfig{}
	if profile.Spec.ProviderConfig != nil {
		if _, _, err := p.decoder.Decode(profile.Spec.ProviderConfig.Raw, nil, cloudProfileConfig); err != nil {
			return fmt.Errorf("could not decode providerConfig of namespacedCloudProfile for '%s': %w", profile.Name, err)
		}
	}

	parentCloudProfile := profile.Spec.Parent
	if parentCloudProfile.Kind != constants.CloudProfileReferenceKindCloudProfile {
		return fmt.Errorf("parent reference must be of kind CloudProfile (unsupported kind: %s)", parentCloudProfile.Kind)
	}
	parentProfile := &gardencorev1beta1.CloudProfile{}
	if err := p.client.Get(ctx, client.ObjectKey{Name: parentCloudProfile.Name}, parentProfile); err != nil {
		return err
	}

	return p.ValidateNamespacedCloudProfileProviderConfig(cloudProfileConfig, profile.Spec.MachineImages, parentProfile).ToAggregate()
}

func (p *namespacedCloudProfile) ValidateNamespacedCloudProfileProviderConfig(providerConfig *api.CloudProfileConfig, machineImages []core.MachineImage, parentProfile *gardencorev1beta1.CloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	profileImages := newProfileImagesContext(machineImages)
	parentImages := newParentImagesContext(parentProfile.Spec.MachineImages)
	providerImages := newProviderImagesContext(providerConfig.MachineImages)

	for _, machineImage := range profileImages.images {
		// Check that for each new image version defined in the NamespacedCloudProfile, the image is also defined in the providerConfig.
		_, existsInParent := parentImages.getImage(machineImage.Name)
		if _, existsInProvider := providerImages.getImage(machineImage.Name); !existsInParent && !existsInProvider {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages"),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name),
			))
			continue
		}
		for _, version := range machineImage.Versions {
			_, existsInParent := parentImages.getImageVersion(machineImage.Name, version.Version)
			if _, exists := providerImages.getImageVersion(machineImage.Name, version.Version); !existsInParent && !exists {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec.providerConfig.machineImages"),
					fmt.Sprintf("machine image version %s@%s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name, version.Version),
				))
			}
		}
	}
	for imageIdx, machineImage := range providerConfig.MachineImages {
		// Check that the machine image version is not already defined in the parent CloudProfile.
		if _, exists := parentImages.getImage(machineImage.Name); exists {
			for versionIdx, version := range machineImage.Versions {
				if _, exists := parentImages.getImageVersion(machineImage.Name, version.Version); exists {
					allErrs = append(allErrs, field.Forbidden(
						field.NewPath("spec.providerConfig.machineImages").Index(imageIdx).Child("versions").Index(versionIdx),
						fmt.Sprintf("machine image version %s@%s is already defined in the parent CloudProfile", machineImage.Name, version.Version),
					))
				}
			}
		}

		// Check that the machine image version is defined in the NamespacedCloudProfile.
		if _, exists := profileImages.getImage(machineImage.Name); !exists {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages").Index(imageIdx),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile .spec.machineImages", machineImage.Name),
			))
			continue
		}
		for versionIdx, version := range machineImage.Versions {
			if _, exists := profileImages.getImageVersion(machineImage.Name, version.Version); !exists {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec.providerConfig.machineImages").Index(imageIdx).Child("versions").Index(versionIdx),
					fmt.Sprintf("%s@%s", machineImage.Name, version.Version),
					"machine image version is not defined in the NamespacedCloudProfile",
				))
			}
		}
	}

	return allErrs
}

type imagesContext[A any, B any] struct {
	images            map[string]A
	createVersionsMap func(A) map[string]B
	// imageVersions will be calculated lazy on first access of each key.
	imageVersions map[string]map[string]B
}

func (vc *imagesContext[A, B]) getImage(imageName string) (A, bool) {
	o, exists := vc.images[imageName]
	return o, exists
}

func (vc *imagesContext[A, B]) getImageVersion(imageName string, version string) (B, bool) {
	o, exists := vc.getImageVersions(imageName)[version]
	return o, exists
}

func (vc *imagesContext[A, B]) getImageVersions(imageName string) map[string]B {
	if versions, exists := vc.imageVersions[imageName]; exists {
		return versions
	}
	vc.imageVersions[imageName] = vc.createVersionsMap(vc.images[imageName])
	return vc.imageVersions[imageName]
}

func newProfileImagesContext(profileImages []core.MachineImage) *imagesContext[core.MachineImage, core.MachineImageVersion] {
	return &imagesContext[core.MachineImage, core.MachineImageVersion]{
		images:        utils.CreateMapFromSlice(profileImages, func(mi core.MachineImage) string { return mi.Name }),
		imageVersions: make(map[string]map[string]core.MachineImageVersion),
		createVersionsMap: func(mi core.MachineImage) map[string]core.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v core.MachineImageVersion) string { return v.Version })
		},
	}
}

func newParentImagesContext(parentImages []gardencorev1beta1.MachineImage) *imagesContext[gardencorev1beta1.MachineImage, gardencorev1beta1.MachineImageVersion] {
	return &imagesContext[gardencorev1beta1.MachineImage, gardencorev1beta1.MachineImageVersion]{
		images:        utils.CreateMapFromSlice(parentImages, func(mi gardencorev1beta1.MachineImage) string { return mi.Name }),
		imageVersions: make(map[string]map[string]gardencorev1beta1.MachineImageVersion),
		createVersionsMap: func(mi gardencorev1beta1.MachineImage) map[string]gardencorev1beta1.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v gardencorev1beta1.MachineImageVersion) string { return v.Version })
		},
	}
}

func newProviderImagesContext(providerImages []api.MachineImages) *imagesContext[api.MachineImages, api.MachineImageVersion] {
	return &imagesContext[api.MachineImages, api.MachineImageVersion]{
		images:        utils.CreateMapFromSlice(providerImages, func(mi api.MachineImages) string { return mi.Name }),
		imageVersions: make(map[string]map[string]api.MachineImageVersion),
		createVersionsMap: func(mi api.MachineImages) map[string]api.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v api.MachineImageVersion) string { return v.Version })
		},
	}
}
