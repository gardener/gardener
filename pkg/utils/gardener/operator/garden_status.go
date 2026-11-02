// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"k8s.io/component-base/version"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// IsGardenSuccessfullyReconciled returns true if the passed garden resource reports a successful reconciliation.
func IsGardenSuccessfullyReconciled(garden *operatorv1alpha1.Garden) bool {
	lastOp := garden.Status.LastOperation
	return lastOp != nil &&
		lastOp.Type == gardencorev1beta1.LastOperationTypeReconcile && lastOp.State == gardencorev1beta1.LastOperationStateSucceeded && lastOp.Progress == 100
}

// IsGardenUpToDate returns true if the garden status contains the same version the operator is running.
func IsGardenUpToDate(garden *operatorv1alpha1.Garden) bool {
	return garden.Status.Gardener != nil && garden.Status.Gardener.Version == version.Get().GitVersion
}
