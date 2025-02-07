// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/json"
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
func ValidateMachineImages(machineImages []core.MachineImage, capabilitiesDefinition *core.Capabilities, fldPath *field.Path) field.ErrorList {
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

		if len(image.Versions) == 0 {
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

			if IsDefined(capabilitiesDefinition) {
				allErrs = append(allErrs, validateMachineImageVersionCapabilities(machineVersion, capabilitiesDefinition, versionsPath)...)
			} else {
				allErrs = append(allErrs, validateMachineImageVersionArchitecture(machineVersion.Architectures, versionsPath.Child("architecture"))...)
				if machineVersion.CapabilitiesSet != nil {
					allErrs = append(allErrs, field.Forbidden(versionsPath.Child("capabilitiesSet"), "must not provide CapabilitiesSet when no capabilitiesDefinition is defined"))
				}
			}

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
func validateMachineTypes(machineTypes []core.MachineType, capabilitiesDefinition *core.Capabilities, fldPath *field.Path) field.ErrorList {
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

		if IsDefined(capabilitiesDefinition) {
			allErrs = append(allErrs, ValidateMachineTypeCapabilities(machineType, *capabilitiesDefinition, archPath)...)

		} else {
			allErrs = append(allErrs, validateMachineTypeArchitecture(machineType.Architecture, archPath)...)
			if machineType.Capabilities != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("capabilities"), "must not provide capabilities when no capabilitiesDefinition is defined"))
			}
		}

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

	if arch == nil {
		arch = new(string)
	}

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

// CloudProfile Admission: Begin

// IsDefined checks if the capabilitiesDefinition is set and not empty
// it is intended to be used only during the transition period to capabilities and should be removed after capabilitiesDefinition is required
// then only validateCapabilitiesDefinition should be used
func IsDefined(capabilitiesDefinition *core.Capabilities) bool {
	valid := false
	if capabilitiesDefinition != nil {
		if len(*capabilitiesDefinition) != 0 {
			valid = true
		}
	}
	return valid
}

func ValidateCapabilitiesDefinition(capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	var errList field.ErrorList

	parsedCapabilitiesDefinition := ParseCapabilityValues(capabilitiesDefinition)
	errList = validateCapabilitiesDefinition(parsedCapabilitiesDefinition, path)

	// TODO (Roncossek): Add validation for providerConfigurations
	return errList
}

func validateMachineImageVersionCapabilities(machineImageVersion core.MachineImageVersion, capabilitiesDefinition *core.Capabilities, path *field.Path) field.ErrorList {
	errList := field.ErrorList{}

	if machineImageVersion.Architectures != nil {
		errList = append(errList, field.Invalid(path.Child("architectures"), machineImageVersion.Architectures, "must not be set when capabilitiesSet are used and capabilitiesDefinition is set"))
	}

	capabilitiesSet, unmarshalErrorList := UnmarshalCapabilitiesSet(machineImageVersion.CapabilitiesSet, path)
	if unmarshalErrorList != nil {
		errList = append(errList, unmarshalErrorList...)

	} else {

		parsedCapabilitiesSet := ParseCapabilitiesSet(capabilitiesSet)
		for _, parsedCapabilities := range parsedCapabilitiesSet {
			errList = append(errList, validateCapabilitiesAgainstDefinition(parsedCapabilities.toCapabilityMap(), *capabilitiesDefinition, path.Child("capabilitiesSet"))...)
		}

	}
	return errList
}

func ValidateMachineTypeCapabilities(machineType core.MachineType, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	errList := field.ErrorList{}

	if machineType.Architecture != nil {
		errList = append(errList, field.Invalid(path.Child("architecture"), machineType.Architecture, "must not be set when capabilities are used and capabilitiesDefinition is set"))
	}

	errList = validateCapabilitiesAgainstDefinition(machineType.Capabilities, capabilitiesDefinition, path)

	return errList
}

func validateCapabilitiesAgainstDefinition(capabilities core.Capabilities, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	parsedCapabilities := ParseCapabilityValues(capabilities)
	parsedCapabilitiesDefinition := ParseCapabilityValues(capabilitiesDefinition)
	var errList field.ErrorList

	if !IsDefined(&capabilitiesDefinition) {
		//  do not create errors if capabilitiesDefinition is not set, as it would pollute the error list
		//  capabilitiesDefinition must be checked before calling this function
		return errList
	}

	for capabilityName, capabilityValues := range parsedCapabilities {
		if len(capabilityValues) == 0 {
			errList = append(errList, field.Invalid(path.Child(capabilityName), capabilityValues, "must not be empty"))
			continue
		}
		if !capabilityValues.IsSubsetOf(parsedCapabilitiesDefinition[capabilityName]) {
			errList = append(errList, field.Invalid(path.Child(capabilityName), capabilityValues, "must be a subset of spec.capabilitiesDefinition of the providers cloudProfile"))
		}
	}

	return errList
}

func validateCapabilitiesDefinition(definition ParsedCapabilities, path *field.Path) field.ErrorList {
	var errList field.ErrorList

	// during the transition period to capabilities, capabilitiesDefinition is optional thus the empty definition is allowed
	// this check must be removed after capabilitiesDefinition is required
	if len(definition) == 0 {
		return errList
	}

	// architecture is a required capability
	val, ok := definition["architecture"]
	if ok {
		errList = append(errList, validateMachineImageVersionArchitecture(val.Values(), path.Child("architecture"))...)
	} else {
		errList = append(errList, field.Required(path.Child("architecture"),
			"allowed architectures are: "+strings.Join(v1beta1constants.ValidArchitectures, ", ")))
	}

	// No empty capabilities allowed
	for capabilityName, capabilityValues := range definition {
		if len(capabilityValues) == 0 {
			errList = append(errList, field.Required(path.Child(capabilityName), "must not be empty"))
		}
	}
	return errList
}

// CloudProfile Admission: End

// Generic Capabilities Functions:: Begin

func ParseCapabilitiesSet(capabilitySets []core.Capabilities) []ParsedCapabilities {
	parsedImageCapabilitySets := make([]ParsedCapabilities, len(capabilitySets))
	for j, capabilitySet := range capabilitySets {
		parsedImageCapabilitySets[j] = ParseCapabilityValues(capabilitySet)
	}
	return parsedImageCapabilitySets
}

func UnmarshalCapabilitiesSet(rawCapabilitySets []apiextensionsv1.JSON, path *field.Path) ([]core.Capabilities, field.ErrorList) {
	var allErrs field.ErrorList
	capabilitySets := make([]core.Capabilities, len(rawCapabilitySets))
	for i, rawCapabilitySet := range rawCapabilitySets {
		capabilities := core.Capabilities{}
		err := json.Unmarshal(rawCapabilitySet.Raw, &capabilities)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(path.Index(i), rawCapabilitySet, "must be a valid capabilities definition: "+err.Error()))
		}
		capabilitySets[i] = capabilities
	}

	// TODO (Roncossek): Validate that the capabilities are not empty and correctly unmarshalled
	return capabilitySets, allErrs
}

func MarshalCapabilitiesSets(capabilitiesSets []core.Capabilities, path *field.Path) ([]apiextensionsv1.JSON, field.ErrorList) {
	var allErrs field.ErrorList
	returnJSONs := make([]apiextensionsv1.JSON, len(capabilitiesSets))

	for _, capabilities := range capabilitiesSets {
		rawJSON, err := json.Marshal(capabilities)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(path, capabilities, "must be a valid capabilities definition: "+err.Error()))
		}
		returnJSONs = append(returnJSONs, apiextensionsv1.JSON{Raw: rawJSON})
	}
	return returnJSONs, allErrs
}

// GetCapabilitiesIntersection creates intersection of two parsed capabilities
func GetCapabilitiesIntersection(capabilities ParsedCapabilities, otherCapabilities ParsedCapabilities) ParsedCapabilities {
	intersection := make(ParsedCapabilities)
	for capabilityName, capabilityValues := range capabilities {
		intersection[capabilityName] = capabilityValues.Intersection(otherCapabilities[capabilityName])
	}
	return intersection
}

func HasEmptyCapabilityValue(capabilities ParsedCapabilities) bool {
	for _, capabilityValues := range capabilities {
		if len(capabilityValues) == 0 {
			return true
		}
	}
	return false
}

func ParseCapabilityValues(capabilities core.Capabilities) ParsedCapabilities {
	parsedCapabilities := make(ParsedCapabilities)
	for capabilityName, capabilityValuesString := range capabilities {
		capabilityValues := splitAndSanitize(capabilityValuesString)
		parsedCapabilities[capabilityName] = CreateCapabilityValueSet(capabilityValues)

	}
	return parsedCapabilities
}

// function to return sanitized values of a comma separated string
// e.g. ",a ,'b', c" -> ["a", "b", "c"]
func splitAndSanitize(valueString string) []string {
	values := strings.Split(valueString, ",")
	for i := 0; i < len(values); i++ {

		// strip leading and trailing whitespaces, single quotes & double quotes
		values[i] = strings.Trim(values[i], "' \"")

		if len(strings.TrimSpace(values[i])) == 0 {
			values = append(values[:i], values[i+1:]...)
			i--
		}
	}
	return values
}

// ParsedCapabilities is the internal runtime representation of Capabilities
type ParsedCapabilities map[string]CapabilityValueSet

func (c ParsedCapabilities) Copy() ParsedCapabilities {
	capabilities := make(ParsedCapabilities)
	for capabilityName, capabilityValueSet := range c {
		capabilities[capabilityName] = CreateCapabilityValueSet(capabilityValueSet.Values())
	}
	return capabilities
}

func (c ParsedCapabilities) toCapabilityMap() core.Capabilities {
	var capabilities = core.Capabilities{}
	for capabilityName, capabilityValueSet := range c {
		capabilities[capabilityName] = strings.Join(capabilityValueSet.Values(), ",")
	}
	return capabilities
}

// CapabilityValueSet is a set of capability values
type CapabilityValueSet map[string]bool

func CreateCapabilityValueSet(values []string) CapabilityValueSet {
	capabilityValueSet := make(CapabilityValueSet)
	for _, value := range values {
		capabilityValueSet[value] = true
	}
	return capabilityValueSet
}

func (c CapabilityValueSet) Add(values ...string) {
	for _, value := range values {
		c[value] = true
	}
}

func (c CapabilityValueSet) Contains(values ...string) bool {
	for _, value := range values {
		if !c[value] {
			return false
		}
	}
	return true
}

func (c CapabilityValueSet) Remove(value string) {
	delete(c, value)
}

func (c CapabilityValueSet) Values() []string {
	values := make([]string, 0, len(c))
	for value := range c {
		values = append(values, value)
	}
	return values
}
func (c CapabilityValueSet) Intersection(other CapabilityValueSet) CapabilityValueSet {
	intersection := make(CapabilityValueSet)
	for value := range c {
		if other.Contains(value) {
			intersection.Add(value)
		}
	}
	return intersection
}

func (c CapabilityValueSet) IsSubsetOf(other CapabilityValueSet) bool {
	for value := range c {
		if !other.Contains(value) {
			return false
		}
	}
	return true
}

// Generic Capabilities Functions: End
