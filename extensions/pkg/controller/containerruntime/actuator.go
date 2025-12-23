// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerruntime

import (
	"context"

	"github.com/go-logr/logr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon [extensionsv1alpha1.ContainerRuntime] resources.
type Actuator interface {
	// Reconcile reconciles the [extensionsv1alpha1.ContainerRuntime]
	// resource.
	//
	// Implementations should ensure that the desired container runtime is
	// deployed or updated for relevant worker pools.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.ContainerRuntime, *extensionscontroller.Cluster) error

	// Delete is invoked when the [extensionsv1alpha1.ContainerRuntime]
	// resource is deleted.
	//
	// Implementations should take care of cleaning up any resources such as
	// ManagedResources, Secrets, etc., which were created by the Extension.
	//
	// Implementations must also wait until all resources managed by the
	// extension have been gracefully cleaned up.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.ContainerRuntime, *extensionscontroller.Cluster) error

	// ForceDelete is invoked when the shoot cluster associated with the
	// [extensionsv1alpha1.ContainerRuntime] resource is being deleted in a
	// forceful manner.
	//
	// Implementations should take care of unblocking the deletion flow by
	// attempting to cleanup any resources created by the extension, remove
	// any finalizers created for custom resources, and also skip waiting
	// for external resources, if they cannot be deleted gracefully.
	//
	// Even if some resources managed by the extension implementation cannot
	// be deleted gracefully, this method should succeed, even at the cost
	// of leaving some leftover resources behind.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.ContainerRuntime, *extensionscontroller.Cluster) error

	// Restore restores the [extensionsv1alpha1.ContainerRuntime] resource
	// from a previously saved state.
	//
	// This method is invoked when the shoot cluster associated with the
	// [extensionsv1alpha1.ContainerRuntime] resource is being restored on
	// the target seed cluster.
	//
	// Implementations may use the persisted data in the .status.state field
	// for restoring the state, when the shoot is being migrated to a
	// different seed cluster.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.ContainerRuntime, *extensionscontroller.Cluster) error

	// Migrate prepares the [extensionsv1alpha1.ContainerRuntime] resource
	// for migration.
	//
	// This method is invoked when the shoot cluster associated with the
	// [extensionsv1alpha1.ContainerRuntime] resource is being migrated to
	// another seed cluster.
	//
	// Implementations should take care of storing any required state in the
	// .status.state field, so that it can later be restored from this
	// state.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.ContainerRuntime, *extensionscontroller.Cluster) error
}
