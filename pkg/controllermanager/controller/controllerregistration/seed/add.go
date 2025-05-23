// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"

	"github.com/go-logr/logr"
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

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "controllerregistration-seed"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.APIReader == nil {
		r.APIReader = mgr.GetAPIReader()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Seed{}, builder.WithPredicates(r.SeedPredicate())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			&gardencorev1beta1.ControllerRegistration{},
			handler.EnqueueRequestsFromMapFunc(r.MapToAllSeeds(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		Watches(
			&gardencorev1beta1.BackupBucket{},
			handler.EnqueueRequestsFromMapFunc(r.MapBackupBucketToSeed),
			builder.WithPredicates(r.BackupBucketPredicate()),
		).
		Watches(
			&gardencorev1beta1.BackupEntry{},
			handler.EnqueueRequestsFromMapFunc(r.MapBackupEntryToSeed),
			builder.WithPredicates(r.BackupEntryPredicate()),
		).
		Watches(
			&gardencorev1beta1.ControllerInstallation{},
			handler.EnqueueRequestsFromMapFunc(r.MapControllerInstallationToSeed),
			builder.WithPredicates(r.ControllerInstallationPredicate()),
		).
		Watches(
			&gardencorev1.ControllerDeployment{},
			handler.EnqueueRequestsFromMapFunc(r.MapControllerDeploymentToAllSeeds(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		Watches(
			&gardencorev1beta1.Shoot{},
			handler.EnqueueRequestsFromMapFunc(r.MapShootToSeed),
			builder.WithPredicates(r.ShootPredicate()),
		).
		Complete(r)
}

// SeedPredicate returns true for all Seed events except for updates. Here, it returns true when there is a change
// in the spec or labels or annotations or when the deletion timestamp is set.
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

			return !apiequality.Semantic.DeepEqual(oldSeed.Spec, seed.Spec) ||
				!apiequality.Semantic.DeepEqual(oldSeed.Annotations, seed.Annotations) ||
				!apiequality.Semantic.DeepEqual(oldSeed.Labels, seed.Labels) ||
				seed.DeletionTimestamp != nil
		},
	}
}

// BackupBucketPredicate returns true for all BackupBucket events when there is a non-nil .spec.seedName. For updates,
// it only returns true when there is a change in the .spec.seedName or .spec.provider.type fields.
func (r *Reconciler) BackupBucketPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			backupBucket, ok := e.Object.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return false
			}
			return backupBucket.Spec.SeedName != nil
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			backupBucket, ok := e.ObjectNew.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return false
			}

			oldBackupBucket, ok := e.ObjectOld.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldBackupBucket.Spec.SeedName, backupBucket.Spec.SeedName) ||
				oldBackupBucket.Spec.Provider.Type != backupBucket.Spec.Provider.Type
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			backupBucket, ok := e.Object.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return false
			}
			return backupBucket.Spec.SeedName != nil
		},
	}
}

// BackupEntryPredicate returns true for all BackupEntry events when there is a non-nil .spec.seedName. For updates,
// it only returns true when there is a change in the .spec.seedName or .spec.bucketName fields.
func (r *Reconciler) BackupEntryPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			backupEntry, ok := e.Object.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return false
			}
			return backupEntry.Spec.SeedName != nil
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			backupEntry, ok := e.ObjectNew.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return false
			}

			oldBackupEntry, ok := e.ObjectOld.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldBackupEntry.Spec.SeedName, backupEntry.Spec.SeedName) ||
				oldBackupEntry.Spec.BucketName != backupEntry.Spec.BucketName
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			backupEntry, ok := e.Object.(*gardencorev1beta1.BackupEntry)
			if !ok {
				return false
			}
			return backupEntry.Spec.SeedName != nil
		},
	}
}

// ShootPredicate returns true for all Shoot events when there is a non-nil .spec.seedName. For updates, it only returns
// true when there is a change in the .spec.seedName or .spec.provider.workers or .spec.extensions or .spec.dns or
// .spec.networking.type or .spec.provider.type fields.
func (r *Reconciler) ShootPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			shoot, ok := e.Object.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}
			return shoot.Spec.SeedName != nil
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			shoot, ok := e.ObjectNew.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			oldShoot, ok := e.ObjectOld.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			return !apiequality.Semantic.DeepEqual(oldShoot.Spec.SeedName, shoot.Spec.SeedName) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.Extensions, shoot.Spec.Extensions) ||
				!apiequality.Semantic.DeepEqual(oldShoot.Spec.DNS, shoot.Spec.DNS) ||
				shootNetworkingTypeHasChanged(oldShoot.Spec.Networking, shoot.Spec.Networking) ||
				oldShoot.Spec.Provider.Type != shoot.Spec.Provider.Type
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			shoot, ok := e.Object.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}
			return shoot.Spec.SeedName != nil
		},
	}
}

// ControllerInstallationPredicate returns true for all ControllerInstallation 'create' events. For updates, it only
// returns true when the Required condition's status has changed. For other events, false is returned.
func (r *Reconciler) ControllerInstallationPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			controllerInstallation, ok := e.ObjectNew.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}

			oldControllerInstallation, ok := e.ObjectOld.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}

			return v1beta1helper.IsControllerInstallationRequired(*oldControllerInstallation) != v1beta1helper.IsControllerInstallationRequired(*controllerInstallation)
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// MapToAllSeeds returns reconcile.Request objects for all existing seeds in the system.
func (r *Reconciler) MapToAllSeeds(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		seedList := &metav1.PartialObjectMetadataList{}
		seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
		if err := r.Client.List(ctx, seedList); err != nil {
			log.Error(err, "Failed to list seeds")
			return nil
		}

		return mapper.ObjectListToRequests(seedList)
	}
}

// MapBackupBucketToSeed returns a reconcile.Request object for the seed specified in the .spec.seedName field.
func (r *Reconciler) MapBackupBucketToSeed(_ context.Context, obj client.Object) []reconcile.Request {
	backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return nil
	}

	if backupBucket.Spec.SeedName == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: *backupBucket.Spec.SeedName}}}
}

// MapBackupEntryToSeed returns a reconcile.Request object for the seed specified in the .spec.seedName field.
func (r *Reconciler) MapBackupEntryToSeed(_ context.Context, obj client.Object) []reconcile.Request {
	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return nil
	}

	if backupEntry.Spec.SeedName == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: *backupEntry.Spec.SeedName}}}
}

// MapShootToSeed returns a reconcile.Request object for the seed specified in the .spec.seedName field.
func (r *Reconciler) MapShootToSeed(_ context.Context, obj client.Object) []reconcile.Request {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return nil
	}

	if shoot.Spec.SeedName == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: *shoot.Spec.SeedName}}}
}

// MapControllerInstallationToSeed returns a reconcile.Request object for the seed specified in the .spec.seedRef.name field.
func (r *Reconciler) MapControllerInstallationToSeed(_ context.Context, obj client.Object) []reconcile.Request {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: controllerInstallation.Spec.SeedRef.Name}}}
}

// MapControllerDeploymentToAllSeeds returns reconcile.Request objects for all seeds in case there is at least one
// ControllerRegistration which references the ControllerDeployment.
func (r *Reconciler) MapControllerDeploymentToAllSeeds(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		controllerDeployment, ok := obj.(*gardencorev1.ControllerDeployment)
		if !ok {
			return nil
		}

		controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
		if err := r.Client.List(ctx, controllerRegistrationList); err != nil {
			log.Error(err, "Failed to list ControllerRegistrations")
			return nil
		}

		for _, controllerReg := range controllerRegistrationList.Items {
			if controllerReg.Spec.Deployment == nil {
				continue
			}

			for _, ref := range controllerReg.Spec.Deployment.DeploymentRefs {
				if ref.Name == controllerDeployment.Name {
					return r.MapToAllSeeds(log)(ctx, nil)
				}
			}
		}

		return nil
	}
}

func shootNetworkingTypeHasChanged(old, new *gardencorev1beta1.Networking) bool {
	if old == nil && new == nil {
		return false
	}
	if old == nil && new != nil {
		// if new is non-nil then return true if new has a type set
		return new.Type != nil
	}
	if old != nil && new == nil {
		// if old was non-nil and had a type set, return true
		return old.Type != nil
	}
	return !ptr.Equal(old.Type, new.Type)
}
