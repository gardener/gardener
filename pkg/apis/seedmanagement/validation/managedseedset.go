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
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	corevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

// ValidateManagedSeedSet validates a ManagedSeedSet object.
func ValidateManagedSeedSet(ManagedSeedSet *seedmanagement.ManagedSeedSet) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure namespace is garden
	if ManagedSeedSet.Namespace != v1beta1constants.GardenNamespace {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "namespace"), ManagedSeedSet.Namespace, "namespace must be garden"))
	}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&ManagedSeedSet.ObjectMeta, true, corevalidation.ValidateName, field.NewPath("metadata"))...)
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

// ValidateManagedSeedStatusUpdate validates a ManagedSeedSet object before a status update.
func ValidateManagedSeedSetStatusUpdate(newManagedSeedSet, oldManagedSeedSet *seedmanagement.ManagedSeedSet) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newManagedSeedSet.ObjectMeta, &oldManagedSeedSet.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateManagedSeedSetStatus(&newManagedSeedSet.Status, field.NewPath("status"))...)

	statusPath := field.NewPath("status")
	if newManagedSeedSet.Status.NextReplicaNumber < oldManagedSeedSet.Status.NextReplicaNumber {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("nextReplicaNumber"), newManagedSeedSet.Status.NextReplicaNumber, "cannot be decremented"))
	}
	if isDecremented(newManagedSeedSet.Status.CollisionCount, oldManagedSeedSet.Status.CollisionCount) {
		value := pointer.Int32PtrDerefOr(newManagedSeedSet.Status.CollisionCount, 0)
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
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&spec.Selector, selectorPath)...)
		var err error
		if selector, err = metav1.LabelSelectorAsSelector(&spec.Selector); err != nil {
			allErrs = append(allErrs, field.Invalid(selectorPath, spec.Selector, err.Error()))
		}
	}

	// Validate template and shootTemplate
	allErrs = append(allErrs, ValidateManagedSeedTemplateForManagedSeedSet(&spec.Template, selector, fldPath.Child("template"))...)
	allErrs = append(allErrs, ValidateShootTemplateForManagedSeedSet(&spec.ShootTemplate, selector, fldPath.Child("shootTemplate"))...)

	// Ensure updateStrategy.type is RollingUpdate if specified
	if spec.UpdateStrategy != nil && spec.UpdateStrategy.Type != nil {
		updateStrategyPath := fldPath.Child("updateStrategy")

		switch *spec.UpdateStrategy.Type {
		case "":
			allErrs = append(allErrs, field.Required(updateStrategyPath.Child("type"), ""))
		case seedmanagement.RollingUpdateStrategyType:
			// Ensure rollingUpdate.partition is non-negative if specified
			if spec.UpdateStrategy.RollingUpdate != nil && spec.UpdateStrategy.RollingUpdate.Partition != nil {
				allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*spec.UpdateStrategy.RollingUpdate.Partition),
					updateStrategyPath.Child("rollingUpdate", "partition"))...)
			}
		default:
			allErrs = append(allErrs, field.NotSupported(updateStrategyPath.Child("type"),
				*spec.UpdateStrategy.Type, []string{string(seedmanagement.RollingUpdateStrategyType)}))
		}
	}

	// Ensure revisionHistoryLimit is non-negative if specified
	if spec.RevisionHistoryLimit != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*spec.RevisionHistoryLimit), fldPath.Child("revisionHistoryLimit"))...)
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
	allErrs = append(allErrs, corevalidation.ValidateShootTemplateUpdate(&newSpec.ShootTemplate, &oldSpec.ShootTemplate, fldPath.Child("shootTemplate"))...)

	return allErrs
}

// ValidateManagedSeedTemplateForManagedSeedSet validates the given ManagedSeedTemplate.
func ValidateManagedSeedTemplateForManagedSeedSet(template *seedmanagement.ManagedSeedTemplate, selector labels.Selector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Verify that the selector matches the labels in template.
	if selector != nil && !selector.Empty() && !selector.Matches(labels.Set(template.Labels)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("metadata", "labels"), template.Labels, "selector does not match template labels"))
	}

	allErrs = append(allErrs, ValidateManagedSeedTemplate(template, fldPath)...)

	return allErrs
}

// ValidateShootTemplateForManagedSeedSet validates the given ShootTemplate.
func ValidateShootTemplateForManagedSeedSet(template *gardencore.ShootTemplate, selector labels.Selector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Verify that the selector matches the labels in template.
	if selector != nil && !selector.Empty() && !selector.Matches(labels.Set(template.Labels)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("metadata", "labels"), template.Labels, "selector does not match template labels"))
	}

	allErrs = append(allErrs, corevalidation.ValidateShootTemplate(template, fldPath)...)

	return allErrs
}

// ValidateManagedSeedSetStatus validates the given ManagedSeedSetStatus.
func ValidateManagedSeedSetStatus(status *seedmanagement.ManagedSeedSetStatus, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure integer fields are non-negative
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(status.ObservedGeneration, fieldPath.Child("observedGeneration"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.Replicas), fieldPath.Child("replicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.ReadyReplicas), fieldPath.Child("readyReplicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.CurrentReplicas), fieldPath.Child("currentReplicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.UpdatedReplicas), fieldPath.Child("updatedReplicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.NextReplicaNumber), fieldPath.Child("nextReplicaNumber"))...)
	if status.CollisionCount != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*status.CollisionCount), fieldPath.Child("collisionCount"))...)
	}

	// Ensure the numbers or ready, current, and updated replicas are not greater than the number of replicas
	if status.ReadyReplicas > status.Replicas {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("readyReplicas"), status.ReadyReplicas, "cannot be greater than status.replicas"))
	}
	if status.CurrentReplicas > status.Replicas {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("currentReplicas"), status.CurrentReplicas, "cannot be greater than status.replicas"))
	}
	if status.UpdatedReplicas > status.Replicas {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("updatedReplicas"), status.UpdatedReplicas, "cannot be greater than status.replicas"))
	}

	// Ensure the next replica number is greater than or equal to the number of replicas
	if status.NextReplicaNumber < status.Replicas {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("nextReplicaNumber"), status.ReadyReplicas, "cannot be less than status.replicas"))
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
