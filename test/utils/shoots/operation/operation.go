// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ReconciliationSuccessful checks if a shoot is successfully reconciled. In case it is not, it also returns a descriptive message stating the reason.
func ReconciliationSuccessful(shoot *gardencorev1beta1.Shoot) (bool, string) {
	if shoot.Generation != shoot.Status.ObservedGeneration {
		return false, "shoot generation did not equal observed generation"
	}
	if len(shoot.Status.Conditions) == 0 && shoot.Status.LastOperation == nil {
		return false, "no conditions and last operation present yet"
	}

	workerlessShoot := v1beta1helper.IsWorkerless(shoot)
	shootConditions := sets.New(gardenerutils.GetShootConditionTypes(workerlessShoot)...)

	for _, condition := range shoot.Status.Conditions {
		if condition.Status != gardencorev1beta1.ConditionTrue {
			// Only return false if the status of a shoot condition is not True during hibernation. If the shoot also acts as a seed and
			// the `gardenlet` that operates the seed has already been shut down as part of the hibernation, the seed conditions will never
			// be updated to True if they were previously not True.
			hibernation := shoot.Spec.Hibernation
			if !shootConditions.Has(condition.Type) && hibernation != nil && ptr.Deref(hibernation.Enabled, false) {
				continue
			}
			return false, fmt.Sprintf("condition type %s is not true yet, had message %s with reason %s", condition.Type, condition.Message, condition.Reason)
		}
	}

	if shoot.Status.LastOperation != nil {
		switch shoot.Status.LastOperation.Type {
		case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile, gardencorev1beta1.LastOperationTypeRestore:
			if shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
				return false, "last operation type was create, reconcile or restore but state was not succeeded"
			}
		case gardencorev1beta1.LastOperationTypeMigrate:
			return false, "last operation type was migrate, the migration process is not finished yet"
		}
	}

	return true, ""
}
