// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

// ValidateControllerDeployment validates a ControllerDeployment object.
func ValidateControllerDeployment(controllerDeployment *core.ControllerDeployment) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&controllerDeployment.ObjectMeta, false, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)

	if len(controllerDeployment.Type) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("type"), "must provide a type"))
	}

	return allErrs
}

// ValidateControllerDeploymentUpdate validates a ControllerDeployment object before an update.
func ValidateControllerDeploymentUpdate(new, _ *core.ControllerDeployment) field.ErrorList {
	return ValidateControllerDeployment(new)
}
