// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"
	"strings"

	"k8s.io/utils/clock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// BackupBucketIsErroneous returns `true` if the given BackupBucket has a last error.
// It also returns the error description if available.
func BackupBucketIsErroneous(bb *gardencorev1beta1.BackupBucket) (bool, string) {
	if bb == nil {
		return false, ""
	}

	lastErr := bb.Status.LastError
	if lastErr == nil {
		return false, ""
	}
	return true, lastErr.Description
}

// ComputeBackupBucketsCondition computes a BackupBucketsReady condition from the given list of BackupBuckets.
func ComputeBackupBucketsCondition(
	clock clock.Clock,
	condition gardencorev1beta1.Condition,
	backupBuckets []gardencorev1beta1.BackupBucket,
) gardencorev1beta1.Condition {
	var erroneousBuckets []string
	for _, bb := range backupBuckets {
		if occurred, msg := BackupBucketIsErroneous(&bb); occurred {
			erroneousBuckets = append(erroneousBuckets, fmt.Sprintf("Name: %s, Error: %s", bb.Name, msg))
		}
	}

	if len(erroneousBuckets) > 0 {
		msg := "The following BackupBuckets have issues:\n* " + strings.Join(erroneousBuckets, "\n* ")
		return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionFalse, "BackupBucketsError", msg)
	}

	if len(backupBuckets) == 0 {
		return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionUnknown, "BackupBucketsGone", "Backup Buckets are gone.")
	}

	return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionTrue, "BackupBucketsAvailable", "Backup Buckets are available.")
}
