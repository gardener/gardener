// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"

	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon [extensionsv1alpha1.BackupEntry] resources.
type Actuator interface {
	// Reconcile reconciles the [extensionsv1alpha1.BackupEntry] resource.
	//
	// Implementations should ensure that the backup entry is created or
	// updated to reach the desired state.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.BackupEntry) error

	// Delete is invoked when [extensionsv1alpha1.BackupEntry] resource is
	// deleted.
	//
	// Implementations must wait until the backup entry is gracefully
	// deleted.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.BackupEntry) error

	// Restore restores the [extensionsv1alpha1.BackupEntry] from a
	// previously saved state.
	//
	// This method is invoked when the shoot cluster associated with the
	// [extensionsv1alpha1.BackupEntry] resource is being restored on the
	// target seed cluster.
	//
	// Implementations may use the persisted data in the .status.state field
	// for restoring the state, when the shoot is being migrated to a
	// different seed cluster.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.BackupEntry) error

	// Migrate prepares the [extensionsv1alpha1.BackupEntry] resource for
	// migration.
	//
	// This method is invoked when the shoot cluster associated with the
	// [extensionsv1alpha1.BackupEntry] resource is being migrated to
	// another seed cluster.
	//
	// Implementations should take care of storing any required state in the
	// .status.state field, so that it can later be restored from this
	// state.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.BackupEntry) error
}
