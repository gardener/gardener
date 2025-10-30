// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	gardencoreapi "github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/admission"
	"github.com/gardener/gardener/pkg/provider-local/admission/mutator"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/helper"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/validation"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// NewNamespacedCloudProfileValidator returns a new instance of a NamespacedCloudProfile validator.
func NewNamespacedCloudProfileValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &namespacedCloudProfileValidator{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type namespacedCloudProfileValidator struct {
	client  client.Client
	decoder runtime.Decoder
}

// Validate validates the given NamespacedCloudProfile objects.
func (p *namespacedCloudProfileValidator) Validate(ctx context.Context, new, _ client.Object) error {
	cloudProfile, ok := new.(*core.NamespacedCloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", new)
	}

	if cloudProfile.DeletionTimestamp != nil {
		return nil
	}
	cloudProfileConfig := &api.CloudProfileConfig{}

	if cloudProfile.Spec.ProviderConfig != nil {
		cpConfig, err := admission.DecodeCloudProfileConfig(p.decoder, cloudProfile.Spec.ProviderConfig)
		if err != nil {
			return fmt.Errorf("could not decode providerConfig of NamespacedCloudProfile for '%s': %w", cloudProfile.Name, err)
		}
		cloudProfileConfig = cpConfig
	}

	parentCloudProfile := cloudProfile.Spec.Parent
	if parentCloudProfile.Kind != constants.CloudProfileReferenceKindCloudProfile {
		return fmt.Errorf("parent reference must be of kind CloudProfile (unsupported kind: %s)", parentCloudProfile.Kind)
	}
	parentProfile := &gardencorev1beta1.CloudProfile{}
	if err := p.client.Get(ctx, client.ObjectKey{Name: parentCloudProfile.Name}, parentProfile); err != nil {
		return err
	}

	if err := SimulateTransformToParentFormat(cloudProfileConfig, cloudProfile, parentProfile.Spec.MachineCapabilities); err != nil {
		return err
	}

	return p.validateNamespacedCloudProfileProviderConfig(cloudProfileConfig, cloudProfile.Spec.MachineImages, parentProfile).ToAggregate()
}

func (p *namespacedCloudProfileValidator) validateNamespacedCloudProfileProviderConfig(providerConfig *api.CloudProfileConfig, machineImages []core.MachineImage, parentProfile *gardencorev1beta1.CloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	profileImages := gardenerutils.NewCoreImagesContext(machineImages)
	parentImages := gardenerutils.NewV1beta1ImagesContext(parentProfile.Spec.MachineImages)
	providerImages := validation.NewProviderImagesContext(providerConfig.MachineImages)

	for _, machineImage := range profileImages.Images {
		// Check that for each new image version defined in the NamespacedCloudProfile, the image is also defined in the providerConfig.
		_, existsInParent := parentImages.GetImage(machineImage.Name)
		if _, existsInProvider := providerImages.GetImage(machineImage.Name); !existsInParent && !existsInProvider {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages"),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name),
			))
			continue
		}
		for _, version := range machineImage.Versions {
			_, existsInParent := parentImages.GetImageVersion(machineImage.Name, version.Version)
			if _, exists := providerImages.GetImageVersion(machineImage.Name, version.Version); !existsInParent && !exists {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec.providerConfig.machineImages"),
					fmt.Sprintf("machine image version %s@%s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name, version.Version),
				))
			}
		}
	}
	for imageIdx, machineImage := range providerConfig.MachineImages {
		// Check that the machine image version is not already defined in the parent CloudProfile.
		if _, exists := parentImages.GetImage(machineImage.Name); exists {
			for versionIdx, version := range machineImage.Versions {
				if _, exists := parentImages.GetImageVersion(machineImage.Name, version.Version); exists {
					allErrs = append(allErrs, field.Forbidden(
						field.NewPath("spec.providerConfig.machineImages").Index(imageIdx).Child("versions").Index(versionIdx),
						fmt.Sprintf("machine image version %s@%s is already defined in the parent CloudProfile", machineImage.Name, version.Version),
					))
				}
			}
		}

		// Check that the machine image version is defined in the NamespacedCloudProfile.
		if _, exists := profileImages.GetImage(machineImage.Name); !exists {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages").Index(imageIdx),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile .spec.machineImages", machineImage.Name),
			))
			continue
		}
		for versionIdx, version := range machineImage.Versions {
			if _, exists := profileImages.GetImageVersion(machineImage.Name, version.Version); !exists {
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

// SimulateTransformToParentFormat simulates the transformation of the given NamespacedCloudProfile and its providerConfig
// to the parent CloudProfile format. This includes the transformation of both the providerConfig and the spec.
func SimulateTransformToParentFormat(cloudProfileConfig *api.CloudProfileConfig, cloudProfile *core.NamespacedCloudProfile, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) error {
	cloudProfileConfigV1alpha1 := &v1alpha1.CloudProfileConfig{}
	path := field.NewPath("spec").Child("providerConfig")

	if err := helper.Scheme.Convert(cloudProfileConfig, cloudProfileConfigV1alpha1, nil); err != nil {
		return field.InternalError(path, err)
	}
	namespacedCloudProfileSpecV1beta1 := gardencorev1beta1.NamespacedCloudProfileSpec{}
	if err := gardencoreapi.Scheme.Convert(&cloudProfile.Spec, &namespacedCloudProfileSpecV1beta1, nil); err != nil {
		return field.InternalError(path, err)
	}

	// simulate transformation to parent spec format
	// - performed in mutating extension webhook
	transformedSpecConfig := mutator.TransformProviderConfigToParentFormat(cloudProfileConfigV1alpha1, capabilityDefinitions)
	// - performed in namespaced cloud profile controller
	transformedSpec := gardenerutils.TransformSpecToParentFormat(namespacedCloudProfileSpecV1beta1, capabilityDefinitions)

	if err := helper.Scheme.Convert(transformedSpecConfig, cloudProfileConfig, nil); err != nil {
		return field.InternalError(path, err)
	}
	if err := gardencoreapi.Scheme.Convert(&transformedSpec, &cloudProfile.Spec, nil); err != nil {
		return field.InternalError(path, err)
	}
	return nil
}
