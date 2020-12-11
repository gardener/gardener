// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	corevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateManagedSeed validates a ManagedSeed object.
func ValidateManagedSeed(managedSeed *seedmanagement.ManagedSeed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&managedSeed.ObjectMeta, true, corevalidation.ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateManagedSeedSpec(&managedSeed.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateManagedSeedUpdate validates a ManagedSeed object before an update.
func ValidateManagedSeedUpdate(newManagedSeed, oldManagedSeed *seedmanagement.ManagedSeed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newManagedSeed.ObjectMeta, &oldManagedSeed.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateManagedSeedSpecUpdate(&newManagedSeed.Spec, &oldManagedSeed.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateManagedSeed(newManagedSeed)...)

	return allErrs
}

// ValidateManagedSeedSpec validates the specification of a ManagedSeed object.
func ValidateManagedSeedSpec(seedSpec *seedmanagement.ManagedSeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// TODO Add validation logic

	return allErrs
}

// ValidateManagedSeedSpecUpdate validates the specification updates of a ManagedSeed object.
func ValidateManagedSeedSpecUpdate(newManagedSeedSpec, oldManagedSeedSpec *seedmanagement.ManagedSeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// TODO Add validation logic

	return allErrs
}

// ValidateManagedSeedStatusUpdate validates the status field of a ManagedSeed object.
func ValidateManagedSeedStatusUpdate(newManagedSeed, oldManagedSeed *seedmanagement.ManagedSeed) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}
