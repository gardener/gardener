// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strconv"
	"strings"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/features"
)

// ValidateName is a helper function for validating that a name is a DNS sub domain.
func ValidateName(name string, prefix bool) []string {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
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

func validateCrossVersionObjectReference(ref autoscalingv1.CrossVersionObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Kind) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "must provide a kind"))
	}
	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}
	if len(ref.APIVersion) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("apiVersion"), "must provide a apiVersion"))
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

// ValidateDNS1123Subdomain validates that a name is a proper DNS subdomain.
func ValidateDNS1123Subdomain(value string, fldPath *field.Path) field.ErrorList {
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

var availableFailureTolerance = sets.New[string](
	string(core.FailureToleranceTypeNode),
	string(core.FailureToleranceTypeZone),
)

// ValidateFailureToleranceTypeValue validates if the passed value is a valid failureToleranceType.
func ValidateFailureToleranceTypeValue(value core.FailureToleranceType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	failureToleranceType := string(value)
	if !availableFailureTolerance.Has(failureToleranceType) {
		allErrs = append(allErrs, field.NotSupported(fldPath, failureToleranceType, sets.List(availableFailureTolerance)))
	}

	return allErrs
}

// shootReconciliationSuccessful checks if a shoot is successfully reconciled.
// In case it is not, it also returns a descriptive message stating the reason.
func shootReconciliationSuccessful(shoot *core.Shoot) (bool, string) {
	if shoot.Generation != shoot.Status.ObservedGeneration {
		return false, "shoot generation did not equal observed generation"
	}
	if shoot.Status.LastOperation == nil {
		return false, "no last operation present yet"
	}

	if shoot.Status.LastOperation.Type == core.LastOperationTypeCreate ||
		shoot.Status.LastOperation.Type == core.LastOperationTypeReconcile {
		if shoot.Status.LastOperation.State == core.LastOperationStateSucceeded {
			return true, ""
		} else {
			return false, fmt.Sprintf("last operation type was %s but state was not succeeded", shoot.Status.LastOperation.Type)
		}
	}

	return false, fmt.Sprintf("last operation was %s, not Reconcile", shoot.Status.LastOperation.Type)
}

var availableIPFamilies = sets.New[string](
	string(core.IPFamilyIPv4),
	string(core.IPFamilyIPv6),
)

// ValidateIPFamilies validates the given list of IP families for valid values and combinations.
func ValidateIPFamilies(ipFamilies []core.IPFamily, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	ipFamiliesSeen := sets.New[string]()
	for i, ipFamily := range ipFamilies {
		// validate: only supported IP families
		if !availableIPFamilies.Has(string(ipFamily)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Index(i), ipFamily, sets.List(availableIPFamilies)))
		}

		// validate: no duplicate IP families
		if ipFamiliesSeen.Has(string(ipFamily)) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i), ipFamily))
		} else {
			ipFamiliesSeen.Insert(string(ipFamily))
		}
	}

	if len(allErrs) > 0 {
		// further validation doesn't make any sense, because there are unsupported or duplicate IP families
		return allErrs
	}

	// validate: only supported single-stack/dual-stack combinations
	if len(ipFamilies) > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath, ipFamilies, "dual-stack networking is not supported"))
	}
	if len(ipFamilies) > 0 && ipFamilies[0] == core.IPFamilyIPv6 && !features.DefaultFeatureGate.Enabled(features.IPv6SingleStack) {
		allErrs = append(allErrs, field.Invalid(fldPath, ipFamilies, "IPv6 single-stack networking is not supported"))
	}

	return allErrs
}
