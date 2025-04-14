// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// IsGardenSuccessfullyReconciled returns true if the passed garden resource reports a successful reconciliation.
func IsGardenSuccessfullyReconciled(garden *operatorv1alpha1.Garden) bool {
	lastOp := garden.Status.LastOperation
	return lastOp != nil &&
		lastOp.Type == gardencorev1beta1.LastOperationTypeReconcile && lastOp.State == gardencorev1beta1.LastOperationStateSucceeded && lastOp.Progress == 100
}
