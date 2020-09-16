// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon ControlPlane resources.
type Actuator interface {
	// Reconcile reconciles the ControlPlane.
	Reconcile(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) (bool, error)
	// Delete deletes the ControlPlane.
	Delete(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) error
	// Restore restores the ControlPlane.
	Restore(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) (bool, error)
	// Migrate migrates the ControlPlane.
	Migrate(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) error
}
