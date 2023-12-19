// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package oscommon

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/actuator"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"
)

// DefaultAddOptions are the default AddOptions for AddToManager.
var DefaultAddOptions = AddOptions{}

// AddOptions are options to apply when adding the OSC controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
// Deprecated: The `oscommon` package is deprecated and will be removed as soon as the UseGardenerNodeAgent feature gate
// has been promoted to GA.
// TODO(rfranzke): Remove the `oscommon` package after the UseGardenerNodeAgent feature gate has been promoted to GA.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, ctrlName string, osTypes []string, generator generator.Generator, opts AddOptions) error {
	return operatingsystemconfig.Add(mgr, operatingsystemconfig.AddArgs{
		Actuator:          actuator.NewActuator(mgr, ctrlName, generator),
		Predicates:        operatingsystemconfig.DefaultPredicates(ctx, mgr, opts.IgnoreOperationAnnotation),
		Types:             osTypes,
		ControllerOptions: opts.Controller,
	})
}

// AddToManager adds a controller with the default Options.
// Deprecated: The `oscommon` package is deprecated and will be removed as soon as the UseGardenerNodeAgent feature gate
// has been promoted to GA.
// TODO(rfranzke): Remove the `oscommon` package after the UseGardenerNodeAgent feature gate has been promoted to GA.
func AddToManager(ctx context.Context, mgr manager.Manager, ctrlName string, osTypes []string, generator generator.Generator) error {
	return AddToManagerWithOptions(ctx, mgr, ctrlName, osTypes, generator, DefaultAddOptions)
}
