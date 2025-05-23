// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"context"

	"github.com/go-logr/logr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon Network resources.
type Actuator interface {
	// Reconcile reconciles the Network resource.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.Network, *extensionscontroller.Cluster) error
	// Delete deletes the Network resource.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.Network, *extensionscontroller.Cluster) error
	// ForceDelete forcefully deletes the Network resource.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.Network, *extensionscontroller.Cluster) error
	// Restore restores the Network resource.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.Network, *extensionscontroller.Cluster) error
	// Migrate migrates the Network resource.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.Network, *extensionscontroller.Cluster) error
}
