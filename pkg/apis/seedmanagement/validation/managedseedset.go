// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// ValidateManagedSeedSet validates a ManagedSeedSet object.
func ValidateManagedSeedSet(ManagedSeedSet *seedmanagement.ManagedSeedSet) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure namespace is garden
	if ManagedSeedSet.Namespace != v1beta1constants.GardenNamespace {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "namespace"), ManagedSeedSet.Namespace, "namespace must be garden"))
	}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&ManagedSeedSet.ObjectMeta, true, gardencorevalidation.ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateManagedSeedSetSpec(&ManagedSeedSet.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateManagedSeedSetUpdate validates a ManagedSeedSet object before an update.
func ValidateManagedSeedSetUpdate(newManagedSeedSet, oldManagedSeedSet *seedmanagement.ManagedSeedSet) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newManagedSeedSet.ObjectMeta, &oldManagedSeedSet.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateManagedSeedSetSpecUpdate(&newManagedSeedSet.Spec, &oldManagedSeedSet.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateManagedSeedSet(newManagedSeedSet)...)

	return allErrs
}

// ValidateManagedSeedSetStatusUpdate validates a ManagedSeedSet object before a status update.
func ValidateManagedSeedSetStatusUpdate(newManagedSeedSet, oldManagedSeedSet *seedmanagement.ManagedSeedSet) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newManagedSeedSet.ObjectMeta, &oldManagedSeedSet.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateManagedSeedSetStatus(&newManagedSeedSet.Status, newManagedSeedSet.Name, field.NewPath("status"))...)

	statusPath := field.NewPath("status")
	if newManagedSeedSet.Status.NextReplicaNumber < oldManagedSeedSet.Status.NextReplicaNumber {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("nextReplicaNumber"), newManagedSeedSet.Status.NextReplicaNumber, "cannot be decremented"))
	}
	if isDecremented(newManagedSeedSet.Status.CollisionCount, oldManagedSeedSet.Status.CollisionCount) {
		value := ptr.Deref(newManagedSeedSet.Status.CollisionCount, 0)
		allErrs = append(allErrs, field.Invalid(statusPath.Child("collisionCount"), value, "cannot be decremented"))
	}

	return allErrs
}

// ValidateManagedSeedSetSpec validates the specification of a ManagedSeed object.
func ValidateManagedSeedSetSpec(spec *seedmanagement.ManagedSeedSetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure replicas is non-negative if specified
	if spec.Replicas != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*spec.Replicas), fldPath.Child("replicas"))...)
	}

	// Ensure selector is specified, non-empty and valid
	selectorPath := fldPath.Child("selector")
	var selector labels.Selector
	if len(spec.Selector.MatchLabels) == 0 && len(spec.Selector.MatchExpressions) == 0 {
		allErrs = append(allErrs, field.Invalid(selectorPath, spec.Selector, "empty selector is invalid for ManagedSeedSet"))
	} else {
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&spec.Selector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, selectorPath)...)
		var err error
		if selector, err = metav1.LabelSelectorAsSelector(&spec.Selector); err != nil {
			allErrs = append(allErrs, field.Invalid(selectorPath, spec.Selector, err.Error()))
		}
	}

	// Validate template and shootTemplate
	allErrs = append(allErrs, ValidateManagedSeedTemplateForManagedSeedSet(&spec.Template, selector, fldPath.Child("template"))...)
	allErrs = append(allErrs, ValidateShootTemplateForManagedSeedSet(&spec.ShootTemplate, selector, fldPath.Child("shootTemplate"))...)

	if spec.UpdateStrategy != nil {
		allErrs = append(allErrs, validateUpdateStrategy(spec.UpdateStrategy, fldPath.Child("updateStrategy"))...)
	}

	// Ensure revisionHistoryLimit is non-negative if specified
	if spec.RevisionHistoryLimit != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*spec.RevisionHistoryLimit), fldPath.Child("revisionHistoryLimit"))...)
	}

	return allErrs
}

func validateUpdateStrategy(updateStrategy *seedmanagement.UpdateStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure updateStrategy.type is RollingUpdate if specified
	if updateStrategy.Type != nil {
		switch *updateStrategy.Type {
		case "":
			allErrs = append(allErrs, field.Required(fldPath.Child("type"), ""))
		case seedmanagement.RollingUpdateStrategyType:
			if updateStrategy.RollingUpdate != nil {
				allErrs = append(allErrs, validateRollingUpdateStrategy(updateStrategy.RollingUpdate, fldPath.Child("rollingUpdate"))...)
			}
		default:
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"),
				*updateStrategy.Type, []string{string(seedmanagement.RollingUpdateStrategyType)}))
		}
	}

	return allErrs
}

func validateRollingUpdateStrategy(rus *seedmanagement.RollingUpdateStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure partition is non-negative if specified
	if rus.Partition != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*rus.Partition), fldPath.Child("partition"))...)
	}

	return allErrs
}

// ValidateManagedSeedSetSpecUpdate validates a ManagedSeedSetSpec object before an update.
func ValidateManagedSeedSetSpecUpdate(newSpec, oldSpec *seedmanagement.ManagedSeedSetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure selector is not changed
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Selector, oldSpec.Selector, fldPath.Child("selector"))...)

	// Ensure revisionHistoryLimit is not changed
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.RevisionHistoryLimit, oldSpec.RevisionHistoryLimit, fldPath.Child("revisionHistoryLimit"))...)

	// Validate updates to template and shootTemplate
	allErrs = append(allErrs, ValidateManagedSeedTemplateUpdate(&newSpec.Template, &oldSpec.Template, fldPath.Child("template"))...)
	allErrs = append(allErrs, gardencorevalidation.ValidateShootTemplateUpdate(&newSpec.ShootTemplate, &oldSpec.ShootTemplate, fldPath.Child("shootTemplate"))...)

	return allErrs
}

// ValidateManagedSeedTemplateForManagedSeedSet validates the given ManagedSeedTemplate.
func ValidateManagedSeedTemplateForManagedSeedSet(template *seedmanagement.ManagedSeedTemplate, selector labels.Selector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateTemplateLabels(&template.ObjectMeta, selector, fldPath.Child("metadata"))...)
	allErrs = append(allErrs, ValidateManagedSeedTemplate(template, fldPath)...)

	if template.Spec.Shoot != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("spec", "shoot"), "shoot is forbidden"))
	}

	// TODO(timuthy): Remove this check once `config` is required.
	if template.Spec.Gardenlet.Config != nil {
		configPath := fldPath.Child("spec", "gardenlet", "config")
		gardenletConfig, ok := template.Spec.Gardenlet.Config.(*gardenletconfigv1alpha1.GardenletConfiguration)
		if !ok {
			allErrs = append(allErrs, field.Invalid(configPath, template.Spec.Gardenlet.Config, fmt.Sprintf("expected *gardenletconfigv1alpha1.GardenletConfiguration but got %T", template.Spec.Gardenlet.Config)))
			return allErrs
		}
		if gardenletConfig.SeedConfig == nil {
			allErrs = append(allErrs, field.Required(configPath.Child("seedConfig"), "seedConfig is required"))
		} else {
			allErrs = append(allErrs, validateTemplateLabels(&gardenletConfig.SeedConfig.SeedTemplate.ObjectMeta, selector, configPath.Child("seedConfig").Child("metadata"))...)
		}
	}

	return allErrs
}

// ValidateShootTemplateForManagedSeedSet validates the given ShootTemplate.
func ValidateShootTemplateForManagedSeedSet(template *gardencore.ShootTemplate, selector labels.Selector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateTemplateLabels(&template.ObjectMeta, selector, fldPath.Child("metadata"))...)
	allErrs = append(allErrs, validateIfWorkerless(&template.Spec, fldPath.Child("spec"))...)
	allErrs = append(allErrs, gardencorevalidation.ValidateShootTemplate(template, fldPath)...)

	return allErrs
}

func validateIfWorkerless(spec *gardencore.ShootSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Provider.Workers) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("provider", "workers"), spec.Provider.Workers, "workers cannot be empty in the Shoot template for a ManagedSeedSet"))
	}

	return allErrs
}

func validateTemplateLabels(meta *metav1.ObjectMeta, selector labels.Selector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Verify that the selector matches the labels
	if selector != nil && !selector.Empty() && !selector.Matches(labels.Set(meta.Labels)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("labels"), meta.Labels, "selector does not match template labels"))
	}

	return allErrs
}

// ValidateManagedSeedSetStatus validates the given ManagedSeedSetStatus.
func ValidateManagedSeedSetStatus(status *seedmanagement.ManagedSeedSetStatus, name string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure integer fields are non-negative
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(status.ObservedGeneration, fldPath.Child("observedGeneration"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.Replicas), fldPath.Child("replicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.ReadyReplicas), fldPath.Child("readyReplicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.CurrentReplicas), fldPath.Child("currentReplicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.UpdatedReplicas), fldPath.Child("updatedReplicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.NextReplicaNumber), fldPath.Child("nextReplicaNumber"))...)
	if status.CollisionCount != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*status.CollisionCount), fldPath.Child("collisionCount"))...)
	}

	// Ensure the numbers or ready, current, and updated replicas are not greater than the number of replicas
	if status.ReadyReplicas > status.Replicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("readyReplicas"), status.ReadyReplicas, "cannot be greater than status.replicas"))
	}
	if status.CurrentReplicas > status.Replicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("currentReplicas"), status.CurrentReplicas, "cannot be greater than status.replicas"))
	}
	if status.UpdatedReplicas > status.Replicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("updatedReplicas"), status.UpdatedReplicas, "cannot be greater than status.replicas"))
	}

	if status.PendingReplica != nil {
		allErrs = append(allErrs, validatePendingReplica(status.PendingReplica, name, fldPath.Child("pendingReplica"))...)
	}

	return allErrs
}

func validatePendingReplica(pendingReplica *seedmanagement.PendingReplica, name string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if parentName, ordinal := getParentNameAndOrdinal(pendingReplica.Name); parentName != name || ordinal < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), pendingReplica.Name, "must contain the parent name and a valid ordinal"))
	}

	validValues := []string{
		string(seedmanagement.ShootReconcilingReason),
		string(seedmanagement.ShootDeletingReason),
		string(seedmanagement.ShootReconcileFailedReason),
		string(seedmanagement.ShootDeleteFailedReason),
		string(seedmanagement.ManagedSeedPreparingReason),
		string(seedmanagement.ManagedSeedDeletingReason),
		string(seedmanagement.SeedNotReadyReason),
		string(seedmanagement.ShootNotHealthyReason),
	}
	if !slices.Contains(validValues, string(pendingReplica.Reason)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("reason"), pendingReplica.Reason, validValues))
	}

	if pendingReplica.Retries != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*pendingReplica.Retries), fldPath.Child("retries"))...)
	}

	return allErrs
}

func isDecremented(new, old *int32) bool {
	if new == nil && old != nil {
		return true
	}
	if new == nil || old == nil {
		return false
	}
	return *new < *old
}

// parentNameAndOrdinalRegex is a regular expression that extracts the parent name and ordinal from a replica name.
var parentNameAndOrdinalRegex = regexp.MustCompile("(.*)-([0-9]+)$")

// getParentNameAndOrdinal gets the ordinal from the given replica object name.
// If the object was not created by a ManagedSeedSet, its ordinal is considered to be -1.
func getParentNameAndOrdinal(name string) (string, int) {
	subMatches := parentNameAndOrdinalRegex.FindStringSubmatch(name)
	if len(subMatches) < 3 {
		return "", -1
	}
	ordinal, err := strconv.ParseInt(subMatches[2], 10, 32)
	if err != nil {
		return "", -1
	}
	return subMatches[1], int(ordinal)
}
