// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// DetermineMachineImageForName finds the cloud specific machine images in the <cloudProfile> for the given <name> and
// region. In case it does not find the machine image with the <name>, it returns false. Otherwise, true and the
// cloud-specific machine image will be returned.
func DetermineMachineImageForName(cloudProfile *gardencorev1beta1.CloudProfile, name string) (bool, gardencorev1beta1.MachineImage) {
	for _, image := range cloudProfile.Spec.MachineImages {
		if strings.EqualFold(image.Name, name) {
			return true, image
		}
	}
	return false, gardencorev1beta1.MachineImage{}
}

// FindMachineImageVersion finds the machine image version in the <cloudProfile> for the given <name> and <version>.
// In case no machine image version can be found with the given <name> or <version>, false is being returned.
func FindMachineImageVersion(machineImages []gardencorev1beta1.MachineImage, name, version string) (gardencorev1beta1.MachineImageVersion, bool) {
	for _, image := range machineImages {
		if image.Name == name {
			for _, imageVersion := range image.Versions {
				if imageVersion.Version == version {
					return imageVersion, true
				}
			}
		}
	}

	return gardencorev1beta1.MachineImageVersion{}, false
}

// ShootMachineImageVersionExists checks if the shoot machine image (name, version) exists in the machine image constraint and returns true if yes and the index in the versions slice
func ShootMachineImageVersionExists(constraint gardencorev1beta1.MachineImage, image gardencorev1beta1.ShootMachineImage) (bool, int) {
	if constraint.Name != image.Name {
		return false, 0
	}

	for index, v := range constraint.Versions {
		if image.Version != nil && v.Version == *image.Version {
			return true, index
		}
	}

	return false, 0
}

// ToExpirableVersions returns the expirable versions from the given machine image versions.
func ToExpirableVersions(versions []gardencorev1beta1.MachineImageVersion) []gardencorev1beta1.ExpirableVersion {
	expVersions := []gardencorev1beta1.ExpirableVersion{}
	for _, version := range versions {
		expVersions = append(expVersions, version.ExpirableVersion)
	}
	return expVersions
}

// FindMachineTypeByName tries to find the machine type details with the given name. If it cannot be found it returns nil.
func FindMachineTypeByName(machines []gardencorev1beta1.MachineType, name string) *gardencorev1beta1.MachineType {
	for _, m := range machines {
		if m.Name == name {
			return &m
		}
	}
	return nil
}

// KubernetesVersionExistsInCloudProfile checks if the given Kubernetes version exists in the CloudProfile
func KubernetesVersionExistsInCloudProfile(cloudProfile *gardencorev1beta1.CloudProfile, currentVersion string) (bool, gardencorev1beta1.ExpirableVersion, error) {
	for _, version := range cloudProfile.Spec.Kubernetes.Versions {
		ok, err := versionutils.CompareVersions(version.Version, "=", currentVersion)
		if err != nil {
			return false, gardencorev1beta1.ExpirableVersion{}, err
		}
		if ok {
			return true, version, nil
		}
	}
	return false, gardencorev1beta1.ExpirableVersion{}, nil
}

// SetMachineImageVersionsToMachineImage sets imageVersions to the matching imageName in the machineImages.
func SetMachineImageVersionsToMachineImage(machineImages []gardencorev1beta1.MachineImage, imageName string, imageVersions []gardencorev1beta1.MachineImageVersion) ([]gardencorev1beta1.MachineImage, error) {
	for index, image := range machineImages {
		if strings.EqualFold(image.Name, imageName) {
			machineImages[index].Versions = imageVersions
			return machineImages, nil
		}
	}
	return nil, fmt.Errorf("machine image with name '%s' could not be found", imageName)
}

// GetDefaultMachineImageFromCloudProfile gets the first MachineImage from the CloudProfile
func GetDefaultMachineImageFromCloudProfile(profile gardencorev1beta1.CloudProfile) *gardencorev1beta1.MachineImage {
	if len(profile.Spec.MachineImages) == 0 {
		return nil
	}
	return &profile.Spec.MachineImages[0]
}

// VersionPredicate is a function that evaluates a condition on the given versions.
type VersionPredicate func(expirableVersion gardencorev1beta1.ExpirableVersion, version *semver.Version) (bool, error)

// GetLatestVersionForPatchAutoUpdate finds the latest patch version for a given <currentVersion> for the current minor version from a given slice of versions.
// The current version, preview and expired versions do not qualify.
// In case no newer patch version is found, returns false and an empty string. Otherwise, returns true and the found version.
func GetLatestVersionForPatchAutoUpdate(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	predicates := []VersionPredicate{FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor(*currentSemVerVersion)}

	return getVersionForAutoUpdate(versions, currentSemVerVersion, predicates)
}

// GetLatestVersionForMinorAutoUpdate finds the latest minor with the latest patch version higher than a given <currentVersion> for the current major version from a given slice of versions.
// Returns the highest patch version for the current minor in case the current version is not the highest patch version yet.
// The current version, preview and expired versions do not qualify.
// In case no newer version is found, returns false and an empty string. Otherwise, returns true and the found version.
func GetLatestVersionForMinorAutoUpdate(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	// always first check if there is a higher patch version available
	found, version, err := GetLatestVersionForPatchAutoUpdate(versions, currentVersion)
	if found {
		return found, version, nil
	}
	if err != nil {
		return false, version, err
	}

	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	predicates := []VersionPredicate{FilterDifferentMajorVersion(*currentSemVerVersion)}

	return getVersionForAutoUpdate(versions, currentSemVerVersion, predicates)
}

// GetOverallLatestVersionForAutoUpdate finds the overall latest version higher than a given <currentVersion> for the current major version from a given slice of versions.
// Returns the highest patch version for the current minor in case the current version is not the highest patch version yet.
// The current, preview and expired versions do not qualify.
// In case no newer version is found, returns false and an empty string. Otherwise, returns true and the found version.
func GetOverallLatestVersionForAutoUpdate(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	// always first check if there is a higher patch version available to update to
	found, version, err := GetLatestVersionForPatchAutoUpdate(versions, currentVersion)
	if found {
		return found, version, nil
	}
	if err != nil {
		return false, version, err
	}

	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	// if there is no higher patch version available, get the overall latest
	return getVersionForAutoUpdate(versions, currentSemVerVersion, []VersionPredicate{})
}

// getVersionForAutoUpdate finds the latest eligible version higher than a given <currentVersion> from a slice of versions.
// Versions <= the current version, preview and expired versions do not qualify for patch updates.
// First tries to find a non-deprecated version.
// In case no newer patch version is found, returns false and an empty string. Otherwise, returns true and the found version.
func getVersionForAutoUpdate(versions []gardencorev1beta1.ExpirableVersion, currentSemVerVersion *semver.Version, predicates []VersionPredicate) (bool, string, error) {
	versionPredicates := append([]VersionPredicate{FilterExpiredVersion(), FilterSameVersion(*currentSemVerVersion), FilterLowerVersion(*currentSemVerVersion)}, predicates...)

	// Try to find non-deprecated version first
	qualifyingVersionFound, latestNonDeprecatedImageVersion, err := GetLatestQualifyingVersion(versions, append(versionPredicates, FilterDeprecatedVersion())...)
	if err != nil {
		return false, "", err
	}
	if qualifyingVersionFound {
		return true, latestNonDeprecatedImageVersion.Version, nil
	}

	// otherwise, also consider deprecated versions
	qualifyingVersionFound, latestVersion, err := GetLatestQualifyingVersion(versions, versionPredicates...)
	if err != nil {
		return false, "", err
	}
	// latest version cannot be found. Do not return an error, but allow for forceful upgrade if Shoot's version is expired.
	if !qualifyingVersionFound {
		return false, "", nil
	}

	return true, latestVersion.Version, nil
}

// GetVersionForForcefulUpdateToConsecutiveMinor finds a version from a slice of expirable versions that qualifies for a minor level update given a <currentVersion>.
// A qualifying version is a non-preview version having the minor version increased by exactly one version (required for Kubernetes version upgrades).
// In case the consecutive minor version has only expired versions, picks the latest expired version (will try another update during the next maintenance time).
// If a version can be found, returns true and the qualifying patch version of the next minor version.
// In case it does not find a version, it returns false and an empty string.
func GetVersionForForcefulUpdateToConsecutiveMinor(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	// filters out any version that does not have minor version +1
	predicates := []VersionPredicate{FilterDifferentMajorVersion(*currentSemVerVersion), FilterNonConsecutiveMinorVersion(*currentSemVerVersion)}

	qualifyingVersionFound, latestVersion, err := GetLatestQualifyingVersion(versions, append(predicates, FilterExpiredVersion())...)
	if err != nil {
		return false, "", err
	}

	// if no qualifying version is found, allow force update to an expired version
	if !qualifyingVersionFound {
		qualifyingVersionFound, latestVersion, err = GetLatestQualifyingVersion(versions, predicates...)
		if err != nil {
			return false, "", err
		}
		if !qualifyingVersionFound {
			return false, "", nil
		}
	}

	return true, latestVersion.Version, nil
}

// GetVersionForForcefulUpdateToNextHigherMinor finds a version from a slice of expirable versions that qualifies for a minor level update given a <currentVersion>.
// A qualifying version is the highest non-preview version with the next higher minor version from the given slice of versions.
// In case the consecutive minor version has only expired versions, picks the latest expired version (will try another update during the next maintenance time).
// If a version can be found, returns true and the qualifying version.
// In case it does not find a version, it returns false and an empty string.
func GetVersionForForcefulUpdateToNextHigherMinor(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	predicates := []VersionPredicate{FilterDifferentMajorVersion(*currentSemVerVersion), FilterEqualAndSmallerMinorVersion(*currentSemVerVersion)}

	// prefer non-expired version
	return getVersionForMachineImageForceUpdate(versions, func(v semver.Version) uint64 { return v.Minor() }, currentSemVerVersion, predicates)
}

// GetVersionForForcefulUpdateToNextHigherMajor finds a version from a slice of expirable versions that qualifies for a major level update given a <currentVersion>.
// A qualifying version is a non-preview version with the next (as defined in the CloudProfile for the image) higher major version.
// In case the next major version has only expired versions, picks the latest expired version (will try another update during the next maintenance time).
// If a version can be found, returns true and the qualifying version of the next major version.
// In case it does not find a version, it returns false and an empty string.
func GetVersionForForcefulUpdateToNextHigherMajor(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	predicates := []VersionPredicate{FilterEqualAndSmallerMajorVersion(*currentSemVerVersion)}

	// prefer non-expired version
	return getVersionForMachineImageForceUpdate(versions, func(v semver.Version) uint64 { return v.Major() }, currentSemVerVersion, predicates)
}

// getVersionForMachineImageForceUpdate finds a version from a slice of expirable versions that qualifies for an update given a <currentVersion>.
// In contrast to determining a version for an auto-update, also allows update to an expired version in case a not-expired version cannot be determined.
// Used only for machine image updates, as finds a qualifying version from the next higher minor version, which is not necessarily consecutive (n+1).
func getVersionForMachineImageForceUpdate(versions []gardencorev1beta1.ExpirableVersion, getMajorOrMinor GetMajorOrMinor, currentSemVerVersion *semver.Version, predicates []VersionPredicate) (bool, string, error) {
	foundVersion, qualifyingVersion, nextMinorOrMajorVersion, err := GetQualifyingVersionForNextHigher(versions, getMajorOrMinor, currentSemVerVersion, append(predicates, FilterExpiredVersion())...)
	if err != nil {
		return false, "", err
	}

	skippedNextMajorMinor := false

	if foundVersion {
		parse, err := semver.NewVersion(qualifyingVersion.Version)
		if err != nil {
			return false, "", err
		}

		skippedNextMajorMinor = getMajorOrMinor(*parse) > nextMinorOrMajorVersion
	}

	// Two options when allowing updates to expired versions
	// 1) No higher non-expired qualifying version could be found at all
	// 2) Found a qualifying non-expired version, but we skipped the next minor/major.
	//    Potentially skipped expired versions in the next minor/major that qualify.
	//    Prefer update to expired version in next minor/major instead of skipping over minor/major altogether.
	//    Example: current version: 1.1.0, qualifying version : 1.4.1, next minor: 2. We skipped over the next minor which might have qualifying expired versions.
	if !foundVersion || skippedNextMajorMinor {
		foundVersion, qualifyingVersion, _, err = GetQualifyingVersionForNextHigher(versions, getMajorOrMinor, currentSemVerVersion, predicates...)
		if err != nil {
			return false, "", err
		}
		if !foundVersion {
			return false, "", nil
		}
	}

	return true, qualifyingVersion.Version, nil
}

// GetLatestQualifyingVersion returns the latest expirable version from a set of expirable versions.
// A version qualifies if its classification is not preview and the optional predicate does not filter out the version.
// If the predicate returns true, the version is not considered for the latest qualifying version.
func GetLatestQualifyingVersion(versions []gardencorev1beta1.ExpirableVersion, predicate ...VersionPredicate) (qualifyingVersionFound bool, latest *gardencorev1beta1.ExpirableVersion, err error) {
	var (
		latestSemanticVersion = &semver.Version{}
		latestVersion         *gardencorev1beta1.ExpirableVersion
	)
OUTER:
	for _, v := range versions {
		if v.Classification != nil && *v.Classification == gardencorev1beta1.ClassificationPreview {
			continue
		}

		semver, err := semver.NewVersion(v.Version)
		if err != nil {
			return false, nil, fmt.Errorf("error while parsing version '%s': %s", v.Version, err.Error())
		}

		for _, p := range predicate {
			if p == nil {
				continue
			}

			shouldFilter, err := p(v, semver)
			if err != nil {
				return false, nil, fmt.Errorf("error while evaluation predicate: '%s'", err.Error())
			}
			if shouldFilter {
				continue OUTER
			}
		}

		if semver.GreaterThan(latestSemanticVersion) {
			latestSemanticVersion = semver
			// avoid DeepCopy
			latest := v
			latestVersion = &latest
		}
	}
	// unable to find qualified versions
	if latestVersion == nil {
		return false, nil, nil
	}
	return true, latestVersion, nil
}

// GetMajorOrMinor returns either the major or the minor version from a semVer version.
type GetMajorOrMinor func(v semver.Version) uint64

// GetQualifyingVersionForNextHigher returns the latest expirable version for the next higher {minor/major} (not necessarily consecutive n+1) version from a set of expirable versions.
// A version qualifies if its classification is not preview and the optional predicate does not filter out the version.
// If the predicate returns true, the version is not considered for the latest qualifying version.
func GetQualifyingVersionForNextHigher(versions []gardencorev1beta1.ExpirableVersion, majorOrMinor GetMajorOrMinor, currentSemVerVersion *semver.Version, predicates ...VersionPredicate) (qualifyingVersionFound bool, qualifyingVersion *gardencorev1beta1.ExpirableVersion, nextMinorOrMajor uint64, err error) {
	// How to find the highest version with the next higher (not necessarily consecutive n+1) minor version (if the next higher minor version has no qualifying version, skip it to avoid consecutive updates)
	// 1) Sort the versions in ascending order
	// 2) Loop over the sorted array until the minor version changes (select all versions for the next higher minor)
	//    - predicates filter out version with minor/major <= current_minor/major
	// 3) Then select the last version in the array (that's the highest)

	slices.SortFunc(versions, func(a, b gardencorev1beta1.ExpirableVersion) int {
		return semver.MustParse(a.Version).Compare(semver.MustParse(b.Version))
	})

	var (
		highestVersionNextHigherMinorOrMajor   *semver.Version
		nextMajorOrMinorVersion                uint64
		isNextMajorOrMinorVersionSet           bool
		expirableVersionNextHigherMinorOrMajor = gardencorev1beta1.ExpirableVersion{}
	)

OUTER:
	for _, v := range versions {
		parse, err := semver.NewVersion(v.Version)
		if err != nil {
			return false, nil, 0, err
		}

		// Determine the next higher minor/major version, even though all versions from that minor/major might be filtered (e.g, all expired)
		// That's required so that the caller can determine if the next minor/major version has been skipped or not.
		if majorOrMinor(*parse) > majorOrMinor(*currentSemVerVersion) && (majorOrMinor(*parse) < nextMajorOrMinorVersion || !isNextMajorOrMinorVersionSet) {
			nextMajorOrMinorVersion = majorOrMinor(*parse)
			isNextMajorOrMinorVersionSet = true
		}

		// never update to preview versions
		if v.Classification != nil && *v.Classification == gardencorev1beta1.ClassificationPreview {
			continue
		}

		for _, p := range predicates {
			if p == nil {
				continue
			}

			shouldFilter, err := p(v, parse)
			if err != nil {
				return false, nil, nextMajorOrMinorVersion, fmt.Errorf("error while evaluation predicate: %w", err)
			}
			if shouldFilter {
				continue OUTER
			}
		}

		// last version is the highest version for next larger minor/major
		if highestVersionNextHigherMinorOrMajor != nil && majorOrMinor(*parse) > majorOrMinor(*highestVersionNextHigherMinorOrMajor) {
			break
		}
		highestVersionNextHigherMinorOrMajor = parse
		expirableVersionNextHigherMinorOrMajor = v
	}

	// unable to find qualified versions
	if highestVersionNextHigherMinorOrMajor == nil {
		return false, nil, nextMajorOrMinorVersion, nil
	}
	return true, &expirableVersionNextHigherMinorOrMajor, nextMajorOrMinorVersion, nil
}

// FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor returns a VersionPredicate(closure) that returns true if a given version v
//   - has a different major.minor version compared to the currentSemVerVersion
//   - has a lower patch version (acts as >= relational operator)
//
// Uses the tilde range operator.
func FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		isWithinRange, err := versionutils.CompareVersions(v.String(), "~", currentSemVerVersion.String())
		if err != nil {
			return true, err
		}
		return !isWithinRange, nil
	}
}

// FilterNonConsecutiveMinorVersion returns a VersionPredicate(closure) that evaluates whether a given version v has a consecutive minor version compared to the currentSemVerVersion
//   - implicitly, therefore also versions cannot be smaller than the current version
//
// returns true if v does not have a consecutive minor version.
func FilterNonConsecutiveMinorVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		if v.Major() != currentSemVerVersion.Major() {
			return true, nil
		}

		hasIncorrectMinor := currentSemVerVersion.Minor()+1 != v.Minor()
		return hasIncorrectMinor, nil
	}
}

// FilterDifferentMajorVersion returns a VersionPredicate(closure) that evaluates whether a given version v has the same major version compared to the currentSemVerVersion.
// Returns true if v does not have the same major version.
func FilterDifferentMajorVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		return v.Major() != currentSemVerVersion.Major(), nil
	}
}

// FilterEqualAndSmallerMajorVersion returns a VersionPredicate(closure) that evaluates whether a given version v has a smaller major version compared to the currentSemVerVersion.
// Returns true if v has a smaller or equal major version.
func FilterEqualAndSmallerMajorVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		return v.Major() <= currentSemVerVersion.Major(), nil
	}
}

// FilterEqualAndSmallerMinorVersion returns a VersionPredicate(closure) that evaluates whether a given version v has a smaller or equal minor version compared to the currentSemVerVersion.
// Returns true if v has a smaller or equal minor version.
func FilterEqualAndSmallerMinorVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		return v.Minor() <= currentSemVerVersion.Minor(), nil
	}
}

// FilterSameVersion returns a VersionPredicate(closure) that evaluates whether a given version v is equal to the currentSemVerVersion.
// returns true if it is equal.
func FilterSameVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		return v.Equal(&currentSemVerVersion), nil
	}
}

// FilterLowerVersion returns a VersionPredicate(closure) that evaluates whether a given version v is lower than the currentSemVerVersion
// returns true if it is lower
func FilterLowerVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		return v.LessThan(&currentSemVerVersion), nil
	}
}

// FilterExpiredVersion returns a closure that evaluates whether a given expirable version is expired
// returns true if it is expired
func FilterExpiredVersion() func(expirableVersion gardencorev1beta1.ExpirableVersion, version *semver.Version) (bool, error) {
	return func(expirableVersion gardencorev1beta1.ExpirableVersion, _ *semver.Version) (bool, error) {
		return expirableVersion.ExpirationDate != nil && (time.Now().UTC().After(expirableVersion.ExpirationDate.UTC()) || time.Now().UTC().Equal(expirableVersion.ExpirationDate.UTC())), nil
	}
}

// FilterDeprecatedVersion returns a closure that evaluates whether a given expirable version is deprecated
// returns true if it is deprecated
func FilterDeprecatedVersion() func(expirableVersion gardencorev1beta1.ExpirableVersion, version *semver.Version) (bool, error) {
	return func(expirableVersion gardencorev1beta1.ExpirableVersion, _ *semver.Version) (bool, error) {
		return expirableVersion.Classification != nil && *expirableVersion.Classification == gardencorev1beta1.ClassificationDeprecated, nil
	}
}

// GetResourceByName returns the NamedResourceReference with the given name in the given slice, or nil if not found.
func GetResourceByName(resources []gardencorev1beta1.NamedResourceReference, name string) *gardencorev1beta1.NamedResourceReference {
	for _, resource := range resources {
		if resource.Name == name {
			return &resource
		}
	}
	return nil
}
