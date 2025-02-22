// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
)

// reconciler reconciles OperatingSystemConfig resources of Gardener's `extensions.gardener.cloud`
// API group.
type reconciler struct {
	actuator      Actuator
	client        client.Client
	reader        client.Reader
	scheme        *runtime.Scheme
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// OperatingSystemConfig resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		mgr,
		func() client.Object { return &extensionsv1alpha1.OperatingSystemConfig{} },
		&reconciler{
			actuator:      actuator,
			client:        mgr.GetClient(),
			reader:        mgr.GetAPIReader(),
			scheme:        mgr.GetScheme(),
			statusUpdater: extensionscontroller.NewStatusUpdater(mgr.GetClient()),
		},
	)
}

// Reconcile is the reconciler function that gets executed in case there are new events for the `OperatingSystemConfig`
// resources.
func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	osc := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := r.client.Get(ctx, request.NamespacedName, osc); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, osc.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsFailed(cluster) {
		log.Info("Skip the reconciliation of OperatingSystemConfig of failed shoot")
		return reconcile.Result{}, nil
	}

	operationType := v1beta1helper.ComputeOperationType(osc.ObjectMeta, osc.Status.LastOperation)

	switch {
	case extensionscontroller.ShouldSkipOperation(operationType, osc):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, log, osc)
	case osc.DeletionTimestamp != nil:
		return r.delete(ctx, log, osc, cluster != nil && v1beta1helper.ShootNeedsForceDeletion(cluster.Shoot))
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, log, osc)
	default:
		return r.reconcile(ctx, log, osc, operationType)
	}
}

func (r *reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	osc *extensionsv1alpha1.OperatingSystemConfig,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(osc, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, osc, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, osc, operationType, "Reconciling the OperatingSystemConfig"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the reconciliation of OperatingSystemConfig")
	userData, extensionUnits, extensionFiles, inPlaceUpdates, err := r.actuator.Reconcile(ctx, log, osc)
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, osc, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling OperatingSystemConfig")
		return reconcilerutils.ReconcileErr(err)
	}

	secret, err := r.reconcileOSCResultSecret(ctx, osc, userData)
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, osc, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Could not apply secret for generated cloud config")
		return reconcilerutils.ReconcileErr(err)
	}

	patch := client.MergeFrom(osc.DeepCopy())
	setOSCStatus(osc, secret, extensionUnits, extensionFiles, inPlaceUpdates)
	if err := r.client.Status().Patch(ctx, osc, patch); err != nil {
		_ = r.statusUpdater.Error(ctx, log, osc, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Could not update status")
		return reconcilerutils.ReconcileErr(err)
	}
	if err := r.statusUpdater.Success(ctx, log, osc, operationType, "Successfully reconciled OperatingSystemConfig"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(
	ctx context.Context,
	log logr.Logger,
	osc *extensionsv1alpha1.OperatingSystemConfig,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(osc, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, osc, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, osc, gardencorev1beta1.LastOperationTypeRestore, "Restoring the OperatingSystemConfig"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the restoration of OperatingSystemConfig")
	userData, extensionUnits, extensionFiles, inPlaceUpdates, err := r.actuator.Restore(ctx, log, osc)
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, osc, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring OperatingSystemConfig")
		return reconcilerutils.ReconcileErr(err)
	}

	secret, err := r.reconcileOSCResultSecret(ctx, osc, userData)
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, osc, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Could not apply secret for generated cloud config")
		return reconcilerutils.ReconcileErr(err)
	}

	patch := client.MergeFrom(osc.DeepCopy())
	setOSCStatus(osc, secret, extensionUnits, extensionFiles, inPlaceUpdates)
	if err := r.client.Status().Patch(ctx, osc, patch); err != nil {
		_ = r.statusUpdater.Error(ctx, log, osc, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Could not update units and secret ref.")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, osc, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored OperatingSystemConfig"); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, osc, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from OperatingSystemConfig: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	osc *extensionsv1alpha1.OperatingSystemConfig,
	forceDelete bool,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(osc, FinalizerName) {
		log.Info("Deleting operating system config causes a no-op as there is no finalizer", "osc", osc.Name)
		return reconcile.Result{}, nil
	}

	if err := r.statusUpdater.Processing(ctx, log, osc, gardencorev1beta1.LastOperationTypeDelete, "Deleting the OperatingSystemConfig"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the deletion of OperatingSystemConfig")
	var err error
	if forceDelete {
		err = r.actuator.ForceDelete(ctx, log, osc)
	} else {
		err = r.actuator.Delete(ctx, log, osc)
	}
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, osc, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error deleting OperatingSystemConfig")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, osc, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted OperatingSystemConfig"); err != nil {
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(osc, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, osc, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(
	ctx context.Context,
	log logr.Logger,
	osc *extensionsv1alpha1.OperatingSystemConfig,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(osc, FinalizerName) {
		log.Info("Migrating operating system config causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	if err := r.statusUpdater.Processing(ctx, log, osc, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the OperatingSystemConfig"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the migration of OperatingSystemConfig")
	if err := r.actuator.Migrate(ctx, log, osc); err != nil {
		_ = r.statusUpdater.Error(ctx, log, osc, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating OperatingSystemConfig")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, osc, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated OperatingSystemConfig"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Removing finalizer", "osc", osc.Name)
	if err := controllerutils.RemoveAllFinalizers(ctx, r.client, osc); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers: %w", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, osc, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from OperatingSystemConfig: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) reconcileOSCResultSecret(ctx context.Context, osc *extensionsv1alpha1.OperatingSystemConfig, userData []byte) (*corev1.Secret, error) {
	// For backwards-compatibility, we have to always create a secret since gardenlet expects to find it - even if the
	// user data is nil/empty (which should always be the case when purpose=reconcile).
	// https://github.com/gardener/gardener/blob/328e10d975c7b6caa5db139badcc42ac8f772d31/pkg/component/extensions/operatingsystemconfig/operatingsystemconfig.go#L257-L259
	// TODO(rfranzke): Activate the `if`-block after Gardener v1.112 has been released.
	// if userData == nil {
	// 	return nil, nil
	// }

	secret := &corev1.Secret{ObjectMeta: SecretObjectMetaForConfig(osc)}
	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, r.client, secret, func() error {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[extensionsv1alpha1.OperatingSystemConfigSecretDataKey] = userData
		return controllerutil.SetControllerReference(osc, secret, r.scheme)
	}); err != nil {
		return nil, err
	}
	return secret, nil
}

func setOSCStatus(
	osc *extensionsv1alpha1.OperatingSystemConfig,
	secret *corev1.Secret,
	extensionUnits []extensionsv1alpha1.Unit,
	extensionFiles []extensionsv1alpha1.File,
	inPlaceUpdates *extensionsv1alpha1.InPlaceUpdatesStatus,
) {
	if secret != nil {
		osc.Status.CloudConfig = &extensionsv1alpha1.CloudConfig{
			SecretRef: corev1.SecretReference{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			},
		}
	}
	osc.Status.ExtensionUnits = extensionUnits
	osc.Status.ExtensionFiles = extensionFiles
	osc.Status.InPlaceUpdates = inPlaceUpdates
}
