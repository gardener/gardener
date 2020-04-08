// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
