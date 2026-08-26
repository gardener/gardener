// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package imagevector

import (
	"time"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/utils"
)

// ValidateImageVector validates the given ImageVector.
func ValidateImageVector(imageVector ImageVector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, imageSource := range imageVector {
		allErrs = append(allErrs, validateImageSource(imageSource, fldPath.Index(i))...)
	}

	return allErrs
}

// ValidateComponentImageVectors validates the given ComponentImageVectors.
func ValidateComponentImageVectors(componentImageVectors ComponentImageVectors, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for key, value := range componentImageVectors {
		componentImageVector := &ComponentImageVector{
			Name:                 key,
			ImageVectorOverwrite: value,
		}
		allErrs = append(allErrs, validateComponentImageVector(componentImageVector, fldPath.Key(key))...)
	}

	return allErrs
}

func validateImageSource(imageSource *ImageSource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure name and repository are non-empty
	if imageSource.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "image name is required"))
	}
	if imageSource.Ref == nil && imageSource.Repository == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("ref/repository"), "either image ref or repository+tag is required"))
	}

	if imageSource.Ref != nil {
		if *imageSource.Ref == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("ref"), *imageSource.Ref, "ref must not be empty if specified"))
		}
		if imageSource.Repository != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("repository"), "cannot specify repository when ref is set"))
		}
		if imageSource.Tag != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("tag"), "cannot specify tag when ref is set"))
		}
	} else {
		// Ensure tag is non-empty if specified
		if imageSource.Repository != nil && *imageSource.Repository == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("repository"), *imageSource.Repository, "repository must not be empty if specified"))
		}
		if imageSource.Tag != nil && *imageSource.Tag == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("tag"), *imageSource.Tag, "image tag must not be empty if specified"))
		}
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

// ValidateCABundle validates the given CABundle.
func ValidateCABundle(source *CABundle, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if source == nil || source.Inline == nil {
		return allErrs
	}

	if cert, err := utils.DecodeCertificate([]byte(*source.Inline)); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("inline"), *source.Inline, "not a valid PEM-encoded certificate"))
	} else if time.Now().After(cert.NotAfter) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("inline"), *source.Inline, "certificate has expired"))
	}

	return allErrs
}

func validateComponentImageVector(componentImageVector *ComponentImageVector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure name is non-empty
	if componentImageVector.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "component name is required"))
	}

	// Read (and validate) imageVectorOverwrite as image vector
	if _, _, err := Read([]byte(componentImageVector.ImageVectorOverwrite)); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("imageVectorOverwrite"), componentImageVector.ImageVectorOverwrite, err.Error()))
	}

	return allErrs
}
