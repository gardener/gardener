// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"github.com/Masterminds/semver/v3"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	celutils "github.com/gardener/gardener/pkg/utils/cel"
)

// celCompiler compiles ExtensionProfile compatibility CEL rules for syntax validation. It is created
// once; a nil compiler (should the environment fail to build) degrades gracefully to skipping the
// syntax check, which the admission plugin then catches at evaluation time.
var celCompiler, _ = celutils.NewCompiler()

// ValidateExtensionProfile validates an ExtensionProfile object.
func ValidateExtensionProfile(extensionProfile *core.ExtensionProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	// The ExtensionProfile is bound to an extension type by its name (metadata.name == the extension
	// type, i.e. Shoot.spec.extensions[].type). The name must therefore be a valid extension type: a
	// DNS1123 label, exactly as validated for Extension.type. This binding only addresses the generic
	// `Extension` kind, whose type is unique; types of other kinds (ControlPlane/Infrastructure/Worker)
	// are configured via spec.provider/spec.networking and are not addressable here, so no collision arises.
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&extensionProfile.ObjectMeta, false, extensionTypeNameValidator, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateExtensionProfileSpec(&extensionProfile.Spec, field.NewPath("spec"))...)

	return allErrs
}

// extensionTypeNameValidator validates that an ExtensionProfile name is a valid extension type
// (a DNS1123 label), matching the constraint applied to Shoot.spec.extensions[].type.
func extensionTypeNameValidator(name string, _ bool) []string {
	return utilvalidation.IsDNS1123Label(name)
}

// ValidateExtensionProfileUpdate validates an ExtensionProfile object before an update.
func ValidateExtensionProfileUpdate(newProfile, oldProfile *core.ExtensionProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newProfile.ObjectMeta, &oldProfile.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateExtensionProfile(newProfile)...)

	return allErrs
}

// ValidateExtensionProfileStatusUpdate validates the status of an ExtensionProfile object before an update.
func ValidateExtensionProfileStatusUpdate(newProfile, oldProfile *core.ExtensionProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newProfile.ObjectMeta, &oldProfile.ObjectMeta, field.NewPath("metadata"))...)

	statusPath := field.NewPath("status")
	componentNames := sets.New[string]()
	for i, component := range newProfile.Status.Components {
		idxPath := statusPath.Child("components").Index(i)
		if component.Name == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "component name must not be empty"))
		}
		if componentNames.Has(component.Name) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("name"), component.Name))
		}
		componentNames.Insert(component.Name)
	}

	return allErrs
}

func validateExtensionProfileSpec(spec *core.ExtensionProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateExtensionUpdateStrategy(&spec.UpdateStrategy, fldPath.Child("updateStrategy"))...)

	componentNames := sets.New[string]()
	for i, component := range spec.Components {
		idxPath := fldPath.Child("components").Index(i)

		if component.Name == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "component name must not be empty"))
		} else {
			allErrs = append(allErrs, validateDNS1123Label(component.Name, idxPath.Child("name"))...)
		}

		if componentNames.Has(component.Name) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("name"), component.Name))
		}
		componentNames.Insert(component.Name)

		if component.UpdateStrategy != nil {
			allErrs = append(allErrs, validateExtensionUpdateStrategy(component.UpdateStrategy, idxPath.Child("updateStrategy"))...)
		}

		allErrs = append(allErrs, validateExtensionVersions(component.Versions, idxPath.Child("versions"))...)
	}

	return allErrs
}

func validateExtensionUpdateStrategy(updateStrategy *core.ExtensionUpdateStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if updateStrategy.Default == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("default"), "must provide a default update strategy"))
	} else if !availableUpdateStrategiesForMachineImage.Has(string(updateStrategy.Default)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("default"), updateStrategy.Default, sets.List(availableUpdateStrategiesForMachineImage)))
	}

	supported := sets.New[string]()
	for i, strategy := range updateStrategy.Supported {
		if !availableUpdateStrategiesForMachineImage.Has(string(strategy)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("supported").Index(i), strategy, sets.List(availableUpdateStrategiesForMachineImage)))
		}
		supported.Insert(string(strategy))
	}

	if updateStrategy.Default != "" && len(updateStrategy.Supported) > 0 && !supported.Has(string(updateStrategy.Default)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("default"), updateStrategy.Default, "default update strategy must be contained in the supported strategies"))
	}

	return allErrs
}

func validateExtensionVersions(versions []core.ExtensionVersion, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	seenVersions := sets.New[string]()
	for i, version := range versions {
		idxPath := fldPath.Index(i)

		if version.Version == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("version"), "version must not be empty"))
		} else if _, err := semver.NewVersion(version.Version); err != nil {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("version"), version.Version, "could not parse version: "+err.Error()))
		}

		if seenVersions.Has(version.Version) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("version"), version.Version))
		}
		seenVersions.Insert(version.Version)

		allErrs = append(allErrs, validateLifecycleClassificationsValid(version.Lifecycle, idxPath.Child("lifecycle"))...)
		allErrs = append(allErrs, validateLifecycleNoDuplicates(version.Lifecycle, idxPath.Child("lifecycle"))...)
		allErrs = append(allErrs, validateLifecycleInOrder(version.Lifecycle, idxPath.Child("lifecycle"))...)
		allErrs = append(allErrs, validateLifecycleStartTimes(version.Lifecycle, idxPath.Child("lifecycle"))...)

		if version.Compatibility != nil {
			allErrs = append(allErrs, validateExtensionCompatibility(version.Compatibility, idxPath.Child("compatibility"))...)
		}
	}

	return allErrs
}

func validateExtensionCompatibility(compatibility *core.ExtensionCompatibility, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, validation := range compatibility.Validations {
		idxPath := fldPath.Child("validations").Index(i)

		if validation.Rule == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("rule"), "rule must not be empty"))
			continue
		}

		if celCompiler != nil {
			if _, err := celCompiler.Compile(validation.Rule); err != nil {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("rule"), validation.Rule, "invalid CEL rule: "+err.Error()))
			}
		}
	}

	return allErrs
}
