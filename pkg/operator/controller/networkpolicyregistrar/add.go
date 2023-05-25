// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicyregistrar

import (
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// ControllerName is the name of this controller.
const ControllerName = "networkpolicy-controller-registrar"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Manager == nil {
		r.Manager = mgr
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&operatorv1alpha1.Garden{}, builder.WithPredicates(NetworkingPredicate())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

// NetworkingPredicate returns true for Create and Update events if the
// garden.Spec.RuntimeCluster.Networking.{Pods,Services} fields are not empty
func NetworkingPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return networkingFieldsSet(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return networkingFieldsSet(e.ObjectNew)
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func networkingFieldsSet(obj client.Object) bool {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return false
	}

	return garden.Spec.RuntimeCluster.Networking.Pods != "" && garden.Spec.RuntimeCluster.Networking.Services != ""
}
