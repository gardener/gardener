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

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateName is a helper function for validating that a name is a DNS sub domain.
func ValidateName(name string, prefix bool) []string {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

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
	}

	if spec.OpenStack != nil {
		if len(spec.OpenStack.KeyStoneURL) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("openstack", "keyStoneURL"), "must provide the URL to KeyStone"))
		}
	}

	return allErrs
}

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
