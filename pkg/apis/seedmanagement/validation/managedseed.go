// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	gardenletvalidation "github.com/gardener/gardener/pkg/gardenlet/apis/config/validation"
	"github.com/gardener/gardener/pkg/utils"
)

var availableManagedSeedOperations = sets.New(
	v1beta1constants.GardenerOperationReconcile,
	v1beta1constants.GardenerOperationRenewKubeconfig,
)

// ValidateManagedSeed validates a ManagedSeed object.
func ValidateManagedSeed(managedSeed *seedmanagement.ManagedSeed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&managedSeed.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateManagedSeedOperation(managedSeed.Annotations[v1beta1constants.GardenerOperation], field.NewPath("metadata", "annotations").Key(v1beta1constants.GardenerOperation))...)
	allErrs = append(allErrs, ValidateManagedSeedSpec(&managedSeed.Spec, field.NewPath("spec"), false)...)

	return allErrs
}

// ValidateManagedSeedUpdate validates a ManagedSeed object before an update.
func ValidateManagedSeedUpdate(newManagedSeed, oldManagedSeed *seedmanagement.ManagedSeed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newManagedSeed.ObjectMeta, &oldManagedSeed.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateManagedSeedOperationUpdate(newManagedSeed.Annotations[v1beta1constants.GardenerOperation], oldManagedSeed.Annotations[v1beta1constants.GardenerOperation], field.NewPath("metadata", "annotations").Key(v1beta1constants.GardenerOperation))...)
	allErrs = append(allErrs, ValidateManagedSeedSpecUpdate(&newManagedSeed.Spec, &oldManagedSeed.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateManagedSeed(newManagedSeed)...)

	return allErrs
}

// ValidateManagedSeedStatusUpdate validates a ManagedSeed object before a status update.
func ValidateManagedSeedStatusUpdate(newManagedSeed, oldManagedSeed *seedmanagement.ManagedSeed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newManagedSeed.ObjectMeta, &oldManagedSeed.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateManagedSeedStatus(&newManagedSeed.Status, field.NewPath("status"))...)

	return allErrs
}

// ValidateManagedSeedTemplate validates a ManagedSeedTemplate.
func ValidateManagedSeedTemplate(managedSeedTemplate *seedmanagement.ManagedSeedTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, metav1validation.ValidateLabels(managedSeedTemplate.Labels, fldPath.Child("metadata", "labels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(managedSeedTemplate.Annotations, fldPath.Child("metadata", "annotations"))...)
	allErrs = append(allErrs, ValidateManagedSeedSpec(&managedSeedTemplate.Spec, fldPath.Child("spec"), true)...)

	return allErrs
}

// ValidateManagedSeedTemplateUpdate validates a ManagedSeedTemplate before an update.
func ValidateManagedSeedTemplateUpdate(newManagedSeedTemplate, oldManagedSeedTemplate *seedmanagement.ManagedSeedTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateManagedSeedSpecUpdate(&newManagedSeedTemplate.Spec, &oldManagedSeedTemplate.Spec, fldPath.Child("spec"))...)

	return allErrs
}

// ValidateManagedSeedSpec validates the specification of a ManagedSeed object.
func ValidateManagedSeedSpec(spec *seedmanagement.ManagedSeedSpec, fldPath *field.Path, inTemplate bool) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure shoot is specified (if not in template)
	if !inTemplate && spec.Shoot == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("shoot"), "shoot is required"))
	}

	if spec.Shoot != nil {
		allErrs = append(allErrs, validateShoot(spec.Shoot, fldPath.Child("shoot"), inTemplate)...)
	}

	if spec.Gardenlet == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("gardenlet"), "gardenlet is required"))
	} else {
		allErrs = append(allErrs, validateGardenlet(spec.Gardenlet, fldPath.Child("gardenlet"), inTemplate)...)
	}

	return allErrs
}

// ValidateManagedSeedSpecUpdate validates the specification updates of a ManagedSeed object.
func ValidateManagedSeedSpecUpdate(newSpec, oldSpec *seedmanagement.ManagedSeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure shoot is not changed
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Shoot, oldSpec.Shoot, fldPath.Child("shoot"))...)

	if newSpec.Gardenlet != nil && oldSpec.Gardenlet != nil {
		allErrs = append(allErrs, validateGardenletUpdate(newSpec.Gardenlet, oldSpec.Gardenlet, fldPath.Child("gardenlet"))...)
	}

	return allErrs
}

// ValidateManagedSeedStatus validates the given ManagedSeedStatus.
func ValidateManagedSeedStatus(status *seedmanagement.ManagedSeedStatus, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure integer fields are non-negative
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(status.ObservedGeneration, fieldPath.Child("observedGeneration"))...)

	return allErrs
}

func validateShoot(shoot *seedmanagement.Shoot, fldPath *field.Path, inTemplate bool) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure shoot name is specified (if not in template)
	if !inTemplate && shoot.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "shoot name is required"))
	}

	return allErrs
}

func validateGardenlet(gardenlet *seedmanagement.Gardenlet, fldPath *field.Path, inTemplate bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if gardenlet.Deployment != nil {
		allErrs = append(allErrs, ValidateGardenletDeployment(gardenlet.Deployment, fldPath.Child("deployment"))...)
	}

	if gardenlet.Config != nil {
		configPath := fldPath.Child("config")

		// Convert gardenlet config to an internal version
		gardenletConfig, err := gardenlethelper.ConvertGardenletConfiguration(gardenlet.Config)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(configPath, gardenlet.Config, fmt.Sprintf("could not convert gardenlet config: %v", err)))
			return allErrs
		}

		// Validate gardenlet config
		allErrs = append(allErrs, validateGardenletConfiguration(gardenletConfig, helper.GetBootstrap(gardenlet.Bootstrap), ptr.Deref(gardenlet.MergeWithParent, false), configPath, inTemplate)...)
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
		newGardenletConfig, err := gardenlethelper.ConvertGardenletConfiguration(newGardenlet.Config)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(configPath, newGardenlet.Config, fmt.Sprintf("could not convert gardenlet config: %v", err)))
			return allErrs
		}

		// Convert old gardenlet config to an internal version
		oldGardenletConfig, err := gardenlethelper.ConvertGardenletConfiguration(oldGardenlet.Config)
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

// ValidateGardenletDeployment validates the configuration for the gardenlet deployment
func ValidateGardenletDeployment(deployment *seedmanagement.GardenletDeployment, fldPath *field.Path) field.ErrorList {
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

func validateGardenletConfiguration(gardenletConfig *config.GardenletConfiguration, bootstrap seedmanagement.Bootstrap, mergeWithParent bool, fldPath *field.Path, inTemplate bool) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure name is not specified since it will be set by the controller
	if gardenletConfig.SeedConfig != nil && gardenletConfig.SeedConfig.Name != "" {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("seedConfig", "metadata", "name"), "seed name is forbidden"))
	}

	// Validate gardenlet config
	allErrs = append(allErrs, gardenletvalidation.ValidateGardenletConfiguration(gardenletConfig, fldPath, inTemplate)...)

	if gardenletConfig.GardenClientConnection != nil {
		allErrs = append(allErrs, validateGardenClientConnection(gardenletConfig.GardenClientConnection, bootstrap, mergeWithParent, fldPath.Child("gardenClientConnection"))...)
	}

	return allErrs
}

func validateGardenletConfigurationUpdate(newGardenletConfig, oldGardenletConfig *config.GardenletConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, gardenletvalidation.ValidateGardenletConfigurationUpdate(newGardenletConfig, oldGardenletConfig, fldPath)...)

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

func validateManagedSeedOperation(operation string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if operation == "" {
		return allErrs
	}

	if operation != "" && !availableManagedSeedOperations.Has(operation) {
		allErrs = append(allErrs, field.NotSupported(fldPath, operation, sets.List(availableManagedSeedOperations)))
	}

	return allErrs
}

func validateManagedSeedOperationUpdate(newOperation, oldOperation string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if newOperation == "" || oldOperation == "" {
		return allErrs
	}

	if newOperation != oldOperation {
		allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("must not overwrite operation %q with %q", oldOperation, newOperation)))
	}

	return allErrs
}
