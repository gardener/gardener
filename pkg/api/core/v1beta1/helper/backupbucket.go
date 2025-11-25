// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
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
