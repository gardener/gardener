// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oscommon

import (
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/actuator"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
func AddToManagerWithOptions(mgr manager.Manager, ctrlName string, osTypes []string, generator generator.Generator, opts AddOptions) error {
	return operatingsystemconfig.Add(mgr, operatingsystemconfig.AddArgs{
		Actuator:          actuator.NewActuator(ctrlName, generator),
		Predicates:        operatingsystemconfig.DefaultPredicates(opts.IgnoreOperationAnnotation),
		Types:             osTypes,
		ControllerOptions: opts.Controller,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(mgr manager.Manager, ctrlName string, osTypes []string, generator generator.Generator) error {
	return AddToManagerWithOptions(mgr, ctrlName, osTypes, generator, DefaultAddOptions)
}
