// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package conditions

import (
	"context"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "shoot-conditions"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Shoot{}, builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create))).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Build(r)
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind(mgr.GetCache(),
			&gardencorev1beta1.Seed{},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapSeedToShoot), mapper.UpdateWithNew, c.GetLogger()),
			r.SeedPredicate(),
		))
}

// SeedPredicate reacts on Seed events that indicate that the conditions of the registered Seed changed.
func (r *Reconciler) SeedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			seed, ok := e.ObjectNew.(*gardencorev1beta1.Seed)
			if !ok {
				return false
			}

			oldSeed, ok := e.ObjectOld.(*gardencorev1beta1.Seed)
			if !ok {
				return false
			}

			if !apiequality.Semantic.DeepEqual(seed.Status.Conditions, oldSeed.Status.Conditions) {
				return true
			}

			// We want to enqueue on periodic cache resync events to catch up if we missed updates.
			return seed.ResourceVersion == oldSeed.ResourceVersion
		},
	}
}

// MapSeedToShoot is a mapper.MapFunc for mapping a Seed to a Shoot in case it is managed by a ManagedSeed.
func (r *Reconciler) MapSeedToShoot(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return nil
	}

	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := reader.Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: seed.Name}, managedSeed); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get ManagedSeed for Seed", "seed", client.ObjectKeyFromObject(seed))
		}
		return nil
	}

	if managedSeed.Spec.Shoot == nil {
		return nil
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := reader.Get(ctx, client.ObjectKey{Namespace: managedSeed.Namespace, Name: managedSeed.Spec.Shoot.Name}, shoot); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get Shoot for ManagedSeed", "managedSeed", client.ObjectKeyFromObject(managedSeed))
		}
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: shoot.Namespace, Name: shoot.Name}}}
}
