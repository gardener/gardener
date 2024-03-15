// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/utils"
)

// ValidateNamespacedCloudProfile validates a CloudProfile object.
func ValidateNamespacedCloudProfile(cloudProfile *core.NamespacedCloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&cloudProfile.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateNamespacedCloudProfileSpec(&cloudProfile.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateNamespacedCloudProfileUpdate validates a CloudProfile object before an update.
func ValidateNamespacedCloudProfileUpdate(newProfile, oldProfile *core.NamespacedCloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newProfile.ObjectMeta, &oldProfile.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateNamespacedCloudProfileSpecUpdate(&newProfile.Spec, &oldProfile.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateNamespacedCloudProfile(newProfile)...)

	return allErrs
}

// ValidateNamespacedCloudProfileSpecUpdate validates the spec update of a CloudProfile
func ValidateNamespacedCloudProfileSpecUpdate(oldProfile, newProfile *core.NamespacedCloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldProfile.Parent, newProfile.Parent, fldPath.Child("parent"))...)

	return allErrs
}

// ValidateNamespacedCloudProfileSpec validates the specification of a CloudProfile object.
func ValidateNamespacedCloudProfileSpec(spec *core.NamespacedCloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateParent(spec.Parent, fldPath.Child("parent"))...)

	if spec.Kubernetes != nil {
		allErrs = append(allErrs, validateKubernetesSettings(*spec.Kubernetes, fldPath.Child("kubernetes"))...)
	}
	if spec.MachineTypes != nil {
		allErrs = append(allErrs, validateMachineImages(spec.MachineImages, fldPath.Child("machineImages"))...)
	}
	if spec.MachineTypes != nil {
		allErrs = append(allErrs, validateMachineTypes(spec.MachineTypes, fldPath.Child("machineTypes"))...)
	}
	if spec.VolumeTypes != nil {
		allErrs = append(allErrs, validateVolumeTypes(spec.VolumeTypes, fldPath.Child("volumeTypes"))...)
	}
	if spec.Regions != nil {
		allErrs = append(allErrs, validateRegions(spec.Regions, fldPath.Child("regions"))...)
	}
	if spec.SeedSelector != nil {
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&spec.SeedSelector.LabelSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, fldPath.Child("seedSelector"))...)
	}
	if spec.CABundle != nil {
		_, err := utils.DecodeCertificate([]byte(*(spec.CABundle)))
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("caBundle"), *(spec.CABundle), "caBundle is not a valid PEM-encoded certificate"))
		}
	}

	return allErrs
}

func validateParent(parent core.CloudProfileReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if parent.Kind != "CloudProfile" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), parent.Kind, "kind must be CloudProfile"))
	}
	if len(parent.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a parent name"))
	}

	return allErrs
}
