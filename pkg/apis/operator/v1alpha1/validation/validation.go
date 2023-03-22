// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// ValidateGarden contains functionality for performing extended validation of a Garden object which is not possible
// with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateGarden(garden *operatorv1alpha1.Garden) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateOperation(garden.Annotations[v1beta1constants.GardenerOperation], garden, field.NewPath("metadata", "annotations"))...)

	return allErrs
}

// ValidateGardenUpdate contains functionality for performing extended validation of a Garden object under update which
// is not possible with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateGardenUpdate(oldGarden, newGarden *operatorv1alpha1.Garden) field.ErrorList {
	allErrs := field.ErrorList{}

	if oldGarden.Spec.VirtualCluster.ControlPlane != nil && oldGarden.Spec.VirtualCluster.ControlPlane.HighAvailability != nil &&
		(newGarden.Spec.VirtualCluster.ControlPlane == nil || newGarden.Spec.VirtualCluster.ControlPlane.HighAvailability == nil) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldGarden.Spec.VirtualCluster.ControlPlane, newGarden.Spec.VirtualCluster.ControlPlane, field.NewPath("spec", "virtualCluster", "controlPlane", "highAvailability"))...)
	}

	allErrs = append(allErrs, ValidateGarden(newGarden)...)

	return allErrs
}

func validateOperation(operation string, garden *operatorv1alpha1.Garden, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if operation == "" {
		return allErrs
	}

	fldPathOp := fldPath.Key(v1beta1constants.GardenerOperation)

	if operation != "" && !operatorv1alpha1.AvailableOperationAnnotations.Has(operation) {
		allErrs = append(allErrs, field.NotSupported(fldPathOp, operation, sets.List(operatorv1alpha1.AvailableOperationAnnotations)))
	}
	allErrs = append(allErrs, validateOperationContext(operation, garden, fldPathOp)...)

	return allErrs
}

func validateOperationContext(operation string, garden *operatorv1alpha1.Garden, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch operation {
	case v1beta1constants.OperationRotateCredentialsStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if garden has deletion timestamp"))
		}
		if phase := getCARotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.certificateAuthorities.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateCredentialsComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if garden has deletion timestamp"))
		}
		if getCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.certificateAuthorities.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateCAStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start CA rotation if garden has deletion timestamp"))
		}
		if phase := getCARotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start CA rotation if .status.credentials.rotation.certificateAuthorities.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateCAComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete CA rotation if garden has deletion timestamp"))
		}
		if getCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete CA rotation if .status.credentials.rotation.certificateAuthorities.phase is not 'Prepared'"))
		}
	}

	return allErrs
}

func getCARotationPhase(credentials *operatorv1alpha1.Credentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.CertificateAuthorities != nil {
		return credentials.Rotation.CertificateAuthorities.Phase
	}
	return ""
}
