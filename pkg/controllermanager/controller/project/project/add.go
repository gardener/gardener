// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package project

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "project"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName + "-controller")
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Project{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Namespace{}, builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Delete))).
		Owns(&rbacv1.RoleBinding{}, builder.WithPredicates(r.RoleBindingPredicate())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			RateLimiter:             r.RateLimiter,
		}).
		Complete(r)
}

// RoleBindingPredicate filters for events for RoleBindings that we might need to reconcile back.
func (r *Reconciler) RoleBindingPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			// enqueue on periodic cache resyncs
			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				return true
			}

			roleBinding, ok := e.ObjectNew.(*rbacv1.RoleBinding)
			if !ok {
				return false
			}

			oldRoleBinding, ok := e.ObjectOld.(*rbacv1.RoleBinding)
			if !ok {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldRoleBinding.RoleRef, roleBinding.RoleRef) ||
				!apiequality.Semantic.DeepEqual(oldRoleBinding.Subjects, roleBinding.Subjects)
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}
