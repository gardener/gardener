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

package operatingsystemconfig

import (
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// FinalizerName is the name of the finalizer written by this controller.
	FinalizerName = "extensions.gardener.cloud/operatingsystemconfigs"

	// ControllerName is the name of the operating system configuration controller.
	ControllerName = "operatingsystemconfig_controller"

	name = "operatingsystemconfig-controller"
)

// AddArgs are arguments for adding an operatingsystemconfig controller to a manager.
type AddArgs struct {
	// Actuator is an operatingsystemconfig actuator.
	Actuator Actuator
	// ControllerOptions are the controller options used for creating a controller.
	// The options.Reconciler is always overridden with a reconciler created from the
	// given actuator.
	ControllerOptions controller.Options
	// Predicates are the predicates to use.
	// If unset, GenerationChangedPredicate will be used.
	Predicates []predicate.Predicate
	// Types are the similar types which can be combined with a logic or,
	// of the resource considered for reconciliation.
	Types []string
}

// Add adds an operatingsystemconfig controller to the given manager using the given AddArgs.
func Add(mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(args.Actuator)
	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Types...)
	return add(mgr, args.ControllerOptions, predicates)
}

// DefaultPredicates returns the default predicates for an operatingsystemconfig reconciler.
func DefaultPredicates(ignoreOperationAnnotation bool) []predicate.Predicate {
	if ignoreOperationAnnotation {
		return []predicate.Predicate{
			predicate.GenerationChangedPredicate{},
		}
	}

	return []predicate.Predicate{
		extensionspredicate.Or(
			extensionspredicate.HasOperationAnnotation(),
			extensionspredicate.LastOperationNotSuccessful(),
			extensionspredicate.IsDeleting(),
		),
	}
}

func add(mgr manager.Manager, options controller.Options, predicates []predicate.Predicate) error {
	ctrl, err := controller.New(ControllerName, mgr, options)
	if err != nil {
		return err
	}

	if err := ctrl.Watch(&source.Kind{Type: &extensionsv1alpha1.OperatingSystemConfig{}}, &handler.EnqueueRequestForObject{}, predicates...); err != nil {
		return err
	}

	return nil
}
