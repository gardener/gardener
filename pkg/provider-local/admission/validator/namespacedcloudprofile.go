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

	profileImages := utils.CreateMapFromSlice(machineImages, func(mi core.MachineImage) string { return mi.Name })
	parentImages := utils.CreateMapFromSlice(parentProfile.Spec.MachineImages, func(mi gardencorev1beta1.MachineImage) string { return mi.Name })
	providerImages := utils.CreateMapFromSlice(providerConfig.MachineImages, func(mi api.MachineImages) string { return mi.Name })
	for _, machineImage := range profileImages {
		// Check that for each new image version defined in the NamespacedCloudProfile, the image is also defined in the providerConfig.
		_, existsInParent := parentImages[machineImage.Name]
		if _, existsInProvider := providerImages[machineImage.Name]; !existsInParent && !existsInProvider {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages"),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name),
			))
			continue
		}
		for _, version := range machineImage.Versions {
			parentVersions := utils.CreateMapFromSlice(parentImages[machineImage.Name].Versions, func(v gardencorev1beta1.MachineImageVersion) string { return v.Version })
			providerVersions := utils.CreateMapFromSlice(providerImages[machineImage.Name].Versions, func(v api.MachineImageVersion) string { return v.Version })
			_, existsInParent := parentVersions[version.Version]
			if _, exists := providerVersions[version.Version]; !existsInParent && !exists {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec.providerConfig.machineImages"),
					fmt.Sprintf("machine image version %s@%s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name, version.Version),
				))
			}
		}
	}
	for imageIdx, machineImage := range providerConfig.MachineImages {
		// Check that the machine image version is not already defined in the parent CloudProfile.
		if _, exists := parentImages[machineImage.Name]; exists {
			parentVersions := utils.CreateMapFromSlice(parentImages[machineImage.Name].Versions, func(v gardencorev1beta1.MachineImageVersion) string { return v.Version })
			for versionIdx, version := range machineImage.Versions {
				if _, exists := parentVersions[version.Version]; exists {
					allErrs = append(allErrs, field.Forbidden(
						field.NewPath("spec.providerConfig.machineImages").Index(imageIdx).Child("versions").Index(versionIdx),
						fmt.Sprintf("machine image version %s@%s is already defined in the parent CloudProfile", machineImage.Name, version.Version),
					))
				}
			}
		}
		// Check that the machine image version is defined in the NamespacedCloudProfile.
		profileImage, exists := profileImages[machineImage.Name]
		if !exists {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages").Index(imageIdx),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile .spec.machineImages", machineImage.Name),
			))
			continue
		}
		profileVersions := utils.CreateMapFromSlice(profileImage.Versions, func(v core.MachineImageVersion) string { return v.Version })
		for versionIdx, version := range machineImage.Versions {
			if _, exists := profileVersions[version.Version]; !exists {
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
