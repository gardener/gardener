// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"

	"github.com/go-logr/logr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon [extensionsv1alpha1.Bastion] resources.
type Actuator interface {
	// Reconcile reconciles the [extensionsv1alpha1.Bastion] resource.
	//
	// Implementations should ensure that the bastion host is created or
	// updated in order to reach its desired state.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.Bastion, *extensionscontroller.Cluster) error

	// Delete is invoked when the [extensionsv1alpha1.Bastion] resource is
	// deleted.
	//
	// Implementations should take care of cleaning up any resources created
	// by the extension during its lifecycle.
	//
	// Implementations must wait until all resources managed by the
	// extension have been gracefully cleaned up.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.Bastion, *extensionscontroller.Cluster) error

	// ForceDelete is invoked when the [extensionsv1alpha1.Bastion] resource
	// is being deleted in a forceful manner.
	//
	// Implementations should take care of unblocking the deletion flow by
	// attempting to cleanup any resources created by the extension, and
	// also skip waiting for external resources, if they cannot be deleted
	// gracefully.
	//
	// Even if some resources managed by the extension implementation cannot
	// be deleted gracefully, this method should succeed, even at the cost
	// of leaving some leftover resources behind.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.Bastion, *extensionscontroller.Cluster) error
}
