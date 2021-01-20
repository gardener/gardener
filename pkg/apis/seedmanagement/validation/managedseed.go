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
	"fmt"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	corevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	configvalidation "github.com/gardener/gardener/pkg/gardenlet/apis/config/validation"
	"github.com/gardener/gardener/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateManagedSeed validates a ManagedSeed object.
func ValidateManagedSeed(managedSeed *seedmanagement.ManagedSeed) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure namespace is garden
	if managedSeed.Namespace != v1beta1constants.GardenNamespace {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "namespace"), managedSeed.Namespace, "namespace must be garden"))
	}

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
func ValidateManagedSeedSpec(spec *seedmanagement.ManagedSeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure shoot name is specified
	if spec.Shoot.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("shoot", "name"), "shoot name is required"))
	}

	// Ensure either seed template or gardenlet is specified
	if (spec.SeedTemplate == nil && spec.Gardenlet == nil) || (spec.SeedTemplate != nil && spec.Gardenlet != nil) {
		allErrs = append(allErrs, field.Invalid(fldPath, spec, "either seed template or gardenlet is required"))
	}

	switch {
	case spec.SeedTemplate != nil:
		allErrs = append(allErrs, validateSeedTemplate(&spec.SeedTemplate.ObjectMeta, &spec.SeedTemplate.Spec, fldPath.Child("seedTemplate"))...)
	case spec.Gardenlet != nil:
		allErrs = append(allErrs, validateGardenlet(spec.Gardenlet, fldPath.Child("gardenlet"))...)
	}

	return allErrs
}

// ValidateManagedSeedSpecUpdate validates the specification updates of a ManagedSeed object.
func ValidateManagedSeedSpecUpdate(newSpec, oldSpec *seedmanagement.ManagedSeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure shoot name is not changed
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Shoot.Name, oldSpec.Shoot.Name, fldPath.Child("shoot", "name"))...)

	// Ensure no changing from seed template to gardenlet or vice versa takes place
	if (newSpec.SeedTemplate != nil) != (oldSpec.SeedTemplate != nil) || (newSpec.Gardenlet != nil) != (oldSpec.Gardenlet != nil) {
		allErrs = append(allErrs, field.Invalid(fldPath, newSpec, "changing from seed template to gardenlet and vice versa is not allowed"))
	}

	switch {
	case newSpec.SeedTemplate != nil && oldSpec.SeedTemplate != nil:
		allErrs = append(allErrs, validateSeedTemplateUpdate(&newSpec.SeedTemplate.ObjectMeta, &oldSpec.SeedTemplate.ObjectMeta, &newSpec.SeedTemplate.Spec, &oldSpec.SeedTemplate.Spec, fldPath.Child("seedTemplate"))...)
	case newSpec.Gardenlet != nil && oldSpec.Gardenlet != nil:
		allErrs = append(allErrs, validateGardenletUpdate(newSpec.Gardenlet, oldSpec.Gardenlet, fldPath.Child("gardenlet"))...)
	}

	return allErrs
}

// ValidateManagedSeedStatusUpdate validates a ManagedSeed object before a status update.
func ValidateManagedSeedStatusUpdate(newManagedSeed, oldManagedSeed *seedmanagement.ManagedSeed) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateSeedTemplate(metadata *metav1.ObjectMeta, seedSpec *gardencore.SeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure name is not specified since it will be set by the controller
	if metadata.Name != "" {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("metadata", "name"), "seed name is forbidden"))
	}

	// Validate labels, annotations, and spec
	allErrs = append(allErrs, metav1validation.ValidateLabels(metadata.Labels, fldPath.Child("metadata", "labels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(metadata.Annotations, fldPath.Child("metadata", "annotations"))...)
	allErrs = append(allErrs, corevalidation.ValidateSeedSpec(seedSpec, fldPath.Child("spec"))...)

	return allErrs
}

func validateSeedTemplateUpdate(_, _ *metav1.ObjectMeta, newSeedSpec, oldSeedSpec *gardencore.SeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, corevalidation.ValidateSeedSpecUpdate(newSeedSpec, oldSeedSpec, fldPath.Child("spec"))...)

	return allErrs
}

func validateGardenlet(gardenlet *seedmanagement.Gardenlet, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if gardenlet.Deployment != nil {
		allErrs = append(allErrs, validateGardenletDeployment(gardenlet.Deployment, fldPath.Child("deployment"))...)
	}

	if gardenlet.Config != nil {
		configPath := fldPath.Child("config")

		// Convert gardenlet config to an internal version
		gardenletConfig, err := helper.ConvertGardenletConfig(gardenlet.Config)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(configPath, gardenlet.Config, fmt.Sprintf("could not convert gardenlet config: %v", err)))
			return allErrs
		}

		// Validate gardenlet config
		allErrs = append(allErrs, validateGardenletConfiguration(gardenletConfig, helper.GetBootstrap(gardenlet.Bootstrap), utils.IsTrue(gardenlet.MergeWithParent), configPath)...)
	}

	if gardenlet.Bootstrap != nil {
		validValues := []string{string(seedmanagement.BootstrapServiceAccount), string(seedmanagement.BootstrapToken), string(seedmanagement.BootstrapNone)}
		if !utils.ValueExists(string(*gardenlet.Bootstrap), validValues) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("bootstrap"), *gardenlet.Bootstrap, validValues))
		}
	}

	return allErrs
}

func validateGardenletUpdate(newGardenlet, oldGardenlet *seedmanagement.Gardenlet, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if newGardenlet.Config != nil && oldGardenlet.Config != nil {
		configPath := fldPath.Child("config")

		// Convert new gardenlet config to an internal version
		newGardenletConfig, err := helper.ConvertGardenletConfig(newGardenlet.Config)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(configPath, newGardenlet.Config, fmt.Sprintf("could not convert gardenlet config: %v", err)))
			return allErrs
		}

		// Convert old gardenlet config to an internal version
		oldGardenletConfig, err := helper.ConvertGardenletConfig(oldGardenlet.Config)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(configPath, oldGardenlet.Config, fmt.Sprintf("could not convert gardenlet config: %v", err)))
			return allErrs
		}

		// Validate gardenlet config update
		allErrs = append(allErrs, validateGardenletConfigurationUpdate(newGardenletConfig, oldGardenletConfig, configPath)...)
	}

	// Ensure bootstrap is not changed
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newGardenlet.Bootstrap, oldGardenlet.Bootstrap, fldPath.Child("bootstrap"))...)

	// Ensure merge with parent is not changed
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newGardenlet.MergeWithParent, oldGardenlet.MergeWithParent, fldPath.Child("mergeWithParent"))...)

	return allErrs
}

func validateGardenletDeployment(deployment *seedmanagement.GardenletDeployment, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deployment.ReplicaCount != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*deployment.ReplicaCount), fldPath.Child("replicaCount"))...)
	}
	if deployment.RevisionHistoryLimit != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*deployment.RevisionHistoryLimit), fldPath.Child("revisionHistoryLimit"))...)
	}
	if deployment.ServiceAccountName != nil {
		for _, msg := range apivalidation.ValidateServiceAccountName(*deployment.ServiceAccountName, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("serviceAccountName"), *deployment.ServiceAccountName, msg))
		}
	}

	if deployment.Image != nil {
		allErrs = append(allErrs, validateImage(deployment.Image, fldPath.Child("image"))...)
	}

	allErrs = append(allErrs, metav1validation.ValidateLabels(deployment.PodLabels, fldPath.Child("podLabels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(deployment.PodAnnotations, fldPath.Child("podAnnotations"))...)

	return allErrs
}

func validateImage(image *seedmanagement.Image, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if image.Repository != nil && *image.Repository == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("repository"), *image.Repository, "repository must not be empty if specified"))
	}
	if image.Tag != nil && *image.Tag == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("tag"), *image.Tag, "tag must not be empty if specified"))
	}
	if image.PullPolicy != nil {
		validValues := []string{string(corev1.PullAlways), string(corev1.PullIfNotPresent), string(corev1.PullNever)}
		if !utils.ValueExists(string(*image.PullPolicy), validValues) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("pullPolicy"), *image.PullPolicy, validValues))
		}
	}

	return allErrs
}

func validateGardenletConfiguration(gardenletConfig *config.GardenletConfiguration, bootstrap seedmanagement.Bootstrap, mergeWithParent bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure seed selector is not specified
	if gardenletConfig.SeedSelector != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("seedSelector"), "seed selector is forbidden"))
	}

	// Validate gardenlet config
	allErrs = append(allErrs, configvalidation.ValidateGardenletConfiguration(gardenletConfig, fldPath)...)

	if gardenletConfig.SeedConfig != nil {
		seedConfigPath := fldPath.Child("seedConfig")

		// Convert seed to internal version
		seed, err := helper.ConvertSeed(&gardenletConfig.SeedConfig.Seed)
		if err != nil {
			allErrs = append(allErrs, field.InternalError(seedConfigPath, fmt.Errorf("could not convert seed config: %v", err)))
			return allErrs
		}

		// Validate seed
		allErrs = append(allErrs, validateSeedTemplate(&seed.ObjectMeta, &seed.Spec, seedConfigPath)...)
	}

	if gardenletConfig.GardenClientConnection != nil {
		allErrs = append(allErrs, validateGardenClientConnection(gardenletConfig.GardenClientConnection, bootstrap, mergeWithParent, fldPath.Child("gardenClientConnection"))...)
	}

	return allErrs
}

func validateGardenletConfigurationUpdate(newGardenletConfig, oldGardenletConfig *config.GardenletConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if newGardenletConfig.SeedConfig != nil && oldGardenletConfig != nil {
		seedConfigPath := fldPath.Child("seedConfig")

		// Convert new seed to internal version
		newSeed, err := helper.ConvertSeed(&newGardenletConfig.SeedConfig.Seed)
		if err != nil {
			allErrs = append(allErrs, field.InternalError(seedConfigPath, fmt.Errorf("could not convert seed config: %v", err)))
			return allErrs
		}

		// Convert old seed to internal version
		oldSeed, err := helper.ConvertSeed(&oldGardenletConfig.SeedConfig.Seed)
		if err != nil {
			allErrs = append(allErrs, field.InternalError(seedConfigPath, fmt.Errorf("could not convert seed config: %v", err)))
			return allErrs
		}

		// Validate seed
		allErrs = append(allErrs, validateSeedTemplateUpdate(&newSeed.ObjectMeta, &oldSeed.ObjectMeta, &newSeed.Spec, &oldSeed.Spec, seedConfigPath)...)
	}

	return allErrs
}

func validateGardenClientConnection(gcc *config.GardenClientConnection, bootstrap seedmanagement.Bootstrap, mergeWithParent bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch bootstrap {
	case seedmanagement.BootstrapServiceAccount, seedmanagement.BootstrapToken:
		if gcc.Kubeconfig != "" {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("kubeconfig"), "kubeconfig is forbidden if bootstrap is specified"))
		}
	case seedmanagement.BootstrapNone:
		if gcc.Kubeconfig == "" && !mergeWithParent {
			allErrs = append(allErrs, field.Required(fldPath.Child("kubeconfig"), "kubeconfig is required if bootstrap is not specified and merging with parent is disabled"))
		}
		if gcc.BootstrapKubeconfig != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("bootstrapKubeconfig"), "bootstrap kubeconfig is forbidden if bootstrap is not specified"))
		}
		if gcc.KubeconfigSecret != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("kubeconfigSecret"), "kubeconfig secret is forbidden if bootstrap is not specified"))
		}
	}

	return allErrs
}
