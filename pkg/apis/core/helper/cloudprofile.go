// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

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

		if filterPreviewVersions && version.Classification != nil && *version.Classification == core.ClassificationPreview {
			continue
		}

		if latestSemVerVersion == nil || v.GreaterThan(latestSemVerVersion) {
			latestSemVerVersion = v
			latestExpirableVersion = version
		}

		if version.Classification != nil && *version.Classification != core.ClassificationDeprecated {
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
		if version.Classification == nil || *version.Classification != classification {
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

// SyncArchitectureCapabilityFields syncs the architecture capabilities and the architecture fields.
func SyncArchitectureCapabilityFields(newCloudProfileSpec core.CloudProfileSpec, oldCloudProfileSpec core.CloudProfileSpec) {
	hasCapabilities := len(newCloudProfileSpec.Capabilities) > 0
	if !hasCapabilities {
		return
	}

	isInitialMigration := hasCapabilities && len(oldCloudProfileSpec.Capabilities) == 0

	// For the initial migration to capabilities, sync the architecture fields to the capability definitions.
	// Subsequently only sync the architecture fields if they have not changed.
	syncMachineImageArchitectureCapabilities(newCloudProfileSpec.MachineImages, oldCloudProfileSpec.MachineImages, isInitialMigration)
	syncMachineTypeArchitectureCapabilities(newCloudProfileSpec.MachineTypes, oldCloudProfileSpec.MachineTypes, isInitialMigration)
}

func syncMachineImageArchitectureCapabilities(newMachineImages, oldMachineImages []core.MachineImage, isInitialMigration bool) {
	oldMachineImagesMap := util.NewCoreImagesContext(oldMachineImages)

	for imageIdx, image := range newMachineImages {
		for versionIdx, version := range newMachineImages[imageIdx].Versions {
			oldMachineImageVersion, oldVersionExists := oldMachineImagesMap.GetImageVersion(image.Name, version.Version)
			capabilityArchitectures := core.ExtractArchitectures(version.CapabilitySets)

			// Skip any architecture field syncing if
			// - architecture field has been modified and changed to any value other than empty.
			architecturesFieldHasBeenChanged := oldVersionExists && len(version.Architectures) > 0 &&
				(len(oldMachineImageVersion.Architectures) == 0 ||
					!apiequality.Semantic.DeepEqual(oldMachineImageVersion.Architectures, version.Architectures))
			// - both the architecture field and the architecture capability are empty or filled equally.
			if architecturesFieldHasBeenChanged || slices.Equal(capabilityArchitectures, version.Architectures) {
				continue
			}

			// Sync architecture field to capabilities if filled on initial migration.
			if isInitialMigration && len(version.Architectures) > 0 && len(version.CapabilitySets) == 0 {
				newMachineImages[imageIdx].Versions[versionIdx].CapabilitySets = append(newMachineImages[imageIdx].Versions[versionIdx].CapabilitySets,
					core.CapabilitySet{
						Capabilities: core.Capabilities{
							constants.ArchitectureKey: core.CapabilityValues{
								Values: version.Architectures,
							},
						},
					})
				continue
			}

			// Sync capability architectures to architecture field.
			if len(capabilityArchitectures) > 0 {
				newMachineImages[imageIdx].Versions[versionIdx].Architectures = capabilityArchitectures
			}
		}
	}
}

func syncMachineTypeArchitectureCapabilities(newMachineTypes, oldMachineTypes []core.MachineType, isInitialMigration bool) {
	oldMachineTypesMap := utils.CreateMapFromSlice(oldMachineTypes, func(machineType core.MachineType) string { return machineType.Name })

	for i, machineType := range newMachineTypes {
		oldMachineType, oldMachineTypeExists := oldMachineTypesMap[machineType.Name]
		architectureValue := ptr.Deref(machineType.Architecture, "")
		oldArchitectureValue := ptr.Deref(oldMachineType.Architecture, "")
		capabilityArchitectures := machineType.Capabilities[constants.ArchitectureKey].Values

		// Skip any architecture field syncing if
		// - architecture field has been modified and changed to any value other than empty.
		architectureFieldHasBeenChanged := oldMachineTypeExists && architectureValue != "" &&
			(oldArchitectureValue == "" || oldArchitectureValue != architectureValue)
		// - both the architecture field and the architecture capability are empty or filled equally.
		architecturesInSync := len(capabilityArchitectures) == 0 && architectureValue == "" ||
			len(capabilityArchitectures) == 1 && capabilityArchitectures[0] == architectureValue
		if architectureFieldHasBeenChanged || architecturesInSync {
			continue
		}

		// Sync architecture field to capabilities if filled on initial migration.
		if isInitialMigration && architectureValue != "" && len(capabilityArchitectures) == 0 {
			if newMachineTypes[i].Capabilities == nil {
				newMachineTypes[i].Capabilities = make(core.Capabilities)
			}
			newMachineTypes[i].Capabilities[constants.ArchitectureKey] = core.CapabilityValues{
				Values: []string{architectureValue},
			}
			continue
		}

		// Sync capability architecture to architecture field.
		if len(capabilityArchitectures) == 1 {
			newMachineTypes[i].Architecture = ptr.To(capabilityArchitectures[0])
		}
	}
}
