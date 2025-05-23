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

// Actuator acts upon Worker resources.
type Actuator interface {
	// Reconcile reconciles the Worker.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error
	// Delete deletes the Worker.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error
	// ForceDelete forcefully deletes the Worker.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error
	// Restore reads from the worker.status.state field and deploys the machines and machineSet
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error
	// Migrate deletes the MCM, machineDeployments, machineClasses, machineClassSecrets,
	// machineSets and the machines. The underlying VMs representing the Shoot nodes are not deleted
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.Worker, *extensionscontroller.Cluster) error
}
