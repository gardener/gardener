// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"github.com/Masterminds/semver/v3"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	celutils "github.com/gardener/gardener/pkg/utils/cel"
)

// celCompiler compiles ExtensionProfile compatibility CEL rules for syntax validation. The compiler only
// fails to build on an environment error (never on user input), so a build failure is a programming error
// and panics at package initialization.
var celCompiler = celutils.MustNewCompiler()

// ValidateExtensionProfile validates an ExtensionProfile object.
func ValidateExtensionProfile(extensionProfile *core.ExtensionProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	// The ExtensionProfile is bound to an extension type by its name (metadata.name == the extension type,
	// i.e. Shoot.spec.extensions[].type), so the name must be a valid extension type (a DNS1123 label).
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&extensionProfile.ObjectMeta, false, extensionTypeNameValidator, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateExtensionProfileSpec(&extensionProfile.Spec, field.NewPath("spec"))...)

	return allErrs
}

// extensionTypeNameValidator validates that an ExtensionProfile name is a valid extension type, which is a
// DNS1123 label (matching Shoot.spec.extensions[].type), not the DNS1123 subdomain that ValidateName allows.
func extensionTypeNameValidator(name string, _ bool) []string {
	return apivalidation.NameIsDNSLabel(name, false)
}

// ValidateExtensionProfileUpdate validates an ExtensionProfile object before an update.
func ValidateExtensionProfileUpdate(newProfile, oldProfile *core.ExtensionProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newProfile.ObjectMeta, &oldProfile.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateExtensionProfile(newProfile)...)
	allErrs = append(allErrs, ValidateExtensionProfileSpecUpdate(&newProfile.Spec, &oldProfile.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateExtensionProfileSpecUpdate validates the spec update of an ExtensionProfile. Offered versions use
// the same immutability rules as CloudProfile versions (a version's classification may only move forward).
func ValidateExtensionProfileSpecUpdate(newSpec, oldSpec *core.ExtensionProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	oldComponents := map[string]core.ExtensionProfileComponent{}
	for _, component := range oldSpec.Components {
		oldComponents[component.Name] = component
	}

	for i, newComponent := range newSpec.Components {
		oldComponent, ok := oldComponents[newComponent.Name]
		if !ok {
			continue
		}

		var oldVersions, newVersions []core.ExpirableVersion
		for _, oldVersion := range oldComponent.Versions {
			oldVersions = append(oldVersions, core.ExpirableVersion{Version: oldVersion.Version, Lifecycle: oldVersion.Lifecycle})
		}
		for _, newVersion := range newComponent.Versions {
			newVersions = append(newVersions, core.ExpirableVersion{Version: newVersion.Version, Lifecycle: newVersion.Lifecycle})
		}

		allErrs = append(allErrs, ValidateCloudProfileExpirableVersionsUpdate(newVersions, oldVersions, fldPath.Child("components").Index(i).Child("versions"))...)
	}

	return allErrs
}

// ValidateExtensionProfileStatusUpdate validates the status of an ExtensionProfile object before an update.
func ValidateExtensionProfileStatusUpdate(_, _ *core.ExtensionProfile) field.ErrorList {
	return field.ErrorList{}
}

func validateExtensionProfileSpec(spec *core.ExtensionProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateExtensionUpdateStrategy(&spec.UpdateStrategy, false, fldPath.Child("updateStrategy"))...)

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
			// The component-level override may adjust the supported strategies without repeating the
			// default, which is inherited from the profile-level updateStrategy, so default is optional here.
			allErrs = append(allErrs, validateExtensionUpdateStrategy(component.UpdateStrategy, true, idxPath.Child("updateStrategy"))...)
		}

		allErrs = append(allErrs, validateExtensionVersions(component.Versions, idxPath.Child("versions"))...)
	}

	return allErrs
}

// validateExtensionUpdateStrategy validates an update strategy. At the profile level a default is required;
// at the component level (defaultOptional) it may be omitted and inherited from the profile.
func validateExtensionUpdateStrategy(updateStrategy *core.ExtensionUpdateStrategy, defaultOptional bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch {
	case updateStrategy.Default == "":
		if !defaultOptional {
			allErrs = append(allErrs, field.Required(fldPath.Child("default"), "must provide a default update strategy"))
		}
	case !availableExtensionUpdateStrategies.Has(string(updateStrategy.Default)):
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("default"), updateStrategy.Default, sets.List(availableExtensionUpdateStrategies)))
	}

	if len(updateStrategy.Supported) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("supported"), "must list at least one supported update strategy"))
	}

	supported := sets.New[string]()
	for i, strategy := range updateStrategy.Supported {
		if !availableExtensionUpdateStrategies.Has(string(strategy)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("supported").Index(i), strategy, sets.List(availableExtensionUpdateStrategies)))
		}
		supported.Insert(string(strategy))
	}

	if updateStrategy.Default != "" && !supported.Has(string(updateStrategy.Default)) {
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

		// An empty lifecycle leaves the version without any classification, so it can never be selected.
		if len(version.Lifecycle) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("lifecycle"), "must provide at least one lifecycle stage"))
		}
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

	if len(compatibility.Validations) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("validations"), "compatibility must list at least one validation; omit the compatibility field entirely if there are none"))
	}

	for i, validation := range compatibility.Validations {
		idxPath := fldPath.Child("validations").Index(i)

		if validation.Rule == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("rule"), "rule must not be empty"))
			// An empty rule cannot be compiled; skip the compile to avoid a second, redundant error.
			continue
		}

		if _, err := celCompiler.Compile(validation.Rule); err != nil {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("rule"), validation.Rule, "invalid CEL rule: "+err.Error()))
		}
	}

	return allErrs
}
