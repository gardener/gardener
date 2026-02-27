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

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerinstallation"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "controllerinstallation-seed"

// AddToManager adds the ControllerInstallation Reconciler to the given manager.
func AddToManager(mgr manager.Manager, config controllermanagerconfigv1alpha1.ControllerRegistrationControllerConfiguration) error {
	var (
		log = mgr.GetLogger().WithValues("controller", ControllerName)
		r   = &controllerinstallation.Reconciler{
			APIReader:           mgr.GetAPIReader(),
			Client:              mgr.GetClient(),
			NewTargetObjectFunc: func() client.Object { return &gardencorev1beta1.Seed{} },
			Kind:                controllerinstallation.SeedKind,
		}
	)

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Seed{}, builder.WithPredicates(SeedPredicate())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(config.ConcurrentSyncs, 0),
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		Watches(
			&gardencorev1beta1.ControllerRegistration{},
			handler.EnqueueRequestsFromMapFunc(MapToAllSeeds(log, r)),
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		Watches(
			&gardencorev1beta1.BackupBucket{},
			handler.EnqueueRequestsFromMapFunc(MapBackupBucketToSeed),
			builder.WithPredicates(controllerinstallation.BackupBucketPredicate(controllerinstallation.SeedKind)),
		).
		Watches(
			&gardencorev1beta1.BackupEntry{},
			handler.EnqueueRequestsFromMapFunc(MapBackupEntryToSeed),
			builder.WithPredicates(controllerinstallation.BackupEntryPredicate(controllerinstallation.SeedKind)),
		).
		Watches(
			&gardencorev1beta1.ControllerInstallation{},
			handler.EnqueueRequestsFromMapFunc(MapControllerInstallationToSeed),
			builder.WithPredicates(controllerinstallation.ControllerInstallationPredicate(controllerinstallation.SeedKind)),
		).
		Watches(
			&gardencorev1.ControllerDeployment{},
			handler.EnqueueRequestsFromMapFunc(MapControllerDeploymentToAllSeeds(log, r)),
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		Watches(
			&gardencorev1beta1.Shoot{},
			handler.EnqueueRequestsFromMapFunc(MapShootToSeed),
			builder.WithPredicates(controllerinstallation.ShootPredicate(controllerinstallation.SeedKind)),
		).
		Complete(r)
}

// SeedPredicate returns true for all Seed events except for updates. Here, it returns true when there is a change
// in the spec or labels or annotations or when the deletion timestamp is set.
func SeedPredicate() predicate.Predicate {
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

// MapToAllSeeds returns reconcile.Request objects for all existing seeds in the system.
func MapToAllSeeds(log logr.Logger, r *controllerinstallation.Reconciler) handler.MapFunc {
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
func MapBackupBucketToSeed(_ context.Context, obj client.Object) []reconcile.Request {
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
func MapBackupEntryToSeed(_ context.Context, obj client.Object) []reconcile.Request {
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
func MapShootToSeed(_ context.Context, obj client.Object) []reconcile.Request {
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
func MapControllerInstallationToSeed(_ context.Context, obj client.Object) []reconcile.Request {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok || controllerInstallation.Spec.SeedRef == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: controllerInstallation.Spec.SeedRef.Name}}}
}

// MapControllerDeploymentToAllSeeds returns reconcile.Request objects for all seeds in case there is at least one
// ControllerRegistration which references the ControllerDeployment.
func MapControllerDeploymentToAllSeeds(log logr.Logger, r *controllerinstallation.Reconciler) handler.MapFunc {
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
					return MapToAllSeeds(log, r)(ctx, nil)
				}
			}
		}

		return nil
	}
}
