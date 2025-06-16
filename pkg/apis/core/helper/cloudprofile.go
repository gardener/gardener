// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

// CurrentLifecycleClassification returns the current lifecycle classification of the given version.
// An empty classification is interpreted as supported. If the version is expired, it returns ClassificationExpired.
func CurrentLifecycleClassification(version core.ExpirableVersion) core.VersionClassification {
	var currentTime = time.Now()

	if version.ExpirationDate != nil && !currentTime.Before(version.ExpirationDate.Time) {
		return core.ClassificationExpired
	}

	return ptr.Deref(version.Classification, core.ClassificationSupported)
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

// GetMachineImageDiff returns the removed and added machine images and versions from the diff of two slices.
func GetMachineImageDiff(old, new []core.MachineImage) (removedMachineImages sets.Set[string], removedMachineImageVersions map[string]sets.Set[string], addedMachineImages sets.Set[string], addedMachineImageVersions map[string]sets.Set[string]) {
	removedMachineImages = sets.Set[string]{}
	removedMachineImageVersions = map[string]sets.Set[string]{}
	addedMachineImages = sets.Set[string]{}
	addedMachineImageVersions = map[string]sets.Set[string]{}

	oldImages := utils.CreateMapFromSlice(old, func(image core.MachineImage) string { return image.Name })
	newImages := utils.CreateMapFromSlice(new, func(image core.MachineImage) string { return image.Name })

	for imageName, oldImage := range oldImages {
		oldImageVersions := utils.CreateMapFromSlice(oldImage.Versions, func(version core.MachineImageVersion) string { return version.Version })
		oldImageVersionsSet := sets.KeySet(oldImageVersions)
		newImage, exists := newImages[imageName]
		if !exists {
			// Completely removed images.
			removedMachineImages.Insert(imageName)
			removedMachineImageVersions[imageName] = oldImageVersionsSet
		} else {
			// Check for image versions diff.
			newImageVersions := utils.CreateMapFromSlice(newImage.Versions, func(version core.MachineImageVersion) string { return version.Version })
			newImageVersionsSet := sets.KeySet(newImageVersions)

			removedDiff := oldImageVersionsSet.Difference(newImageVersionsSet)
			if removedDiff.Len() > 0 {
				removedMachineImageVersions[imageName] = removedDiff
			}
			addedDiff := newImageVersionsSet.Difference(oldImageVersionsSet)
			if addedDiff.Len() > 0 {
				addedMachineImageVersions[imageName] = addedDiff
			}
		}
	}

	for imageName, newImage := range newImages {
		if _, exists := oldImages[imageName]; !exists {
			// Completely new image.
			newImageVersions := utils.CreateMapFromSlice(newImage.Versions, func(version core.MachineImageVersion) string { return version.Version })
			newImageVersionsSet := sets.KeySet(newImageVersions)

			addedMachineImages.Insert(imageName)
			addedMachineImageVersions[imageName] = newImageVersionsSet
		}
	}
	return
}

// FilterVersionsWithClassification filters versions for a classification
func FilterVersionsWithClassification(versions []core.ExpirableVersion, classification core.VersionClassification) []core.ExpirableVersion {
	var result []core.ExpirableVersion
	for _, version := range versions {
		// TODO(LucaBernstein): Check whether this behavior should be corrected (i.e. changed) in a later GEP-32-PR.
		//  The current behavior for nil classifications is treated differently across the codebase.
		if version.Classification == nil || CurrentLifecycleClassification(version) != classification {
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
