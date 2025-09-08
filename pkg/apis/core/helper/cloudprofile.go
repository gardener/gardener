// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

func CurrentLifecycleClassification(version core.ExpirableVersion) core.VersionClassification {
	var (
		currentClassification = core.ClassificationUnavailable
		currentTime           = time.Now()
	)

	if version.Classification != nil || version.ExpirationDate != nil {
		// old cloud profile definition, convert to lifecycle
		// this can be removed as soon as we remove the old classification and expiration date fields

		if version.Classification != nil {
			version.Lifecycle = append(version.Lifecycle, core.LifecycleStage{
				Classification: *version.Classification,
			})
		}

		if version.ExpirationDate != nil {
			if version.Classification == nil {
				version.Lifecycle = append(version.Lifecycle, core.LifecycleStage{
					Classification: core.ClassificationSupported,
				})
			}

			version.Lifecycle = append(version.Lifecycle, core.LifecycleStage{
				Classification: core.ClassificationExpired,
				StartTime:      version.ExpirationDate,
			})
		}
	}

	if len(version.Lifecycle) == 0 {
		// when there is no classification lifecycle defined then default to supported
		version.Lifecycle = append(version.Lifecycle, core.LifecycleStage{
			Classification: core.ClassificationSupported,
		})
	}

	for _, stage := range version.Lifecycle {
		startTime := time.Time{}
		if stage.StartTime != nil {
			startTime = stage.StartTime.Time
		}

		if startTime.Before(currentTime) {
			currentClassification = stage.Classification
		}
	}

	return currentClassification
}

func VersionIsSupported(version core.ExpirableVersion) bool {
	return CurrentLifecycleClassification(version) == core.ClassificationSupported
}

func SupportedLifecycleClassification(version core.ExpirableVersion) *core.LifecycleStage {
	for _, stage := range version.Lifecycle {
		if stage.Classification == core.ClassificationSupported {
			return &stage
		}
	}
	return nil
}

// FindMachineImageVersion finds the machine image version in the <cloudProfile> for the given <name> and <version>.
// In case no machine image version can be found with the given <name> or <version>, false is being returned.
func FindMachineImageVersion(machineImages []core.MachineImage, name, version string) (core.MachineImageVersion, bool) {
	for _, image := range machineImages {
		if image.Name == name {
			for _, imageVersion := range image.Versions {
				if imageVersion.Version == version {
					return imageVersion, true
				}
			}
		}
	}

	return core.MachineImageVersion{}, false
}

// DetermineLatestMachineImageVersions determines the latest versions (semVer) of the given machine images from a slice of machine images
func DetermineLatestMachineImageVersions(images []core.MachineImage) (map[string]core.MachineImageVersion, error) {
	resultMapVersions := make(map[string]core.MachineImageVersion)

	for _, image := range images {
		if len(image.Versions) == 0 {
			continue
		}
		latestMachineImageVersion, err := DetermineLatestMachineImageVersion(image.Versions, false)
		if err != nil {
			return nil, fmt.Errorf("failed to determine latest machine image version for image '%s': %w", image.Name, err)
		}
		resultMapVersions[image.Name] = latestMachineImageVersion
	}
	return resultMapVersions, nil
}

// DetermineLatestMachineImageVersion determines the latest MachineImageVersion from a slice of MachineImageVersion.
// When filterPreviewVersions is set, versions with classification preview are not considered.
// It will prefer older but non-deprecated versions over newer but deprecated versions.
func DetermineLatestMachineImageVersion(versions []core.MachineImageVersion, filterPreviewVersions bool) (core.MachineImageVersion, error) {
	latestVersion, latestNonDeprecatedVersion, err := DetermineLatestExpirableVersion(ToExpirableVersions(versions), filterPreviewVersions)
	if err != nil {
		return core.MachineImageVersion{}, err
	}

	// Try to find non-deprecated version first.
	for _, version := range versions {
		if version.Version == latestNonDeprecatedVersion.Version {
			return version, nil
		}
	}

	// It looks like there is no non-deprecated version, now look also into the deprecated versions
	for _, version := range versions {
		if version.Version == latestVersion.Version {
			return version, nil
		}
	}

	return core.MachineImageVersion{}, errors.New("the latest machine version has been removed")
}

// DetermineLatestExpirableVersion determines the latest expirable version and the latest non-deprecated version from a slice of ExpirableVersions.
// When filterPreviewVersions is set, versions with classification preview are not considered.
func DetermineLatestExpirableVersion(versions []core.ExpirableVersion, filterPreviewVersions bool) (core.ExpirableVersion, core.ExpirableVersion, error) {
	var (
		latestSemVerVersion              *semver.Version
		latestNonDeprecatedSemVerVersion *semver.Version

		latestExpirableVersion              core.ExpirableVersion
		latestNonDeprecatedExpirableVersion core.ExpirableVersion
	)

	for _, version := range versions {
		v, err := semver.NewVersion(version.Version)
		if err != nil {
			return core.ExpirableVersion{}, core.ExpirableVersion{}, fmt.Errorf("error while parsing expirable version '%s': %s", version.Version, err.Error())
		}

		if filterPreviewVersions && CurrentLifecycleClassification(version) == core.ClassificationPreview {
			continue
		}

		if latestSemVerVersion == nil || v.GreaterThan(latestSemVerVersion) {
			latestSemVerVersion = v
			latestExpirableVersion = version
		}

		if CurrentLifecycleClassification(version) != core.ClassificationDeprecated {
			if latestNonDeprecatedSemVerVersion == nil || v.GreaterThan(latestNonDeprecatedSemVerVersion) {
				latestNonDeprecatedSemVerVersion = v
				latestNonDeprecatedExpirableVersion = version
			}
		}
	}

	if latestSemVerVersion == nil {
		return core.ExpirableVersion{}, core.ExpirableVersion{}, errors.New("unable to determine latest expirable version")
	}

	return latestExpirableVersion, latestNonDeprecatedExpirableVersion, nil
}

// ToExpirableVersions converts MachineImageVersion to ExpirableVersion
func ToExpirableVersions(versions []core.MachineImageVersion) []core.ExpirableVersion {
	expirableVersions := []core.ExpirableVersion{}
	for _, version := range versions {
		expirableVersions = append(expirableVersions, version.ExpirableVersion)
	}
	return expirableVersions
}

// GetRemovedVersions finds versions that have been removed in the old compared to the new version slice.
// returns a map associating the version with its index in the old version slice.
func GetRemovedVersions(old, new []core.ExpirableVersion) map[string]int {
	return getVersionDiff(old, new)
}

// GetAddedVersions finds versions that have been added in the new compared to the new version slice.
// returns a map associating the version with its index in the old version slice.
func GetAddedVersions(old, new []core.ExpirableVersion) map[string]int {
	return getVersionDiff(new, old)
}

// getVersionDiff gets versions that are in v1 but not in v2.
// Returns versions mapped to their index in v1.
func getVersionDiff(v1, v2 []core.ExpirableVersion) map[string]int {
	v2Versions := sets.Set[string]{}
	for _, x := range v2 {
		v2Versions.Insert(x.Version)
	}

	diff := map[string]int{}
	for index, x := range v1 {
		if !v2Versions.Has(x.Version) {
			diff[x.Version] = index
		}
	}
	return diff
}

// MachineImageDiff contains the diff of machine images and versions between two slices of machine images.
type MachineImageDiff struct {
	RemovedImages                 sets.Set[string]
	RemovedVersions               map[string]sets.Set[string]
	RemovedVersionClassifications map[string]map[string][]core.VersionClassification
	AddedImages                   sets.Set[string]
	AddedVersions                 map[string]sets.Set[string]
}

// GetMachineImageDiff returns the removed and added machine images and versions from the diff of two slices.
func GetMachineImageDiff(old, new []core.MachineImage) MachineImageDiff {
	diff := MachineImageDiff{
		RemovedImages:                 sets.Set[string]{},
		RemovedVersions:               map[string]sets.Set[string]{},
		RemovedVersionClassifications: map[string]map[string][]core.VersionClassification{},
		AddedImages:                   sets.Set[string]{},
		AddedVersions:                 map[string]sets.Set[string]{},
	}

	oldImages := utils.CreateMapFromSlice(old, func(image core.MachineImage) string { return image.Name })
	newImages := utils.CreateMapFromSlice(new, func(image core.MachineImage) string { return image.Name })

	for imageName, oldImage := range oldImages {
		oldImageVersions := utils.CreateMapFromSlice(oldImage.Versions, func(version core.MachineImageVersion) string { return version.Version })
		oldImageVersionsSet := sets.KeySet(oldImageVersions)
		newImage, exists := newImages[imageName]
		if !exists {
			// Completely removed images.
			diff.RemovedImages.Insert(imageName)
			diff.RemovedVersions[imageName] = oldImageVersionsSet
		} else {
			// Check for image versions diff.
			newImageVersions := utils.CreateMapFromSlice(newImage.Versions, func(version core.MachineImageVersion) string { return version.Version })
			newImageVersionsSet := sets.KeySet(newImageVersions)

			removedDiff := oldImageVersionsSet.Difference(newImageVersionsSet)
			if removedDiff.Len() > 0 {
				diff.RemovedVersions[imageName] = removedDiff
			}
			addedDiff := newImageVersionsSet.Difference(oldImageVersionsSet)
			if addedDiff.Len() > 0 {
				diff.AddedVersions[imageName] = addedDiff
			}

			for _, version := range oldImageVersions {
				if removedDiff.Has(version.Version) {
					continue
				}
				for _, existingStage := range version.Lifecycle {
					if slices.ContainsFunc(newImageVersions[version.Version].Lifecycle, func(newStage core.LifecycleStage) bool {
						return newStage.Classification == existingStage.Classification
					}) {
						continue
					}
					removedClassifications := diff.RemovedVersionClassifications[imageName]
					if removedClassifications == nil {
						removedClassifications = make(map[string][]core.VersionClassification)
						diff.RemovedVersionClassifications[imageName] = removedClassifications
					}
					removedClassifications[version.Version] = append(removedClassifications[version.Version], existingStage.Classification)
				}
			}
		}
	}

	for imageName, newImage := range newImages {
		if _, exists := oldImages[imageName]; !exists {
			// Completely new image.
			newImageVersions := utils.CreateMapFromSlice(newImage.Versions, func(version core.MachineImageVersion) string { return version.Version })
			newImageVersionsSet := sets.KeySet(newImageVersions)

			diff.AddedImages.Insert(imageName)
			diff.AddedVersions[imageName] = newImageVersionsSet
		}
	}
	return diff
}

// FilterVersionsWithClassification filters versions for a classification
func FilterVersionsWithClassification(versions []core.ExpirableVersion, classification core.VersionClassification) []core.ExpirableVersion {
	var result []core.ExpirableVersion
	for _, version := range versions {
		if (version.Classification == nil || *version.Classification != classification) &&
			!slices.ContainsFunc(version.Lifecycle, func(s core.LifecycleStage) bool {
				return s.Classification == classification
			}) {
			continue
		}

		result = append(result, version)
	}
	return result
}

// FindVersionsWithSameMajorMinor filters the given versions slice for versions other the given one, having the same major and minor version as the given version
func FindVersionsWithSameMajorMinor(versions []core.ExpirableVersion, version semver.Version) ([]core.ExpirableVersion, error) {
	var result []core.ExpirableVersion
	for _, v := range versions {
		// semantic version already checked by validator
		semVer, err := semver.NewVersion(v.Version)
		if err != nil {
			return nil, err
		}
		if semVer.Equal(&version) || semVer.Minor() != version.Minor() || semVer.Major() != version.Major() {
			continue
		}

		result = append(result, v)
	}
	return result, nil
}

// HasCapability returns true of the passed capabilities contain the capability with the given name.
func HasCapability(capabilities []core.CapabilityDefinition, capabilityName string) bool {
	for _, capability := range capabilities {
		if capability.Name == capabilityName {
			return true
		}
	}
	return false
}

// ExtractArchitecturesFromCapabilitySets extracts all architectures from a list of CapabilitySets.
func ExtractArchitecturesFromCapabilitySets(capabilities []core.CapabilitySet) []string {
	architectures := sets.New[string]()
	for _, capabilitySet := range capabilities {
		for _, architectureValue := range capabilitySet.Capabilities[constants.ArchitectureName] {
			architectures.Insert(architectureValue)
		}
	}
	return architectures.UnsortedList()
}

// CapabilityDefinitionsToCapabilities takes the capability definitions and converts them to capabilities.
func CapabilityDefinitionsToCapabilities(capabilityDefinitions []core.CapabilityDefinition) core.Capabilities {
	if len(capabilityDefinitions) == 0 {
		return nil
	}
	capabilities := make(core.Capabilities, len(capabilityDefinitions))
	for _, capability := range capabilityDefinitions {
		capabilities[capability.Name] = capability.Values
	}
	return capabilities
}

// GetCapabilitiesWithAppliedDefaults returns new capabilities with applied defaults from the capability definitions.
func GetCapabilitiesWithAppliedDefaults(capabilities core.Capabilities, capabilitiesDefinitions []core.CapabilityDefinition) core.Capabilities {
	result := make(core.Capabilities, len(capabilitiesDefinitions))
	for _, capabilityDefinition := range capabilitiesDefinitions {
		if values, ok := capabilities[capabilityDefinition.Name]; ok {
			result[capabilityDefinition.Name] = values
		} else {
			result[capabilityDefinition.Name] = capabilityDefinition.Values
		}
	}
	return result
}

// GetCapabilitySetsWithAppliedDefaults returns new capability sets with applied defaults from the capability definitions.
func GetCapabilitySetsWithAppliedDefaults(capabilitySets []core.CapabilitySet, capabilitiesDefinitions []core.CapabilityDefinition) []core.CapabilitySet {
	if len(capabilitySets) == 0 {
		// If no capability sets are defined, assume all capabilities are supported.
		return []core.CapabilitySet{{Capabilities: GetCapabilitiesWithAppliedDefaults(core.Capabilities{}, capabilitiesDefinitions)}}
	}

	result := make([]core.CapabilitySet, len(capabilitySets))
	for i, capabilitySet := range capabilitySets {
		result[i] = core.CapabilitySet{
			Capabilities: GetCapabilitiesWithAppliedDefaults(capabilitySet.Capabilities, capabilitiesDefinitions),
		}
	}
	return result
}
