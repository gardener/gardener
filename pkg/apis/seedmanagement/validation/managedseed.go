// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletvalidation "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1/validation"
)

var availableManagedSeedOperations = sets.New[string](
	v1beta1constants.GardenerOperationReconcile,
	v1beta1constants.GardenerOperationRenewKubeconfig,
)

// ValidateManagedSeed validates a ManagedSeed object.
func ValidateManagedSeed(managedSeed *seedmanagement.ManagedSeed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&managedSeed.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateOperation(managedSeed.Annotations[v1beta1constants.GardenerOperation], availableManagedSeedOperations, field.NewPath("metadata", "annotations").Key(v1beta1constants.GardenerOperation))...)
	allErrs = append(allErrs, ValidateManagedSeedSpec(&managedSeed.Spec, field.NewPath("spec"), false)...)

	return allErrs
}

// ValidateManagedSeedUpdate validates a ManagedSeed object before an update.
func ValidateManagedSeedUpdate(newManagedSeed, oldManagedSeed *seedmanagement.ManagedSeed) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newManagedSeed.ObjectMeta, &oldManagedSeed.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateOperationUpdate(newManagedSeed.Annotations[v1beta1constants.GardenerOperation], oldManagedSeed.Annotations[v1beta1constants.GardenerOperation], field.NewPath("metadata", "annotations").Key(v1beta1constants.GardenerOperation))...)
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

	allErrs = append(allErrs, validateGardenlet(&spec.Gardenlet, fldPath.Child("gardenlet"), inTemplate)...)

	return allErrs
}

// ValidateManagedSeedSpecUpdate validates the specification updates of a ManagedSeed object.
func ValidateManagedSeedSpecUpdate(newSpec, oldSpec *seedmanagement.ManagedSeedSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure shoot is not changed
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Shoot, oldSpec.Shoot, fldPath.Child("shoot"))...)

	allErrs = append(allErrs, validateGardenletUpdate(&newSpec.Gardenlet, &oldSpec.Gardenlet, fldPath.Child("gardenlet"))...)

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

func validateGardenlet(gardenlet *seedmanagement.GardenletConfig, fldPath *field.Path, inTemplate bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if gardenlet.Deployment != nil {
		allErrs = append(allErrs, ValidateGardenletDeployment(gardenlet.Deployment, fldPath.Child("deployment"))...)
	}

	if gardenlet.Config != nil {
		allErrs = append(allErrs, validateGardenletConfig(gardenlet.Config, ptr.Deref(gardenlet.Bootstrap, seedmanagement.BootstrapNone), ptr.Deref(gardenlet.MergeWithParent, false), fldPath.Child("config"), inTemplate)...)
	}

	if gardenlet.Bootstrap != nil {
		validValues := []string{string(seedmanagement.BootstrapServiceAccount), string(seedmanagement.BootstrapToken), string(seedmanagement.BootstrapNone)}
		if !slices.Contains(validValues, string(*gardenlet.Bootstrap)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("bootstrap"), *gardenlet.Bootstrap, validValues))
		}
	}

	return allErrs
}

func validateGardenletConfig(config runtime.Object, bootstrap seedmanagement.Bootstrap, mergeWithParent bool, fldPath *field.Path, inTemplate bool) field.ErrorList {
	allErrs := field.ErrorList{}

	gardenletConfig, ok := config.(*gardenletconfigv1alpha1.GardenletConfiguration)
	if !ok {
		allErrs = append(allErrs, field.Invalid(fldPath, config, fmt.Sprintf("expected *gardenletconfigv1alpha1.GardenletConfiguration but got %T", config)))
		return allErrs
	}

	// Validate gardenlet config
	allErrs = append(allErrs, validateGardenletConfiguration(gardenletConfig, bootstrap, mergeWithParent, fldPath, inTemplate)...)

	return allErrs
}

func validateGardenletUpdate(newGardenlet, oldGardenlet *seedmanagement.GardenletConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateGardenletConfigUpdate(newGardenlet.Config, oldGardenlet.Config, fldPath.Child("config"))...)

	// Ensure bootstrap is not changed
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newGardenlet.Bootstrap, oldGardenlet.Bootstrap, fldPath.Child("bootstrap"))...)

	// Ensure merge with parent is not changed
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newGardenlet.MergeWithParent, oldGardenlet.MergeWithParent, fldPath.Child("mergeWithParent"))...)

	return allErrs
}

func validateGardenletConfigUpdate(newConfig, oldConfig runtime.Object, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if newConfig != nil && oldConfig != nil {
		// Convert new gardenlet config to an internal version
		newGardenletConfig, ok := newConfig.(*gardenletconfigv1alpha1.GardenletConfiguration)
		if !ok {
			allErrs = append(allErrs, field.Invalid(fldPath, newConfig, fmt.Sprintf("expected *gardenletconfigv1alpha1.GardenletConfiguration but got %T", newConfig)))
			return allErrs
		}

		// Convert old gardenlet config to an internal version
		oldGardenletConfig, ok := oldConfig.(*gardenletconfigv1alpha1.GardenletConfiguration)
		if !ok {
			allErrs = append(allErrs, field.Invalid(fldPath, oldConfig, fmt.Sprintf("expected *gardenletconfigv1alpha1.GardenletConfiguration but got %T", oldConfig)))
			return allErrs
		}

		// Validate gardenlet config update
		allErrs = append(allErrs, validateGardenletConfigurationUpdate(newGardenletConfig, oldGardenletConfig, fldPath)...)
	}

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
		if !slices.Contains(validValues, string(*image.PullPolicy)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("pullPolicy"), *image.PullPolicy, validValues))
		}
	}

	return allErrs
}

func validateGardenletConfiguration(gardenletConfig *gardenletconfigv1alpha1.GardenletConfiguration, bootstrap seedmanagement.Bootstrap, mergeWithParent bool, fldPath *field.Path, inTemplate bool) field.ErrorList {
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

func validateGardenletConfigurationUpdate(newGardenletConfig, oldGardenletConfig *gardenletconfigv1alpha1.GardenletConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, gardenletvalidation.ValidateGardenletConfigurationUpdate(newGardenletConfig, oldGardenletConfig, fldPath)...)

	return allErrs
}

func validateGardenClientConnection(gcc *gardenletconfigv1alpha1.GardenClientConnection, bootstrap seedmanagement.Bootstrap, mergeWithParent bool, fldPath *field.Path) field.ErrorList {
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

func validateOperation(operation string, availableOperations sets.Set[string], fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if operation == "" {
		return allErrs
	}

	if !availableOperations.Has(operation) {
		allErrs = append(allErrs, field.NotSupported(fldPath, operation, sets.List(availableOperations)))
	}

	return allErrs
}

func validateOperationUpdate(newOperation, oldOperation string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if newOperation == "" || oldOperation == "" {
		return allErrs
	}

	if newOperation != oldOperation {
		allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("must not overwrite operation %q with %q", oldOperation, newOperation)))
	}

	return allErrs
}
