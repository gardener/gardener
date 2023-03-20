// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controlplane

import (
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
)

const (
	// FinalizerName is the controlplane controller finalizer.
	FinalizerName = "extensions.gardener.cloud/controlplane"
	// ControllerName is the name of the controller
	ControllerName = "controlplane"
)

// AddArgs are arguments for adding an controlplane controller to a manager.
type AddArgs struct {
	// Actuator is an controlplane actuator.
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

// DefaultPredicates returns the default predicates for a controlplane reconciler.
func DefaultPredicates(ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation, extensionspredicate.ShootNotFailedPredicate())
}

// Add creates a new ControlPlane Controller and adds it to the Manager.
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(args.Actuator)

	ctrl, err := controller.New(ControllerName, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Type)
	if args.IgnoreOperationAnnotation {
		if err := ctrl.Watch(
			&source.Kind{Type: &extensionsv1alpha1.Cluster{}},
			mapper.EnqueueRequestsFrom(ClusterToControlPlaneMapper(predicates), mapper.UpdateWithNew, ctrl.GetLogger()),
		); err != nil {
			return err
		}
	}

	return ctrl.Watch(&source.Kind{Type: &extensionsv1alpha1.ControlPlane{}}, &handler.EnqueueRequestForObject{}, predicates...)
}
