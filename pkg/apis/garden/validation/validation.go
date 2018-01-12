// Copyright 2018 The Gardener Authors.
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

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateName is a helper function for validating that a name is a DNS sub domain.
func ValidateName(name string, prefix bool) []string {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
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
	allErrs := apivalidation.ValidateObjectMetaUpdate(&newProfile.ObjectMeta, &oldProfile.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateCloudProfile(newProfile)...)
	return allErrs
}

// ValidateCloudProfileSpec validates the specification of a CloudProfile object.
func ValidateCloudProfileSpec(spec *garden.CloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if _, err := helper.DetermineCloudProviderInProfile(*spec); err != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("aws/azure/gcp/openstack"), "cloud profile must only contain exactly one field of aws/azure/gcp/openstack"))
		return allErrs
	}

	if spec.AWS != nil {
		allErrs = append(allErrs, validateDNSProviders(spec.AWS.Constraints.DNSProviders, fldPath.Child("aws", "constraints", "dnsProviders"))...)
		allErrs = append(allErrs, validateKubernetesConstraints(spec.AWS.Constraints.Kubernetes, fldPath.Child("aws", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.AWS.Constraints.MachineTypes, fldPath.Child("aws", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypeConstraints(spec.AWS.Constraints.VolumeTypes, fldPath.Child("aws", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.AWS.Constraints.Zones, fldPath.Child("aws", "constraints", "zones"))...)

		if len(spec.AWS.MachineImages) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("aws", "machineImages"), "must provide at least one machine image"))
		}
		r, _ := regexp.Compile(`^ami-[a-z0-9]+$`)
		for i, image := range spec.AWS.MachineImages {
			idxPath := fldPath.Child("aws", "machineImages").Index(i)
			regionPath := idxPath.Child("region")
			amiPath := idxPath.Child("ami")
			if len(image.Region) == 0 {
				allErrs = append(allErrs, field.Required(regionPath, "must provide a region"))
			}
			if !r.MatchString(image.AMI) {
				allErrs = append(allErrs, field.Invalid(amiPath, image.AMI, `ami's must match the regex ^ami-[a-z0-9]+$`))
			}
		}
	}

	if spec.Azure != nil {
		allErrs = append(allErrs, validateDNSProviders(spec.Azure.Constraints.DNSProviders, fldPath.Child("azure", "constraints", "dnsProviders"))...)
		allErrs = append(allErrs, validateKubernetesConstraints(spec.Azure.Constraints.Kubernetes, fldPath.Child("azure", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.Azure.Constraints.MachineTypes, fldPath.Child("azure", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypeConstraints(spec.Azure.Constraints.VolumeTypes, fldPath.Child("azure", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateAzureDomainCount(spec.Azure.CountFaultDomains, fldPath.Child("azure", "countFaultDomains"))...)
		allErrs = append(allErrs, validateAzureDomainCount(spec.Azure.CountUpdateDomains, fldPath.Child("azure", "countUpdateDomains"))...)

		if len(spec.Azure.MachineImage.Channel) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("azure", "machineImage", "channel"), "must provide a channel"))
		}
		if len(spec.Azure.MachineImage.Version) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("azure", "machineImage", "version"), "must provide a version"))
		}
	}

	if spec.GCP != nil {
		allErrs = append(allErrs, validateDNSProviders(spec.GCP.Constraints.DNSProviders, fldPath.Child("gcp", "constraints", "dnsProviders"))...)
		allErrs = append(allErrs, validateKubernetesConstraints(spec.GCP.Constraints.Kubernetes, fldPath.Child("gcp", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.GCP.Constraints.MachineTypes, fldPath.Child("gcp", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypeConstraints(spec.GCP.Constraints.VolumeTypes, fldPath.Child("gcp", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.GCP.Constraints.Zones, fldPath.Child("gcp", "constraints", "zones"))...)

		if len(spec.GCP.MachineImage.Name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("gcp", "machineImage", "name"), "must provide an image name"))
		}
	}

	if spec.OpenStack != nil {
		allErrs = append(allErrs, validateDNSProviders(spec.OpenStack.Constraints.DNSProviders, fldPath.Child("openstack", "constraints", "dnsProviders"))...)
		allErrs = append(allErrs, validateKubernetesConstraints(spec.OpenStack.Constraints.Kubernetes, fldPath.Child("openstack", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.OpenStack.Constraints.MachineTypes, fldPath.Child("openstack", "constraints", "machineTypes"))...)
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

		if len(spec.OpenStack.MachineImage.Name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("openstack", "machineImage", "name"), "must provide an image name"))
		}

		if len(spec.OpenStack.KeyStoneURL) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("openstack", "keyStoneURL"), "must provide the URL to KeyStone"))
		}

		_, err := utils.DecodeCertificate([]byte(spec.OpenStack.CABundle))
		if err != nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("openstack", "caBundle"), "caBundle is not a valid PEM-encoded certificate"))
		}
	}

	return allErrs
}

func validateDNSProviders(providers []garden.DNSProviderConstraint, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(providers) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one DNS provider"))
	}

	for i, provider := range providers {
		idxPath := fldPath.Index(i)
		if provider.Name != garden.DNSUnmanaged && provider.Name != garden.DNSAWSRoute53 {
			allErrs = append(allErrs, field.NotSupported(idxPath, provider.Name, []string{string(garden.DNSUnmanaged), string(garden.DNSAWSRoute53)}))
		}
	}

	return allErrs
}

func validateKubernetesConstraints(kubernetes garden.KubernetesConstraints, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(kubernetes.Versions) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("versions"), "must provide at least one Kubernetes version"))
	}

	r, _ := regexp.Compile(`^([0-9]+\.){2}[0-9]+$`)
	for i, version := range kubernetes.Versions {
		idxPath := fldPath.Child("versions").Index(i)
		if !r.MatchString(version) {
			allErrs = append(allErrs, field.Invalid(idxPath, version, `all Kubernetes versions must match the regex ^([0-9]+\.){2}[0-9]+$`))
		}
	}

	return allErrs
}

func validateMachineTypeConstraints(machineTypes []garden.MachineType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineTypes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine type"))
	}

	for i, machineType := range machineTypes {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		cpusPath := idxPath.Child("cpus")
		gpusPath := idxPath.Child("gpus")
		memoryPath := idxPath.Child("memoryPath")

		if len(machineType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		}
		if machineType.CPUs < 0 {
			allErrs = append(allErrs, field.Invalid(cpusPath, machineType.CPUs, "cpus cannot be negative"))
		}
		if machineType.GPUs < 0 {
			allErrs = append(allErrs, field.Invalid(gpusPath, machineType.GPUs, "gpus cannot be negative"))
		}
		allErrs = append(allErrs, ValidateResourceQuantityValue("memory", machineType.Memory, memoryPath)...)
	}

	return allErrs
}

func validateVolumeTypeConstraints(volumeTypes []garden.VolumeType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(volumeTypes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one volume type"))
	}

	for i, volumeType := range volumeTypes {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		classPath := idxPath.Child("class")

		if len(volumeType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		}
		if len(volumeType.Class) == 0 {
			allErrs = append(allErrs, field.Required(classPath, "must provide a class"))
		}
	}

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
//                      SEEDS                     //
////////////////////////////////////////////////////

// ValidateSeed validates a Seed object.
func ValidateSeed(seed *garden.Seed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&seed.ObjectMeta, false, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateSeedSpec(&seed.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateSeedUpdate validates a Seed object before an update.
func ValidateSeedUpdate(newSeed, oldSeed *garden.Seed) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&newSeed.ObjectMeta, &oldSeed.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateSeed(newSeed)...)
	return allErrs
}

// ValidateSeedSpec validates the specification of a Seed object.
func ValidateSeedSpec(seedSpec *garden.SeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// seedFQDN := s.Info.Spec.Domain
	// if len(seedFQDN) > 32 {
	// 	return "", fmt.Errorf("Seed cluster's FQDN '%s' must not exceed 32 characters", seedFQDN)
	// }

	return allErrs
}

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

	scope := quotaSpec.Scope
	if scope != garden.QuotaScopeProject && scope != garden.QuotaScopeSecret {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("scope"), scope, []string{string(garden.QuotaScopeProject), string(garden.QuotaScopeSecret)}))
	}

	metricsFldPath := fldPath.Child("metrics")
	for k, v := range quotaSpec.Metrics {
		keyPath := metricsFldPath.Key(string(k))
		allErrs = append(allErrs, ValidateResourceQuantityValue(string(k), v, keyPath)...)
	}

	return allErrs
}

// ValidateResourceQuantityValue validates the value of a resource quantity.
func ValidateResourceQuantityValue(key string, value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if value.Cmp(resource.Quantity{}) < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value.String(), fmt.Sprintf("%s value must not be negative", key)))
	}

	return allErrs
}

// ValidatePrivateSecretBinding validates a PrivateSecretBinding object.
func ValidatePrivateSecretBinding(binding *garden.PrivateSecretBinding) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	return allErrs
}

// ValidatePrivateSecretBindingUpdate validates a PrivateSecretBinding object before an update.
func ValidatePrivateSecretBindingUpdate(newBinding, oldBinding *garden.PrivateSecretBinding) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&newBinding.ObjectMeta, &oldBinding.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidatePrivateSecretBinding(newBinding)...)
	return allErrs
}

// ValidateCrossSecretBinding validates a CrossSecretBinding object.
func ValidateCrossSecretBinding(binding *garden.CrossSecretBinding) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	return allErrs
}

// ValidateCrossSecretBindingUpdate validates a CrossSecretBinding object before an update.
func ValidateCrossSecretBindingUpdate(newBinding, oldBinding *garden.CrossSecretBinding) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&newBinding.ObjectMeta, &oldBinding.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateCrossSecretBinding(newBinding)...)
	return allErrs
}

// ValidateShoot validates a Shoot object.
func ValidateShoot(shoot *garden.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&shoot.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)

	return allErrs
}

// ValidateShootUpdate validates a Shoot object before an update.
func ValidateShootUpdate(newShoot, oldShoot *garden.Shoot) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&newShoot.ObjectMeta, &oldShoot.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateShoot(newShoot)...)
	return allErrs
}

// ValidateShootStatusUpdate validates the status field of a Shoot object.
func ValidateShootStatusUpdate(newShoot, oldShoot *garden.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
