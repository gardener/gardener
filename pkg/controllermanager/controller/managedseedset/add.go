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

package managedseedset

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
)

// ControllerName is the name of this controller.
const ControllerName = "managedseedset"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.Actuator == nil {
		replicaFactory := ReplicaFactoryFunc(NewReplica)
		replicaGetter := NewReplicaGetter(r.Client, mgr.GetAPIReader(), replicaFactory)
		r.Actuator = NewActuator(r.Client, replicaGetter, replicaFactory, &r.Config, mgr.GetEventRecorderFor(ControllerName+"-controller"))
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&seedmanagementv1alpha1.ManagedSeedSet{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
			RecoverPanic:            true,
		}).
		Build(r)
	if err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &gardencorev1beta1.Shoot{}},
		&handler.EnqueueRequestForOwner{
			OwnerType: &seedmanagementv1alpha1.ManagedSeedSet{},
		},
		r.ShootPredicate(),
	); err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &seedmanagementv1alpha1.ManagedSeed{}},
		&handler.EnqueueRequestForOwner{
			OwnerType: &seedmanagementv1alpha1.ManagedSeedSet{},
		},
		r.ManagedSeedPredicate(),
	); err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &gardencorev1beta1.Seed{}},
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapSeedToManagedSeedSet), mapper.UpdateWithNew, c.GetLogger()),
		r.SeedPredicate(),
	)
}

// MapSeedToManagedSeedSet is a mapper.MapFunc for mapping Seeds to referencing ManagedSeedSet.
func (r *Reconciler) MapSeedToManagedSeedSet(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
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

	setName := GetManagedSeedSetNameFromOwnerReferences(managedSeed)
	if len(setName) == 0 {
		return nil
	}

	set := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := reader.Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, setName), set); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get ManagedSeedSet for ManagedSeed", "managedseed", client.ObjectKeyFromObject(managedSeed))
		}
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: set.Namespace, Name: set.Name}}}
}
