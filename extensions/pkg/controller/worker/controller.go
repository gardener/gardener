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

package worker

import (
	extensionshandler "github.com/gardener/gardener/extensions/pkg/handler"
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// FinalizerName is the worker controller finalizer.
	FinalizerName = "extensions.gardener.cloud/worker"
	// ControllerName is the name of the controller.
	ControllerName = "worker_controller"
	// StateUpdatingControllerName is the name of the controller responsible for updating the worker's state.
	StateUpdatingControllerName = "worker_state_controller"
)

// AddArgs are arguments for adding an worker controller to a manager.
type AddArgs struct {
	// Actuator is an worker actuator.
	Actuator Actuator
	// ControllerOptions are the controller options used for creating a controller.
	// The options.Reconciler is always overridden with a reconciler created from the
	// given actuator.
	ControllerOptions controller.Options
	// Predicates are the predicates to use.
	// If unset, GenerationChangedPredicate will be used.
	Predicates []predicate.Predicate
	// Type is the type of the resource considered for reconciliation.
	Type string
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	// If the annotation is not ignored, the extension controller will only reconcile
	// with a present operation annotation typically set during a reconcile (e.g in the maintenance time) by the Gardenlet
	IgnoreOperationAnnotation bool
}

// DefaultPredicates returns the default predicates for a Worker reconciler.
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
		extensionspredicate.ShootNotFailed(),
		extensionspredicate.Or(
			extensionspredicate.HasOperationAnnotation(),
			predicate.GenerationChangedPredicate{},
		),
	}
}

// Add creates a new Worker Controller and adds it to the Manager.
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(mgr, args.Actuator)
	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Type)
	if err := add(mgr, args, predicates); err != nil {
		return err
	}

	return addStateUpdatingController(mgr, args.ControllerOptions, args.Type)
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, args AddArgs, predicates []predicate.Predicate) error {
	ctrl, err := controller.New(ControllerName, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	if args.IgnoreOperationAnnotation {
		if err := ctrl.Watch(&source.Kind{Type: &extensionsv1alpha1.Cluster{}}, &extensionshandler.EnqueueRequestsFromMapFunc{
			ToRequests: extensionshandler.SimpleMapper(ClusterToWorkerMapper(predicates), extensionshandler.UpdateWithNew),
		}); err != nil {
			return err
		}
	}

	return ctrl.Watch(&source.Kind{Type: &extensionsv1alpha1.Worker{}}, &handler.EnqueueRequestForObject{}, predicates...)
}

func addStateUpdatingController(mgr manager.Manager, options controller.Options, extensionType string) error {
	var (
		stateActuator   = NewStateActuator(log.Log.WithName("worker-state-actuator"))
		stateReconciler = NewStateReconciler(mgr, stateActuator)

		addStateUpdatingControllerOptions = controller.Options{
			MaxConcurrentReconciles: options.MaxConcurrentReconciles,
			Reconciler:              stateReconciler,
		}

		machinePredicates = []predicate.Predicate{
			extensionspredicate.Or(
				MachineStatusHasChanged(),
				predicate.GenerationChangedPredicate{},
			),
		}
		workerPredicates = []predicate.Predicate{
			extensionspredicate.HasType(extensionType),
		}
	)

	ctrl, err := controller.New(StateUpdatingControllerName, mgr, addStateUpdatingControllerOptions)
	if err != nil {
		return err
	}

	if err := ctrl.Watch(&source.Kind{Type: &machinev1alpha1.MachineSet{}}, &extensionshandler.EnqueueRequestsFromMapFunc{
		ToRequests: extensionshandler.SimpleMapper(MachineSetToWorkerMapper(workerPredicates), extensionshandler.UpdateWithNew),
	}, machinePredicates...); err != nil {
		return err
	}

	return ctrl.Watch(&source.Kind{Type: &machinev1alpha1.Machine{}}, &extensionshandler.EnqueueRequestsFromMapFunc{
		ToRequests: extensionshandler.SimpleMapper(MachineToWorkerMapper(workerPredicates), extensionshandler.UpdateWithNew),
	}, machinePredicates...)
}
