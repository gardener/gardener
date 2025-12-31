// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"

	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon [extensionsv1alpha1.OperatingSystemConfig] resources.
type Actuator interface {
	// Reconcile reconciles the [extensionsv1alpha1.OperatingSystemConfig]
	// resource.
	//
	// Implementations should ensure that any resources (e.g. systemd units,
	// files, etc.) are created or updated in order to reach their desired
	// state.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.OperatingSystemConfig) ([]byte, []extensionsv1alpha1.Unit, []extensionsv1alpha1.File, *extensionsv1alpha1.InPlaceUpdatesStatus, error)

	// Delete is invoked when the [extensionsv1alpha1.OperatingSystemConfig]
	// resource is deleted.
	//
	// Implementations should take care of cleaning up any resources
	// (e.g. systemd units, files, etc.), which were created by the
	// Extension.
	//
	// Implementations must wait until all resources managed by the
	// extension have been gracefully cleaned up.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.OperatingSystemConfig) error

	// ForceDelete is invoked when the shoot cluster associated with the
	// [extensionsv1alpha1.OperatingSystemConfig] is deleted in a forceful
	// manner.
	//
	// Implementations should take care of unblocking the deletion flow by
	// attempting to cleanup any related resources, and skip waiting for
	// external resources, if they cannot be deleted gracefully.
	//
	// Even if some resources managed by the extension implementation cannot
	// be deleted gracefully, this method should succeed, even at the cost
	// of leaving some leftover resources behind.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.OperatingSystemConfig) error

	// Restore restores the operating system config from a previously saved
	// state.
	//
	// This method is invoked when the shoot cluster associated with the
	// [extensionsv1alpha1.OperatingSystemConfig] resource is being restored
	// on the target seed cluster.
	//
	// Implementations may use the persisted data in the .status.state field
	// for restoring the state, when the shoot is being migrated to a
	// different seed cluster.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.OperatingSystemConfig) ([]byte, []extensionsv1alpha1.Unit, []extensionsv1alpha1.File, *extensionsv1alpha1.InPlaceUpdatesStatus, error)

	// Migrate prepares the extension for migration.
	//
	// This method is invoked when the shoot cluster associated with the
	// [extensionsv1alpha1.OperatingSystemResource] resource is being
	// migrated to another seed cluster.
	//
	// Implementations should take care of storing any required state in the
	// .status.state field, so that it can later be restored from this
	// state.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.OperatingSystemConfig) error
}
