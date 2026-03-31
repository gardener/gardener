// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"fmt"
	"strings"

	"k8s.io/utils/clock"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ComputeBackupBucketsCondition computes a BackupBucketsReady condition from the given list of BackupBuckets.
func ComputeBackupBucketsCondition(
	clock clock.Clock,
	condition gardencorev1beta1.Condition,
	backupBuckets []gardencorev1beta1.BackupBucket,
) gardencorev1beta1.Condition {
	var erroneousBuckets []string
	for _, bb := range backupBuckets {
		if hasError, msg := v1beta1helper.BackupBucketIsErroneous(&bb); hasError {
			erroneousBuckets = append(erroneousBuckets, fmt.Sprintf("Name: %s, Error: %s", bb.Name, msg))
		}
	}

	if len(erroneousBuckets) > 0 {
		msg := "The following BackupBuckets have issues:\n* " + strings.Join(erroneousBuckets, "\n* ")
		return v1beta1helper.UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionFalse, "BackupBucketsError", msg)
	}

	if len(backupBuckets) == 0 {
		return v1beta1helper.UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionUnknown, "BackupBucketsGone", "Backup Buckets are gone.")
	}

	return v1beta1helper.UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionTrue, "BackupBucketsAvailable", "Backup Buckets are available.")
}
