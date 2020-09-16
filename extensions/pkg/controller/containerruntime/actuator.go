// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerruntime

import (
	"context"

	extensioncontroller "github.com/gardener/gardener/extensions/pkg/controller"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon ContainerRuntime resources.
type Actuator interface {
	// Reconcile the ContainerRuntime resource.
	Reconcile(context.Context, *extensionsv1alpha1.ContainerRuntime, *extensioncontroller.Cluster) error
	// Delete the ContainerRuntime resource.
	Delete(context.Context, *extensionsv1alpha1.ContainerRuntime, *extensioncontroller.Cluster) error
	// Restore the ContainerRuntime resource.
	Restore(context.Context, *extensionsv1alpha1.ContainerRuntime, *extensioncontroller.Cluster) error
	// Migrate the ContainerRuntime resource.
	Migrate(context.Context, *extensionsv1alpha1.ContainerRuntime, *extensioncontroller.Cluster) error
}
