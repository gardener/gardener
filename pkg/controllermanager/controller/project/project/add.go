// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
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

	shootMeta := &metav1.PartialObjectMetadata{}
	shootMeta.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Project{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Namespace{}, builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Delete))).
		Owns(&rbacv1.RoleBinding{}, builder.WithPredicates(r.RoleBindingPredicate())).
		Watches(
			shootMeta,
			handler.EnqueueRequestsFromMapFunc(r.MapShootToProjectInDeletion(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Delete)),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			RateLimiter:             r.RateLimiter,
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		Complete(r)
}

// MapShootToProjectInDeletion returns a mapper that returns requests for the Project to which the Shoot belongs,
// if the Project is being deleted and does not contain any other Shoots.
func (r *Reconciler) MapShootToProjectInDeletion(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		shoot, ok := obj.(*metav1.PartialObjectMetadata)
		if !ok {
			return nil
		}

		projectNamespaceStillContainsShoots, err := kubernetesutils.ResourcesExist(ctx, r.Client, &gardencorev1beta1.ShootList{}, r.Client.Scheme(), client.InNamespace(shoot.Namespace))
		if err != nil {
			log.Error(err, "Failed to check if namespace still contains shoots", "namespace", shoot.Namespace)
			return nil
		}

		if projectNamespaceStillContainsShoots {
			return nil
		}

		project, _, err := gardenerutils.ProjectAndNamespaceFromReader(ctx, r.Client, shoot.Namespace)
		if err != nil {
			log.Error(err, "Failed to get project for namespace", "namespace", shoot.Namespace)
			return nil
		}

		if project.DeletionTimestamp == nil {
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: project.Name}}}
	}
}

// RoleBindingPredicate filters for events for RoleBindings that we might need to reconcile back.
func (r *Reconciler) RoleBindingPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
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
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
