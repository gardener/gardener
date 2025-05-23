// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// Reconciler reconciles ControllerInstallations, checks their health status and reports it via conditions.
type Reconciler struct {
	GardenClient    client.Client
	SeedClient      client.Client
	Config          gardenletconfigv1alpha1.ControllerInstallationCareControllerConfiguration
	Clock           clock.Clock
	GardenNamespace string
}

// Reconcile reconciles ControllerInstallations, checks their health status and reports it via conditions.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	gardenCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	seedCtx, cancel := controllerutils.GetChildReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
	if err := r.GardenClient.Get(gardenCtx, request.NamespacedName, controllerInstallation); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if controllerInstallation.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	var (
		conditionControllerInstallationInstalled   = v1beta1helper.GetOrInitConditionWithClock(r.Clock, controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationInstalled)
		conditionControllerInstallationHealthy     = v1beta1helper.GetOrInitConditionWithClock(r.Clock, controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationHealthy)
		conditionControllerInstallationProgressing = v1beta1helper.GetOrInitConditionWithClock(r.Clock, controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationProgressing)
	)

	managedResource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerInstallation.Name,
			Namespace: r.GardenNamespace,
		},
	}

	if err := r.SeedClient.Get(seedCtx, client.ObjectKeyFromObject(managedResource), managedResource); err != nil {
		msg := fmt.Sprintf("Failed to get ManagedResource %q: %s", client.ObjectKeyFromObject(managedResource).String(), err.Error())
		conditionControllerInstallationInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionControllerInstallationInstalled, gardencorev1beta1.ConditionUnknown, "SeedReadError", msg)
		conditionControllerInstallationHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionControllerInstallationHealthy, gardencorev1beta1.ConditionUnknown, "SeedReadError", msg)
		conditionControllerInstallationProgressing = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionControllerInstallationProgressing, gardencorev1beta1.ConditionUnknown, "SeedReadError", msg)

		patch := client.StrategicMergeFrom(controllerInstallation.DeepCopy())
		controllerInstallation.Status.Conditions = v1beta1helper.MergeConditions(controllerInstallation.Status.Conditions, conditionControllerInstallationHealthy, conditionControllerInstallationInstalled, conditionControllerInstallationProgressing)
		if err := r.GardenClient.Status().Patch(gardenCtx, controllerInstallation, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to patch conditions: %w", err)
		}

		if apierrors.IsNotFound(err) {
			log.Info("ManagedResource was not found yet, requeuing", "managedResource", client.ObjectKeyFromObject(managedResource))
			return reconcile.Result{RequeueAfter: time.Second}, nil
		}

		return reconcile.Result{}, err
	}

	if err := health.CheckManagedResourceApplied(managedResource); err != nil {
		conditionControllerInstallationInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionControllerInstallationInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending", err.Error())
	} else {
		conditionControllerInstallationInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionControllerInstallationInstalled, gardencorev1beta1.ConditionTrue, "InstallationSuccessful", "The controller was successfully installed in the seed cluster.")
	}

	if err := health.CheckManagedResourceHealthy(managedResource); err != nil {
		conditionControllerInstallationHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionControllerInstallationHealthy, gardencorev1beta1.ConditionFalse, "ControllerNotHealthy", err.Error())
	} else {
		conditionControllerInstallationHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionControllerInstallationHealthy, gardencorev1beta1.ConditionTrue, "ControllerHealthy", "The controller running in the seed cluster is healthy.")
	}

	if err := health.CheckManagedResourceProgressing(managedResource); err != nil {
		conditionControllerInstallationProgressing = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionControllerInstallationProgressing, gardencorev1beta1.ConditionTrue, "ControllerNotRolledOut", err.Error())
	} else {
		conditionControllerInstallationProgressing = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionControllerInstallationProgressing, gardencorev1beta1.ConditionFalse, "ControllerRolledOut", "The controller has been rolled out successfully.")
	}

	patch := client.StrategicMergeFrom(controllerInstallation.DeepCopy())
	controllerInstallation.Status.Conditions = v1beta1helper.MergeConditions(controllerInstallation.Status.Conditions, conditionControllerInstallationHealthy, conditionControllerInstallationInstalled, conditionControllerInstallationProgressing)
	if err := r.GardenClient.Status().Patch(gardenCtx, controllerInstallation, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to patch conditions: %w", err)
	}

	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}
