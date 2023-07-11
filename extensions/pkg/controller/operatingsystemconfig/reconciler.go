// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
		return r.delete(ctx, log, osc)
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
	userData, command, units, err := r.actuator.Reconcile(ctx, log, osc)
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
	setOSCStatus(osc, secret, command, units)
	if err := r.client.Status().Patch(ctx, osc, patch); err != nil {
		_ = r.statusUpdater.Error(ctx, log, osc, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Could not update units and secret ref.")
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
	userData, command, units, err := r.actuator.Restore(ctx, log, osc)
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
	setOSCStatus(osc, secret, command, units)
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
	if err := r.actuator.Delete(ctx, log, osc); err != nil {
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

func setOSCStatus(osc *extensionsv1alpha1.OperatingSystemConfig, secret *corev1.Secret, command *string, units []string) {
	osc.Status.CloudConfig = &extensionsv1alpha1.CloudConfig{
		SecretRef: corev1.SecretReference{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
	}
	osc.Status.Units = units
	if command != nil {
		osc.Status.Command = command
	}
}
