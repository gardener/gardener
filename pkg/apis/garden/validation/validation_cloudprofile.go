// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/Masterminds/semver"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateCloudProfile validates a CloudProfile object.
func ValidateCloudProfile(cloudProfile *garden.CloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&cloudProfile.ObjectMeta, false, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateCloudProfileSpec(&cloudProfile.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateCloudProfileUpdate validates a CloudProfile object before an update.
func ValidateCloudProfileUpdate(newProfile, oldProfile *garden.CloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newProfile.ObjectMeta, &oldProfile.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateCloudProfile(newProfile)...)

	return allErrs
}

// ValidateCloudProfileSpec validates the specification of a CloudProfile object.
func ValidateCloudProfileSpec(spec *garden.CloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must provide a provider type"))
	}

	allErrs = append(allErrs, validateKubernetesSettings(spec.Kubernetes, fldPath.Child("kubernetes"))...)
	allErrs = append(allErrs, validateCloudProfileMachineImages(spec.MachineImages, fldPath.Child("machineImages"))...)
	allErrs = append(allErrs, validateMachineTypes(spec.MachineTypes, fldPath.Child("machineTypes"))...)
	allErrs = append(allErrs, validateVolumeTypes(spec.VolumeTypes, fldPath.Child("volumeTypes"))...)
	allErrs = append(allErrs, validateRegions(spec.Regions, fldPath.Child("regions"))...)
	if spec.SeedSelector != nil {
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(spec.SeedSelector, fldPath.Child("seedSelector"))...)
	}

	switch {
	case spec.AWS != nil:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.AWS.Constraints.Kubernetes, fldPath.Child("aws", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.AWS.Constraints.MachineImages, fldPath.Child("aws", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypes(spec.AWS.Constraints.MachineTypes, fldPath.Child("aws", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypes(spec.AWS.Constraints.VolumeTypes, fldPath.Child("aws", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.AWS.Constraints.Zones, fldPath.Child("aws", "constraints", "zones"))...)

	case spec.Azure != nil:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.Azure.Constraints.Kubernetes, fldPath.Child("azure", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.Azure.Constraints.MachineImages, fldPath.Child("azure", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypes(spec.Azure.Constraints.MachineTypes, fldPath.Child("azure", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypes(spec.Azure.Constraints.VolumeTypes, fldPath.Child("azure", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZonesOnly(spec.Azure.Constraints.Zones, fldPath.Child("azure", "constraints", "zones"))...)
		allErrs = append(allErrs, validateAzureDomainCount(spec.Azure.CountFaultDomains, fldPath.Child("azure", "countFaultDomains"))...)
		allErrs = append(allErrs, validateAzureDomainCount(spec.Azure.CountUpdateDomains, fldPath.Child("azure", "countUpdateDomains"))...)

	case spec.GCP != nil:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.GCP.Constraints.Kubernetes, fldPath.Child("gcp", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.GCP.Constraints.MachineImages, fldPath.Child("gcp", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypes(spec.GCP.Constraints.MachineTypes, fldPath.Child("gcp", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypes(spec.GCP.Constraints.VolumeTypes, fldPath.Child("gcp", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.GCP.Constraints.Zones, fldPath.Child("gcp", "constraints", "zones"))...)

	case spec.Alicloud != nil:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.Alicloud.Constraints.Kubernetes, fldPath.Child("alicloud", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.Alicloud.Constraints.MachineImages, fldPath.Child("alicloud", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateAlicloudMachineTypeConstraints(spec.Alicloud.Constraints.MachineTypes, spec.Alicloud.Constraints.Zones, fldPath.Child("alicloud", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateAlicloudVolumeTypeConstraints(spec.Alicloud.Constraints.VolumeTypes, spec.Alicloud.Constraints.Zones, fldPath.Child("alicloud", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.Alicloud.Constraints.Zones, fldPath.Child("alicloud", "constraints", "zones"))...)

	case spec.Packet != nil:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.Packet.Constraints.Kubernetes, fldPath.Child("packet", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.Packet.Constraints.MachineImages, fldPath.Child("packet", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypes(spec.Packet.Constraints.MachineTypes, fldPath.Child("packet", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypes(spec.Packet.Constraints.VolumeTypes, fldPath.Child("packet", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.Packet.Constraints.Zones, fldPath.Child("packet", "constraints", "zones"))...)

	case spec.OpenStack != nil:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.OpenStack.Constraints.Kubernetes, fldPath.Child("openstack", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.OpenStack.Constraints.MachineImages, fldPath.Child("openstack", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateOpenStackMachineTypeConstraints(spec.OpenStack.Constraints.MachineTypes, fldPath.Child("openstack", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateZones(spec.OpenStack.Constraints.Zones, fldPath.Child("openstack", "constraints", "zones"))...)

		floatingPoolPath := fldPath.Child("openstack", "constraints", "floatingPools")
		if len(spec.OpenStack.Constraints.FloatingPools) == 0 {
			allErrs = append(allErrs, field.Required(floatingPoolPath, "must provide at least one floating pool"))
		}
		for i, pool := range spec.OpenStack.Constraints.FloatingPools {
			idxPath := floatingPoolPath.Index(i)
			namePath := idxPath.Child("name")
			if len(pool.Name) == 0 {
				allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
			}
		}

		loadBalancerProviderPath := fldPath.Child("openstack", "constraints", "loadBalancerProviders")
		if len(spec.OpenStack.Constraints.LoadBalancerProviders) == 0 {
			allErrs = append(allErrs, field.Required(loadBalancerProviderPath, "must provide at least one load balancer provider"))
		}
		for i, pool := range spec.OpenStack.Constraints.LoadBalancerProviders {
			idxPath := loadBalancerProviderPath.Index(i)
			namePath := idxPath.Child("name")
			if len(pool.Name) == 0 {
				allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
			}
		}

		if len(spec.OpenStack.KeyStoneURL) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("openstack", "keystoneURL"), "must provide the URL to KeyStone"))
		}

		if spec.OpenStack.DHCPDomain != nil && len(*spec.OpenStack.DHCPDomain) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("openstack", "dhcpDomain"), "must provide a dhcp domain when the key is specified"))
		}

		if spec.OpenStack.RequestTimeout != nil {
			_, err := time.ParseDuration(*spec.OpenStack.RequestTimeout)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("openstack", "requestTimeout"), *spec.OpenStack.RequestTimeout, fmt.Sprintf("invalid duration: %v", err)))
			}
		}
	}

	if spec.CABundle != nil {
		_, err := utils.DecodeCertificate([]byte(*(spec.CABundle)))
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("caBundle"), *(spec.CABundle), "caBundle is not a valid PEM-encoded certificate"))
		}
	}

	return allErrs
}

func validateKubernetesConstraints(kubernetes garden.KubernetesConstraints, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(kubernetes.OfferedVersions) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("offeredVersions"), "must provide at least one Kubernetes version"))
	}
	latestKubernetesVersion, err := helper.DetermineLatestKubernetesVersion(kubernetes.OfferedVersions)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, latestKubernetesVersion, "failed to determine latest kubernetes version from cloud profile"))
	}
	if latestKubernetesVersion.ExpirationDate != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("offeredVersions[]").Child("expirationDate"), latestKubernetesVersion.ExpirationDate, fmt.Sprintf("expiration date of latest kubernetes version ('%s') must not be set", latestKubernetesVersion.Version)))
	}

	versionsFound := sets.NewString()
	r, _ := regexp.Compile(`^([0-9]+\.){2}[0-9]+$`)
	for i, version := range kubernetes.OfferedVersions {
		idxPath := fldPath.Child("offeredVersions").Index(i)
		if !r.MatchString(version.Version) {
			allErrs = append(allErrs, field.Invalid(idxPath, version, fmt.Sprintf("all Kubernetes versions must match the regex %s", r)))
		} else if versionsFound.Has(version.Version) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("version"), version.Version))
		} else {
			versionsFound.Insert(version.Version)
		}
	}

	return allErrs
}

func validateKubernetesSettings(kubernetes garden.KubernetesSettings, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(kubernetes.Versions) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("versions"), "must provide at least one Kubernetes version"))
	}
	latestKubernetesVersion, err := helper.DetermineLatestExpirableVersion(kubernetes.Versions)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, latestKubernetesVersion, "failed to determine latest kubernetes version from cloud profile"))
	}
	if latestKubernetesVersion.ExpirationDate != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("versions[]").Child("expirationDate"), latestKubernetesVersion.ExpirationDate, fmt.Sprintf("expiration date of latest kubernetes version ('%s') must not be set", latestKubernetesVersion.Version)))
	}

	versionsFound := sets.NewString()
	r, _ := regexp.Compile(`^([0-9]+\.){2}[0-9]+$`)
	for i, version := range kubernetes.Versions {
		idxPath := fldPath.Child("versions").Index(i)
		if !r.MatchString(version.Version) {
			allErrs = append(allErrs, field.Invalid(idxPath, version, fmt.Sprintf("all Kubernetes versions must match the regex %s", r)))
		} else if versionsFound.Has(version.Version) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("version"), version.Version))
		} else {
			versionsFound.Insert(version.Version)
		}
	}

	return allErrs
}

func validateMachineTypes(machineTypes []garden.MachineType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineTypes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine type"))
	}

	names := make(map[string]struct{}, len(machineTypes))

	for i, machineType := range machineTypes {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		cpuPath := idxPath.Child("cpu")
		gpuPath := idxPath.Child("gpu")
		memoryPath := idxPath.Child("memory")

		if len(machineType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		}

		if _, ok := names[machineType.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(namePath, machineType.Name))
			break
		}
		names[machineType.Name] = struct{}{}

		allErrs = append(allErrs, validateResourceQuantityValue("cpu", machineType.CPU, cpuPath)...)
		allErrs = append(allErrs, validateResourceQuantityValue("gpu", machineType.GPU, gpuPath)...)
		allErrs = append(allErrs, validateResourceQuantityValue("memory", machineType.Memory, memoryPath)...)
	}

	return allErrs
}

func validateAlicloudMachineTypeConstraints(machineTypes []garden.AlicloudMachineType, zones []garden.Zone, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	types := []garden.MachineType{}
	for i, machineType := range machineTypes {
		types = append(types, machineType.MachineType)

		idxPath := fldPath.Index(i)
		zonesPath := idxPath.Child("zones")

	foundInZones:
		for idx, zoneName := range machineType.Zones {
			for _, zone := range zones {
				for _, zoneNameDefined := range zone.Names {
					if zoneName == zoneNameDefined {
						continue foundInZones
					}
				}
			}
			// Can't find zoneName in zones
			allErrs = append(allErrs, field.Invalid(zonesPath.Index(idx), zoneName, fmt.Sprintf("zone name %q is not in defined zones list", zoneName)))
		}
	}

	allErrs = append(allErrs, validateMachineTypes(types, fldPath)...)

	return allErrs
}

func validateMachineImages(machineImages []garden.MachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImages) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
	}

	latestMachineImages, err := helper.DetermineLatestMachineImageVersions(machineImages)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, latestMachineImages, "failed to determine latest machine images from cloud profile"))
	}

	for imageName, latestImage := range latestMachineImages {
		if latestImage.ExpirationDate != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("expirationDate"), latestImage.ExpirationDate, fmt.Sprintf("expiration date of latest image ('%s','%s') must not be set", imageName, latestImage.Version)))
		}
	}

	duplicateNameVersion := sets.String{}
	duplicateName := sets.String{}
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
				allErrs = append(allErrs, field.Invalid(versionsPath.Child("version"), machineVersion.Version, "could not parse version. Use SemanticVersioning. In case there is no semVer version for this image use the extensibility provider (define mapping in the ControllerRegistration) to map to the actual non-semVer version"))
			}
		}
	}

	return allErrs
}

func validateCloudProfileMachineImages(machineImages []garden.CloudProfileMachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImages) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
	}

	latestMachineImages, err := helper.DetermineLatestCloudProfileMachineImageVersions(machineImages)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, latestMachineImages, "failed to determine latest machine images from cloud profile"))
	}

	for imageName, latestImage := range latestMachineImages {
		if latestImage.ExpirationDate != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("expirationDate"), latestImage.ExpirationDate, fmt.Sprintf("expiration date of latest image ('%s','%s') must not be set", imageName, latestImage.Version)))
		}
	}

	duplicateNameVersion := sets.String{}
	duplicateName := sets.String{}
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
				allErrs = append(allErrs, field.Invalid(versionsPath.Child("version"), machineVersion.Version, "could not parse version. Use SemanticVersioning. In case there is no semVer version for this image use the extensibility provider (define mapping in the ControllerRegistration) to map to the actual non-semVer version"))
			}
		}
	}

	return allErrs
}

func validateOpenStackMachineTypeConstraints(machineTypes []garden.OpenStackMachineType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	types := []garden.MachineType{}
	for i, machineType := range machineTypes {
		types = append(types, machineType.MachineType)

		idxPath := fldPath.Index(i)
		volumeTypePath := idxPath.Child("volumeType")
		volumeSizePath := idxPath.Child("volumeSize")

		if len(machineType.VolumeType) == 0 {
			allErrs = append(allErrs, field.Required(volumeTypePath, "must provide a volume type"))
		}
		allErrs = append(allErrs, validateResourceQuantityValue("volumeSize", machineType.VolumeSize, volumeSizePath)...)
	}

	allErrs = append(allErrs, validateMachineTypes(types, fldPath)...)

	return allErrs
}

func validateVolumeTypes(volumeTypes []garden.VolumeType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	names := make(map[string]struct{}, len(volumeTypes))

	for i, volumeType := range volumeTypes {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		classPath := idxPath.Child("class")

		if len(volumeType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		}

		if _, ok := names[volumeType.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(namePath, volumeType.Name))
			break
		}
		names[volumeType.Name] = struct{}{}

		if len(volumeType.Class) == 0 {
			allErrs = append(allErrs, field.Required(classPath, "must provide a class"))
		}
	}

	return allErrs
}

func validateAlicloudVolumeTypeConstraints(volumeTypes []garden.AlicloudVolumeType, zones []garden.Zone, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	types := []garden.VolumeType{}
	for i, volumeType := range volumeTypes {
		types = append(types, volumeType.VolumeType)

		idxPath := fldPath.Index(i)
		zonesPath := idxPath.Child("zones")

	foundInZones:
		for idx, zoneName := range volumeType.Zones {
			for _, zone := range zones {
				for _, zoneNameDefined := range zone.Names {
					if zoneName == zoneNameDefined {
						continue foundInZones
					}
				}
			}
			// Can't find zoneName in zones
			allErrs = append(allErrs, field.Invalid(zonesPath.Index(idx), zoneName, fmt.Sprintf("Zone name [%s] is not in defined zones list", zoneName)))
		}
	}

	allErrs = append(allErrs, validateVolumeTypes(types, fldPath)...)

	return allErrs
}

func validateRegions(regions []garden.Region, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// TODO: enable this once we switched fully to corev1alpha1.CloudProfile.
	// if len(regions) == 0 {
	// 	allErrs = append(allErrs, field.Required(fldPath, "must provide at least one region"))
	// }

	regionsFound := sets.NewString()
	for i, region := range regions {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		zonesPath := idxPath.Child("zones")

		if len(region.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a region name"))
		} else if regionsFound.Has(region.Name) {
			allErrs = append(allErrs, field.Duplicate(namePath, region.Name))
		} else {
			regionsFound.Insert(region.Name)
		}

		zonesFound := sets.NewString()
		for j, zone := range region.Zones {
			namePath := zonesPath.Index(j).Child("name")
			if len(zone.Name) == 0 {
				allErrs = append(allErrs, field.Required(namePath, "zone name cannot be empty"))
			} else if zonesFound.Has(zone.Name) {
				allErrs = append(allErrs, field.Duplicate(namePath, zone.Name))
			} else {
				zonesFound.Insert(zone.Name)
			}
		}
	}
	return allErrs
}

func validateZones(zones []garden.Zone, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(zones) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one zone"))
	}

	allErrs = append(allErrs, validateZonesOnly(zones, fldPath)...)
	return allErrs
}

func validateZonesOnly(zones []garden.Zone, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	regionsFound := sets.NewString()
	for i, zone := range zones {
		idxPath := fldPath.Index(i)
		regionPath := idxPath.Child("region")
		namesPath := idxPath.Child("names")

		if len(zone.Region) == 0 {
			allErrs = append(allErrs, field.Required(regionPath, "must provide a region"))
		} else if regionsFound.Has(zone.Region) {
			allErrs = append(allErrs, field.Duplicate(regionPath, zone.Region))
		} else {
			regionsFound.Insert(zone.Region)
		}

		zonesFound := sets.NewString()
		for j, name := range zone.Names {
			namePath := namesPath.Index(j)
			if len(name) == 0 {
				allErrs = append(allErrs, field.Required(namePath, "zone name cannot be empty"))
			} else if zonesFound.Has(name) {
				allErrs = append(allErrs, field.Duplicate(namePath, name))
			} else {
				zonesFound.Insert(name)
			}
		}
	}
	return allErrs
}

func validateAzureDomainCount(domainCount []garden.AzureDomainCount, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(domainCount) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one domain count"))
	}

	for i, count := range domainCount {
		idxPath := fldPath.Index(i)
		regionPath := idxPath.Child("region")
		countPath := idxPath.Child("count")

		if len(count.Region) == 0 {
			allErrs = append(allErrs, field.Required(regionPath, "must provide a region"))
		}
		if count.Count < 0 {
			allErrs = append(allErrs, field.Invalid(countPath, count.Count, "count must not be negative"))
		}
	}

	return allErrs
}
