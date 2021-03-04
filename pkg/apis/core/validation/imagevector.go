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
	"github.com/Masterminds/semver"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

// ValidateImageVector validates the given ImageVector.
func ValidateImageVector(imageVector core.ImageVector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, imageSource := range imageVector {
		allErrs = append(allErrs, validateImageSource(&imageSource, fldPath.Child("images").Index(i))...)
	}

	return allErrs
}

// ValidateComponentImageVectors validates the given ComponentImageVectors.
func ValidateComponentImageVectors(componentImageVectors core.ComponentImageVectors, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, componentImageVector := range componentImageVectors {
		allErrs = append(allErrs, validateComponentImageVector(&componentImageVector, fldPath.Child("components").Index(i))...)
	}

	return allErrs
}

func validateImageSource(imageSource *core.ImageSource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure name and repository are non-empty
	if imageSource.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "image name is required"))
	}
	if imageSource.Repository == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("repository"), "image repository is required"))
	}

	// Ensure tag is non-empty if specified
	if imageSource.Tag != nil && *imageSource.Tag == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("tag"), *imageSource.Tag, "image tag must not be empty if specified "))
	}

	// Ensure runtimeVersion and targetVersion are valid semver constraints if specified
	if imageSource.RuntimeVersion != nil {
		if _, err := semver.NewConstraint(*imageSource.RuntimeVersion); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("runtimeVersion"), imageSource.RuntimeVersion, err.Error()))
		}
	}
	if imageSource.TargetVersion != nil {
		if _, err := semver.NewConstraint(*imageSource.TargetVersion); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("targetVersion"), imageSource.TargetVersion, err.Error()))
		}
	}

	return allErrs
}

func validateComponentImageVector(componentImageVector *core.ComponentImageVector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure name is non-empty
	if componentImageVector.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "component name is required"))
	}

	// Validate image vector
	allErrs = append(allErrs, ValidateImageVector(componentImageVector.ImageVector, fldPath.Child("imageVector"))...)

	return allErrs
}
