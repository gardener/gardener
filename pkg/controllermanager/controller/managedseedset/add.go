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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	operationshoot "github.com/gardener/gardener/pkg/operation/shoot"
	contextutil "github.com/gardener/gardener/pkg/utils/context"
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
		For(&seedmanagementv1alpha1.ManagedSeedSet{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
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
			OwnerType:    &seedmanagementv1alpha1.ManagedSeedSet{},
			IsController: true,
		},
		r.ShootPredicate(),
	); err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &seedmanagementv1alpha1.ManagedSeed{}},
		&handler.EnqueueRequestForOwner{
			OwnerType:    &seedmanagementv1alpha1.ManagedSeedSet{},
			IsController: true,
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

// ShootPredicate returns the predicate for Shoot events.
// ShootPredicate reacts only on 'CREATE','UPDATE' and 'DELETE' events. It returns true when shoot is deleted.
// For updates,returns true if:
// - shoot's health status changed.
// - shoot belongs to the pending replica and it progressed from the state that caused the replica to be pending.
func (r *Reconciler) ShootPredicate() predicate.Predicate {
	return &shootPredicate{}
}

type shootPredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *shootPredicate) InjectStopChannel(stopChan <-chan struct{}) error {
	p.ctx = contextutil.FromStopChannel(stopChan)
	return nil
}

func (p *shootPredicate) InjectClient(client client.Client) error {
	p.reader = client
	return nil
}

func (p *shootPredicate) Create(_ event.CreateEvent) bool { return true }

func (p *shootPredicate) Update(e event.UpdateEvent) bool {
	shoot, ok := e.ObjectNew.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	oldShoot, ok := e.ObjectOld.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	controllerRef := metav1.GetControllerOf(shoot)
	if controllerRef == nil {
		return false
	}

	managedSeedSet := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := p.reader.Get(p.ctx, kutil.Key(shoot.Namespace, controllerRef.Name), managedSeedSet); err != nil {
		return false
	}

	if managedSeedSet.Status.PendingReplica == nil || managedSeedSet.Status.PendingReplica.Name != shoot.Name {
		return false
	}

	if !reflect.DeepEqual(shoot.DeletionTimestamp, oldShoot.DeletionTimestamp) || shootHealthStatus(shoot) != shootHealthStatus(oldShoot) {
		return true
	}

	switch managedSeedSet.Status.PendingReplica.Reason {
	case seedmanagementv1alpha1.ShootReconcilingReason:
		return shootReconcileFailed(shoot) || shootReconcileSucceeded(shoot)
	case seedmanagementv1alpha1.ShootDeletingReason:
		return shootDeleteFailed(shoot)
	case seedmanagementv1alpha1.ShootReconcileFailedReason:
		return !shootReconcileFailed(shoot)
	case seedmanagementv1alpha1.ShootDeleteFailedReason:
		return !shootDeleteFailed(shoot)
	case seedmanagementv1alpha1.ShootNotHealthyReason:
		return shootHealthStatus(shoot) == operationshoot.StatusHealthy
	default:
		return false
	}
}

func (p *shootPredicate) Delete(_ event.DeleteEvent) bool { return true }

func (p *shootPredicate) Generic(_ event.GenericEvent) bool { return false }

// ManagedSeedPredicate returns the predicate for ManagedSeed events.
// ManagedSeedPredicate reacts only on 'CREATE','UPDATE' and 'DELETE' events. It returns true when shoot is deleted.
// For updates,returns true only if the managed seed belongs to the pending replica and it progressed from the state
// that caused the replica to be pending.
func (r *Reconciler) ManagedSeedPredicate() predicate.Predicate {
	return &managedSeedPredicate{}
}

type managedSeedPredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *managedSeedPredicate) InjectStopChannel(stopChan <-chan struct{}) error {
	p.ctx = contextutil.FromStopChannel(stopChan)
	return nil
}

func (p *managedSeedPredicate) InjectClient(client client.Client) error {
	p.reader = client
	return nil
}

func (p *managedSeedPredicate) Create(_ event.CreateEvent) bool { return true }

func (p *managedSeedPredicate) Update(e event.UpdateEvent) bool {
	managedSeed, ok := e.ObjectNew.(*seedmanagementv1alpha1.ManagedSeed)
	if !ok {
		return false
	}

	oldManagedSeed, ok := e.ObjectOld.(*seedmanagementv1alpha1.ManagedSeed)
	if !ok {
		return false
	}

	if !reflect.DeepEqual(managedSeed.DeletionTimestamp, oldManagedSeed.DeletionTimestamp) {
		return true
	}

	controllerRef := metav1.GetControllerOf(managedSeed)
	if controllerRef == nil {
		return false
	}

	managedSeedSet := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := p.reader.Get(p.ctx, kutil.Key(managedSeed.Namespace, controllerRef.Name), managedSeedSet); err != nil {
		return false
	}

	if managedSeedSet.Status.PendingReplica == nil || managedSeedSet.Status.PendingReplica.Name != managedSeed.Name {
		return false
	}

	switch managedSeedSet.Status.PendingReplica.Reason {
	case seedmanagementv1alpha1.ManagedSeedPreparingReason:
		return managedSeedRegistered(managedSeed) || managedSeed.DeletionTimestamp != nil
	default:
		return false
	}
}

func (p *managedSeedPredicate) Delete(_ event.DeleteEvent) bool { return true }

func (p *managedSeedPredicate) Generic(_ event.GenericEvent) bool { return false }

// SeedPredicate returns the predicate for Seed events.
// SeedPredicate reacts only on 'CREATE' and 'UPDATE'events. It returns true if the seed readiness changed.
// For updates,returns true only if the seed belongs to the pending replica and it progressed from the state
// that caused the replica to be pending.
func (r *Reconciler) SeedPredicate() predicate.Predicate {
	return &seedPredicate{}
}

type seedPredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *seedPredicate) InjectStopChannel(stopChan <-chan struct{}) error {
	p.ctx = contextutil.FromStopChannel(stopChan)
	return nil
}

func (p *seedPredicate) InjectClient(client client.Client) error {
	p.reader = client
	return nil
}

func (p *seedPredicate) Create(_ event.CreateEvent) bool { return true }

func (p *seedPredicate) Update(e event.UpdateEvent) bool {
	seed, ok := e.ObjectNew.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}

	oldSeed, ok := e.ObjectOld.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}

	if seedReady(seed) != seedReady(oldSeed) {
		return true
	}

	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := p.reader.Get(p.ctx, kutil.Key(v1beta1constants.GardenNamespace, seed.Name), managedSeed); err != nil {
		return false
	}

	controllerRef := metav1.GetControllerOf(managedSeed)
	if controllerRef == nil {
		return false
	}

	managedSeedSet := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := p.reader.Get(p.ctx, kutil.Key(seed.Namespace, controllerRef.Name), managedSeedSet); err != nil {
		return false
	}

	if managedSeedSet.Status.PendingReplica == nil || managedSeedSet.Status.PendingReplica.Name != seed.Name {
		return false
	}

	switch managedSeedSet.Status.PendingReplica.Reason {
	case seedmanagementv1alpha1.SeedNotReadyReason:
		return seedReady(seed)
	default:
		return false
	}
}

func (p *seedPredicate) Delete(_ event.DeleteEvent) bool { return false }

func (p *seedPredicate) Generic(_ event.GenericEvent) bool { return false }

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

	controllerRef := metav1.GetControllerOf(managedSeed)
	if controllerRef == nil {
		return nil
	}

	managedSeedSet := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := reader.Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, controllerRef.Name), managedSeedSet); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get ManagedSeedSet for ManagedSeed", "managedseed", client.ObjectKeyFromObject(managedSeed))
		}
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: managedSeedSet.Namespace, Name: managedSeedSet.Name}}}
}
