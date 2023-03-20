// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction"
)

// ValidateConfiguration validates the configuration.
func ValidateConfiguration(config *shoottolerationrestriction.Configuration) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validation.ValidateTolerations(config.Defaults, field.NewPath("defaults"))...)
	allErrs = append(allErrs, validation.ValidateTolerations(config.Whitelist, field.NewPath("whitelist"))...)

	return allErrs
}
