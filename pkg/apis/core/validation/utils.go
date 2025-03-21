// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
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

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	kubernetescorevalidation "github.com/gardener/gardener/pkg/utils/validation/kubernetes/core"
)

// ValidateName is a helper function for validating that a name is a DNS subdomain.
func ValidateName(name string, prefix bool) []string {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

func validateSecretReference(ref corev1.SecretReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}
	if len(ref.Namespace) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "must provide a namespace"))
	}

	return allErrs
}

func validateCrossVersionObjectReference(ref autoscalingv1.CrossVersionObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.APIVersion) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("apiVersion"), "must provide an apiVersion"))
	} else {
		if ref.APIVersion != corev1.SchemeGroupVersion.String() {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiVersion"), ref.APIVersion, []string{corev1.SchemeGroupVersion.String()}))
		}
	}

	if len(ref.Kind) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "must provide a kind"))
	} else {
		if ref.Kind != "Secret" && ref.Kind != "ConfigMap" {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("kind"), ref.Kind, []string{"Secret", "ConfigMap"}))
		}
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
func ValidateMachineImages(machineImages []core.MachineImage, fldPath *field.Path, allowEmptyVersions bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImages) == 0 {
		return allErrs
	}

	latestMachineImages, err := helper.DetermineLatestMachineImageVersions(machineImages)
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
					allErrs = append(allErrs, field.Invalid(versionsPath.Child("minVersionForInPlaceUpdate"), machineVersion.Version, "could not parse version. Use a semantic version."))
				}
			}

			if machineVersion.Classification != nil && !supportedVersionClassifications.Has(string(*machineVersion.Classification)) {
				allErrs = append(allErrs, field.NotSupported(versionsPath.Child("classification"), *machineVersion.Classification, sets.List(supportedVersionClassifications)))
			}

			allErrs = append(allErrs, validateMachineImageVersionArchitecture(machineVersion.Architectures, versionsPath.Child("architecture"))...)

			if machineVersion.KubeletVersionConstraint != nil {
				if _, err := semver.NewConstraint(*machineVersion.KubeletVersionConstraint); err != nil {
					allErrs = append(allErrs, field.Invalid(versionsPath.Child("kubeletVersionConstraint"), machineVersion.KubeletVersionConstraint, fmt.Sprintf("cannot parse the kubeletVersionConstraint: %s", err.Error())))
				}
			}
		}
	}

	return allErrs
}

// validateMachineTypes validates the given list of machine types for valid values and combinations.
func validateMachineTypes(machineTypes []core.MachineType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	names := make(map[string]struct{}, len(machineTypes))

	for i, machineType := range machineTypes {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		cpuPath := idxPath.Child("cpu")
		gpuPath := idxPath.Child("gpu")
		memoryPath := idxPath.Child("memory")
		archPath := idxPath.Child("architecture")

		if len(machineType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		}

		if _, ok := names[machineType.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(namePath, machineType.Name))
			break
		}
		names[machineType.Name] = struct{}{}

		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("cpu", machineType.CPU, cpuPath)...)
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("gpu", machineType.GPU, gpuPath)...)
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("memory", machineType.Memory, memoryPath)...)
		allErrs = append(allErrs, validateMachineTypeArchitecture(machineType.Architecture, archPath)...)

		if machineType.Storage != nil {
			allErrs = append(allErrs, validateMachineTypeStorage(*machineType.Storage, idxPath.Child("storage"))...)
		}
	}

	return allErrs
}

// validateVolumeTypes validates the given list of volume types for valid values and combinations.
func validateVolumeTypes(volumeTypes []core.VolumeType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	names := make(map[string]struct{}, len(volumeTypes))

	for i, volumeType := range volumeTypes {
		idxPath := fldPath.Index(i)

		namePath := idxPath.Child("name")
		if len(volumeType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		}

		if _, ok := names[volumeType.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(namePath, volumeType.Name))
			break
		}
		names[volumeType.Name] = struct{}{}

		if len(volumeType.Class) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("class"), "must provide a class"))
		}

		if volumeType.MinSize != nil {
			allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("minSize", *volumeType.MinSize, idxPath.Child("minSize"))...)
		}
	}

	return allErrs
}

func validateMachineImageVersionArchitecture(archs []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, arch := range archs {
		if !slices.Contains(v1beta1constants.ValidArchitectures, arch) {
			allErrs = append(allErrs, field.NotSupported(fldPath, arch, v1beta1constants.ValidArchitectures))
		}
	}

	return allErrs
}

func validateMachineTypeArchitecture(arch *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if !slices.Contains(v1beta1constants.ValidArchitectures, *arch) {
		allErrs = append(allErrs, field.NotSupported(fldPath, *arch, v1beta1constants.ValidArchitectures))
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
		} else if types.Has(extension.Type) {
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
