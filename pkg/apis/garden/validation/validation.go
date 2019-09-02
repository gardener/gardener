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
	"encoding/json"
	"fmt"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"

	"github.com/Masterminds/semver"
	"github.com/robfig/cron"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	availableProxyMode = sets.NewString(
		string(garden.ProxyModeIPTables),
		string(garden.ProxyModeIPVS),
	)
	availableKubernetesDashboardAuthenticationModes = sets.NewString(
		garden.KubernetesDashboardAuthModeBasic,
		garden.KubernetesDashboardAuthModeToken,
	)
)

// ValidateName is a helper function for validating that a name is a DNS sub domain.
func ValidateName(name string, prefix bool) []string {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

// ValidatePositiveDuration validates that a duration is positive.
func ValidatePositiveDuration(duration *metav1.Duration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if duration == nil {
		return allErrs
	}
	if duration.Seconds() < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, duration.Duration.String(), "must be non-negative"))
	}
	return allErrs
}

// ValidateResourceQuantityOrPercent checks if a value can be parsed to either a resource.quantity, a positive int or percent.
func ValidateResourceQuantityOrPercent(valuePtr *string, fldPath *field.Path, key string) field.ErrorList {
	allErrs := field.ErrorList{}

	if valuePtr == nil {
		return allErrs
	}
	value := *valuePtr
	// check for resource quantity
	if quantity, err := resource.ParseQuantity(value); err == nil {
		if len(validateResourceQuantityValue(key, quantity, fldPath)) == 0 {
			return allErrs
		}
	}

	if validation.IsValidPercent(value) != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(key), value, "field must be either a valid resource quantity (e.g 200Mi) or a percentage (e.g '5%')"))
		return allErrs
	}

	percentValue, _ := strconv.Atoi(value[:len(value)-1])
	if percentValue > 100 || percentValue < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(key), value, "must not be greater than 100% and not smaller than 0%"))
	}
	return allErrs
}

func ValidatePositiveIntOrPercent(intOrPercent intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if intOrPercent.Type == intstr.String {
		if validation.IsValidPercent(intOrPercent.StrVal) != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, intOrPercent, "must be an integer or percentage (e.g '5%')"))
		}
	} else if intOrPercent.Type == intstr.Int {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(intOrPercent.IntValue()), fldPath)...)
	}
	return allErrs
}

func IsNotMoreThan100Percent(intOrStringValue intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	value, isPercent := getPercentValue(intOrStringValue)
	if !isPercent || value <= 100 {
		return nil
	}
	allErrs = append(allErrs, field.Invalid(fldPath, intOrStringValue, "must not be greater than 100%"))
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

////////////////////////////////////////////////////
//                  CLOUD PROFILES                //
////////////////////////////////////////////////////

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

	cloud, err := helper.DetermineCloudProviderInProfile(*spec)
	if err != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("aws/azure/gcp/alicloud/openstack/packet"), "cloud profile must only contain exactly one field of aws/azure/gcp/alicloud/openstack/packet"))
		return allErrs
	}

	switch cloud {
	case garden.CloudProviderAWS:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.AWS.Constraints.Kubernetes, fldPath.Child("aws", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.AWS.Constraints.MachineImages, fldPath.Child("aws", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.AWS.Constraints.MachineTypes, fldPath.Child("aws", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypeConstraints(spec.AWS.Constraints.VolumeTypes, fldPath.Child("aws", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.AWS.Constraints.Zones, fldPath.Child("aws", "constraints", "zones"))...)

	case garden.CloudProviderAzure:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.Azure.Constraints.Kubernetes, fldPath.Child("azure", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.Azure.Constraints.MachineImages, fldPath.Child("azure", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.Azure.Constraints.MachineTypes, fldPath.Child("azure", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypeConstraints(spec.Azure.Constraints.VolumeTypes, fldPath.Child("azure", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateAzureDomainCount(spec.Azure.CountFaultDomains, fldPath.Child("azure", "countFaultDomains"))...)
		allErrs = append(allErrs, validateAzureDomainCount(spec.Azure.CountUpdateDomains, fldPath.Child("azure", "countUpdateDomains"))...)

	case garden.CloudProviderGCP:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.GCP.Constraints.Kubernetes, fldPath.Child("gcp", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.GCP.Constraints.MachineImages, fldPath.Child("gcp", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.GCP.Constraints.MachineTypes, fldPath.Child("gcp", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypeConstraints(spec.GCP.Constraints.VolumeTypes, fldPath.Child("gcp", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.GCP.Constraints.Zones, fldPath.Child("gcp", "constraints", "zones"))...)

	case garden.CloudProviderAlicloud:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.Alicloud.Constraints.Kubernetes, fldPath.Child("alicloud", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.Alicloud.Constraints.MachineImages, fldPath.Child("alicloud", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateAlicloudMachineTypeConstraints(spec.Alicloud.Constraints.MachineTypes, spec.Alicloud.Constraints.Zones, fldPath.Child("alicloud", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateAlicloudVolumeTypeConstraints(spec.Alicloud.Constraints.VolumeTypes, spec.Alicloud.Constraints.Zones, fldPath.Child("alicloud", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.Alicloud.Constraints.Zones, fldPath.Child("alicloud", "constraints", "zones"))...)

	case garden.CloudProviderPacket:
		allErrs = append(allErrs, validateKubernetesConstraints(spec.Packet.Constraints.Kubernetes, fldPath.Child("packet", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineImages(spec.Packet.Constraints.MachineImages, fldPath.Child("packet", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.Packet.Constraints.MachineTypes, fldPath.Child("packet", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypeConstraints(spec.Packet.Constraints.VolumeTypes, fldPath.Child("packet", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.Packet.Constraints.Zones, fldPath.Child("packet", "constraints", "zones"))...)

	case garden.CloudProviderOpenStack:
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
			allErrs = append(allErrs, field.Required(fldPath.Child("openstack", "keyStoneURL"), "must provide the URL to KeyStone"))
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

	r, _ := regexp.Compile(`^([0-9]+\.){2}[0-9]+$`)
	for i, version := range kubernetes.OfferedVersions {
		idxPath := fldPath.Child("offeredVersions").Index(i)
		if !r.MatchString(version.Version) {
			allErrs = append(allErrs, field.Invalid(idxPath, version, fmt.Sprintf("all Kubernetes versions must match the regex %s", r)))
		}
	}

	return allErrs
}

func validateMachineTypeConstraints(machineTypes []garden.MachineType, fldPath *field.Path) field.ErrorList {
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

	allErrs = append(allErrs, validateMachineTypeConstraints(types, fldPath)...)

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

	allErrs = append(allErrs, validateMachineTypeConstraints(types, fldPath)...)

	return allErrs
}

func validateVolumeTypeConstraints(volumeTypes []garden.VolumeType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(volumeTypes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one volume type"))
	}

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

	allErrs = append(allErrs, validateVolumeTypeConstraints(types, fldPath)...)

	return allErrs
}

func validateZones(zones []garden.Zone, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(zones) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one zone"))
	}

	for i, zone := range zones {
		idxPath := fldPath.Index(i)
		regionPath := idxPath.Child("region")
		namesPath := idxPath.Child("names")

		if len(zone.Region) == 0 {
			allErrs = append(allErrs, field.Required(regionPath, "must provide a region"))
		}

		if len(zone.Names) == 0 {
			allErrs = append(allErrs, field.Required(namesPath, "must provide at least one zone for this region"))
		}

		for j, name := range zone.Names {
			namePath := namesPath.Index(j)
			if len(name) == 0 {
				allErrs = append(allErrs, field.Required(namePath, "zone name cannot be empty"))
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

////////////////////////////////////////////////////
//                    PROJECTS                    //
////////////////////////////////////////////////////

// ValidateProject validates a Project object.
func ValidateProject(project *garden.Project) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&project.ObjectMeta, false, ValidateName, field.NewPath("metadata"))...)
	maxProjectNameLength := 10
	if len(project.Name) > maxProjectNameLength {
		allErrs = append(allErrs, field.TooLong(field.NewPath("metadata", "name"), project.Name, maxProjectNameLength))
	}
	allErrs = append(allErrs, validateNameConsecutiveHyphens(project.Name, field.NewPath("metadata", "name"))...)
	allErrs = append(allErrs, ValidateProjectSpec(&project.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateProjectUpdate validates a Project object before an update.
func ValidateProjectUpdate(newProject, oldProject *garden.Project) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newProject.ObjectMeta, &oldProject.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateProject(newProject)...)

	if oldProject.Spec.CreatedBy != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newProject.Spec.CreatedBy, oldProject.Spec.CreatedBy, field.NewPath("spec", "createdBy"))...)
	}
	if oldProject.Spec.Namespace != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newProject.Spec.Namespace, oldProject.Spec.Namespace, field.NewPath("spec", "namespace"))...)
	}
	if oldProject.Spec.Owner != nil && newProject.Spec.Owner == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "owner"), newProject.Spec.Owner, "owner cannot be reset"))
	}

	return allErrs
}

// ValidateProjectSpec validates the specification of a Project object.
func ValidateProjectSpec(projectSpec *garden.ProjectSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, member := range projectSpec.ProjectMembers {
		allErrs = append(allErrs, ValidateSubject(member.Subject, fldPath.Child("members").Index(i))...)
	}
	if createdBy := projectSpec.CreatedBy; createdBy != nil {
		allErrs = append(allErrs, ValidateSubject(*createdBy, fldPath.Child("createdBy"))...)
	}
	if owner := projectSpec.Owner; owner != nil {
		allErrs = append(allErrs, ValidateSubject(*owner, fldPath.Child("owner"))...)
	}
	if description := projectSpec.Description; description != nil && len(*description) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("description"), "must provide a description when key is present"))
	}
	if purpose := projectSpec.Description; purpose != nil && len(*purpose) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("purpose"), "must provide a purpose when key is present"))
	}

	return allErrs
}

// ValidateSubject validates the subject representing the owner.
func ValidateSubject(subject rbacv1.Subject, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(subject.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
	}

	switch subject.Kind {
	case rbacv1.ServiceAccountKind:
		if len(subject.Name) > 0 {
			for _, msg := range apivalidation.ValidateServiceAccountName(subject.Name, false) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), subject.Name, msg))
			}
		}
		if len(subject.APIGroup) > 0 {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiGroup"), subject.APIGroup, []string{""}))
		}
		if len(subject.Namespace) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), ""))
		}

	case rbacv1.UserKind, rbacv1.GroupKind:
		if subject.APIGroup != rbacv1.GroupName {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiGroup"), subject.APIGroup, []string{rbacv1.GroupName}))
		}

	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kind"), subject.Kind, []string{rbacv1.ServiceAccountKind, rbacv1.UserKind, rbacv1.GroupKind}))
	}

	return allErrs
}

// ValidateProjectStatusUpdate validates the status field of a Project object.
func ValidateProjectStatusUpdate(newProject, oldProject *garden.Project) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(oldProject.Status.Phase) > 0 && len(newProject.Status.Phase) == 0 {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("status").Child("phase"), "phase cannot be updated to an empty string"))
	}

	return allErrs
}

////////////////////////////////////////////////////
//                      SEEDS                     //
////////////////////////////////////////////////////

// ValidateSeed validates a Seed object.
func ValidateSeed(seed *garden.Seed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&seed.ObjectMeta, false, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateSeedSpec(&seed.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateSeedAnnotation(seed.ObjectMeta.Annotations, field.NewPath("metadata", "annotations"))...)

	return allErrs
}

// ValidateSeedUpdate validates a Seed object before an update.
func ValidateSeedUpdate(newSeed, oldSeed *garden.Seed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newSeed.ObjectMeta, &oldSeed.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateSeedSpecUpdate(&newSeed.Spec, &oldSeed.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateSeed(newSeed)...)

	return allErrs
}

//ValidateSeedAnnotation validates the annotations of seed
func ValidateSeedAnnotation(annotations map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if annotations != nil {
		if v, ok := annotations[common.AnnotatePersistentVolumeMinimumSize]; ok {
			volumeSizeRegex, _ := regexp.Compile(`^(\d)+Gi$`)
			if !volumeSizeRegex.MatchString(v) {
				allErrs = append(allErrs, field.Invalid(fldPath.Key(common.AnnotatePersistentVolumeMinimumSize), v, fmt.Sprintf("volume size must match the regex %s", volumeSizeRegex)))
			}
		}
	}
	return allErrs
}

// ValidateSeedSpec validates the specification of a Seed object.
func ValidateSeedSpec(seedSpec *garden.SeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	cloudPath := fldPath.Child("cloud")
	if len(seedSpec.Cloud.Profile) == 0 {
		allErrs = append(allErrs, field.Required(cloudPath.Child("profile"), "must provide a cloud profile name"))
	}
	if len(seedSpec.Cloud.Region) == 0 {
		allErrs = append(allErrs, field.Required(cloudPath.Child("region"), "must provide a cloud region"))
	}

	providerPath := fldPath.Child("provider")
	if len(seedSpec.Provider.Type) == 0 {
		allErrs = append(allErrs, field.Required(providerPath.Child("type"), "must provide a provider type"))
	}
	if len(seedSpec.Provider.Region) == 0 {
		allErrs = append(allErrs, field.Required(providerPath.Child("region"), "must provide a provider region"))
	}

	allErrs = append(allErrs, validateDNS1123Subdomain(seedSpec.IngressDomain, fldPath.Child("ingressDomain"))...)
	allErrs = append(allErrs, validateSecretReference(seedSpec.SecretRef, fldPath.Child("secretRef"))...)

	networksPath := fldPath.Child("networks")

	networks := []cidrvalidation.CIDR{
		cidrvalidation.NewCIDR(seedSpec.Networks.Nodes, networksPath.Child("nodes")),
		cidrvalidation.NewCIDR(seedSpec.Networks.Pods, networksPath.Child("pods")),
		cidrvalidation.NewCIDR(seedSpec.Networks.Services, networksPath.Child("services")),
	}
	if shootDefaults := seedSpec.Networks.ShootDefaults; shootDefaults != nil {
		if shootDefaults.Pods != nil {
			networks = append(networks, cidrvalidation.NewCIDR(*shootDefaults.Pods, networksPath.Child("shootDefaults", "pods")))
		}
		if shootDefaults.Services != nil {
			networks = append(networks, cidrvalidation.NewCIDR(*shootDefaults.Services, networksPath.Child("shootDefaults", "services")))
		}
	}

	allErrs = append(allErrs, validateCIDRParse(networks...)...)
	allErrs = append(allErrs, validateCIDROverlap(networks, networks, false)...)

	if seedSpec.Backup != nil {
		if len(seedSpec.Backup.Provider) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("backup", "provider"), "must provide a backup cloud provider name"))
		}

		// TOADD: Currently, getting cloud provider of seed requires fetching cloudProfile which requires gardenClient.
		// Hence we are not handling it here.
		// This should change with new `coreV1alpha1.Seed` api as per https://github.com/gardener/gardener/pull/1284/files#diff-bf2774d9954baab517306db45a5b80bbR241-R243,
		// and we will get direct seed cloud provider here.
		//
		//if seedSpec.Cloud.Type != seedSpec.Backup.Cloud &&( seedSpec.Backup.Region == nil || len(*seedSpec.Backup.Region) == 0) {
		//	allErrs = append(allErrs, field.Invalid(fldPath.Child("backup", "region"), "", "region must be specified for if backup provider is different from provider used in `spec.cloud`"))
		//}

		allErrs = append(allErrs, validateSecretReference(seedSpec.Backup.SecretRef, fldPath.Child("backup", "secretRef"))...)
	}

	taintKeys := sets.NewString()
	for i, taint := range seedSpec.Taints {
		idxPath := fldPath.Child("taints").Index(i)
		if len(taint.Key) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("key"), "cannot be empty"))
		}
		if taintKeys.Has(taint.Key) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("key"), taint.Key))
		}
		if taint.Key != garden.SeedTaintProtected && taint.Key != garden.SeedTaintInvisible {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("key"), taint.Key, []string{garden.SeedTaintProtected, garden.SeedTaintInvisible}))
		}
		taintKeys.Insert(taint.Key)
	}

	if seedSpec.Volume != nil {
		if seedSpec.Volume.MinimumSize != nil {
			allErrs = append(allErrs, validateResourceQuantityValue("minimumSize", *seedSpec.Volume.MinimumSize, fldPath.Child("volume", "minimumSize"))...)
		}

		volumeProviderPurposes := make(map[string]struct{}, len(seedSpec.Volume.Providers))
		for i, provider := range seedSpec.Volume.Providers {
			idxPath := fldPath.Child("volume", "providers").Index(i)
			if len(provider.Purpose) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("purpose"), "cannot be empty"))
			}
			if len(provider.Name) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("name"), "cannot be empty"))
			}
			if _, ok := volumeProviderPurposes[provider.Purpose]; ok {
				allErrs = append(allErrs, field.Duplicate(idxPath.Child("purpose"), provider.Purpose))
			}
			volumeProviderPurposes[provider.Purpose] = struct{}{}
		}
	}

	return allErrs
}

func validateCIDR(cidr garden.CIDR, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if _, _, err := net.ParseCIDR(string(cidr)); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, cidr, err.Error()))
	}

	return allErrs
}

// ValidateSeedSpecUpdate validates the specification updates of a Seed object.
func ValidateSeedSpecUpdate(newSeedSpec, oldSeedSpec *garden.SeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedSpec.Networks.Pods, oldSeedSpec.Networks.Pods, fldPath.Child("networks", "pods"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedSpec.Networks.Services, oldSeedSpec.Networks.Services, fldPath.Child("networks", "services"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedSpec.Networks.Nodes, oldSeedSpec.Networks.Nodes, fldPath.Child("networks", "nodes"))...)

	if oldSeedSpec.Backup != nil {
		if newSeedSpec.Backup != nil {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedSpec.Backup.Provider, oldSeedSpec.Backup.Provider, fldPath.Child("backup", "provider"))...)
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedSpec.Backup.Region, oldSeedSpec.Backup.Region, fldPath.Child("backup", "region"))...)
		} else {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeedSpec.Backup, oldSeedSpec.Backup, fldPath.Child("backup"))...)
		}
	}
	// If oldSeedSpec doesn't have backup configured, we allow to add it; but not the vice versa.

	return allErrs
}

// ValidateSeedStatusUpdate validates the status field of a Seed object.
func ValidateSeedStatusUpdate(newSeed, oldSeed *garden.Seed) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

////////////////////////////////////////////////////
//                     QUOTAS                     //
////////////////////////////////////////////////////

// ValidateQuota validates a Quota object.
func ValidateQuota(quota *garden.Quota) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&quota.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateQuotaSpec(&quota.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateQuotaUpdate validates a Quota object before an update.
func ValidateQuotaUpdate(newQuota, oldQuota *garden.Quota) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&newQuota.ObjectMeta, &oldQuota.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(&newQuota.Spec.Scope, &oldQuota.Spec.Scope, field.NewPath("spec").Child("scope"))...)
	allErrs = append(allErrs, ValidateQuota(newQuota)...)
	return allErrs
}

// ValidateQuotaStatusUpdate validates the status field of a Quota object.
func ValidateQuotaStatusUpdate(newQuota, oldQuota *garden.Quota) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateQuotaSpec validates the specification of a Quota object.
func ValidateQuotaSpec(quotaSpec *garden.QuotaSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	scopeRef := quotaSpec.Scope
	if _, err := helper.QuotaScope(scopeRef); err != nil {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("scope"), scopeRef, []string{"project", "secret"}))
	}

	metricsFldPath := fldPath.Child("metrics")
	for k, v := range quotaSpec.Metrics {
		keyPath := metricsFldPath.Key(string(k))
		if !isValidQuotaMetric(corev1.ResourceName(k)) {
			allErrs = append(allErrs, field.Invalid(keyPath, v.String(), fmt.Sprintf("%s is no supported quota metric", string(k))))
		}
		allErrs = append(allErrs, validateResourceQuantityValue(string(k), v, keyPath)...)
	}

	return allErrs
}

func isValidQuotaMetric(metric corev1.ResourceName) bool {
	switch metric {
	case
		garden.QuotaMetricCPU,
		garden.QuotaMetricGPU,
		garden.QuotaMetricMemory,
		garden.QuotaMetricStorageStandard,
		garden.QuotaMetricStoragePremium,
		garden.QuotaMetricLoadbalancer:
		return true
	}
	return false
}

// validateResourceQuantityValue validates the value of a resource quantity.
func validateResourceQuantityValue(key string, value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if value.Cmp(resource.Quantity{}) < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value.String(), fmt.Sprintf("%s value must not be negative", key)))
	}

	return allErrs
}

////////////////////////////////////////////////////
//                  SECRET BINDINGS               //
////////////////////////////////////////////////////

// ValidateSecretBinding validates a SecretBinding object.
func ValidateSecretBinding(binding *garden.SecretBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateSecretReferenceOptionalNamespace(binding.SecretRef, field.NewPath("secretRef"))...)
	for i, quota := range binding.Quotas {
		allErrs = append(allErrs, validateObjectReferenceOptionalNamespace(quota, field.NewPath("quotas").Index(i))...)
	}

	return allErrs
}

// ValidateSecretBindingUpdate validates a SecretBinding object before an update.
func ValidateSecretBindingUpdate(newBinding, oldBinding *garden.SecretBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBinding.ObjectMeta, &oldBinding.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.SecretRef, oldBinding.SecretRef, field.NewPath("secretRef"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.Quotas, oldBinding.Quotas, field.NewPath("quotas"))...)
	allErrs = append(allErrs, ValidateSecretBinding(newBinding)...)

	return allErrs
}

func validateLocalObjectReference(ref *corev1.LocalObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	return allErrs
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

func validateObjectReferenceOptionalNamespace(ref corev1.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	return allErrs
}

func validateSecretReferenceOptionalNamespace(ref corev1.SecretReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	return allErrs
}

////////////////////////////////////////////////////
//                     SHOOTS                     //
////////////////////////////////////////////////////

// ValidateShoot validates a Shoot object.
func ValidateShoot(shoot *garden.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&shoot.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateNameConsecutiveHyphens(shoot.Name, field.NewPath("metadata", "name"))...)
	allErrs = append(allErrs, ValidateShootSpec(&shoot.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateShootUpdate validates a Shoot object before an update.
func ValidateShootUpdate(newShoot, oldShoot *garden.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newShoot.ObjectMeta, &oldShoot.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootSpecUpdate(&newShoot.Spec, &oldShoot.Spec, newShoot.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateShoot(newShoot)...)

	return allErrs
}

// ValidateShootSpec validates the specification of a Shoot object.
func ValidateShootSpec(spec *garden.ShootSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	cloudPath := fldPath.Child("cloud")
	if _, err := helper.DetermineCloudProviderInShoot(spec.Cloud); err != nil {
		allErrs = append(allErrs, field.Forbidden(cloudPath.Child("aws/azure/gcp/alicloud/openstack/packet"), "cloud section must only contain exactly one field of aws/azure/gcp/alicloud/openstack/packet"))
		return allErrs
	}

	allErrs = append(allErrs, validateAddons(spec.Addons, spec.Kubernetes.KubeAPIServer, fldPath.Child("addons"))...)
	allErrs = append(allErrs, validateCloud(spec.Cloud, spec.Kubernetes, fldPath.Child("cloud"))...)
	allErrs = append(allErrs, validateDNS(spec.DNS, fldPath.Child("dns"))...)
	allErrs = append(allErrs, validateExtensions(spec.Extensions, fldPath.Child("extensions"))...)
	allErrs = append(allErrs, validateKubernetes(spec.Kubernetes, fldPath.Child("kubernetes"))...)
	allErrs = append(allErrs, validateNetworking(spec.Networking, fldPath.Child("networking"))...)
	allErrs = append(allErrs, validateMaintenance(spec.Maintenance, fldPath.Child("maintenance"))...)
	allErrs = append(allErrs, ValidateHibernation(spec.Hibernation, fldPath.Child("hibernation"))...)

	return allErrs
}

// ValidateShootStatusUpdate validates the status field of a Shoot object.
func ValidateShootStatusUpdate(newStatus, oldStatus garden.ShootStatus) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		fldPath = field.NewPath("status")
	)

	if len(oldStatus.UID) > 0 {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newStatus.UID, oldStatus.UID, fldPath.Child("uid"))...)
	}
	if len(oldStatus.TechnicalID) > 0 {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newStatus.TechnicalID, oldStatus.TechnicalID, fldPath.Child("technicalID"))...)
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

func validateAddons(addons *garden.Addons, kubeAPIServerConfig *garden.KubeAPIServerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if addons == nil {
		return allErrs
	}

	if addons.Kube2IAM != nil && addons.Kube2IAM.Enabled {
		kube2iamPath := fldPath.Child("kube2iam")
		for i, role := range addons.Kube2IAM.Roles {
			idxPath := kube2iamPath.Child("roles").Index(i)
			namePath := idxPath.Child("name")
			descriptionPath := idxPath.Child("description")
			policyPath := idxPath.Child("policy")

			if len(role.Name) == 0 {
				allErrs = append(allErrs, field.Required(namePath, "must provide a role name"))
			}
			if len(role.Description) == 0 {
				allErrs = append(allErrs, field.Required(descriptionPath, "must provide a role description"))
			}
			var js map[string]interface{}
			if json.Unmarshal([]byte(role.Policy), &js) != nil {
				allErrs = append(allErrs, field.Invalid(policyPath, role.Policy, "must provide a valid json document"))
			}
		}
	}

	if addons.KubeLego != nil && addons.KubeLego.Enabled {
		if !utils.TestEmail(addons.KubeLego.Mail) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("kube-lego", "mail"), addons.KubeLego.Mail, "must provide a valid email address when kube-lego is enabled"))
		}
	}

	if addons.KubernetesDashboard != nil && addons.KubernetesDashboard.Enabled {
		if authMode := addons.KubernetesDashboard.AuthenticationMode; authMode != nil {
			if !availableKubernetesDashboardAuthenticationModes.Has(*authMode) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("kubernetes-dashboard", "authenticationMode"), *authMode, availableKubernetesDashboardAuthenticationModes.List()))
			}

			if *authMode == garden.KubernetesDashboardAuthModeBasic && !helper.ShootWantsBasicAuthentication(kubeAPIServerConfig) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("kubernetes-dashboard", "authenticationMode"), *authMode, "cannot use basic auth mode when basic auth is not enabled in kube-apiserver configuration"))
			}
		}
	}

	return allErrs
}

func validateCloud(cloud garden.Cloud, kubernetes garden.Kubernetes, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	workerNames := make(map[string]bool)

	if len(cloud.Profile) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("profile"), "must specify a cloud profile"))
	}
	if len(cloud.Region) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "must specify a region"))
	}
	if len(cloud.SecretBindingRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretBindingRef", "name"), "must specify a name"))
	}
	if cloud.Seed != nil && len(*cloud.Seed) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("seed"), cloud.Seed, "seed name must not be empty when providing the key"))
	}

	var maxPod int32

	if kubernetes.Kubelet != nil && kubernetes.Kubelet.MaxPods != nil {
		maxPod = *kubernetes.Kubelet.MaxPods
	}

	aws := cloud.AWS
	awsPath := fldPath.Child("aws")
	if aws != nil {
		zoneCount := len(aws.Zones)
		if zoneCount == 0 {
			allErrs = append(allErrs, field.Required(awsPath.Child("zones"), "must specify at least one zone"))
			return allErrs
		}

		nodes, pods, services, networkErrors := transformK8SNetworks(aws.Networks.K8SNetworks, awsPath.Child("networks"))
		allErrs = append(allErrs, networkErrors...)

		if len(aws.Networks.Internal) != zoneCount {
			allErrs = append(allErrs, field.Invalid(awsPath.Child("networks", "internal"), aws.Networks.Internal, "must specify as many internal networks as zones"))
		}

		allVPCCIDRs := make([]cidrvalidation.CIDR, 0, len(aws.Networks.Internal)+len(aws.Networks.Public)+len(aws.Networks.Workers))
		for i, cidr := range aws.Networks.Internal {
			allVPCCIDRs = append(allVPCCIDRs, cidrvalidation.NewCIDR(cidr, awsPath.Child("networks", "internal").Index(i)))
		}

		if len(aws.Networks.Public) != zoneCount {
			allErrs = append(allErrs, field.Invalid(awsPath.Child("networks", "public"), aws.Networks.Public, "must specify as many public networks as zones"))
		}

		for i, cidr := range aws.Networks.Public {
			allVPCCIDRs = append(allVPCCIDRs, cidrvalidation.NewCIDR(cidr, awsPath.Child("networks", "public").Index(i)))
		}

		if len(aws.Networks.Workers) != zoneCount {
			allErrs = append(allErrs, field.Invalid(awsPath.Child("networks", "workers"), aws.Networks.Workers, "must specify as many workers networks as zones"))
		}

		// validate before appending
		allErrs = append(allErrs, validateCIDRParse(allVPCCIDRs...)...)

		workerCIDRs := make([]cidrvalidation.CIDR, 0, len(aws.Networks.Workers))
		for i, cidr := range aws.Networks.Workers {
			workerCIDRs = append(workerCIDRs, cidrvalidation.NewCIDR(cidr, awsPath.Child("networks", "workers").Index(i)))
			allVPCCIDRs = append(allVPCCIDRs, cidrvalidation.NewCIDR(cidr, awsPath.Child("networks", "workers").Index(i)))
		}
		allErrs = append(allErrs, validateCIDRParse(workerCIDRs...)...)

		if nodes != nil {
			allErrs = append(allErrs, nodes.ValidateSubset(workerCIDRs...)...)
		}

		if (aws.Networks.VPC.ID == nil && aws.Networks.VPC.CIDR == nil) || (aws.Networks.VPC.ID != nil && aws.Networks.VPC.CIDR != nil) {
			allErrs = append(allErrs, field.Invalid(awsPath.Child("networks", "vpc"), aws.Networks.VPC, "must specify either a vpc id or a cidr"))
		} else if aws.Networks.VPC.CIDR != nil && aws.Networks.VPC.ID == nil {
			vpcCIDR := cidrvalidation.NewCIDR(*(aws.Networks.VPC.CIDR), awsPath.Child("networks", "vpc", "cidr"))
			allErrs = append(allErrs, vpcCIDR.ValidateParse()...)
			allErrs = append(allErrs, vpcCIDR.ValidateSubset(nodes)...)
			allErrs = append(allErrs, vpcCIDR.ValidateSubset(allVPCCIDRs...)...)
			allErrs = append(allErrs, vpcCIDR.ValidateNotSubset(pods, services)...)
		}

		// make sure all CIDRs are canonical
		allErrs = append(allErrs, validateCIDRsAreCanonical(awsPath, aws.Networks.VPC.CIDR, &nodes, &pods, &services, aws.Networks.Internal, aws.Networks.Public, aws.Networks.Workers)...)

		// make sure that VPC cidrs don't overlap with eachother
		allErrs = append(allErrs, validateCIDROverlap(allVPCCIDRs, allVPCCIDRs, false)...)

		allErrs = append(allErrs, validateCIDROverlap([]cidrvalidation.CIDR{pods, services}, allVPCCIDRs, false)...)

		workersPath := awsPath.Child("workers")
		if len(aws.Workers) == 0 {
			allErrs = append(allErrs, field.Required(workersPath, "must specify at least one worker"))
			return allErrs
		}

		var workers []garden.Worker

		for i, worker := range aws.Workers {
			idxPath := workersPath.Index(i)
			allErrs = append(allErrs, ValidateWorker(worker.Worker, idxPath)...)
			allErrs = append(allErrs, validateWorkerVolumeSize(worker.VolumeSize, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerMinimumVolumeSize(worker.VolumeSize, 20, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerVolumeType(worker.VolumeType, idxPath.Child("volumeType"))...)
			if workerNames[worker.Name] {
				allErrs = append(allErrs, field.Duplicate(idxPath, worker.Name))
			}
			workerNames[worker.Name] = true
			if worker.Kubelet != nil && worker.Kubelet.MaxPods != nil && *worker.Kubelet.MaxPods > maxPod {
				maxPod = *worker.Kubelet.MaxPods
			}
			workers = append(workers, worker.Worker)
		}
		allErrs = append(allErrs, ValidateWorkers(workers, workersPath)...)
	}

	azure := cloud.Azure
	azurePath := fldPath.Child("azure")
	if azure != nil {
		// Currently, we will not allow deployments into existing resource groups or VNets although this functionality
		// is already implemented, because the Azure cloud provider (v1.9) is not cleaning up self-created resources properly.
		// This resources would be orphaned when the cluster will be deleted. We block these cases thereby that the Azure shoot
		// validation here will fail for those cases.
		// TODO: remove the following block and uncomment below blocks once deployment into existing resource groups/vnets works properly.
		if azure.ResourceGroup != nil {
			allErrs = append(allErrs, field.Invalid(azurePath.Child("resourceGroup", "name"), azure.ResourceGroup.Name, "specifying an existing resource group is not supported yet."))
		}

		nodes, pods, services, networkErrors := transformK8SNetworks(azure.Networks.K8SNetworks, azurePath.Child("networks"))
		allErrs = append(allErrs, networkErrors...)

		workerCIDR := cidrvalidation.NewCIDR(azure.Networks.Workers, azurePath.Child("networks", "workers"))
		allErrs = append(allErrs, workerCIDR.ValidateParse()...)

		if nodes != nil {
			allErrs = append(allErrs, nodes.ValidateSubset(workerCIDR)...)
		}

		if azure.Networks.VNet.Name != nil {
			allErrs = append(allErrs, field.Invalid(azurePath.Child("networks", "vnet", "name"), *(azure.Networks.VNet.Name), "specifying an existing vnet is not supported yet"))
		} else {
			if azure.Networks.VNet.CIDR == nil {
				allErrs = append(allErrs, field.Required(azurePath.Child("networks", "vnet", "cidr"), "must specify a vnet cidr"))
			} else {
				vpcCIDR := cidrvalidation.NewCIDR(*(azure.Networks.VNet.CIDR), azurePath.Child("networks", "vnet", "cidr"))
				allErrs = append(allErrs, vpcCIDR.ValidateParse()...)
				allErrs = append(allErrs, vpcCIDR.ValidateSubset(nodes)...)
				allErrs = append(allErrs, vpcCIDR.ValidateSubset(workerCIDR)...)
				allErrs = append(allErrs, vpcCIDR.ValidateNotSubset(pods, services)...)
			}
		}

		// TODO: re-enable once deployment into existing resource group works properly.
		// if azure.ResourceGroup != nil && len(azure.ResourceGroup.Name) == 0 {
		// 	allErrs = append(allErrs, field.Invalid(azurePath.Child("resourceGroup", "name"), azure.ResourceGroup.Name, "resource group name must not be empty when resource group key is provided"))
		// }

		// TODO: re-enable once deployment into existing vnet works properly.
		// if (azure.Networks.VNet.Name == nil && azure.Networks.VNet.CIDR == nil) || (azure.Networks.VNet.Name != nil && azure.Networks.VNet.CIDR != nil) {
		// 	allErrs = append(allErrs, field.Invalid(azurePath.Child("networks", "vnet"), azure.Networks.VNet, "must specify either a vnet name or a cidr"))
		// } else if azure.Networks.VNet.CIDR != nil && azure.Networks.VNet.Name == nil {
		// 	allErrs = append(allErrs, validateCIDR(*(azure.Networks.VNet.CIDR), azurePath.Child("networks", "vnet", "cidr"))...)
		// }

		// make sure all CIDRs are canonical
		if azure.Networks.VNet.CIDR != nil {
			allErrs = append(allErrs, validateCIDRIsCanonical(azurePath.Child("networks", "vnet", "cidr"), *azure.Networks.VNet.CIDR)...)
		}
		allErrs = append(allErrs, validateCIDRsAreCanonical(azurePath, nil, &nodes, &pods, &services, nil, nil, []garden.CIDR{azure.Networks.Workers})...)

		workersPath := azurePath.Child("workers")
		if len(azure.Workers) == 0 {
			allErrs = append(allErrs, field.Required(workersPath, "must specify at least one worker"))
			return allErrs
		}

		var workers []garden.Worker
		for i, worker := range azure.Workers {
			idxPath := workersPath.Index(i)
			allErrs = append(allErrs, ValidateWorker(worker.Worker, idxPath)...)
			allErrs = append(allErrs, validateWorkerVolumeSize(worker.VolumeSize, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerMinimumVolumeSize(worker.VolumeSize, 35, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerVolumeType(worker.VolumeType, idxPath.Child("volumeType"))...)
			if workerNames[worker.Name] {
				allErrs = append(allErrs, field.Duplicate(idxPath, worker.Name))
			}
			workerNames[worker.Name] = true
			if worker.Kubelet != nil && worker.Kubelet.MaxPods != nil && *worker.Kubelet.MaxPods > maxPod {
				maxPod = *worker.Kubelet.MaxPods
			}
			workers = append(workers, worker.Worker)
		}
		allErrs = append(allErrs, ValidateWorkers(workers, workersPath)...)
	}

	gcp := cloud.GCP
	gcpPath := fldPath.Child("gcp")
	if gcp != nil {
		zoneCount := len(gcp.Zones)
		if zoneCount == 0 {
			allErrs = append(allErrs, field.Required(gcpPath.Child("zones"), "must specify at least one zone"))
			return allErrs
		}

		nodes, pods, services, networkErrors := transformK8SNetworks(gcp.Networks.K8SNetworks, gcpPath.Child("networks"))
		allErrs = append(allErrs, networkErrors...)

		if len(gcp.Networks.Workers) > 1 {
			allErrs = append(allErrs, field.Invalid(gcpPath.Child("networks", "workers"), gcp.Networks.Workers, "must specify only one worker cidr"))
		}

		workerCIDRs := make([]cidrvalidation.CIDR, 0, len(gcp.Networks.Workers))
		for i, cidr := range gcp.Networks.Workers {
			workerCIDRs = append(workerCIDRs, cidrvalidation.NewCIDR(cidr, gcpPath.Child("networks", "workers").Index(i)))
		}

		if gcp.Networks.Internal != nil {
			internalCIDR := make([]cidrvalidation.CIDR, 0, 1)
			internalCIDR = append(internalCIDR, cidrvalidation.NewCIDR(*gcp.Networks.Internal, gcpPath.Child("networks", "internal")))
			allErrs = append(allErrs, validateCIDRParse(internalCIDR...)...)
			allErrs = append(allErrs, validateCIDROverlap([]cidrvalidation.CIDR{pods, services}, internalCIDR, false)...)
			allErrs = append(allErrs, validateCIDROverlap([]cidrvalidation.CIDR{nodes}, internalCIDR, false)...)
			allErrs = append(allErrs, validateCIDROverlap(workerCIDRs, internalCIDR, false)...)
		}

		allErrs = append(allErrs, validateCIDRParse(workerCIDRs...)...)
		allErrs = append(allErrs, validateCIDROverlap(workerCIDRs, workerCIDRs, false)...)

		allErrs = append(allErrs, validateCIDROverlap([]cidrvalidation.CIDR{pods, services}, workerCIDRs, false)...)
		allErrs = append(allErrs, validateCIDROverlap([]cidrvalidation.CIDR{nodes}, workerCIDRs, true)...)

		if gcp.Networks.VPC != nil && len(gcp.Networks.VPC.Name) == 0 {
			allErrs = append(allErrs, field.Invalid(gcpPath.Child("networks", "vpc", "name"), gcp.Networks.VPC.Name, "vpc name must not be empty when vpc key is provided"))
		}

		// make sure all CIDRs are canonical
		internalNetworks := []garden.CIDR{}
		if gcp.Networks.Internal != nil {
			internalNetworks = []garden.CIDR{*gcp.Networks.Internal}
		}

		allErrs = append(allErrs, validateCIDRsAreCanonical(gcpPath, nil, &nodes, &pods, &services, internalNetworks, nil, gcp.Networks.Workers)...)

		workersPath := gcpPath.Child("workers")
		if len(gcp.Workers) == 0 {
			allErrs = append(allErrs, field.Required(workersPath, "must specify at least one worker"))
			return allErrs
		}

		var workers []garden.Worker
		for i, worker := range gcp.Workers {
			idxPath := workersPath.Index(i)
			allErrs = append(allErrs, ValidateWorker(worker.Worker, idxPath)...)
			allErrs = append(allErrs, validateWorkerVolumeSize(worker.VolumeSize, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerMinimumVolumeSize(worker.VolumeSize, 20, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerVolumeType(worker.VolumeType, idxPath.Child("volumeType"))...)
			if workerNames[worker.Name] {
				allErrs = append(allErrs, field.Duplicate(idxPath, worker.Name))
			}
			workerNames[worker.Name] = true
			if worker.Kubelet != nil && worker.Kubelet.MaxPods != nil && *worker.Kubelet.MaxPods > maxPod {
				maxPod = *worker.Kubelet.MaxPods
			}
			workers = append(workers, worker.Worker)
		}
		allErrs = append(allErrs, ValidateWorkers(workers, workersPath)...)
	}

	openStack := cloud.OpenStack
	openStackPath := fldPath.Child("openstack")
	if openStack != nil {
		if len(openStack.FloatingPoolName) == 0 {
			allErrs = append(allErrs, field.Required(openStackPath.Child("floatingPoolName"), "must specify a floating pool name"))
		}

		if len(openStack.LoadBalancerProvider) == 0 {
			allErrs = append(allErrs, field.Required(openStackPath.Child("loadBalancerProvider"), "must specify a load balancer provider"))
		}

		zoneCount := len(openStack.Zones)
		if zoneCount == 0 {
			allErrs = append(allErrs, field.Required(openStackPath.Child("zones"), "must specify at least one zone"))
			return allErrs
		}

		nodes, pods, services, networkErrors := transformK8SNetworks(openStack.Networks.K8SNetworks, openStackPath.Child("networks"))
		allErrs = append(allErrs, networkErrors...)

		if len(openStack.Networks.Workers) > 1 {
			allErrs = append(allErrs, field.Invalid(openStackPath.Child("networks", "workers"), openStack.Networks.Workers, "must specify only one worker cidr"))
		}

		workerCIDRs := make([]cidrvalidation.CIDR, 0, len(openStack.Networks.Workers))
		for i, cidr := range openStack.Networks.Workers {
			workerCIDR := cidrvalidation.NewCIDR(cidr, openStackPath.Child("networks", "workers").Index(i))
			workerCIDRs = append(workerCIDRs, workerCIDR)
			allErrs = append(allErrs, workerCIDR.ValidateParse()...)
		}

		allErrs = append(allErrs, validateCIDROverlap(workerCIDRs, workerCIDRs, false)...)

		if nodes != nil {
			allErrs = append(allErrs, nodes.ValidateSubset(workerCIDRs...)...)
		}

		if openStack.Networks.Router != nil && len(openStack.Networks.Router.ID) == 0 {
			allErrs = append(allErrs, field.Invalid(openStackPath.Child("networks", "router", "id"), openStack.Networks.Router.ID, "router id must not be empty when router key is provided"))
		}

		// make sure all CIDRs are canonical
		allErrs = append(allErrs, validateCIDRsAreCanonical(openStackPath, nil, &nodes, &pods, &services, nil, nil, openStack.Networks.Workers)...)

		workersPath := openStackPath.Child("workers")
		if len(openStack.Workers) == 0 {
			allErrs = append(allErrs, field.Required(workersPath, "must specify at least one worker"))
			return allErrs
		}

		var workers []garden.Worker
		for i, worker := range openStack.Workers {
			idxPath := workersPath.Index(i)
			allErrs = append(allErrs, ValidateWorker(worker.Worker, idxPath)...)
			if workerNames[worker.Name] {
				allErrs = append(allErrs, field.Duplicate(idxPath, worker.Name))
			}
			workerNames[worker.Name] = true
			workers = append(workers, worker.Worker)
			if worker.Kubelet != nil && worker.Kubelet.MaxPods != nil && *worker.Kubelet.MaxPods > maxPod {
				maxPod = *worker.Kubelet.MaxPods
			}
		}
		allErrs = append(allErrs, ValidateWorkers(workers, workersPath)...)
	}

	alicloud := cloud.Alicloud
	alicloudPath := fldPath.Child("alicloud")
	if alicloud != nil {
		zoneCount := len(alicloud.Zones)
		if zoneCount == 0 {
			allErrs = append(allErrs, field.Required(alicloudPath.Child("zones"), "must specify at least one zone"))
			return allErrs
		}

		nodes, pods, services, networkErrors := transformK8SNetworks(alicloud.Networks.K8SNetworks, alicloudPath.Child("networks"))
		allErrs = append(allErrs, networkErrors...)

		if len(alicloud.Networks.Workers) != zoneCount {
			allErrs = append(allErrs, field.Invalid(alicloudPath.Child("networks", "workers"), alicloud.Networks.Workers, "must specify as many workers networks as zones"))
		}

		workerCIDRs := make([]cidrvalidation.CIDR, 0, len(alicloud.Networks.Workers))
		for i, cidr := range alicloud.Networks.Workers {
			workerCIDR := cidrvalidation.NewCIDR(cidr, alicloudPath.Child("networks", "workers").Index(i))
			workerCIDRs = append(workerCIDRs, workerCIDR)
			allErrs = append(allErrs, workerCIDR.ValidateParse()...)
		}

		allErrs = append(allErrs, validateCIDROverlap(workerCIDRs, workerCIDRs, false)...)

		if nodes != nil {
			allErrs = append(allErrs, nodes.ValidateSubset(workerCIDRs...)...)
		}

		if (alicloud.Networks.VPC.ID == nil && alicloud.Networks.VPC.CIDR == nil) || (alicloud.Networks.VPC.ID != nil && alicloud.Networks.VPC.CIDR != nil) {
			allErrs = append(allErrs, field.Invalid(alicloudPath.Child("networks", "vpc"), alicloud.Networks.VPC, "must specify either a vpc id or a cidr"))
		} else if alicloud.Networks.VPC.CIDR != nil && alicloud.Networks.VPC.ID == nil {
			vpcCIDR := cidrvalidation.NewCIDR(*(alicloud.Networks.VPC.CIDR), alicloudPath.Child("networks", "vpc", "cidr"))
			allErrs = append(allErrs, vpcCIDR.ValidateParse()...)
			allErrs = append(allErrs, vpcCIDR.ValidateSubset(nodes)...)
			allErrs = append(allErrs, vpcCIDR.ValidateSubset(workerCIDRs...)...)
			allErrs = append(allErrs, vpcCIDR.ValidateNotSubset(pods, services)...)
		}

		// make sure all CIDRs are canonical
		allErrs = append(allErrs, validateCIDRsAreCanonical(alicloudPath, alicloud.Networks.VPC.CIDR, &nodes, &pods, &services, nil, nil, alicloud.Networks.Workers)...)

		if len(alicloud.Workers) == 0 {
			allErrs = append(allErrs, field.Required(alicloudPath.Child("workers"), "must specify at least one worker"))
			return allErrs
		}
		for i, worker := range alicloud.Workers {
			idxPath := alicloudPath.Child("workers").Index(i)
			allErrs = append(allErrs, ValidateWorker(worker.Worker, idxPath)...)
			allErrs = append(allErrs, validateWorkerVolumeSize(worker.VolumeSize, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerMinimumVolumeSize(worker.VolumeSize, 30, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerVolumeType(worker.VolumeType, idxPath.Child("volumeType"))...)
			if workerNames[worker.Name] {
				allErrs = append(allErrs, field.Duplicate(idxPath, worker.Name))
			}
			if worker.Kubelet != nil && worker.Kubelet.MaxPods != nil && *worker.Kubelet.MaxPods > maxPod {
				maxPod = *worker.Kubelet.MaxPods
			}
			workerNames[worker.Name] = true
		}

	}

	packet := cloud.Packet
	packetPath := fldPath.Child("packet")
	if packet != nil {
		zoneCount := len(packet.Zones)
		if zoneCount == 0 {
			allErrs = append(allErrs, field.Required(packetPath.Child("zones"), "must specify at least one zone"))
			return allErrs
		}

		_, pods, services, networkErrors := transformK8SNetworks(packet.Networks.K8SNetworks, packetPath.Child("networks"))
		allErrs = append(allErrs, networkErrors...)

		//make sure all CIDRs are canonical
		allErrs = append(allErrs, validateCIDRsAreCanonical(packetPath, nil, nil, &pods, &services, nil, nil, nil)...)

		if len(packet.Workers) == 0 {
			allErrs = append(allErrs, field.Required(packetPath.Child("workers"), "must specify at least one worker"))
			return allErrs
		}
		for i, worker := range packet.Workers {
			idxPath := packetPath.Child("workers").Index(i)
			allErrs = append(allErrs, ValidateWorker(worker.Worker, idxPath)...)
			allErrs = append(allErrs, validateWorkerVolumeSize(worker.VolumeSize, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerMinimumVolumeSize(worker.VolumeSize, 20, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerVolumeType(worker.VolumeType, idxPath.Child("volumeType"))...)
			if workerNames[worker.Name] {
				allErrs = append(allErrs, field.Duplicate(idxPath, worker.Name))
			}
			if worker.Kubelet != nil && worker.Kubelet.MaxPods != nil && *worker.Kubelet.MaxPods > maxPod {
				maxPod = *worker.Kubelet.MaxPods
			}
			workerNames[worker.Name] = true
		}

	}

	if maxPod == 0 {
		// default maxPod setting on kubelet
		maxPod = 110
	}
	allErrs = append(allErrs, ValidateNodeCIDRMaskWithMaxPod(maxPod, *kubernetes.KubeControllerManager.NodeCIDRMaskSize)...)

	return allErrs
}

// ValidateNodeCIDRMaskWithMaxPod validates if the Pod Network has enough ip addresses (configured via the NodeCIDRMask on the kube controller manager) to support the highest max pod setting on the shoot
func ValidateNodeCIDRMaskWithMaxPod(maxPod int32, nodeCIDRMaskSize int) field.ErrorList {
	allErrs := field.ErrorList{}
	free := float64(32 - nodeCIDRMaskSize)
	// first and last ips are reserved
	ipAdressesAvailable := int32(math.Pow(2, free) - 2)

	if ipAdressesAvailable < maxPod {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("kubernetes").Child("kubeControllerManager").Child("nodeCIDRMaskSize"), nodeCIDRMaskSize, fmt.Sprintf("kubelet or kube-controller configuration incorrect. Please adjust the NodeCIDRMaskSize of the kube-controller to support the highest maxPod on any worker pool. The NodeCIDRMaskSize of '%d (default: 24)' of the kube-controller only supports '%d' ip adresses. Highest maxPod setting on kubelet is '%d (default: 110)'. Please choose a NodeCIDRMaskSize that at least supports %d ip adresses", nodeCIDRMaskSize, ipAdressesAvailable, maxPod, maxPod)))
	}
	return allErrs
}

// ValidateShootSpecUpdate validates the specification of a Shoot object.
func ValidateShootSpecUpdate(newSpec, oldSpec *garden.ShootSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet {
		// Allow mutability of machine image spec for migration to worker extensibility
		copyNew := newSpec.DeepCopy()
		copyOld := oldSpec.DeepCopy()

		switch {
		case copyNew.Cloud.AWS != nil:
			copyNew.Cloud.AWS.MachineImage = nil
			copyOld.Cloud.AWS.MachineImage = nil
		case copyNew.Cloud.Azure != nil:
			copyNew.Cloud.Azure.MachineImage = nil
			copyOld.Cloud.Azure.MachineImage = nil
		case copyNew.Cloud.GCP != nil:
			copyNew.Cloud.GCP.MachineImage = nil
			copyOld.Cloud.GCP.MachineImage = nil
		case copyNew.Cloud.OpenStack != nil:
			copyNew.Cloud.OpenStack.MachineImage = nil
			copyOld.Cloud.OpenStack.MachineImage = nil
		case copyNew.Cloud.Alicloud != nil:
			copyNew.Cloud.Alicloud.MachineImage = nil
			copyOld.Cloud.Alicloud.MachineImage = nil
		case copyNew.Cloud.Packet != nil:
			copyNew.Cloud.Packet.MachineImage = nil
			copyOld.Cloud.Packet.MachineImage = nil
		}

		if !apiequality.Semantic.DeepEqual(copyNew, copyOld) {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec, oldSpec, fldPath)...)
		}
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.SecretBindingRef, oldSpec.Cloud.SecretBindingRef, fldPath.Child("cloud", "secretBindingRef"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Profile, oldSpec.Cloud.Profile, fldPath.Child("cloud", "profile"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Region, oldSpec.Cloud.Region, fldPath.Child("cloud", "region"))...)
	// allow initial seed assignment
	if oldSpec.Cloud.Seed != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Seed, oldSpec.Cloud.Seed, fldPath.Child("cloud", "seed"))...)
	}

	awsPath := fldPath.Child("cloud", "aws")
	if oldSpec.Cloud.AWS != nil && newSpec.Cloud.AWS == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.AWS, oldSpec.Cloud.AWS, awsPath)...)
		return allErrs
	} else if newSpec.Cloud.AWS != nil {
		allErrs = append(allErrs, validateK8SNetworksImmutability(oldSpec.Cloud.AWS.Networks.K8SNetworks, newSpec.Cloud.AWS.Networks.K8SNetworks, awsPath.Child("networks"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.AWS.Networks.VPC, oldSpec.Cloud.AWS.Networks.VPC, awsPath.Child("networks", "vpc"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.AWS.Networks.Internal, oldSpec.Cloud.AWS.Networks.Internal, awsPath.Child("networks", "internal"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.AWS.Networks.Public, oldSpec.Cloud.AWS.Networks.Public, awsPath.Child("networks", "public"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.AWS.Networks.Workers, oldSpec.Cloud.AWS.Networks.Workers, awsPath.Child("networks", "workers"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.AWS.Zones, oldSpec.Cloud.AWS.Zones, awsPath.Child("zones"))...)
	}

	azurePath := fldPath.Child("cloud", "azure")
	if oldSpec.Cloud.Azure != nil && newSpec.Cloud.Azure == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Azure, oldSpec.Cloud.Azure, azurePath)...)
		return allErrs
	} else if newSpec.Cloud.Azure != nil {
		allErrs = append(allErrs, validateK8SNetworksImmutability(oldSpec.Cloud.Azure.Networks.K8SNetworks, newSpec.Cloud.Azure.Networks.K8SNetworks, azurePath.Child("networks"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Azure.Networks.VNet, oldSpec.Cloud.Azure.Networks.VNet, azurePath.Child("networks", "vnet"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Azure.Networks.Workers, oldSpec.Cloud.Azure.Networks.Workers, azurePath.Child("networks", "workers"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Azure.ResourceGroup, oldSpec.Cloud.Azure.ResourceGroup, azurePath.Child("resourceGroup"))...)
	}

	gcpPath := fldPath.Child("cloud", "gcp")
	if oldSpec.Cloud.GCP != nil && newSpec.Cloud.GCP == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.GCP, oldSpec.Cloud.GCP, gcpPath)...)
		return allErrs
	} else if newSpec.Cloud.GCP != nil {
		allErrs = append(allErrs, validateK8SNetworksImmutability(oldSpec.Cloud.GCP.Networks.K8SNetworks, newSpec.Cloud.GCP.Networks.K8SNetworks, gcpPath.Child("networks"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.GCP.Networks.VPC, oldSpec.Cloud.GCP.Networks.VPC, gcpPath.Child("networks", "vpc"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.GCP.Networks.Internal, oldSpec.Cloud.GCP.Networks.Internal, gcpPath.Child("networks", "internal"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.GCP.Networks.Workers, oldSpec.Cloud.GCP.Networks.Workers, gcpPath.Child("networks", "workers"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.GCP.Zones, oldSpec.Cloud.GCP.Zones, gcpPath.Child("zones"))...)
	}

	openStackPath := fldPath.Child("cloud", "openstack")
	if oldSpec.Cloud.OpenStack != nil && newSpec.Cloud.OpenStack == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.OpenStack, oldSpec.Cloud.OpenStack, openStackPath)...)
		return allErrs
	} else if newSpec.Cloud.OpenStack != nil {
		allErrs = append(allErrs, validateK8SNetworksImmutability(oldSpec.Cloud.OpenStack.Networks.K8SNetworks, newSpec.Cloud.OpenStack.Networks.K8SNetworks, openStackPath.Child("networks"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.OpenStack.Networks.Router, oldSpec.Cloud.OpenStack.Networks.Router, openStackPath.Child("networks", "router"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.OpenStack.Networks.Workers, oldSpec.Cloud.OpenStack.Networks.Workers, openStackPath.Child("networks", "workers"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.OpenStack.Zones, oldSpec.Cloud.OpenStack.Zones, openStackPath.Child("zones"))...)
	}

	alicloudPath := fldPath.Child("cloud", "alicloud")
	if oldSpec.Cloud.Alicloud != nil && newSpec.Cloud.Alicloud == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Alicloud, oldSpec.Cloud.Alicloud, alicloudPath)...)
		return allErrs
	} else if newSpec.Cloud.Alicloud != nil {
		allErrs = append(allErrs, validateK8SNetworksImmutability(oldSpec.Cloud.Alicloud.Networks.K8SNetworks, newSpec.Cloud.Alicloud.Networks.K8SNetworks, alicloudPath.Child("networks"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Alicloud.Networks.VPC, oldSpec.Cloud.Alicloud.Networks.VPC, alicloudPath.Child("networks", "vpc"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Alicloud.Networks.Workers, oldSpec.Cloud.Alicloud.Networks.Workers, alicloudPath.Child("networks", "workers"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Alicloud.Zones, oldSpec.Cloud.Alicloud.Zones, alicloudPath.Child("zones"))...)
	}

	packetPath := fldPath.Child("cloud", "packet")
	if oldSpec.Cloud.Packet != nil && newSpec.Cloud.Packet == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Packet, oldSpec.Cloud.Packet, packetPath)...)
		return allErrs
	} else if newSpec.Cloud.Packet != nil {
		allErrs = append(allErrs, validateK8SNetworksImmutability(oldSpec.Cloud.Packet.Networks.K8SNetworks, newSpec.Cloud.Packet.Networks.K8SNetworks, packetPath.Child("networks"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Packet.Zones, oldSpec.Cloud.Packet.Zones, packetPath.Child("zones"))...)
	}

	allErrs = append(allErrs, validateDNSUpdate(newSpec.DNS, oldSpec.DNS, fldPath.Child("dns"))...)
	allErrs = append(allErrs, validateKubernetesVersionUpdate(newSpec.Kubernetes.Version, oldSpec.Kubernetes.Version, fldPath.Child("kubernetes", "version"))...)
	allErrs = append(allErrs, validateKubeProxyModeUpdate(newSpec.Kubernetes.KubeProxy, oldSpec.Kubernetes.KubeProxy, newSpec.Kubernetes.Version, fldPath.Child("kubernetes", "kubeProxy"))...)
	allErrs = append(allErrs, validateKubeControllerManagerConfiguration(newSpec.Kubernetes.KubeControllerManager, oldSpec.Kubernetes.KubeControllerManager, fldPath.Child("kubernetes", "kubeControllerManager"))...)
	return allErrs
}

func validateK8SNetworksImmutability(oldNetworks, newNetworks garden.K8SNetworks, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if oldNetworks.Pods != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newNetworks.Pods, oldNetworks.Pods, fldPath.Child("pods"))...)
	}
	if oldNetworks.Services != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newNetworks.Services, oldNetworks.Services, fldPath.Child("services"))...)
	}
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newNetworks.Nodes, oldNetworks.Nodes, fldPath.Child("nodes"))...)

	return allErrs
}

func validateKubeControllerManagerConfiguration(newConfig, oldConfig *garden.KubeControllerManagerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	var newSize *int
	var oldSize *int
	if newConfig != nil {
		newSize = newConfig.NodeCIDRMaskSize
	}
	if oldConfig != nil {
		oldSize = oldConfig.NodeCIDRMaskSize
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSize, oldSize, fldPath.Child("nodeCIDRMaskSize"))...)
	return allErrs
}

func validateKubeProxyModeUpdate(newConfig, oldConfig *garden.KubeProxyConfig, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	newMode := garden.ProxyModeIPTables
	oldMode := garden.ProxyModeIPTables
	if newConfig != nil {
		newMode = *newConfig.Mode
	}
	if oldConfig != nil {
		oldMode = *oldConfig.Mode
	}
	if ok, _ := utils.CheckVersionMeetsConstraint(version, ">= 1.14.1"); ok {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newMode, oldMode, fldPath.Child("mode"))...)
	}
	return allErrs
}

func validateDNSUpdate(new, old garden.DNS, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if new.Provider != old.Provider {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Provider, old.Provider, fldPath.Child("provider"))...)
	}
	if new.Domain != old.Domain {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Domain, old.Domain, fldPath.Child("domain"))...)
	}

	return allErrs
}

func validateKubernetesVersionUpdate(new, old string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(new) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, new, "cannot validate kubernetes version upgrade because it is unset"))
		return allErrs
	}

	// Forbid Kubernetes version downgrade
	downgrade, err := utils.CompareVersions(new, "<", old)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, new, err.Error()))
	}
	if downgrade {
		allErrs = append(allErrs, field.Forbidden(fldPath, "kubernetes version downgrade is not supported"))
	}

	// Forbid Kubernetes version upgrade which skips a minor version
	oldVersion, err := semver.NewVersion(old)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, old, err.Error()))
	}
	nextMinorVersion := oldVersion.IncMinor().IncMinor()

	skippingMinorVersion, err := utils.CompareVersions(new, ">=", nextMinorVersion.String())
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, new, err.Error()))
	}
	if skippingMinorVersion {
		allErrs = append(allErrs, field.Forbidden(fldPath, "kubernetes version upgrade cannot skip a minor version"))
	}

	return allErrs
}

func validateDNS(dns garden.DNS, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if dns.Domain != nil {
		allErrs = append(allErrs, validateDNS1123Subdomain(*dns.Domain, fldPath.Child("domain"))...)
	}

	if provider := dns.Provider; provider != nil {
		if *provider == garden.DNSUnmanaged && dns.SecretName != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("secretName"), dns.SecretName, fmt.Sprintf("`.spec.dns.secretName` must not be set when `.spec.dns.provider` is %q", garden.DNSUnmanaged)))
		}

		if *provider != garden.DNSUnmanaged && dns.Domain == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("domain"), fmt.Sprintf("`.spec.dns.domain` must be set when `.spec.dns.provider` is not set to %q", garden.DNSUnmanaged)))
		}
	}

	if dns.SecretName != nil && dns.Provider == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("provider"), "`.spec.dns.provider` must be set when `.spec.dns.secretName` is set"))
	}

	return allErrs
}

func validateExtensions(extensions []garden.Extension, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, extension := range extensions {
		if extension.Type == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("type"), "field must not be empty"))
		}
	}
	return allErrs
}

func validateCIDRParse(cidrPaths ...cidrvalidation.CIDR) (allErrs field.ErrorList) {
	for _, cidrPath := range cidrPaths {
		if cidrPath == nil {
			continue
		}
		allErrs = append(allErrs, cidrPath.ValidateParse()...)
	}
	return allErrs
}

func validateCIDROverlap(leftPaths, rightPaths []cidrvalidation.CIDR, overlap bool) (allErrs field.ErrorList) {
	for _, left := range leftPaths {
		if left == nil {
			continue
		}
		if overlap {
			allErrs = append(allErrs, left.ValidateSubset(rightPaths...)...)
		} else {
			allErrs = append(allErrs, left.ValidateNotSubset(rightPaths...)...)
		}
	}
	return allErrs
}

func validateCIDRsAreCanonical(fldPath *field.Path, vpc *garden.CIDR, nodes *cidrvalidation.CIDR, pods *cidrvalidation.CIDR, services *cidrvalidation.CIDR, internal []garden.CIDR, public []garden.CIDR, workers []garden.CIDR) field.ErrorList {
	allErrs := field.ErrorList{}
	if vpc != nil {
		allErrs = append(allErrs, validateCIDRIsCanonical(fldPath.Child("networks", "vpc", "cidr"), *vpc)...)
	}
	if nodes != nil && *nodes != nil {
		cidr := *nodes
		allErrs = append(allErrs, validateCIDRIsCanonical(fldPath.Child("nodes"), cidr.GetCIDR())...)
	}
	if pods != nil && *pods != nil {
		cidr := *pods
		allErrs = append(allErrs, validateCIDRIsCanonical(fldPath.Child("pods"), cidr.GetCIDR())...)
	}
	if services != nil && *services != nil {
		cidr := *services
		allErrs = append(allErrs, validateCIDRIsCanonical(fldPath.Child("services"), cidr.GetCIDR())...)
	}

	for i, cidr := range internal {
		allErrs = append(allErrs, validateCIDRIsCanonical(fldPath.Child("networks", "internal").Index(i), cidr)...)
	}
	for i, cidr := range public {
		allErrs = append(allErrs, validateCIDRIsCanonical(fldPath.Child("networks", "public").Index(i), cidr)...)
	}
	for i, cidr := range workers {
		allErrs = append(allErrs, validateCIDRIsCanonical(fldPath.Child("networks", "workers").Index(i), cidr)...)
	}
	return allErrs
}

func validateCIDRIsCanonical(fldPath *field.Path, cidrToValidate garden.CIDR) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(cidrToValidate) == 0 {
		return allErrs
	}
	// CIDR parse error already validated
	ipAdress, ipNet, _ := net.ParseCIDR(string(cidrToValidate))
	if ipNet != nil {
		if !ipNet.IP.Equal(ipAdress) {
			allErrs = append(allErrs, field.Invalid(fldPath, cidrToValidate, "must be valid canonical CIDR"))
		}
	}
	return allErrs
}

func transformK8SNetworks(networks garden.K8SNetworks, fldPath *field.Path) (nodes, pods, services cidrvalidation.CIDR, allErrs field.ErrorList) {
	cidrs := []cidrvalidation.CIDR{}

	if networks.Nodes != nil {
		nodes = cidrvalidation.NewCIDR(*networks.Nodes, fldPath.Child("nodes"))
		allErrs = append(allErrs, nodes.ValidateParse()...)
		cidrs = append(cidrs, nodes)
	}

	if networks.Pods != nil {
		pods = cidrvalidation.NewCIDR(*networks.Pods, fldPath.Child("pods"))
		allErrs = append(allErrs, pods.ValidateParse()...)
		cidrs = append(cidrs, pods)
	}

	if networks.Services != nil {
		services = cidrvalidation.NewCIDR(*networks.Services, fldPath.Child("services"))
		allErrs = append(allErrs, services.ValidateParse()...)
		cidrs = append(cidrs, services)
	}

	allErrs = append(allErrs, validateCIDROverlap(cidrs, cidrs, false)...)
	return nodes, pods, services, allErrs
}

func validateKubernetes(kubernetes garden.Kubernetes, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(kubernetes.Version) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "kubernetes version must not be empty"))
		return allErrs
	}

	if kubeAPIServer := kubernetes.KubeAPIServer; kubeAPIServer != nil {
		if oidc := kubeAPIServer.OIDCConfig; oidc != nil {
			oidcPath := fldPath.Child("kubeAPIServer", "oidcConfig")

			geqKubernetes111, err := utils.CheckVersionMeetsConstraint(kubernetes.Version, ">= 1.11")
			if err != nil {
				geqKubernetes111 = false
			}

			if oidc.CABundle != nil {
				if _, err := utils.DecodeCertificate([]byte(*oidc.CABundle)); err != nil {
					allErrs = append(allErrs, field.Invalid(oidcPath.Child("caBundle"), *oidc.CABundle, "caBundle is not a valid PEM-encoded certificate"))
				}
			}
			if oidc.ClientID != nil && len(*oidc.ClientID) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("clientID"), *oidc.ClientID, "client id cannot be empty when key is provided"))
			}
			if oidc.GroupsClaim != nil && len(*oidc.GroupsClaim) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("groupsClaim"), *oidc.GroupsClaim, "groups claim cannot be empty when key is provided"))
			}
			if oidc.GroupsPrefix != nil && len(*oidc.GroupsPrefix) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("groupsPrefix"), *oidc.GroupsPrefix, "groups prefix cannot be empty when key is provided"))
			}
			if oidc.IssuerURL != nil && len(*oidc.IssuerURL) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("issuerURL"), *oidc.IssuerURL, "issuer url cannot be empty when key is provided"))
			}
			if oidc.SigningAlgs != nil && len(oidc.SigningAlgs) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("signingAlgs"), oidc.SigningAlgs, "signings algs cannot be empty when key is provided"))
			}
			if !geqKubernetes111 && oidc.RequiredClaims != nil {
				allErrs = append(allErrs, field.Forbidden(oidcPath.Child("requiredClaims"), "required claims cannot be provided when version is not greater or equal 1.11"))
			}
			if oidc.UsernameClaim != nil && len(*oidc.UsernameClaim) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("usernameClaim"), *oidc.UsernameClaim, "username claim cannot be empty when key is provided"))
			}
			if oidc.UsernamePrefix != nil && len(*oidc.UsernamePrefix) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("usernamePrefix"), *oidc.UsernamePrefix, "username prefix cannot be empty when key is provided"))
			}
		}

		admissionPluginsPath := fldPath.Child("kubeAPIServer", "admissionPlugins")
		for i, plugin := range kubeAPIServer.AdmissionPlugins {
			idxPath := admissionPluginsPath.Index(i)

			if len(plugin.Name) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
			}
		}

		if auditConfig := kubeAPIServer.AuditConfig; auditConfig != nil {
			auditPath := fldPath.Child("kubeAPIServer", "auditConfig")
			if auditPolicy := auditConfig.AuditPolicy; auditPolicy != nil && auditConfig.AuditPolicy.ConfigMapRef != nil {
				allErrs = append(allErrs, validateLocalObjectReference(auditPolicy.ConfigMapRef, auditPath.Child("auditPolicy", "configMapRef"))...)
			}
		}
	}

	allErrs = append(allErrs, validateKubeControllerManager(kubernetes.Version, kubernetes.KubeControllerManager, fldPath.Child("kubeControllerManager"))...)
	allErrs = append(allErrs, validateKubeProxy(kubernetes.KubeProxy, fldPath.Child("kubeProxy"))...)
	if clusterAutoscaler := kubernetes.ClusterAutoscaler; clusterAutoscaler != nil {
		allErrs = append(allErrs, ValidateClusterAutoscaler(*clusterAutoscaler, fldPath.Child("clusterAutoscaler"))...)
	}

	return allErrs
}

func validateNetworking(networking *garden.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networking == nil {
		return append(allErrs, field.Required(fldPath, "networking section must be provided"))
	}

	if len(networking.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "networking type must be provided"))
	}

	return allErrs
}

// ValidateClusterAutoscaler validates the given ClusterAutoscaler fields.
func ValidateClusterAutoscaler(autoScaler garden.ClusterAutoscaler, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if threshold := autoScaler.ScaleDownUtilizationThreshold; threshold != nil {
		if *threshold < 0.0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownUtilizationThreshold"), *threshold, "can not be negative"))
		}
		if *threshold > 1.0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownUtilizationThreshold"), *threshold, "can not be greater than 1.0"))
		}
	}
	return allErrs
}

func validateKubeControllerManager(kubernetesVersion string, kcm *garden.KubeControllerManagerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	k8sVersionLessThan112, err := utils.CompareVersions(kubernetesVersion, "<", "1.12")
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, kubernetesVersion, err.Error()))
	}
	if kcm != nil {
		if maskSize := kcm.NodeCIDRMaskSize; maskSize != nil {
			if *maskSize < 16 || *maskSize > 28 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("nodeCIDRMaskSize"), *maskSize, "nodeCIDRMaskSize must be between 16 and 28"))
			}
		}
		if hpa := kcm.HorizontalPodAutoscalerConfig; hpa != nil {
			fldPath = fldPath.Child("horizontalPodAutoscaler")

			if hpa.SyncPeriod != nil && hpa.SyncPeriod.Duration < 1*time.Second {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("syncPeriod"), *hpa.SyncPeriod, "syncPeriod must not be less than a second"))
			}
			if hpa.Tolerance != nil && *hpa.Tolerance <= 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("tolerance"), *hpa.Tolerance, "tolerance of must be greater than 0"))
			}

			if k8sVersionLessThan112 {
				if hpa.DownscaleDelay != nil && hpa.DownscaleDelay.Duration < 0 {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("downscaleDelay"), *hpa.DownscaleDelay, "downscale delay must not be negative"))
				}
				if hpa.UpscaleDelay != nil && hpa.UpscaleDelay.Duration < 0 {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("upscaleDelay"), *hpa.UpscaleDelay, "upscale delay must not be negative"))
				}
				if hpa.DownscaleStabilization != nil {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("downscaleStabilization"), "downscale stabilization is not supported in k8s versions < 1.12"))
				}
				if hpa.InitialReadinessDelay != nil {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("initialReadinessDelay"), "initial readiness delay is not supported in k8s versions < 1.12"))
				}
				if hpa.CPUInitializationPeriod != nil {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("cpuInitializationPeriod"), "cpu initialization period is not supported in k8s versions < 1.12"))
				}
			} else {
				if hpa.DownscaleDelay != nil {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("downscaleDelay"), "downscale delay is not supported in k8s versions >= 1.12"))
				}
				if hpa.UpscaleDelay != nil {
					allErrs = append(allErrs, field.Forbidden(fldPath.Child("upscaleDelay"), "upscale delay is not supported in k8s versions >= 1.12"))
				}
				if hpa.DownscaleStabilization != nil && hpa.DownscaleStabilization.Duration < 1*time.Second {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("downscaleStabilization"), *hpa.DownscaleStabilization, "downScale stabilization must not be less than a second"))
				}
				if hpa.InitialReadinessDelay != nil && hpa.InitialReadinessDelay.Duration <= 0 {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("initialReadinessDelay"), *hpa.InitialReadinessDelay, "initial readiness delay must be greater than 0"))
				}
				if hpa.CPUInitializationPeriod != nil && hpa.CPUInitializationPeriod.Duration < 1*time.Second {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("cpuInitializationPeriod"), *hpa.CPUInitializationPeriod, "cpu initialization period must not be less than a second"))
				}
			}
		}
	}
	return allErrs
}

func validateKubeProxy(kp *garden.KubeProxyConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if kp != nil {
		if kp.Mode == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("mode"), "must be set when .spec.kubernetes.kubeProxy is set"))
		} else if mode := *kp.Mode; !availableProxyMode.Has(string(mode)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("mode"), mode, availableProxyMode.List()))
		}
	}
	return allErrs
}

func validateMaintenance(maintenance *garden.Maintenance, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if maintenance == nil {
		allErrs = append(allErrs, field.Required(fldPath, "maintenance information is required"))
		return allErrs
	}

	if maintenance.AutoUpdate == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("autoUpdate"), "auto update information is required"))
	}

	if maintenance.TimeWindow == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("timeWindow"), "time window information is required"))
	} else {
		maintenanceTimeWindow, err := utils.ParseMaintenanceTimeWindow(maintenance.TimeWindow.Begin, maintenance.TimeWindow.End)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("timeWindow", "begin/end"), maintenance.TimeWindow, err.Error()))
		}

		if err == nil {
			duration := maintenanceTimeWindow.Duration()
			if duration > 6*time.Hour {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("timeWindow"), "time window must not be greater than 6 hours"))
				return allErrs
			}
			if duration < 30*time.Minute {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("timeWindow"), "time window must not be smaller than 30 minutes"))
				return allErrs
			}
		}
	}

	return allErrs
}

// ValidateWorker validates the worker object.
func ValidateWorker(worker garden.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDNS1123Label(worker.Name, fldPath.Child("name"))...)
	maxWorkerNameLength := 15
	if len(worker.Name) > maxWorkerNameLength {
		allErrs = append(allErrs, field.TooLong(fldPath.Child("name"), worker.Name, maxWorkerNameLength))
	}
	if len(worker.MachineType) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("machineType"), "must specify a machine type"))
	}
	if worker.AutoScalerMin < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("autoScalerMin"), worker.AutoScalerMin, "minimum value must not be negative"))
	}
	if worker.AutoScalerMax < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("autoScalerMax"), worker.AutoScalerMax, "maximum value must not be negative"))
	}
	if worker.AutoScalerMax < worker.AutoScalerMin {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("autoScalerMax"), "maximum value must not be less or equal than minimum value"))
	}
	if worker.AutoScalerMax != 0 && worker.AutoScalerMin == 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("autoScalerMin"), "minimum value must be >= 1 if maximum value > 0 (cluster-autoscaler cannot handle min=0)"))
	}

	allErrs = append(allErrs, ValidatePositiveIntOrPercent(worker.MaxSurge, fldPath.Child("maxSurge"))...)
	allErrs = append(allErrs, ValidatePositiveIntOrPercent(worker.MaxUnavailable, fldPath.Child("maxUnavailable"))...)
	allErrs = append(allErrs, IsNotMoreThan100Percent(worker.MaxUnavailable, fldPath.Child("maxUnavailable"))...)

	if getIntOrPercentValue(worker.MaxUnavailable) == 0 && getIntOrPercentValue(worker.MaxSurge) == 0 {
		// Both MaxSurge and MaxUnavailable cannot be zero.
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), worker.MaxUnavailable, "may not be 0 when `maxSurge` is 0"))
	}

	allErrs = append(allErrs, metav1validation.ValidateLabels(worker.Labels, fldPath.Child("labels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(worker.Annotations, fldPath.Child("annotations"))...)
	if len(worker.Taints) > 0 {
		allErrs = append(allErrs, validateTaints(worker.Taints, fldPath.Child("taints"))...)
	}
	if worker.Kubelet != nil {
		allErrs = append(allErrs, ValidateKubeletConfig(*worker.Kubelet, fldPath.Child("kubelet"))...)
	}

	return allErrs
}

// ValidateWorker validates the worker object.
func ValidateKubeletConfig(kubeletConfig garden.KubeletConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if kubeletConfig.MaxPods != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*kubeletConfig.MaxPods), fldPath.Child("maxPods"))...)
	}

	if kubeletConfig.EvictionPressureTransitionPeriod != nil {
		allErrs = append(allErrs, ValidatePositiveDuration(kubeletConfig.EvictionPressureTransitionPeriod, fldPath.Child("evictionPressureTransitionPeriod"))...)
	}

	if kubeletConfig.EvictionMaxPodGracePeriod != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*kubeletConfig.EvictionMaxPodGracePeriod), fldPath.Child("evictionMaxPodGracePeriod"))...)
	}

	if kubeletConfig.EvictionHard != nil {
		allErrs = append(allErrs, validateKubeletConfigEviction(kubeletConfig.EvictionHard, fldPath.Child("evictionHard"))...)
	}
	if kubeletConfig.EvictionSoft != nil {
		allErrs = append(allErrs, validateKubeletConfigEviction(kubeletConfig.EvictionSoft, fldPath.Child("evictionSoft"))...)
	}
	if kubeletConfig.EvictionMinimumReclaim != nil {
		allErrs = append(allErrs, validateKubeletConfigEvictionMinimumReclaim(kubeletConfig.EvictionMinimumReclaim, fldPath.Child("evictionMinimumReclaim"))...)
	}
	if kubeletConfig.EvictionSoftGracePeriod != nil {
		allErrs = append(allErrs, validateKubeletConfigEvictionSoftGracePeriod(kubeletConfig.EvictionSoftGracePeriod, fldPath.Child("evictionSoftGracePeriod"))...)
	}
	return allErrs
}

func validateKubeletConfigEviction(eviction *garden.KubeletConfigEviction, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.MemoryAvailable, fldPath, "memoryAvailable")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.ImageFSAvailable, fldPath, "imagefsAvailable")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.ImageFSInodesFree, fldPath, "imagefsInodesFree")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.NodeFSAvailable, fldPath, "nodefsAvailable")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.ImageFSInodesFree, fldPath, "imagefsInodesFree")...)
	return allErrs
}

func validateKubeletConfigEvictionMinimumReclaim(eviction *garden.KubeletConfigEvictionMinimumReclaim, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if eviction.MemoryAvailable != nil {
		allErrs = append(allErrs, validateResourceQuantityValue("memoryAvailable", *eviction.MemoryAvailable, fldPath.Child("memoryAvailable"))...)
	}
	if eviction.ImageFSAvailable != nil {
		allErrs = append(allErrs, validateResourceQuantityValue("imagefsAvailable", *eviction.ImageFSAvailable, fldPath.Child("imagefsAvailable"))...)
	}
	if eviction.ImageFSInodesFree != nil {
		allErrs = append(allErrs, validateResourceQuantityValue("imagefsInodesFree", *eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	}
	if eviction.NodeFSAvailable != nil {
		allErrs = append(allErrs, validateResourceQuantityValue("nodefsAvailable", *eviction.NodeFSAvailable, fldPath.Child("nodefsAvailable"))...)
	}
	if eviction.ImageFSInodesFree != nil {
		allErrs = append(allErrs, validateResourceQuantityValue("imagefsInodesFree", *eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	}
	return allErrs
}

func validateKubeletConfigEvictionSoftGracePeriod(eviction *garden.KubeletConfigEvictionSoftGracePeriod, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.MemoryAvailable, fldPath.Child("memoryAvailable"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.ImageFSAvailable, fldPath.Child("imagefsAvailable"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.NodeFSAvailable, fldPath.Child("nodefsAvailable"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	return allErrs
}

// https://github.com/kubernetes/kubernetes/blob/ee9079f8ec39914ff8975b5390749771b9303ea4/pkg/apis/core/validation/validation.go#L4057-L4089
func validateTaints(taints []corev1.Taint, fldPath *field.Path) field.ErrorList {
	allErrors := field.ErrorList{}

	uniqueTaints := map[corev1.TaintEffect]sets.String{}

	for i, taint := range taints {
		idxPath := fldPath.Index(i)
		// validate the taint key
		allErrors = append(allErrors, metav1validation.ValidateLabelName(taint.Key, idxPath.Child("key"))...)
		// validate the taint value
		if errs := validation.IsValidLabelValue(taint.Value); len(errs) != 0 {
			allErrors = append(allErrors, field.Invalid(idxPath.Child("value"), taint.Value, strings.Join(errs, ";")))
		}
		// validate the taint effect
		allErrors = append(allErrors, validateTaintEffect(&taint.Effect, false, idxPath.Child("effect"))...)

		// validate if taint is unique by <key, effect>
		if len(uniqueTaints[taint.Effect]) > 0 && uniqueTaints[taint.Effect].Has(taint.Key) {
			duplicatedError := field.Duplicate(idxPath, taint)
			duplicatedError.Detail = "taints must be unique by key and effect pair"
			allErrors = append(allErrors, duplicatedError)
			continue
		}

		// add taint to existingTaints for uniqueness check
		if len(uniqueTaints[taint.Effect]) == 0 {
			uniqueTaints[taint.Effect] = sets.String{}
		}
		uniqueTaints[taint.Effect].Insert(taint.Key)
	}
	return allErrors
}

// https://github.com/kubernetes/kubernetes/blob/ee9079f8ec39914ff8975b5390749771b9303ea4/pkg/apis/core/validation/validation.go#L2774-L2795
func validateTaintEffect(effect *corev1.TaintEffect, allowEmpty bool, fldPath *field.Path) field.ErrorList {
	if !allowEmpty && len(*effect) == 0 {
		return field.ErrorList{field.Required(fldPath, "")}
	}

	allErrors := field.ErrorList{}
	switch *effect {
	case corev1.TaintEffectNoSchedule, corev1.TaintEffectPreferNoSchedule, corev1.TaintEffectNoExecute:
	default:
		validValues := []string{
			string(corev1.TaintEffectNoSchedule),
			string(corev1.TaintEffectPreferNoSchedule),
			string(corev1.TaintEffectNoExecute),
		}
		allErrors = append(allErrors, field.NotSupported(fldPath, *effect, validValues))
	}
	return allErrors
}

// ValidateWorkers validates worker objects.
func ValidateWorkers(workers []garden.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	atLeastOneActivePool := false
	for _, worker := range workers {
		if worker.AutoScalerMin != 0 && worker.AutoScalerMax != 0 {
			atLeastOneActivePool = true
			break
		}
	}

	if !atLeastOneActivePool {
		allErrs = append(allErrs, field.Forbidden(fldPath, "at least one worker pool with min>0 and max> 0 needed"))
	}

	return allErrs
}

// ValidateHibernation validates a Hibernation object.
func ValidateHibernation(hibernation *garden.Hibernation, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if hibernation == nil {
		return allErrs
	}

	allErrs = append(allErrs, ValidateHibernationSchedules(hibernation.Schedules, fldPath.Child("schedules"))...)

	return allErrs
}

func ValidateHibernationSchedules(schedules []garden.HibernationSchedule, fldPath *field.Path) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		seen    = sets.NewString()
	)

	for i, schedule := range schedules {
		allErrs = append(allErrs, ValidateHibernationSchedule(seen, &schedule, fldPath.Index(i))...)
	}

	return allErrs
}

func ValidateHibernationCronSpec(seenSpecs sets.String, spec string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	_, err := cron.ParseStandard(spec)
	switch {
	case err != nil:
		allErrs = append(allErrs, field.Invalid(fldPath, spec, fmt.Sprintf("not a valid cron spec: %v", err)))
	case seenSpecs.Has(spec):
		allErrs = append(allErrs, field.Duplicate(fldPath, spec))
	default:
		seenSpecs.Insert(spec)
	}

	return allErrs
}

// ValidateHibernationScheduleLocation validates that the location of a HibernationSchedule is correct.
func ValidateHibernationScheduleLocation(location string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if _, err := time.LoadLocation(location); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, location, fmt.Sprintf("not a valid location: %v", err)))
	}

	return allErrs
}

// ValidateHibernationSchedule validates the correctness of a HibernationSchedule.
// It checks whether the set start and end time are valid cron specs.
func ValidateHibernationSchedule(seenSpecs sets.String, schedule *garden.HibernationSchedule, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if schedule.Start == nil && schedule.End == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("start/end"), "either start or end has to be provided"))
	}
	if schedule.Start != nil {
		allErrs = append(allErrs, ValidateHibernationCronSpec(seenSpecs, *schedule.Start, fldPath.Child("start"))...)
	}
	if schedule.End != nil {
		allErrs = append(allErrs, ValidateHibernationCronSpec(seenSpecs, *schedule.End, fldPath.Child("end"))...)
	}
	if schedule.Location != nil {
		allErrs = append(allErrs, ValidateHibernationScheduleLocation(*schedule.Location, fldPath.Child("location"))...)
	}

	return allErrs
}

func validateWorkerVolumeSize(volumeSize string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	volumeSizeRegex, _ := regexp.Compile(`^(\d)+Gi$`)
	if !volumeSizeRegex.MatchString(volumeSize) {
		allErrs = append(allErrs, field.Invalid(fldPath, volumeSize, fmt.Sprintf("domain must match the regex %s", volumeSizeRegex)))
	}

	return allErrs
}

func validateWorkerMinimumVolumeSize(volumeSize string, minmumVolumeSize int, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	volumeSizeRegex, _ := regexp.Compile(`^(\d+)Gi$`)
	match := volumeSizeRegex.FindStringSubmatch(volumeSize)
	if len(match) == 2 {
		volSize, err := strconv.Atoi(match[1])
		if err != nil || volSize < minmumVolumeSize {
			allErrs = append(allErrs, field.Invalid(fldPath, volumeSize, fmt.Sprintf("volume size must be at least %dGi", minmumVolumeSize)))
		}
	}

	return allErrs
}

func validateWorkerVolumeType(volumeType string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(volumeType) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must specify a volume type"))
	}

	return allErrs
}

// validateDNS1123Subdomain validates that a name is a proper DNS subdomain.
func validateDNS1123Subdomain(value string, fldPath *field.Path) field.ErrorList {
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

////////////////////////////////////////////////////
//          BACKUP INFRASTRUCTURE                 //
////////////////////////////////////////////////////

// ValidateBackupInfrastructure validates a BackupInfrastructure object.
func ValidateBackupInfrastructure(backupInfrastructure *garden.BackupInfrastructure) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&backupInfrastructure.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupInfrastructureSpec(&backupInfrastructure.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateBackupInfrastructureUpdate validates a BackupInfrastructure object before an update.
func ValidateBackupInfrastructureUpdate(newBackupInfrastructure, oldBackupInfrastructure *garden.BackupInfrastructure) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBackupInfrastructure.ObjectMeta, &oldBackupInfrastructure.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupInfrastructureSpecUpdate(&newBackupInfrastructure.Spec, &oldBackupInfrastructure.Spec, newBackupInfrastructure.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateBackupInfrastructure(newBackupInfrastructure)...)

	return allErrs
}

// ValidateBackupInfrastructureSpec validates the specification of a BackupInfrastructure object.
func ValidateBackupInfrastructureSpec(spec *garden.BackupInfrastructureSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Seed) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("seed"), spec.Seed, "seed name must not be empty"))
	}
	if len(spec.ShootUID) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("shootUID"), spec.Seed, "shootUID must not be empty"))
	}

	return allErrs
}

// ValidateBackupInfrastructureSpecUpdate validates the specification of a BackupInfrastructure object.
func ValidateBackupInfrastructureSpecUpdate(newSpec, oldSpec *garden.BackupInfrastructureSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Seed, oldSpec.Seed, fldPath.Child("seed"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.ShootUID, oldSpec.ShootUID, fldPath.Child("shootUID"))...)
	return allErrs
}

// ValidateBackupInfrastructureStatusUpdate validates the status field of a BackupInfrastructure object.
func ValidateBackupInfrastructureStatusUpdate(newBackupInfrastructure, oldBackupInfrastructure *garden.BackupInfrastructure) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
