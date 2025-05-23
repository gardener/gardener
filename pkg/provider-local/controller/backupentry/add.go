// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	"github.com/gardener/gardener/pkg/provider-local/controller/backupoptions"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// ControllerName is the name of the controller.
const ControllerName = "backupentry_controller"

// DefaultAddOptions are the default AddOptions for AddToManager.
var DefaultAddOptions = backupoptions.AddOptions{}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(_ context.Context, mgr manager.Manager, opts backupoptions.AddOptions) error {
	return backupentry.Add(mgr, backupentry.AddArgs{
		Actuator:          genericactuator.NewActuator(mgr, newActuator(mgr, opts.ContainerMountPath, opts.BackupBucketPath)),
		ControllerOptions: opts.Controller,
		Predicates:        backupentry.DefaultPredicates(opts.IgnoreOperationAnnotation),
		Type:              local.Type,
		ExtensionClass:    opts.ExtensionClass,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
