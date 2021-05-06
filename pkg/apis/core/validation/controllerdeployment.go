// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/apis/core"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
