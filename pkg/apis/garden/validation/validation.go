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
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/robfig/cron"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation"
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
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newProfile.ObjectMeta, &oldProfile.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateCloudProfile(newProfile)...)

	return allErrs
}

// ValidateCloudProfileSpec validates the specification of a CloudProfile object.
func ValidateCloudProfileSpec(spec *garden.CloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if _, err := helper.DetermineCloudProviderInProfile(*spec); err != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("aws/azure/gcp/openstack/local"), "cloud profile must only contain exactly one field of aws/azure/gcp/openstack"))
		return allErrs
	}

	if spec.AWS != nil {
		allErrs = append(allErrs, validateDNSProviders(spec.AWS.Constraints.DNSProviders, fldPath.Child("aws", "constraints", "dnsProviders"))...)
		allErrs = append(allErrs, validateKubernetesConstraints(spec.AWS.Constraints.Kubernetes, fldPath.Child("aws", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateAWSMachineImages(spec.AWS.Constraints.MachineImages, fldPath.Child("aws", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.AWS.Constraints.MachineTypes, fldPath.Child("aws", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypeConstraints(spec.AWS.Constraints.VolumeTypes, fldPath.Child("aws", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.AWS.Constraints.Zones, fldPath.Child("aws", "constraints", "zones"))...)
	}

	if spec.Azure != nil {
		allErrs = append(allErrs, validateDNSProviders(spec.Azure.Constraints.DNSProviders, fldPath.Child("azure", "constraints", "dnsProviders"))...)
		allErrs = append(allErrs, validateKubernetesConstraints(spec.Azure.Constraints.Kubernetes, fldPath.Child("azure", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateAzureMachineImages(spec.Azure.Constraints.MachineImages, fldPath.Child("azure", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.Azure.Constraints.MachineTypes, fldPath.Child("azure", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypeConstraints(spec.Azure.Constraints.VolumeTypes, fldPath.Child("azure", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateAzureDomainCount(spec.Azure.CountFaultDomains, fldPath.Child("azure", "countFaultDomains"))...)
		allErrs = append(allErrs, validateAzureDomainCount(spec.Azure.CountUpdateDomains, fldPath.Child("azure", "countUpdateDomains"))...)
	}

	if spec.GCP != nil {
		allErrs = append(allErrs, validateDNSProviders(spec.GCP.Constraints.DNSProviders, fldPath.Child("gcp", "constraints", "dnsProviders"))...)
		allErrs = append(allErrs, validateKubernetesConstraints(spec.GCP.Constraints.Kubernetes, fldPath.Child("gcp", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateGCPMachineImages(spec.GCP.Constraints.MachineImages, fldPath.Child("gcp", "constraints", "machineImages"))...)
		allErrs = append(allErrs, validateMachineTypeConstraints(spec.GCP.Constraints.MachineTypes, fldPath.Child("gcp", "constraints", "machineTypes"))...)
		allErrs = append(allErrs, validateVolumeTypeConstraints(spec.GCP.Constraints.VolumeTypes, fldPath.Child("gcp", "constraints", "volumeTypes"))...)
		allErrs = append(allErrs, validateZones(spec.GCP.Constraints.Zones, fldPath.Child("gcp", "constraints", "zones"))...)
	}

	if spec.OpenStack != nil {
		allErrs = append(allErrs, validateDNSProviders(spec.OpenStack.Constraints.DNSProviders, fldPath.Child("openstack", "constraints", "dnsProviders"))...)
		allErrs = append(allErrs, validateKubernetesConstraints(spec.OpenStack.Constraints.Kubernetes, fldPath.Child("openstack", "constraints", "kubernetes"))...)
		allErrs = append(allErrs, validateOpenStackMachineImages(spec.OpenStack.Constraints.MachineImages, fldPath.Child("openstack", "constraints", "machineImages"))...)
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
	}

	if spec.CABundle != nil {
		_, err := utils.DecodeCertificate([]byte(*(spec.CABundle)))
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("caBundle"), *(spec.CABundle), "caBundle is not a valid PEM-encoded certificate"))
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
		if provider.Name != garden.DNSUnmanaged && provider.Name != garden.DNSAWSRoute53 && provider.Name != garden.DNSGoogleCloudDNS {
			allErrs = append(allErrs, field.NotSupported(idxPath, provider.Name, []string{string(garden.DNSUnmanaged), string(garden.DNSAWSRoute53), string(garden.DNSGoogleCloudDNS)}))
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

	for i, machineType := range machineTypes {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		cpuPath := idxPath.Child("cpu")
		gpuPath := idxPath.Child("gpu")
		memoryPath := idxPath.Child("memory")

		if len(machineType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		}
		allErrs = append(allErrs, validateResourceQuantityValue("cpu", machineType.CPU, cpuPath)...)
		allErrs = append(allErrs, validateResourceQuantityValue("gpu", machineType.GPU, gpuPath)...)
		allErrs = append(allErrs, validateResourceQuantityValue("memory", machineType.Memory, memoryPath)...)
	}

	return allErrs
}

func validateMachineImageNames(names []garden.MachineImageName, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(names) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
		return allErrs
	}

	imageNames := map[garden.MachineImageName]bool{}
	for i, name := range names {
		idxPath := fldPath.Index(i)

		if imageNames[name] {
			allErrs = append(allErrs, field.Duplicate(idxPath, name))
		}
		imageNames[name] = true

		if name != garden.MachineImageCoreOS {
			allErrs = append(allErrs, field.NotSupported(idxPath, name, []string{string(garden.MachineImageCoreOS)}))
		}
	}

	return allErrs
}

func validateAWSMachineImages(machineImages []garden.AWSMachineImageMapping, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	machineImageNames := []garden.MachineImageName{}
	r, _ := regexp.Compile(`^ami-[a-z0-9]+$`)

	for i, image := range machineImages {
		machineImageNames = append(machineImageNames, image.Name)
		idxPath := fldPath.Index(i)

		if len(image.Regions) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("regions"), "must provide at least one region per machine image"))
		}

		regionNames := map[string]bool{}
		for j, region := range image.Regions {
			regionIdxPath := idxPath.Child("regions").Index(j)

			if regionNames[region.Name] {
				allErrs = append(allErrs, field.Duplicate(regionIdxPath, region.Name))
			}
			regionNames[region.Name] = true

			if !r.MatchString(region.AMI) {
				allErrs = append(allErrs, field.Invalid(regionIdxPath.Child("ami"), region.AMI, fmt.Sprintf("ami's must match the regex %s", r)))
			}
		}
	}

	allErrs = append(allErrs, validateMachineImageNames(machineImageNames, fldPath)...)
	return allErrs
}

func validateAzureMachineImages(machineImages []garden.AzureMachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	machineImageNames := []garden.MachineImageName{}
	for i, image := range machineImages {
		machineImageNames = append(machineImageNames, image.Name)
		idxPath := fldPath.Index(i)

		if len(image.Publisher) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("publisher"), image.Publisher))
		}
		if len(image.Offer) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("offer"), image.Offer))
		}
		if len(image.SKU) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("sku"), image.SKU))
		}
		if len(image.Version) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("version"), image.Version))
		}
	}

	allErrs = append(allErrs, validateMachineImageNames(machineImageNames, fldPath)...)
	return allErrs
}

func validateGCPMachineImages(machineImages []garden.GCPMachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	machineImageNames := []garden.MachineImageName{}
	for i, image := range machineImages {
		machineImageNames = append(machineImageNames, image.Name)
		idxPath := fldPath.Index(i)

		if len(image.Image) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("image"), image.Image))
		}
	}

	allErrs = append(allErrs, validateMachineImageNames(machineImageNames, fldPath)...)
	return allErrs
}

func validateOpenStackMachineImages(machineImages []garden.OpenStackMachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	machineImageNames := []garden.MachineImageName{}
	for i, image := range machineImages {
		machineImageNames = append(machineImageNames, image.Name)
		idxPath := fldPath.Index(i)

		if len(image.Image) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("image"), image.Image))
		}
	}

	allErrs = append(allErrs, validateMachineImageNames(machineImageNames, fldPath)...)
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
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newSeed.ObjectMeta, &oldSeed.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSeed.Spec.Networks, newSeed.Spec.Networks, field.NewPath("spec", "networks"))...)
	allErrs = append(allErrs, ValidateSeed(newSeed)...)

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
		allErrs = append(allErrs, field.Required(cloudPath.Child("region"), "must provide a region"))
	}

	r, _ := regexp.Compile(`^(?:[a-zA-Z0-9\-]+\.)*[a-zA-Z0-9]+\.[a-zA-Z0-9]{2,6}$`)
	if !r.MatchString(seedSpec.IngressDomain) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ingressDomain"), seedSpec.IngressDomain, fmt.Sprintf("domain must match the regex %s", r)))
	}

	allErrs = append(allErrs, validateSecretReference(seedSpec.SecretRef, fldPath.Child("secretRef"))...)

	networksPath := fldPath.Child("networks")
	allErrs = append(allErrs, validateCIDR(seedSpec.Networks.Nodes, networksPath.Child("nodes"))...)
	allErrs = append(allErrs, validateCIDR(seedSpec.Networks.Pods, networksPath.Child("pods"))...)
	allErrs = append(allErrs, validateCIDR(seedSpec.Networks.Services, networksPath.Child("services"))...)

	return allErrs
}

func validateCIDR(cidr garden.CIDR, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if _, _, err := net.ParseCIDR(string(cidr)); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, cidr, err.Error()))
	}

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

	scope := quotaSpec.Scope
	if scope != garden.QuotaScopeProject && scope != garden.QuotaScopeSecret {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("scope"), scope, []string{string(garden.QuotaScopeProject), string(garden.QuotaScopeSecret)}))
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
	allErrs = append(allErrs, ValidateSecretBinding(newBinding)...)

	return allErrs
}

func validateLocalObjectReference(ref corev1.LocalObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	return allErrs
}

func validateObjectReference(ref corev1.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}
	if len(ref.Namespace) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "must provide a namespace"))
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

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&shoot.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateShootName(shoot.Name, field.NewPath("metadata", "name"))...)
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
	provider, err := helper.DetermineCloudProviderInShoot(spec.Cloud)
	if err != nil {
		allErrs = append(allErrs, field.Forbidden(cloudPath.Child("aws/azure/gcp/openstack/local"), "cloud section must only contain exactly one field of aws/azure/gcp/openstack/local"))
		return allErrs
	}

	allErrs = append(allErrs, validateAddons(spec.Addons, fldPath.Child("addons"))...)
	allErrs = append(allErrs, validateBackup(spec.Backup, provider, fldPath.Child("backup"))...)
	allErrs = append(allErrs, validateCloud(spec.Cloud, fldPath.Child("cloud"))...)
	allErrs = append(allErrs, validateDNS(spec.DNS, fldPath.Child("dns"))...)
	allErrs = append(allErrs, validateKubernetes(spec.Kubernetes, fldPath.Child("kubernetes"))...)
	allErrs = append(allErrs, validateMaintenance(spec.Maintenance, fldPath.Child("maintenance"))...)

	if spec.DNS.Provider == garden.DNSUnmanaged {
		if spec.Addons != nil && spec.Addons.Monocular != nil && spec.Addons.Monocular.Enabled {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("addons", "monocular", "enabled"), spec.Addons.Monocular.Enabled, fmt.Sprintf("`.spec.addons.monocular.enabled` must be false when `.spec.dns.provider` is '%s'", garden.DNSUnmanaged)))
		}
		if spec.DNS.HostedZoneID != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("dns", "hostedZoneID"), spec.DNS.HostedZoneID, fmt.Sprintf("`.spec.dns.hostedZoneID` must not be set when `.spec.dns.provider` is '%s'", garden.DNSUnmanaged)))
		}
		if spec.DNS.SecretName != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("dns", "secretName"), spec.DNS.SecretName, fmt.Sprintf("`.spec.dns.secretName` must not be set when `.spec.dns.provider` is '%s'", garden.DNSUnmanaged)))
		}
	}

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

func validateShootName(name string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if strings.Contains(name, "--") {
		allErrs = append(allErrs, field.Invalid(fldPath, name, "shoot name may not contain two consecutive hyphens"))
	}

	return allErrs
}

func validateAddons(addons *garden.Addons, fldPath *field.Path) field.ErrorList {
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

	return allErrs
}

func validateBackup(backup *garden.Backup, cloudProvider garden.CloudProvider, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if backup == nil {
		return allErrs
	}
	if _, err := cron.ParseStandard(backup.Schedule); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("schedule"), backup.Schedule, "schedule must be in standard cron format"))
	}
	if backup.Maximum <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maximum"), backup.Maximum, "maximum number must be greater than zero"))
	}

	return allErrs
}

func validateCloud(cloud garden.Cloud, fldPath *field.Path) field.ErrorList {
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

	aws := cloud.AWS
	awsPath := fldPath.Child("aws")
	if aws != nil {
		zoneCount := len(aws.Zones)
		if zoneCount == 0 {
			allErrs = append(allErrs, field.Required(awsPath.Child("zones"), "must specify at least one zone"))
			return allErrs
		}

		allErrs = append(allErrs, validateK8SNetworks(aws.Networks.K8SNetworks, awsPath.Child("networks"))...)

		if len(aws.Networks.Internal) != zoneCount {
			allErrs = append(allErrs, field.Invalid(awsPath.Child("networks", "internal"), aws.Networks.Internal, "must specify as many internal networks as zones"))
		}
		for i, cidr := range aws.Networks.Internal {
			allErrs = append(allErrs, validateCIDR(cidr, awsPath.Child("networks", "internal").Index(i))...)
		}

		if len(aws.Networks.Public) != zoneCount {
			allErrs = append(allErrs, field.Invalid(awsPath.Child("networks", "public"), aws.Networks.Public, "must specify as many public networks as zones"))
		}
		for i, cidr := range aws.Networks.Public {
			allErrs = append(allErrs, validateCIDR(cidr, awsPath.Child("networks", "public").Index(i))...)
		}

		if len(aws.Networks.Workers) != zoneCount {
			allErrs = append(allErrs, field.Invalid(awsPath.Child("networks", "workers"), aws.Networks.Workers, "must specify as many workers networks as zones"))
		}
		for i, cidr := range aws.Networks.Workers {
			allErrs = append(allErrs, validateCIDR(cidr, awsPath.Child("networks", "workers").Index(i))...)
		}

		if (aws.Networks.VPC.ID == nil && aws.Networks.VPC.CIDR == nil) || (aws.Networks.VPC.ID != nil && aws.Networks.VPC.CIDR != nil) {
			allErrs = append(allErrs, field.Invalid(awsPath.Child("networks", "vpc"), aws.Networks.VPC, "must specify either a vpc id or a cidr"))
		} else if aws.Networks.VPC.CIDR != nil && aws.Networks.VPC.ID == nil {
			allErrs = append(allErrs, validateCIDR(*(aws.Networks.VPC.CIDR), awsPath.Child("networks", "vpc", "cidr"))...)
		}

		if len(aws.Workers) == 0 {
			allErrs = append(allErrs, field.Required(awsPath.Child("workers"), "must specify at least one worker"))
			return allErrs
		}
		for i, worker := range aws.Workers {
			idxPath := awsPath.Child("workers").Index(i)
			allErrs = append(allErrs, validateWorker(worker.Worker, idxPath)...)
			allErrs = append(allErrs, validateWorkerVolumeSize(worker.VolumeSize, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerMinimumVolumeSize(worker.VolumeSize, 20, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerVolumeType(worker.VolumeType, idxPath.Child("volumeType"))...)
			if workerNames[worker.Name] {
				allErrs = append(allErrs, field.Duplicate(idxPath, worker.Name))
			}
			workerNames[worker.Name] = true
		}
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
		if azure.Networks.VNet.Name != nil {
			allErrs = append(allErrs, field.Invalid(azurePath.Child("networks", "vnet", "name"), *(azure.Networks.VNet.Name), "specifying an existing vnet is not supported yet"))
		} else {
			if azure.Networks.VNet.CIDR == nil {
				allErrs = append(allErrs, field.Required(azurePath.Child("networks", "vnet", "cidr"), "must specify a vnet cidr"))
			} else {
				allErrs = append(allErrs, validateCIDR(*(azure.Networks.VNet.CIDR), azurePath.Child("networks", "vnet", "cidr"))...)
			}
		}

		// TODO: re-enable once deployment into existing resource group works properly.
		// if azure.ResourceGroup != nil && len(azure.ResourceGroup.Name) == 0 {
		// 	allErrs = append(allErrs, field.Invalid(azurePath.Child("resourceGroup", "name"), azure.ResourceGroup.Name, "resource group name must not be empty when resource group key is provided"))
		// }

		allErrs = append(allErrs, validateK8SNetworks(azure.Networks.K8SNetworks, azurePath.Child("networks"))...)

		allErrs = append(allErrs, validateCIDR(azure.Networks.Workers, azurePath.Child("networks", "workers"))...)

		// TODO: re-enable once deployment into existing vnet works properly.
		// if (azure.Networks.VNet.Name == nil && azure.Networks.VNet.CIDR == nil) || (azure.Networks.VNet.Name != nil && azure.Networks.VNet.CIDR != nil) {
		// 	allErrs = append(allErrs, field.Invalid(azurePath.Child("networks", "vnet"), azure.Networks.VNet, "must specify either a vnet name or a cidr"))
		// } else if azure.Networks.VNet.CIDR != nil && azure.Networks.VNet.Name == nil {
		// 	allErrs = append(allErrs, validateCIDR(*(azure.Networks.VNet.CIDR), azurePath.Child("networks", "vnet", "cidr"))...)
		// }

		if len(azure.Workers) == 0 {
			allErrs = append(allErrs, field.Required(azurePath.Child("workers"), "must specify at least one worker"))
			return allErrs
		}
		for i, worker := range azure.Workers {
			idxPath := azurePath.Child("workers").Index(i)
			allErrs = append(allErrs, validateWorker(worker.Worker, idxPath)...)
			allErrs = append(allErrs, validateWorkerVolumeSize(worker.VolumeSize, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerMinimumVolumeSize(worker.VolumeSize, 35, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerVolumeType(worker.VolumeType, idxPath.Child("volumeType"))...)
			if worker.AutoScalerMax != worker.AutoScalerMin {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("autoScalerMax"), "maximum value must be equal to minimum value"))
			}
			if workerNames[worker.Name] {
				allErrs = append(allErrs, field.Duplicate(idxPath, worker.Name))
			}
			workerNames[worker.Name] = true
		}
	}

	gcp := cloud.GCP
	gcpPath := fldPath.Child("gcp")
	if gcp != nil {
		zoneCount := len(gcp.Zones)
		if zoneCount == 0 {
			allErrs = append(allErrs, field.Required(gcpPath.Child("zones"), "must specify at least one zone"))
			return allErrs
		}

		allErrs = append(allErrs, validateK8SNetworks(gcp.Networks.K8SNetworks, gcpPath.Child("networks"))...)

		if len(gcp.Networks.Workers) != zoneCount {
			allErrs = append(allErrs, field.Invalid(gcpPath.Child("networks", "workers"), gcp.Networks.Workers, "must specify as many workers networks as zones"))
		}
		for i, cidr := range gcp.Networks.Workers {
			allErrs = append(allErrs, validateCIDR(cidr, gcpPath.Child("networks", "workers").Index(i))...)
		}

		if gcp.Networks.VPC != nil && len(gcp.Networks.VPC.Name) == 0 {
			allErrs = append(allErrs, field.Invalid(gcpPath.Child("networks", "vpc", "name"), gcp.Networks.VPC.Name, "vpc name must not be empty when vpc key is provided"))
		}

		if len(gcp.Workers) == 0 {
			allErrs = append(allErrs, field.Required(gcpPath.Child("workers"), "must specify at least one worker"))
			return allErrs
		}
		for i, worker := range gcp.Workers {
			idxPath := gcpPath.Child("workers").Index(i)
			allErrs = append(allErrs, validateWorker(worker.Worker, idxPath)...)
			allErrs = append(allErrs, validateWorkerVolumeSize(worker.VolumeSize, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerMinimumVolumeSize(worker.VolumeSize, 20, idxPath.Child("volumeSize"))...)
			allErrs = append(allErrs, validateWorkerVolumeType(worker.VolumeType, idxPath.Child("volumeType"))...)
			if workerNames[worker.Name] {
				allErrs = append(allErrs, field.Duplicate(idxPath, worker.Name))
			}
			workerNames[worker.Name] = true
		}
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

		allErrs = append(allErrs, validateK8SNetworks(openStack.Networks.K8SNetworks, openStackPath.Child("networks"))...)

		if len(openStack.Networks.Workers) != zoneCount {
			allErrs = append(allErrs, field.Invalid(openStackPath.Child("networks", "workers"), openStack.Networks.Workers, "must specify as many workers networks as zones"))
		}
		for i, cidr := range openStack.Networks.Workers {
			allErrs = append(allErrs, validateCIDR(cidr, openStackPath.Child("networks", "workers").Index(i))...)
		}

		if openStack.Networks.Router != nil && len(openStack.Networks.Router.ID) == 0 {
			allErrs = append(allErrs, field.Invalid(openStackPath.Child("networks", "router", "id"), openStack.Networks.Router.ID, "router id must not be empty when router key is provided"))
		}

		if len(openStack.Workers) == 0 {
			allErrs = append(allErrs, field.Required(openStackPath.Child("workers"), "must specify at least one worker"))
			return allErrs
		}
		for i, worker := range openStack.Workers {
			idxPath := openStackPath.Child("workers").Index(i)
			allErrs = append(allErrs, validateWorker(worker.Worker, idxPath)...)
			if worker.AutoScalerMax != worker.AutoScalerMin {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("autoScalerMax"), "maximum value must be equal to minimum value"))
			}
			if workerNames[worker.Name] {
				allErrs = append(allErrs, field.Duplicate(idxPath, worker.Name))
			}
			workerNames[worker.Name] = true
		}
	}

	return allErrs
}

// ValidateShootSpecUpdate validates the specification of a Shoot object.
func ValidateShootSpecUpdate(newSpec, oldSpec *garden.ShootSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec, oldSpec, fldPath)...)
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Profile, oldSpec.Cloud.Profile, fldPath.Child("cloud", "profile"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Region, oldSpec.Cloud.Region, fldPath.Child("cloud", "region"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Seed, oldSpec.Cloud.Seed, fldPath.Child("cloud", "seed"))...)

	awsPath := fldPath.Child("cloud", "aws")
	if oldSpec.Cloud.AWS != nil && newSpec.Cloud.AWS == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.AWS, oldSpec.Cloud.AWS, awsPath)...)
		return allErrs
	} else if newSpec.Cloud.AWS != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.AWS.Networks, oldSpec.Cloud.AWS.Networks, awsPath.Child("networks"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.AWS.Zones, oldSpec.Cloud.AWS.Zones, awsPath.Child("zones"))...)
	}

	azurePath := fldPath.Child("cloud", "azure")
	if oldSpec.Cloud.Azure != nil && newSpec.Cloud.Azure == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Azure, oldSpec.Cloud.Azure, azurePath)...)
		return allErrs
	} else if newSpec.Cloud.Azure != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Azure.ResourceGroup, oldSpec.Cloud.Azure.ResourceGroup, azurePath.Child("resourceGroup"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.Azure.Networks, oldSpec.Cloud.Azure.Networks, azurePath.Child("networks"))...)
	}

	gcpPath := fldPath.Child("cloud", "gcp")
	if oldSpec.Cloud.GCP != nil && newSpec.Cloud.GCP == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.GCP, oldSpec.Cloud.GCP, gcpPath)...)
		return allErrs
	} else if newSpec.Cloud.GCP != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.GCP.Networks, oldSpec.Cloud.GCP.Networks, gcpPath.Child("networks"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.GCP.Zones, oldSpec.Cloud.GCP.Zones, gcpPath.Child("zones"))...)
	}

	openStackPath := fldPath.Child("cloud", "openstack")
	if oldSpec.Cloud.OpenStack != nil && newSpec.Cloud.OpenStack == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.OpenStack, oldSpec.Cloud.OpenStack, openStackPath)...)
		return allErrs
	} else if newSpec.Cloud.OpenStack != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.OpenStack.Networks, oldSpec.Cloud.OpenStack.Networks, openStackPath.Child("networks"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Cloud.OpenStack.Zones, oldSpec.Cloud.OpenStack.Zones, openStackPath.Child("zones"))...)
	}

	allErrs = append(allErrs, validateDNSUpdate(newSpec.DNS, oldSpec.DNS, fldPath.Child("dns"))...)
	allErrs = append(allErrs, validateKubernetesVersionUpdate(newSpec.Kubernetes.Version, oldSpec.Kubernetes.Version, fldPath.Child("kubernetes", "version"))...)

	return allErrs
}

func validateDNSUpdate(new, old garden.DNS, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if new.Provider != old.Provider {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Provider, old.Provider, fldPath.Child("provider"))...)
	}
	if new.HostedZoneID != old.HostedZoneID {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.HostedZoneID, old.HostedZoneID, fldPath.Child("hostedZoneID"))...)
	}
	if new.Domain != old.Domain {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Domain, old.Domain, fldPath.Child("domain"))...)
	}

	return allErrs
}

func validateKubernetesVersionUpdate(new, old string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

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

	if dns.Provider != garden.DNSUnmanaged && dns.Provider != garden.DNSAWSRoute53 && dns.Provider != garden.DNSGoogleCloudDNS {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("provider"), dns.Provider, []string{string(garden.DNSUnmanaged), string(garden.DNSAWSRoute53), string(garden.DNSGoogleCloudDNS)}))
	}

	if dns.HostedZoneID != nil {
		if len(*dns.HostedZoneID) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("hostedZoneID"), dns.HostedZoneID, "hosted zone id cannot be empty when key is provided"))
		}
	}

	if dns.Domain == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("domain"), "domain cannot be empty"))
	} else {
		allErrs = append(allErrs, validateDNS1123Subdomain(*dns.Domain, fldPath.Child("domain"))...)
	}

	return allErrs
}

func validateK8SNetworks(networks garden.K8SNetworks, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networks.Nodes != nil {
		allErrs = append(allErrs, validateCIDR(*networks.Nodes, fldPath.Child("nodes"))...)
	} else {
		allErrs = append(allErrs, field.Required(fldPath.Child("nodes"), "nodes CIDR cannot be unset"))
	}

	if networks.Pods != nil {
		allErrs = append(allErrs, validateCIDR(*networks.Pods, fldPath.Child("pods"))...)
	} else {
		allErrs = append(allErrs, field.Required(fldPath.Child("pods"), "pods CIDR cannot be unset"))
	}

	if networks.Services != nil {
		allErrs = append(allErrs, validateCIDR(*networks.Services, fldPath.Child("services"))...)
	} else {
		allErrs = append(allErrs, field.Required(fldPath.Child("services"), "services CIDR cannot be unset"))
	}

	return allErrs
}

func validateKubernetes(kubernetes garden.Kubernetes, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	kubeAPIServer := kubernetes.KubeAPIServer
	if kubeAPIServer != nil {
		oidc := kubeAPIServer.OIDCConfig
		if oidc != nil {
			oidcPath := fldPath.Child("kubeAPIServer", "oidcConfig")

			_, err := utils.DecodeCertificate([]byte(*oidc.CABundle))
			if err != nil {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("caBundle"), oidc.CABundle, "caBundle is not a valid PEM-encoded certificate"))
			}
			if len(*oidc.ClientID) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("clientID"), oidc.ClientID, "client id cannot be empty when key is provided"))
			}
			if len(*oidc.GroupsClaim) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("groupsClaim"), oidc.GroupsClaim, "groups claim cannot be empty when key is provided"))
			}
			if len(*oidc.GroupsPrefix) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("groupsPrefix"), oidc.GroupsPrefix, "groups prefix cannot be empty when key is provided"))
			}
			if len(*oidc.IssuerURL) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("issuerURL"), oidc.IssuerURL, "issuer url cannot be empty when key is provided"))
			}
			if len(*oidc.UsernameClaim) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("usernameClaim"), oidc.UsernameClaim, "username claim cannot be empty when key is provided"))
			}
			if len(*oidc.UsernamePrefix) == 0 {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("usernamePrefix"), oidc.UsernamePrefix, "username prefix cannot be empty when key is provided"))
			}
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
		begin, beginErr := utils.ParseMaintenanceTime(maintenance.TimeWindow.Begin)
		if beginErr != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("timeWindow", "begin"), maintenance.TimeWindow.Begin, "time window begin is not in the correct format (HHMMSS+ZONE)"))
		}

		end, endErr := utils.ParseMaintenanceTime(maintenance.TimeWindow.End)
		if endErr != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("timeWindow", "end"), maintenance.TimeWindow.End, "time window end is not in the correct format (HHMMSS+ZONE)"))
		}

		if end.Sub(begin) < 0 {
			end = end.Add(24 * time.Hour)
		}

		if beginErr == nil && endErr == nil {
			if end.Sub(begin) < 0 {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("timeWindow"), "time window end must not be before time window begin"))
				return allErrs
			}
			if end.Sub(begin) > 6*time.Hour {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("timeWindow"), "time window must not be greater than 6 hours"))
				return allErrs
			}
			if end.Sub(begin) < 30*time.Minute {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("timeWindow"), "time window must not be smaller than 30 minutes"))
				return allErrs
			}
		}
	}

	return allErrs
}

func validateWorker(worker garden.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDNS1123Subdomain(worker.Name, fldPath.Child("name"))...)
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
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Seed, oldSpec.Seed, fldPath.Child("shootUID"))...)
	return allErrs
}

// ValidateBackupInfrastructureStatusUpdate validates the status field of a BackupInfrastructure object.
func ValidateBackupInfrastructureStatusUpdate(newBackupInfrastructure, oldBackupInfrastructure *garden.BackupInfrastructure) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
