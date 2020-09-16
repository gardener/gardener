// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateWorker validates a Worker object.
func ValidateWorker(worker *extensionsv1alpha1.Worker) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&worker.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateWorkerSpec(&worker.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateWorkerUpdate validates a Worker object before an update.
func ValidateWorkerUpdate(new, old *extensionsv1alpha1.Worker) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateWorkerSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateWorker(new)...)

	return allErrs
}

// ValidateWorkerSpec validates the specification of a Worker object.
func ValidateWorkerSpec(spec *extensionsv1alpha1.WorkerSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if len(spec.Region) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "field is required"))
	}

	if len(spec.SecretRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretRef", "name"), "field is required"))
	}

	allErrs = append(allErrs, ValidateWorkerPools(spec.Pools, fldPath.Child("pools"))...)

	return allErrs
}

// ValidateWorkerPools validates a list of worker pools.
func ValidateWorkerPools(pools []extensionsv1alpha1.WorkerPool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, pool := range pools {
		idxPath := fldPath.Index(i)

		if len(pool.MachineType) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("machineType"), "field is required"))
		}

		if len(pool.MachineImage.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("machineImage", "name"), "field is required"))
		}
		if len(pool.MachineImage.Version) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("machineImage", "version"), "field is required"))
		}

		if len(pool.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "field is required"))
		}

		if pool.UserData == nil {
			allErrs = append(allErrs, field.Required(idxPath.Child("userData"), "field is required"))
		}
	}

	return allErrs
}

// ValidateWorkerSpecUpdate validates the spec of a Worker object before an update.
func ValidateWorkerSpecUpdate(new, old *extensionsv1alpha1.WorkerSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Region, old.Region, fldPath.Child("region"))...)

	return allErrs
}

// ValidateWorkerStatus validates the status of a Worker object.
func ValidateWorkerStatus(spec *extensionsv1alpha1.WorkerStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateWorkerStatusUpdate validates the status field of a Worker object.
func ValidateWorkerStatusUpdate(newStatus, oldStatus extensionsv1alpha1.WorkerStatus) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
