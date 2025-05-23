// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"

	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon BackupBucket resources.
type Actuator interface {
	// Reconcile reconciles the BackupBucket.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.BackupBucket) error
	// Delete deletes the BackupBucket.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.BackupBucket) error
}
