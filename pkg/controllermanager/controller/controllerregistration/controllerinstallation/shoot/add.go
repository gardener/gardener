// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/core"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerinstallation"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "controllerinstallation-shoot"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	log := mgr.GetLogger().WithValues("controller", ControllerName)

	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.APIReader == nil {
		r.APIReader = mgr.GetAPIReader()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Shoot{}, builder.WithPredicates(controllerinstallation.ShootPredicate(true))).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		Watches(
			&gardencorev1beta1.ControllerRegistration{},
			handler.EnqueueRequestsFromMapFunc(r.MapToAllSelfHostedShoots(log)),
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		Watches(
			&gardencorev1beta1.BackupBucket{},
			handler.EnqueueRequestsFromMapFunc(r.MapBackupEntryToShoot),
			builder.WithPredicates(controllerinstallation.BackupBucketPredicate(true)),
		).
		Watches(
			&gardencorev1beta1.BackupEntry{},
			handler.EnqueueRequestsFromMapFunc(r.MapBackupEntryToShoot),
			builder.WithPredicates(controllerinstallation.BackupEntryPredicate(true)),
		).
		Watches(
			&gardencorev1beta1.ControllerInstallation{},
			handler.EnqueueRequestsFromMapFunc(r.MapControllerInstallationToShoot),
			builder.WithPredicates(controllerinstallation.ControllerInstallationPredicate(true)),
		).
		Watches(
			&gardencorev1.ControllerDeployment{},
			handler.EnqueueRequestsFromMapFunc(r.MapControllerDeploymentToAllSelfHostedShoots(log)),
			builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		Complete(r)
}

// MapToAllSelfHostedShoots returns reconcile.Request objects for all existing self-hosted shoots in the system.
func (r *Reconciler) MapToAllSelfHostedShoots(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		shootList := &metav1.PartialObjectMetadataList{}
		shootList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
		if err := r.Client.List(ctx, shootList); err != nil {
			log.Error(err, "Failed to list shoots")
			return nil
		}

		return mapper.ObjectListToRequests(shootList, func(obj client.Object) bool {
			shoot, ok := obj.(*gardencorev1beta1.Shoot)
			return ok && v1beta1helper.IsShootSelfHosted(shoot.Spec.Provider.Workers)
		})
	}
}

// MapBackupBucketToShoot returns a reconcile.Request object for the Shoot for the shoot specified in the
// .spec.shootRef.name field.
func (r *Reconciler) MapBackupBucketToShoot(_ context.Context, obj client.Object) []reconcile.Request {
	backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok || backupBucket.Spec.ShootRef == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: backupBucket.Spec.ShootRef.Name, Namespace: backupBucket.Spec.ShootRef.Namespace}}}
}

// MapBackupEntryToShoot returns a reconcile.Request object for the Shoot for the shoot specified in the
// .spec.shootRef.name field.
func (r *Reconciler) MapBackupEntryToShoot(_ context.Context, obj client.Object) []reconcile.Request {
	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok || backupEntry.Spec.ShootRef == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: backupEntry.Spec.ShootRef.Name, Namespace: backupEntry.Spec.ShootRef.Namespace}}}
}

// MapControllerInstallationToShoot returns a reconcile.Request object for the shoot specified in the
// .spec.shootRef.name field.
func (r *Reconciler) MapControllerInstallationToShoot(_ context.Context, obj client.Object) []reconcile.Request {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok || controllerInstallation.Spec.ShootRef == nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: controllerInstallation.Spec.ShootRef.Name, Namespace: controllerInstallation.Spec.ShootRef.Namespace}}}
}

// MapControllerDeploymentToAllSelfHostedShoots returns reconcile.Request objects for all self-hosted shoots in case
// there is at least one ControllerRegistration which references the ControllerDeployment.
func (r *Reconciler) MapControllerDeploymentToAllSelfHostedShoots(log logr.Logger) handler.MapFunc {
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
					return r.MapToAllSelfHostedShoots(log)(ctx, nil)
				}
			}
		}

		return nil
	}
}
