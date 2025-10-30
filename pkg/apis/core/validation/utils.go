// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	kubernetescorevalidation "github.com/gardener/gardener/pkg/utils/validation/kubernetes/core"
)

// ValidateName is a helper function for validating that a name is a DNS subdomain.
func ValidateName(name string, prefix bool) []string {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

func validateCrossVersionObjectReference(ref autoscalingv1.CrossVersionObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.APIVersion) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("apiVersion"), "must provide an apiVersion"))
	} else if ref.APIVersion != corev1.SchemeGroupVersion.String() {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiVersion"), ref.APIVersion, []string{corev1.SchemeGroupVersion.String()}))
	}

	if len(ref.Kind) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "must provide a kind"))
	} else if ref.Kind != "Secret" && ref.Kind != "ConfigMap" {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kind"), ref.Kind, []string{"Secret", "ConfigMap"}))
	}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	return allErrs
}

func validateNameConsecutiveHyphens(name string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if strings.Contains(name, "--") {
		allErrs = append(allErrs, field.Invalid(fldPath, name, "name may not contain two consecutive hyphens"))
	}

	return allErrs
}

// ValidateDNS1123Subdomain validates that a name is a proper DNS subdomain.
func ValidateDNS1123Subdomain(value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, msg := range validation.IsDNS1123Subdomain(value) {
		allErrs = append(allErrs, field.Invalid(fldPath, value, msg))
	}

	return allErrs
}

// validateDNS1123Label valides a name is a proper RFC1123 DNS label.
func validateDNS1123Label(value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, msg := range validation.IsDNS1123Label(value) {
		allErrs = append(allErrs, field.Invalid(fldPath, value, msg))
	}

	return allErrs
}

func getIntOrPercentValue(intOrStringValue intstr.IntOrString) int {
	value, isPercent := getPercentValue(intOrStringValue)
	if isPercent {
		return value
	}
	return intOrStringValue.IntValue()
}

func getPercentValue(intOrStringValue intstr.IntOrString) (int, bool) {
	if intOrStringValue.Type != intstr.String {
		return 0, false
	}
	if len(validation.IsValidPercent(intOrStringValue.StrVal)) != 0 {
		return 0, false
	}
	value, _ := strconv.Atoi(intOrStringValue.StrVal[:len(intOrStringValue.StrVal)-1])
	return value, true
}

var availableFailureTolerance = sets.New(
	string(core.FailureToleranceTypeNode),
	string(core.FailureToleranceTypeZone),
)

// ValidateFailureToleranceTypeValue validates if the passed value is a valid failureToleranceType.
func ValidateFailureToleranceTypeValue(value core.FailureToleranceType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	failureToleranceType := string(value)
	if !availableFailureTolerance.Has(failureToleranceType) {
		allErrs = append(allErrs, field.NotSupported(fldPath, failureToleranceType, sets.List(availableFailureTolerance)))
	}

	return allErrs
}

var availableIPFamilies = sets.New(
	string(core.IPFamilyIPv4),
	string(core.IPFamilyIPv6),
)

// ValidateIPFamilies validates the given list of IP families for valid values and combinations.
func ValidateIPFamilies(ipFamilies []core.IPFamily, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	ipFamiliesSeen := sets.New[string]()
	for i, ipFamily := range ipFamilies {
		// validate: only supported IP families
		if !availableIPFamilies.Has(string(ipFamily)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Index(i), ipFamily, sets.List(availableIPFamilies)))
		}

		// validate: no duplicate IP families
		if ipFamiliesSeen.Has(string(ipFamily)) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i), ipFamily))
		} else {
			ipFamiliesSeen.Insert(string(ipFamily))
		}
	}

	return allErrs
}

// k8sVersionCPRegex is used to validate kubernetes versions in a cloud profile.
var k8sVersionCPRegex = regexp.MustCompile(`^([0-9]+\.){2}[0-9]+$`)

var supportedVersionClassifications = sets.New(string(core.ClassificationPreview), string(core.ClassificationSupported), string(core.ClassificationDeprecated))

// validateKubernetesVersions validates the given list of ExpirableVersions for valid Kubernetes versions.
func validateKubernetesVersions(versions []core.ExpirableVersion, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(versions) == 0 {
		return allErrs
	}

	versionsFound := sets.New[string]()
	for i, version := range versions {
		idxPath := fldPath.Child("versions").Index(i)
		if !k8sVersionCPRegex.MatchString(version.Version) {
			allErrs = append(allErrs, field.Invalid(idxPath, version, fmt.Sprintf("all Kubernetes versions must match the regex %s", k8sVersionCPRegex)))
		} else if versionsFound.Has(version.Version) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("version"), version.Version))
		} else {
			versionsFound.Insert(version.Version)
		}

		if version.Classification != nil && !supportedVersionClassifications.Has(string(*version.Classification)) {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("classification"), *version.Classification, sets.List(supportedVersionClassifications)))
		}
	}

	return allErrs
}

// ValidateMachineImages validates the given list of machine images for valid values and combinations.
func ValidateMachineImages(machineImages []core.MachineImage, capabilities core.Capabilities, fldPath *field.Path, allowEmptyVersions bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImages) == 0 {
		return allErrs
	}

	latestMachineImages, err := gardencorehelper.DetermineLatestMachineImageVersions(machineImages)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, latestMachineImages, err.Error()))
	}

	duplicateNameVersion := sets.Set[string]{}
	duplicateName := sets.Set[string]{}
	for i, image := range machineImages {
		idxPath := fldPath.Index(i)
		if duplicateName.Has(image.Name) {
			allErrs = append(allErrs, field.Duplicate(idxPath, image.Name))
		}
		duplicateName.Insert(image.Name)

		if len(image.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "machine image name must not be empty"))
		} else if errs := validateUnprefixedQualifiedName(image.Name); len(errs) != 0 {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("name"), image.Name, fmt.Sprintf("machine image name must be a qualified name: %v", errs)))
		}

		if len(image.Versions) == 0 && !allowEmptyVersions {
			allErrs = append(allErrs, field.Required(idxPath.Child("versions"), fmt.Sprintf("must provide at least one version for the machine image '%s'", image.Name)))
		}

		if image.UpdateStrategy != nil {
			if !availableUpdateStrategiesForMachineImage.Has(string(*image.UpdateStrategy)) {
				allErrs = append(allErrs, field.NotSupported(idxPath.Child("updateStrategy"), *image.UpdateStrategy, sets.List(availableUpdateStrategiesForMachineImage)))
			}
		}

		for index, machineVersion := range image.Versions {
			versionsPath := idxPath.Child("versions").Index(index)
			key := fmt.Sprintf("%s-%s", image.Name, machineVersion.Version)
			if duplicateNameVersion.Has(key) {
				allErrs = append(allErrs, field.Duplicate(versionsPath, key))
			}
			duplicateNameVersion.Insert(key)
			if len(machineVersion.Version) == 0 {
				allErrs = append(allErrs, field.Required(versionsPath.Child("version"), machineVersion.Version))
			}

			_, err := semver.NewVersion(machineVersion.Version)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(versionsPath.Child("version"), machineVersion.Version, "could not parse version. Use a semantic version. In case there is no semantic version for this image use the extensibility provider (define mapping in the CloudProfile) to map to the actual non semantic version"))
			}

			if machineVersion.InPlaceUpdates != nil && machineVersion.InPlaceUpdates.MinVersionForUpdate != nil {
				if _, err = semver.NewVersion(*machineVersion.InPlaceUpdates.MinVersionForUpdate); err != nil {
					allErrs = append(allErrs, field.Invalid(versionsPath.Child("minVersionForInPlaceUpdate"), *machineVersion.InPlaceUpdates.MinVersionForUpdate, "could not parse version. Use a semantic version."))
				}
			}

			if machineVersion.Classification != nil && !supportedVersionClassifications.Has(string(*machineVersion.Classification)) {
				allErrs = append(allErrs, field.NotSupported(versionsPath.Child("classification"), *machineVersion.Classification, sets.List(supportedVersionClassifications)))
			}

			allErrs = append(allErrs, validateMachineImageVersionFlavors(machineVersion, capabilities, versionsPath)...)

			if machineVersion.KubeletVersionConstraint != nil {
				if _, err := semver.NewConstraint(*machineVersion.KubeletVersionConstraint); err != nil {
					allErrs = append(allErrs, field.Invalid(versionsPath.Child("kubeletVersionConstraint"), machineVersion.KubeletVersionConstraint, fmt.Sprintf("cannot parse the kubeletVersionConstraint: %s", err.Error())))
				}
			}
		}
	}

	return allErrs
}

func validateUnprefixedQualifiedName(name string) []string {
	if errs := validation.IsQualifiedName(name); len(errs) > 0 {
		return errs
	}
	if strings.Contains(name, "/") {
		return []string{fmt.Sprintf("name '%s' must not contain a prefix", name)}
	}
	return nil
}

// validateMachineTypes validates the given list of machine types for valid values and combinations.
func validateMachineTypes(machineTypes []core.MachineType, capabilities core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	names := make(sets.Set[string], len(machineTypes))

	for i, machineType := range machineTypes {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		cpuPath := idxPath.Child("cpu")
		gpuPath := idxPath.Child("gpu")
		memoryPath := idxPath.Child("memory")

		if len(machineType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		} else if errs := validateUnprefixedQualifiedName(machineType.Name); len(errs) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("name"), machineType.Name, fmt.Sprintf("machine type name must be a qualified name: %v", errs)))
		}

		if names.Has(machineType.Name) {
			allErrs = append(allErrs, field.Duplicate(namePath, machineType.Name))
			break
		}
		names.Insert(machineType.Name)

		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("cpu", machineType.CPU, cpuPath)...)
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("gpu", machineType.GPU, gpuPath)...)
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("memory", machineType.Memory, memoryPath)...)
		allErrs = append(allErrs, validateMachineTypeCapabilities(machineType, capabilities, idxPath)...)

		if machineType.Storage != nil {
			allErrs = append(allErrs, validateMachineTypeStorage(*machineType.Storage, idxPath.Child("storage"))...)
		}
	}

	return allErrs
}

// validateVolumeTypes validates the given list of volume types for valid values and combinations.
func validateVolumeTypes(volumeTypes []core.VolumeType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	names := make(sets.Set[string], len(volumeTypes))

	for i, volumeType := range volumeTypes {
		idxPath := fldPath.Index(i)

		namePath := idxPath.Child("name")
		if len(volumeType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		} else if errs := validateUnprefixedQualifiedName(volumeType.Name); len(errs) != 0 {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("name"), volumeType.Name, fmt.Sprintf("volume type name must be a qualified name: %v", errs)))
		}

		if names.Has(volumeType.Name) {
			allErrs = append(allErrs, field.Duplicate(namePath, volumeType.Name))
			break
		}
		names.Insert(volumeType.Name)

		if len(volumeType.Class) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("class"), "must provide a class"))
		} else if errs := validateUnprefixedQualifiedName(volumeType.Class); len(errs) != 0 {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("class"), volumeType.Class, fmt.Sprintf("volume class must be a qualified name: %v", errs)))
		}

		if volumeType.MinSize != nil {
			allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("minSize", *volumeType.MinSize, idxPath.Child("minSize"))...)
		}
	}

	return allErrs
}

func validateMachineImageVersionFlavors(machineImageVersion core.MachineImageVersion, capabilities core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateMachineImageVersionArchitecture(machineImageVersion, capabilities, fldPath)...)

	if len(capabilities) > 0 {
		supportedCapabilityKeys := slices.Collect(maps.Keys(capabilities))
		capabilitiesPath := fldPath.Child("capabilityFlavors")
		for i, imageFlavor := range machineImageVersion.CapabilityFlavors {
			flavorFldPath := capabilitiesPath.Index(i)
			for capabilityKey, capability := range imageFlavor.Capabilities {
				supportedValues, keyExists := capabilities[capabilityKey]
				if !keyExists {
					allErrs = append(allErrs, field.NotSupported(flavorFldPath, capabilityKey, supportedCapabilityKeys))
					continue
				}
				for valueIndex, value := range capability {
					if !slices.Contains(supportedValues, value) {
						allErrs = append(allErrs, field.NotSupported(flavorFldPath.Child(capabilityKey).Index(valueIndex), value, supportedValues))
					}
				}
			}
		}
	}

	return allErrs
}
func validateMachineImageVersionArchitecture(machineImageVersion core.MachineImageVersion, capabilities core.Capabilities, fldPath *field.Path) field.ErrorList {
	var (
		allErrs                = field.ErrorList{}
		supportedArchitectures = v1beta1constants.ValidArchitectures
	)

	// assert that the architecture values defined do not conflict
	if len(capabilities) > 0 {
		supportedArchitectures = capabilities[v1beta1constants.ArchitectureName]

		if len(supportedArchitectures) > 1 && len(machineImageVersion.CapabilityFlavors) == 0 {
			return append(allErrs, field.Required(fldPath.Child("capabilityFlavors"),
				"must provide at least one image flavor when multiple architectures are defined in spec.machineCapabilities"))
		}

		for flavorIdx, flavor := range machineImageVersion.CapabilityFlavors {
			architectureCapabilityValues := flavor.Capabilities[v1beta1constants.ArchitectureName]
			architectureFieldPath := fldPath.Child("capabilityFlavors").Index(flavorIdx).Child("architecture")
			if len(architectureCapabilityValues) == 0 && len(supportedArchitectures) > 1 {
				allErrs = append(allErrs, field.Required(architectureFieldPath, "must provide one architecture"))
			} else if len(architectureCapabilityValues) > 1 {
				allErrs = append(allErrs, field.Invalid(architectureFieldPath, architectureCapabilityValues, "must not define more than one architecture within an image flavor"))
			}
		}

		allCapabilityArchitectures := sets.New(gardencorehelper.ExtractArchitecturesFromImageFlavors(machineImageVersion.CapabilityFlavors)...)
		if len(allCapabilityArchitectures) == 0 && len(supportedArchitectures) == 1 {
			allCapabilityArchitectures = sets.New(supportedArchitectures...)
		}
		if len(machineImageVersion.Architectures) > 0 && !allCapabilityArchitectures.HasAll(machineImageVersion.Architectures...) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("architectures"), machineImageVersion.Architectures, fmt.Sprintf("architecture field values set (%s) conflict with the capability architectures (%s)", strings.Join(machineImageVersion.Architectures, ","), strings.Join(allCapabilityArchitectures.UnsortedList(), ","))))
		}
	}

	for archIdx, arch := range machineImageVersion.Architectures {
		if !slices.Contains(supportedArchitectures, arch) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("architectures").Index(archIdx), arch, v1beta1constants.ValidArchitectures))
		}
	}

	return allErrs
}

func validateMachineTypeCapabilities(machineType core.MachineType, capabilities core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateMachineTypeArchitecture(machineType, capabilities, fldPath)...)

	if len(capabilities) > 0 {
		supportedCapabilityKeys := slices.Collect(maps.Keys(capabilities))
		capabilitiesPath := fldPath.Child("capabilities")
		for capabilityKey, capability := range machineType.Capabilities {
			supportedValues, keyExists := capabilities[capabilityKey]
			if !keyExists {
				allErrs = append(allErrs, field.NotSupported(capabilitiesPath, capabilityKey, supportedCapabilityKeys))
				continue
			}
			for i, value := range capability {
				if !slices.Contains(supportedValues, value) {
					allErrs = append(allErrs, field.NotSupported(capabilitiesPath.Child(capabilityKey).Index(i), value, supportedValues))
				}
			}
		}
	}

	return allErrs
}
func validateMachineTypeArchitecture(machineType core.MachineType, capabilities core.Capabilities, fldPath *field.Path) field.ErrorList {
	var (
		allErrs = field.ErrorList{}

		arch                   = ptr.Deref(machineType.Architecture, "")
		supportedArchitectures = v1beta1constants.ValidArchitectures
	)

	if len(capabilities) > 0 {
		architectureCapabilityValues := machineType.Capabilities[v1beta1constants.ArchitectureName]
		if len(architectureCapabilityValues) > 1 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("capabilities.architecture"), architectureCapabilityValues, "must not define more than one architecture"))
		}
		// assert that the architecture values defined do not conflict
		if len(architectureCapabilityValues) == 1 && arch != "" && arch != architectureCapabilityValues[0] {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("architecture"), architectureCapabilityValues[0], fmt.Sprintf("machine type architecture (%s) conflicts with the capability architecture (%s)", arch, architectureCapabilityValues[0])))
		}
	}

	if arch != "" {
		if !slices.Contains(supportedArchitectures, arch) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("architecture"), arch, supportedArchitectures))
		}
	}

	return allErrs
}

func validateMachineTypeStorage(storage core.MachineTypeStorage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if storage.StorageSize == nil && storage.MinSize == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, storage, `must either configure "size" or "minSize"`))
		return allErrs
	}

	if storage.StorageSize != nil && storage.MinSize != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, storage, `not allowed to configure both "size" and "minSize"`))
		return allErrs
	}

	if storage.StorageSize != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("size", *storage.StorageSize, fldPath.Child("size"))...)
	}

	if storage.MinSize != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("minSize", *storage.MinSize, fldPath.Child("minSize"))...)
	}

	return allErrs
}

func validateExtensions(extensions []core.Extension, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	types := sets.Set[string]{}
	for i, extension := range extensions {
		if extension.Type == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("type"), "field must not be empty"))
		} else {
			allErrs = append(allErrs, validateDNS1123Label(extension.Type, fldPath.Index(i).Child("type"))...)
		}

		if types.Has(extension.Type) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("type"), extension.Type))
		} else {
			types.Insert(extension.Type)
		}
	}
	return allErrs
}

func validateResources(resources []core.NamedResourceReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	names := sets.Set[string]{}
	for i, resource := range resources {
		if resource.Name == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("name"), "field must not be empty"))
		} else if names.Has(resource.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("name"), resource.Name))
		} else {
			names.Insert(resource.Name)
		}
		allErrs = append(allErrs, validateCrossVersionObjectReference(resource.ResourceRef, fldPath.Index(i).Child("resourceRef"))...)
	}
	return allErrs
}

// ValidateCredentialsRef ensures that a resource of GVK v1.Secret or security.gardener.cloud/v1alpha1.WorkloadIdentity
// is referred, and its name and namespace are properly set.
func ValidateCredentialsRef(ref corev1.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.APIVersion) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("apiVersion"), "must provide an apiVersion"))
	}

	if len(ref.Kind) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "must provide a kind"))
	}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	for _, err := range validation.IsDNS1123Subdomain(ref.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), ref.Name, err))
	}

	if len(ref.Namespace) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "must provide a namespace"))
	}

	for _, err := range validation.IsDNS1123Subdomain(ref.Namespace) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), ref.Namespace, err))
	}

	var (
		secret           = corev1.SchemeGroupVersion.WithKind("Secret")
		workloadIdentity = securityv1alpha1.SchemeGroupVersion.WithKind("WorkloadIdentity")

		allowedGVKs = sets.New(secret, workloadIdentity)
		validGVKs   = []string{secret.String(), workloadIdentity.String()}
	)

	if !allowedGVKs.Has(ref.GroupVersionKind()) {
		allErrs = append(allErrs, field.NotSupported(fldPath, ref.String(), validGVKs))
	}

	return allErrs
}

// ValidateObjectReferenceNameAndNamespace ensures the name in the ObjectReference is set.
// Optionally, it can ensure the namespace is also set when requireNamespace=true.
func ValidateObjectReferenceNameAndNamespace(ref corev1.ObjectReference, fldPath *field.Path, requireNamespace bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}
	if requireNamespace && len(ref.Namespace) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "must provide a namespace"))
	}

	return allErrs
}
