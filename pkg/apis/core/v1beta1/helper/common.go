// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

// ComputeOperationType checks the <lastOperation> and determines whether it is Create, Delete, Reconcile, Migrate or Restore operation
func ComputeOperationType(meta metav1.ObjectMeta, lastOperation *gardencorev1beta1.LastOperation) gardencorev1beta1.LastOperationType {
	switch {
	case meta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationMigrate:
		return gardencorev1beta1.LastOperationTypeMigrate
	case meta.DeletionTimestamp != nil:
		return gardencorev1beta1.LastOperationTypeDelete
	case meta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return gardencorev1beta1.LastOperationTypeRestore
	case lastOperation == nil:
		return gardencorev1beta1.LastOperationTypeCreate
	case lastOperation.Type == gardencorev1beta1.LastOperationTypeCreate && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded:
		return gardencorev1beta1.LastOperationTypeCreate
	case lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded:
		return gardencorev1beta1.LastOperationTypeMigrate
	case lastOperation.Type == gardencorev1beta1.LastOperationTypeRestore && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded:
		return gardencorev1beta1.LastOperationTypeRestore
	}
	return gardencorev1beta1.LastOperationTypeReconcile
}

// WrapWithLastError is wrapper function for gardencorev1beta1.LastError
func WrapWithLastError(err error, lastError *gardencorev1beta1.LastError) error {
	if err == nil || lastError == nil {
		return err
	}
	return fmt.Errorf("last error: %w: %s", err, lastError.Description)
}

// UpsertLastError adds a 'last error' to the given list of existing 'last errors' if it does not exist yet. Otherwise,
// it updates it.
func UpsertLastError(lastErrors []gardencorev1beta1.LastError, lastError gardencorev1beta1.LastError) []gardencorev1beta1.LastError {
	var (
		out   []gardencorev1beta1.LastError
		found bool
	)

	for _, lastErr := range lastErrors {
		if lastErr.TaskID != nil && lastError.TaskID != nil && *lastErr.TaskID == *lastError.TaskID {
			out = append(out, lastError)
			found = true
		} else {
			out = append(out, lastErr)
		}
	}

	if !found {
		out = append(out, lastError)
	}

	return out
}

// DeleteLastErrorByTaskID removes the 'last error' with the given task ID from the given 'last error' list.
func DeleteLastErrorByTaskID(lastErrors []gardencorev1beta1.LastError, taskID string) []gardencorev1beta1.LastError {
	var out []gardencorev1beta1.LastError
	for _, lastErr := range lastErrors {
		if lastErr.TaskID == nil || taskID != *lastErr.TaskID {
			out = append(out, lastErr)
		}
	}
	return out
}

// IsFailureToleranceTypeZone returns true if failureToleranceType is zone else returns false.
func IsFailureToleranceTypeZone(failureToleranceType *gardencorev1beta1.FailureToleranceType) bool {
	return failureToleranceType != nil && *failureToleranceType == gardencorev1beta1.FailureToleranceTypeZone
}

// IsFailureToleranceTypeNode returns true if failureToleranceType is node else returns false.
func IsFailureToleranceTypeNode(failureToleranceType *gardencorev1beta1.FailureToleranceType) bool {
	return failureToleranceType != nil && *failureToleranceType == gardencorev1beta1.FailureToleranceTypeNode
}

// ShootHasOperationType returns true when the 'type' in the last operation matches the provided type.
func ShootHasOperationType(lastOperation *gardencorev1beta1.LastOperation, lastOperationType gardencorev1beta1.LastOperationType) bool {
	return lastOperation != nil && lastOperation.Type == lastOperationType
}

// HasOperationAnnotation returns true if the operation annotation is present and its value is "reconcile", "restore, or "migrate".
func HasOperationAnnotation(annotations map[string]string) bool {
	return annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationReconcile ||
		annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore ||
		annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationMigrate
}

// ResourceReferencesEqual returns true when none of the Secret/ConfigMap/WorkloadIdentity resource references have changed.
func ResourceReferencesEqual(oldResources, newResources []gardencorev1beta1.NamedResourceReference) bool {
	oldNames := namesForResourceReferences(oldResources)
	newNames := namesForResourceReferences(newResources)

	return oldNames.Equal(newNames)
}

func namesForResourceReferences(resources []gardencorev1beta1.NamedResourceReference) sets.Set[string] {
	names := sets.New[string]()
	for _, resource := range resources {
		if resource.ResourceRef.APIVersion == corev1.SchemeGroupVersion.String() && sets.New("Secret", "ConfigMap").Has(resource.ResourceRef.Kind) ||
			resource.ResourceRef.APIVersion == securityv1alpha1.SchemeGroupVersion.String() && resource.ResourceRef.Kind == "WorkloadIdentity" {
			names.Insert(resource.ResourceRef.Kind + "/" + resource.ResourceRef.Name)
		}
	}
	return names
}
