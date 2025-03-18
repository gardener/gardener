// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
)

var (
	// NewHealthCheck is used to create a new Health check instance.
	NewHealthCheck = defaultNewHealthCheck
)

// Reconciler reconciles Extension resources and executes health check operations.
type Reconciler struct {
	RuntimeClient   client.Client
	Config          operatorconfigv1alpha1.OperatorConfiguration
	Clock           clock.Clock
	GardenNamespace string
	// GardenClientMap is the ClientMap used to communicate with the virtual garden cluster. It should be set by AddToManager function but the field is still public for usage in tests.
	GardenClientMap clientmap.ClientMap

	registerManagedResourceWatchFunc func() error
	managedResourceWatchRegistered   bool
}

// Reconcile reconciles Extension resources and executes health check operations.
func (r *Reconciler) Reconcile(reconcileCtx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(reconcileCtx)

	// Timeout for all calls (e.g. status updates), give status updates a bit of headroom if health checks
	// themselves run into timeouts, so that we will still update the status with that timeout error.
	reconcileCtx, cancel := controllerutils.GetMainReconciliationContext(reconcileCtx, r.Config.Controllers.ExtensionCare.SyncPeriod.Duration)
	defer cancel()

	extension := &operatorv1alpha1.Extension{}
	if err := r.RuntimeClient.Get(reconcileCtx, request.NamespacedName, extension); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	gardenList := &operatorv1alpha1.GardenList{}
	// We limit one result because we expect only a single Garden object to be there.
	if err := r.RuntimeClient.List(reconcileCtx, gardenList, client.Limit(1)); err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving Garden object: %w", err)
	}

	var garden *operatorv1alpha1.Garden
	if len(gardenList.Items) != 0 {
		garden = &gardenList.Items[0]
	}

	if garden == nil {
		log.V(1).Info("No garden found, stop reconciling")
		return reconcile.Result{}, nil
	}

	if !r.managedResourceWatchRegistered && r.registerManagedResourceWatchFunc != nil {
		if err := r.registerManagedResourceWatchFunc(); err != nil {
			log.Error(err, "Failed registering watch for resources.gardener.cloud/v1alpha1.ManagedResource now that a operator.gardener.cloud/v1alpha1.Garden object has been created")
		} else {
			r.managedResourceWatchRegistered = true
		}
	}

	ctx, cancel := controllerutils.GetChildReconciliationContext(reconcileCtx, r.Config.Controllers.ExtensionCare.SyncPeriod.Duration)
	defer cancel()

	log.V(1).Info("Starting extension care")

	// Initialize conditions based on the current status.
	extensionConditions := NewExtensionConditions(r.Clock, extension)

	gardenClientSet, err := r.GardenClientMap.GetClient(reconcileCtx, keys.ForGarden(garden))
	if err != nil {
		log.V(1).Info("Could not get garden client", "error", err)
	}

	updatedConditions := NewHealthCheck(
		extension,
		r.RuntimeClient,
		gardenClientSet,
		r.Clock,
		r.conditionThresholdsToProgressingMapping(),
		r.GardenNamespace,
	).Check(
		ctx,
		extensionConditions,
	)

	var existingConditions []gardencorev1beta1.Condition
	for _, condition := range extension.Status.Conditions {
		if slices.Contains(extensionConditions.ConditionTypes(), condition.Type) {
			existingConditions = append(existingConditions, condition)
		}
	}

	// Update extension status conditions if necessary
	if v1beta1helper.ConditionsNeedUpdate(existingConditions, updatedConditions) {
		log.Info("Updating extension status conditions")
		patch := client.MergeFrom(extension.DeepCopy())
		// Rebuild extension conditions to ensure that only the conditions with the
		// correct types will be updated, and any other conditions will remain intact
		extension.Status.Conditions = v1beta1helper.BuildConditions(extension.Status.Conditions, updatedConditions, extensionConditions.ConditionTypes())

		if err := r.RuntimeClient.Status().Patch(reconcileCtx, extension, patch); err != nil {
			log.Error(err, "Could not update extension status")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.ExtensionCare.SyncPeriod.Duration}, nil
}

func (r *Reconciler) conditionThresholdsToProgressingMapping() map[gardencorev1beta1.ConditionType]time.Duration {
	conditions := map[gardencorev1beta1.ConditionType]time.Duration{}
	for _, condition := range r.Config.Controllers.ExtensionCare.ConditionThresholds {
		conditions[gardencorev1beta1.ConditionType(condition.Type)] = condition.Duration.Duration
	}
	return conditions
}
