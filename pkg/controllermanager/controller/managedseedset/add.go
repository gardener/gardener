// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ControllerName is the name of this controller.
const ControllerName = "managedseedset"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.Actuator == nil {
		replicaFactory := ReplicaFactoryFunc(NewReplica)
		replicaGetter := NewReplicaGetter(r.Client, mgr.GetAPIReader(), replicaFactory)
		r.Actuator = NewActuator(r.Client, replicaGetter, replicaFactory, &r.Config, mgr.GetEventRecorderFor(ControllerName+"-controller"))
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&seedmanagementv1alpha1.ManagedSeedSet{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			&gardencorev1beta1.Shoot{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &seedmanagementv1alpha1.ManagedSeedSet{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(r.ShootPredicate(ctx)),
		).
		Watches(
			&seedmanagementv1alpha1.ManagedSeed{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &seedmanagementv1alpha1.ManagedSeedSet{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(r.ManagedSeedPredicate(ctx)),
		).
		Watches(
			&gardencorev1beta1.Seed{},
			handler.EnqueueRequestsFromMapFunc(r.MapSeedToManagedSeedSet(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(r.SeedPredicate(ctx)),
		).
		Complete(r)
}

// ShootPredicate returns the predicate for Shoot events.
func (r *Reconciler) ShootPredicate(ctx context.Context) predicate.Predicate {
	return &shootPredicate{
		ctx:    ctx,
		reader: r.Client,
	}
}

type shootPredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *shootPredicate) Create(e event.CreateEvent) bool { return p.filterShoot(e.Object) }

func (p *shootPredicate) Update(e event.UpdateEvent) bool {
	shoot, ok := e.ObjectNew.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	oldShoot, ok := e.ObjectOld.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	if !reflect.DeepEqual(shoot.DeletionTimestamp, oldShoot.DeletionTimestamp) || shootHealthStatus(shoot) != shootHealthStatus(oldShoot) {
		return true
	}

	return p.filterShoot(e.ObjectNew)
}

func (p *shootPredicate) Delete(e event.DeleteEvent) bool {
	shoot, ok := e.Object.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	managedSeedSetPendingReplicaReason, ok := p.getManagedSeedSetPendingReplicaReason(shoot)
	if !ok {
		return false
	}

	if managedSeedSetPendingReplicaReason == seedmanagementv1alpha1.ShootDeletingReason {
		return true
	}

	return p.filterShoot(e.Object)
}

func (p *shootPredicate) Generic(_ event.GenericEvent) bool { return false }

// Return true only if the Shoot belongs to the pending replica and it progressed from the state
// that caused the replica to be pending
func (p *shootPredicate) filterShoot(obj client.Object) bool {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	managedSeedSetPendingReplicaReason, ok := p.getManagedSeedSetPendingReplicaReason(shoot)
	if !ok {
		return false
	}

	switch managedSeedSetPendingReplicaReason {
	case seedmanagementv1alpha1.ShootReconcilingReason:
		return shootReconcileFailed(shoot) || shootReconcileSucceeded(shoot) || shoot.DeletionTimestamp != nil
	case seedmanagementv1alpha1.ShootDeletingReason:
		return shootDeleteFailed(shoot)
	case seedmanagementv1alpha1.ShootReconcileFailedReason:
		return !shootReconcileFailed(shoot)
	case seedmanagementv1alpha1.ShootDeleteFailedReason:
		return !shootDeleteFailed(shoot)
	case seedmanagementv1alpha1.ShootNotHealthyReason:
		return shootHealthStatus(shoot) == gardenerutils.ShootStatusHealthy
	default:
		return false
	}
}

func (p *shootPredicate) getManagedSeedSetPendingReplicaReason(shoot *gardencorev1beta1.Shoot) (seedmanagementv1alpha1.PendingReplicaReason, bool) {
	controllerRef := metav1.GetControllerOf(shoot)
	if controllerRef == nil {
		return "", false
	}

	managedSeedSet := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := p.reader.Get(p.ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: controllerRef.Name}, managedSeedSet); err != nil {
		return "", false
	}

	if managedSeedSet.Status.PendingReplica == nil || managedSeedSet.Status.PendingReplica.Name != shoot.Name {
		return "", false
	}

	return managedSeedSet.Status.PendingReplica.Reason, true
}

// ManagedSeedPredicate returns the predicate for ManagedSeed events.
func (r *Reconciler) ManagedSeedPredicate(ctx context.Context) predicate.Predicate {
	return &managedSeedPredicate{
		ctx:    ctx,
		reader: r.Client,
	}
}

type managedSeedPredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *managedSeedPredicate) Create(e event.CreateEvent) bool {
	return p.filterManagedSeed(e.Object)
}

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

	return p.filterManagedSeed(e.ObjectNew)
}

func (p *managedSeedPredicate) Delete(e event.DeleteEvent) bool {
	managedSeed, ok := e.Object.(*seedmanagementv1alpha1.ManagedSeed)
	if !ok {
		return false
	}

	managedSeedSetPendingReplicaReason, ok := p.getManagedSeedSetPendingReplicaReason(managedSeed)
	if !ok {
		return false
	}

	if managedSeedSetPendingReplicaReason == seedmanagementv1alpha1.ManagedSeedDeletingReason {
		return true
	}

	return p.filterManagedSeed(e.Object)
}

func (p *managedSeedPredicate) Generic(_ event.GenericEvent) bool { return false }

// Returns true only if the ManagedSeed belongs to the pending replica and it progressed from the state
// that caused the replica to be pending.
func (p *managedSeedPredicate) filterManagedSeed(obj client.Object) bool {
	managedSeed, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
	if !ok {
		return false
	}

	managedSeedSetPendingReplicaReason, ok := p.getManagedSeedSetPendingReplicaReason(managedSeed)
	if !ok {
		return false
	}

	switch managedSeedSetPendingReplicaReason {
	case seedmanagementv1alpha1.ManagedSeedPreparingReason:
		return managedSeedRegistered(managedSeed) || managedSeed.DeletionTimestamp != nil
	default:
		return false
	}
}

func (p *managedSeedPredicate) getManagedSeedSetPendingReplicaReason(managedSeed *seedmanagementv1alpha1.ManagedSeed) (seedmanagementv1alpha1.PendingReplicaReason, bool) {
	controllerRef := metav1.GetControllerOf(managedSeed)
	if controllerRef == nil {
		return "", false
	}

	managedSeedSet := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := p.reader.Get(p.ctx, client.ObjectKey{Namespace: managedSeed.Namespace, Name: controllerRef.Name}, managedSeedSet); err != nil {
		return "", false
	}

	if managedSeedSet.Status.PendingReplica == nil || managedSeedSet.Status.PendingReplica.Name != managedSeed.Name {
		return "", false
	}

	return managedSeedSet.Status.PendingReplica.Reason, true
}

// SeedPredicate returns the predicate for Seed events.
func (r *Reconciler) SeedPredicate(ctx context.Context) predicate.Predicate {
	return &seedPredicate{
		ctx:    ctx,
		reader: r.Client,
	}
}

type seedPredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *seedPredicate) Create(e event.CreateEvent) bool { return p.filterSeed(e.Object) }

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

	return p.filterSeed(e.ObjectNew)
}

func (p *seedPredicate) Delete(e event.DeleteEvent) bool { return p.filterSeed(e.Object) }

func (p *seedPredicate) Generic(_ event.GenericEvent) bool { return false }

// Returns true only if the Seed belongs to the pending replica and it progressed from the state
// that caused the replica to be pending.
func (p *seedPredicate) filterSeed(obj client.Object) bool {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}

	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := p.reader.Get(p.ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: seed.Name}, managedSeed); err != nil {
		return false
	}

	controllerRef := metav1.GetControllerOf(managedSeed)
	if controllerRef == nil {
		return false
	}

	managedSeedSet := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := p.reader.Get(p.ctx, client.ObjectKey{Namespace: seed.Namespace, Name: controllerRef.Name}, managedSeedSet); err != nil {
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

// MapSeedToManagedSeedSet is a handler.MapFunc for mapping Seeds to referencing ManagedSeedSet.
func (r *Reconciler) MapSeedToManagedSeedSet(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		seed, ok := obj.(*gardencorev1beta1.Seed)
		if !ok {
			return nil
		}

		managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: seed.Name}, managedSeed); err != nil {
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
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: managedSeed.Namespace, Name: controllerRef.Name}, managedSeedSet); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to get ManagedSeedSet for ManagedSeed", "managedseed", client.ObjectKeyFromObject(managedSeed))
			}
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: managedSeedSet.Namespace, Name: managedSeedSet.Name}}}
	}
}
