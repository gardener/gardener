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

// Actuator acts upon ContainerRuntime resources.
type Actuator interface {
	// Reconcile the ContainerRuntime resource.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.ContainerRuntime, *extensionscontroller.Cluster) error
	// Delete the ContainerRuntime resource.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.ContainerRuntime, *extensionscontroller.Cluster) error
	// ForceDelete forcefully deletes the ContainerRuntime.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.ContainerRuntime, *extensionscontroller.Cluster) error
	// Restore the ContainerRuntime resource.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.ContainerRuntime, *extensionscontroller.Cluster) error
	// Migrate the ContainerRuntime resource.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.ContainerRuntime, *extensionscontroller.Cluster) error
}
