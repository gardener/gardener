// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"

	"github.com/go-logr/logr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon [extensionsv1alpha1.Worker] resources.
type Actuator interface {
	// Reconcile reconciles the [extensionsv1alpha1.Worker] resource.
	//
	// Implementations should ensure that any resources
	// (e.g. MachineClasses, MachineDeployments, Secrets, etc.) are created
	// or updated in order to reach their desired state.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error

	// Delete is invoked when the [extensionsv1alpha1.Worker] resource is
	// deleted.
	//
	// Implementations should take care of cleaning up any resources
	// (e.g. MachineClasses, MachineDeployments, Secrets, etc.), which were
	// created by the Extension.
	//
	// Implementations must wait until all resources managed by the
	// extension have been gracefully cleaned up.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error

	// ForceDelete is invoked when the shoot cluster associated with the
	// [extensionsv1alpha1.Worker] resource is being deleted in a forceful
	// manner.
	//
	// Implementations should take care of unblocking the deletion flow by
	// attempting to cleanup any resources created by the extension, remove
	// any finalizers created for custom resources, etc., and also skip
	// waiting for external resources, if they cannot be deleted gracefully.
	//
	// Even if some resources managed by the extension implementation cannot
	// be deleted gracefully, this method should succeed, even at the cost
	// of leaving some leftover resources behind.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error

	// Restore restores the [extensionsv1alpha1.Worker] from a previously
	// saved state.
	//
	// For more details please refer to the following documentation.
	//
	// https://gardener.cloud/docs/gardener/extensions/migration/#implementation-details
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error

	// Migrate prepares the [extensionsv1alpha1.Worker] resource for
	// migration.
	//
	// This method is invoked when the shoot cluster associated with the
	// [extensionsv1alpha1.Extension] resource is being migrated to another
	// seed cluster.
	//
	// For more details, please refer to the following documentation.
	//
	// https://gardener.cloud/docs/gardener/extensions/migration/#implementation-details
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error
}
