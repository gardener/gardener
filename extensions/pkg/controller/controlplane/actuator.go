// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"

	"github.com/go-logr/logr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon ControlPlane resources.
type Actuator interface {
	// Reconcile reconciles the ControlPlane.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) (bool, error)
	// Delete deletes the ControlPlane.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) error
	// ForceDelete forcefully deletes the ControlPlane.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) error
	// Restore restores the ControlPlane.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) (bool, error)
	// Migrate migrates the ControlPlane.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) error
}
