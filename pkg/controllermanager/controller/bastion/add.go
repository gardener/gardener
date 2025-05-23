// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/clock"
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
	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "bastion"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&operationsv1alpha1.Bastion{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			&gardencorev1beta1.Shoot{},
			handler.EnqueueRequestsFromMapFunc(r.MapShootToBastions(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(r.ShootPredicate()),
		).
		Complete(r)
}

// ShootPredicate returns the predicate for Shoot events.
func (r *Reconciler) ShootPredicate() predicate.Predicate {
	return predicate.Or(
		predicateutils.IsDeleting(),
		predicate.Funcs{
			CreateFunc:  func(_ event.CreateEvent) bool { return false },
			DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
			GenericFunc: func(_ event.GenericEvent) bool { return false },
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldShoot, ok := e.ObjectOld.(*gardencorev1beta1.Shoot)
				if !ok {
					return false
				}
				newShoot, ok := e.ObjectNew.(*gardencorev1beta1.Shoot)
				if !ok {
					return false
				}

				if oldShoot.Spec.SeedName == nil {
					return false
				}

				return !apiequality.Semantic.DeepEqual(oldShoot.Spec.SeedName, newShoot.Spec.SeedName)
			},
		},
	)
}

// MapShootToBastions is a handler.MapFunc for mapping shoots to referencing Bastions.
func (r *Reconciler) MapShootToBastions(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return nil
		}

		bastionList := &operationsv1alpha1.BastionList{}
		if err := r.Client.List(ctx, bastionList, client.InNamespace(shoot.Namespace), client.MatchingFields{operations.BastionShootName: shoot.Name}); err != nil {
			log.Error(err, "Failed to list Bastions for shoot", "shoot", client.ObjectKeyFromObject(shoot))
			return nil
		}

		return mapper.ObjectListToRequests(bastionList)
	}
}
