// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"context"

	extensioncontroller "github.com/gardener/gardener/extensions/pkg/controller"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon Network resources.
type Actuator interface {
	// Reconcile reconciles the Network resource.
	Reconcile(context.Context, *extensionsv1alpha1.Network, *extensioncontroller.Cluster) error
	// Delete deletes the Network resource.
	Delete(context.Context, *extensionsv1alpha1.Network, *extensioncontroller.Cluster) error
	// Reconcile restores the Network resource.
	Restore(context.Context, *extensionsv1alpha1.Network, *extensioncontroller.Cluster) error
	// Migrate migrates the Network resource.
	Migrate(context.Context, *extensionsv1alpha1.Network, *extensioncontroller.Cluster) error
}
