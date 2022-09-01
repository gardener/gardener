// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package conditions

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of this controller.
const ControllerName = "shoot-conditions"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Shoot{}, builder.WithPredicates(r.ShootPredicate())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
			RecoverPanic:            true,
		}).
		Build(r)
	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &gardencorev1beta1.Seed{}},
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapSeedToShoots), mapper.UpdateWithNew, c.GetLogger()),
		r.SeedPredicate(),
	)
}

// ShootPredicate reacts only on 'CREATE' Shoot events.
func (r *Reconciler) ShootPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
	}
}

// SeedPredicate reacts on Seed events that indicate that the conditions of the registered Seed changed.
func (r *Reconciler) SeedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return true },
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

// MapSeedToShoots is a mapper.MapFunc for mapping seeds to shoots managed by ManagedSeeds.
func (r *Reconciler) MapSeedToShoots(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return nil
	}

	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := reader.Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, seed.Name), managedSeed); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get ManagedSeed for Seed", "seed", client.ObjectKeyFromObject(seed))
		}
		return nil
	}

	if managedSeed.Spec.Shoot == nil {
		return nil
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := reader.Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get Shoot for ManagedSeed", "managedSeed", client.ObjectKeyFromObject(managedSeed))
		}
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: shoot.Namespace, Name: shoot.Name}}}
}
