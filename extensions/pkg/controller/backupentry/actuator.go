// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"

	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon BackupEntry resources.
type Actuator interface {
	// Reconcile reconciles the BackupEntry.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.BackupEntry) error
	// Delete deletes the BackupEntry.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.BackupEntry) error
	// Restore restores the BackupEntry.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.BackupEntry) error
	// Migrate migrates the BackupEntry.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.BackupEntry) error
}
