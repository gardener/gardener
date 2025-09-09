// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// FilterMachineImageVersions filters the machine image versions based on the worker's architecture, machineType capabilities, CRI, and kubelet version.
func FilterMachineImageVersions(
	machineImageFromCloudProfile *gardencorev1beta1.MachineImage,
	worker gardencorev1beta1.Worker,
	kubeletVersion *semver.Version,
	machineTypeFromCloudProfile *gardencorev1beta1.MachineType,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
) *gardencorev1beta1.MachineImage {
	filteredMachineImageVersions := filterForArchitecture(machineImageFromCloudProfile, worker.Machine.Architecture, capabilityDefinitions)
	filteredMachineImageVersions = filterForCapabilities(filteredMachineImageVersions, machineTypeFromCloudProfile.Capabilities, capabilityDefinitions)
	filteredMachineImageVersions = filterForCRI(filteredMachineImageVersions, worker.CRI)
	filteredMachineImageVersions = filterForKubeletVersionConstraint(filteredMachineImageVersions, kubeletVersion)
	filteredMachineImageVersions = filterForInPlaceUpdateConstraint(filteredMachineImageVersions, worker.Machine.Image.Version, v1beta1helper.IsUpdateStrategyInPlace(worker.UpdateStrategy))

	return filteredMachineImageVersions
}

func filterForCapabilities(machineImageFromCloudProfile *gardencorev1beta1.MachineImage, machineCapabilities gardencorev1beta1.Capabilities, capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition) *gardencorev1beta1.MachineImage {
	if len(capabilitiesDefinitions) == 0 {
		return machineImageFromCloudProfile
	}

	filteredMachineImages := gardencorev1beta1.MachineImage{
		Name:           machineImageFromCloudProfile.Name,
		UpdateStrategy: machineImageFromCloudProfile.UpdateStrategy,
		Versions:       []gardencorev1beta1.MachineImageVersion{},
	}

	for _, cloudProfileVersion := range machineImageFromCloudProfile.Versions {
		if v1beta1helper.AreCapabilitiesSupportedByCapabilitySets(machineCapabilities, cloudProfileVersion.Flavors, capabilitiesDefinitions) {
			filteredMachineImages.Versions = append(filteredMachineImages.Versions, cloudProfileVersion)
		}
	}

	return &filteredMachineImages
}

// DetermineMachineImage determines the machine image from cloudprofile based on the provided cloud profile and shoot machine image.
func DetermineMachineImage(cloudProfile *gardencorev1beta1.CloudProfile, shootMachineImage *gardencorev1beta1.ShootMachineImage) (gardencorev1beta1.MachineImage, error) {
	machineImagesFound, machineImageFromCloudProfile := v1beta1helper.DetermineMachineImageForName(cloudProfile, shootMachineImage.Name)
	if !machineImagesFound {
		return gardencorev1beta1.MachineImage{}, fmt.Errorf("failure while determining the default machine image in the CloudProfile: no machineImage with name %q (specified in shoot) could be found in the cloudProfile %q", shootMachineImage.Name, cloudProfile.Name)
	}

	return machineImageFromCloudProfile, nil
}

// GetHigherVersion takes a slice of versions and returns if higher suitable version could be found, the version or an error
type GetHigherVersion func(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error)

// DetermineMachineImageVersion determines the machine image version for a shoot based on the provided shootMachineImage and machineImage.
func DetermineMachineImageVersion(shootMachineImage *gardencorev1beta1.ShootMachineImage, machineImage *gardencorev1beta1.MachineImage, isExpired bool) (string, error) {
	var (
		getHigherVersionAutoUpdate  GetHigherVersion
		getHigherVersionForceUpdate GetHigherVersion
	)

	switch *machineImage.UpdateStrategy {
	case gardencorev1beta1.UpdateStrategyPatch:
		getHigherVersionAutoUpdate = v1beta1helper.GetLatestVersionForPatchAutoUpdate
		getHigherVersionForceUpdate = v1beta1helper.GetVersionForForcefulUpdateToNextHigherMinor
	case gardencorev1beta1.UpdateStrategyMinor:
		getHigherVersionAutoUpdate = v1beta1helper.GetLatestVersionForMinorAutoUpdate
		getHigherVersionForceUpdate = v1beta1helper.GetVersionForForcefulUpdateToNextHigherMajor
	default:
		// auto-update strategy: "major"
		getHigherVersionAutoUpdate = v1beta1helper.GetOverallLatestVersionForAutoUpdate
		// cannot force update the overall latest version if it is expired
		getHigherVersionForceUpdate = func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
			return false, "", fmt.Errorf("either the machine image %q is reaching end of life and migration to another machine image is required or there is a misconfiguration in the CloudProfile. If it is the latter, make sure the machine image in the CloudProfile has at least one version that is not expired, not in preview and greater or equal to the current Shoot image version %q", shootMachineImage.Name, *shootMachineImage.Version)
		}
	}

	version, err := DetermineVersionForStrategy(
		v1beta1helper.ToExpirableVersions(machineImage.Versions),
		*shootMachineImage.Version,
		getHigherVersionAutoUpdate,
		getHigherVersionForceUpdate,
		isExpired)
	if err != nil {
		return version, fmt.Errorf("failed to determine the target version for maintenance of machine image %q with strategy %q: %w", machineImage.Name, *machineImage.UpdateStrategy, err)
	}

	return version, nil
}

// DetermineVersionForStrategy determines the target version for a machine image based on the update strategy.
func DetermineVersionForStrategy(expirableVersions []gardencorev1beta1.ExpirableVersion, currentVersion string, getHigherVersionAutoUpdate GetHigherVersion, getHigherVersionForceUpdate GetHigherVersion, isCurrentVersionExpired bool) (string, error) {
	higherQualifyingVersionFound, latestVersionForMajor, err := getHigherVersionAutoUpdate(expirableVersions, currentVersion)
	if err != nil {
		return "", fmt.Errorf("failed to determine a higher patch version for automatic update: %w", err)
	}

	if higherQualifyingVersionFound {
		return latestVersionForMajor, nil
	}

	// The current version is already up-to date
	//  - Kubernetes version / Auto update strategy "patch": the latest patch version for the current minor version
	//  - Auto update strategy "minor": the latest patch and minor version for the current major version
	//  - Auto update strategy "major": the latest overall version
	if !isCurrentVersionExpired {
		return "", nil
	}

	// The version is already the latest version according to the strategy, but is expired. Force update.
	forceUpdateVersionAvailable, versionForForceUpdate, err := getHigherVersionForceUpdate(expirableVersions, currentVersion)
	if err != nil {
		return "", fmt.Errorf("failed to determine version for forceful update: %w", err)
	}

	// Unable to force update
	//  - Kubernetes version: no consecutive minor version available (e.g. shoot is on 1.24.X, but there is only 1.26.X, available and not 1.25.X)
	//  - Auto update strategy "patch": no higher next minor version available (e.g. shoot is on 1.0.X, but there is only 2.2.X, available and not 1.X.X)
	//  - Auto update strategy "minor": no higher next major version available (e.g. shoot is on 576.3.0, but there is no higher major version available)
	//  - Auto update strategy "major": already on latest overall version, but the latest version is expired. EOL for image or CloudProfile misconfiguration.
	if !forceUpdateVersionAvailable {
		return "", fmt.Errorf("cannot perform forceful update of expired version %q. No suitable version found in CloudProfile - this is most likely a misconfiguration of the CloudProfile", currentVersion)
	}

	return versionForForceUpdate, nil
}

func filterForArchitecture(machineImageFromCloudProfile *gardencorev1beta1.MachineImage, arch *string, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) *gardencorev1beta1.MachineImage {
	filteredMachineImages := gardencorev1beta1.MachineImage{
		Name:           machineImageFromCloudProfile.Name,
		UpdateStrategy: machineImageFromCloudProfile.UpdateStrategy,
		Versions:       []gardencorev1beta1.MachineImageVersion{},
	}

	for _, cloudProfileVersion := range machineImageFromCloudProfile.Versions {
		if slices.Contains(v1beta1helper.GetArchitecturesFromImageVersion(cloudProfileVersion, capabilityDefinitions), *arch) {
			filteredMachineImages.Versions = append(filteredMachineImages.Versions, cloudProfileVersion)
		}
	}

	return &filteredMachineImages
}

func filterForCRI(machineImageFromCloudProfile *gardencorev1beta1.MachineImage, workerCRI *gardencorev1beta1.CRI) *gardencorev1beta1.MachineImage {
	if workerCRI == nil {
		return filterForCRI(machineImageFromCloudProfile, &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD})
	}

	filteredMachineImages := gardencorev1beta1.MachineImage{
		Name:           machineImageFromCloudProfile.Name,
		UpdateStrategy: machineImageFromCloudProfile.UpdateStrategy,
		Versions:       []gardencorev1beta1.MachineImageVersion{},
	}

	for _, cloudProfileVersion := range machineImageFromCloudProfile.Versions {
		criFromCloudProfileVersion, found := findCRIByName(workerCRI.Name, cloudProfileVersion.CRI)
		if !found {
			continue
		}

		if !areAllWorkerCRsPartOfCloudProfileVersion(workerCRI.ContainerRuntimes, criFromCloudProfileVersion.ContainerRuntimes) {
			continue
		}

		filteredMachineImages.Versions = append(filteredMachineImages.Versions, cloudProfileVersion)
	}

	return &filteredMachineImages
}

func filterForKubeletVersionConstraint(machineImageFromCloudProfile *gardencorev1beta1.MachineImage, kubeletVersion *semver.Version) *gardencorev1beta1.MachineImage {
	filteredMachineImages := gardencorev1beta1.MachineImage{
		Name:           machineImageFromCloudProfile.Name,
		UpdateStrategy: machineImageFromCloudProfile.UpdateStrategy,
		Versions:       []gardencorev1beta1.MachineImageVersion{},
	}

	for _, cloudProfileVersion := range machineImageFromCloudProfile.Versions {
		if cloudProfileVersion.KubeletVersionConstraint != nil {
			// CloudProfile cannot contain an invalid kubeletVersionConstraint
			constraint, _ := semver.NewConstraint(*cloudProfileVersion.KubeletVersionConstraint)
			if !constraint.Check(kubeletVersion) {
				continue
			}
		}

		filteredMachineImages.Versions = append(filteredMachineImages.Versions, cloudProfileVersion)
	}

	return &filteredMachineImages
}

func filterForInPlaceUpdateConstraint(machineImageFromCloudProfile *gardencorev1beta1.MachineImage, workerImageVersion *string, isInPlaceUpdateWorker bool) *gardencorev1beta1.MachineImage {
	if !isInPlaceUpdateWorker {
		return machineImageFromCloudProfile
	}

	filteredMachineImages := gardencorev1beta1.MachineImage{
		Name:           machineImageFromCloudProfile.Name,
		UpdateStrategy: machineImageFromCloudProfile.UpdateStrategy,
		Versions:       []gardencorev1beta1.MachineImageVersion{},
	}

	for _, cloudProfileVersion := range machineImageFromCloudProfile.Versions {
		if workerImageVersion != nil && cloudProfileVersion.InPlaceUpdates != nil && cloudProfileVersion.InPlaceUpdates.Supported {
			// add the current version also in the list of possible versions
			if *workerImageVersion == cloudProfileVersion.Version {
				filteredMachineImages.Versions = append(filteredMachineImages.Versions, cloudProfileVersion)
				continue
			}

			if cloudProfileVersion.InPlaceUpdates.MinVersionForUpdate != nil {
				if validVersion, _ := versionutils.CompareVersions(*cloudProfileVersion.InPlaceUpdates.MinVersionForUpdate, "<=", *workerImageVersion); validVersion {
					filteredMachineImages.Versions = append(filteredMachineImages.Versions, cloudProfileVersion)
				}
			}
		}
	}

	return &filteredMachineImages
}

func findCRIByName(wanted gardencorev1beta1.CRIName, cris []gardencorev1beta1.CRI) (gardencorev1beta1.CRI, bool) {
	for _, cri := range cris {
		if cri.Name == wanted {
			return cri, true
		}
	}
	return gardencorev1beta1.CRI{}, false
}

func areAllWorkerCRsPartOfCloudProfileVersion(workerCRs []gardencorev1beta1.ContainerRuntime, crsFromCloudProfileVersion []gardencorev1beta1.ContainerRuntime) bool {
	if workerCRs == nil {
		return true
	}
	for _, workerCr := range workerCRs {
		if !isWorkerCRPartOfCloudProfileVersionCRs(workerCr, crsFromCloudProfileVersion) {
			return false
		}
	}
	return true
}

func isWorkerCRPartOfCloudProfileVersionCRs(wanted gardencorev1beta1.ContainerRuntime, cloudProfileVersionCRs []gardencorev1beta1.ContainerRuntime) bool {
	for _, cr := range cloudProfileVersionCRs {
		if wanted.Type == cr.Type {
			return true
		}
	}
	return false
}
