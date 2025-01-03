// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"
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

// Reconciler reconciles garden resources and executes health check operations.
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

// Reconcile reconciles Garden resources and executes health check operations.
func (r *Reconciler) Reconcile(reconcileCtx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(reconcileCtx)

	// Timeout for all calls (e.g. status updates), give status updates a bit of headroom if health checks
	// themselves run into timeouts, so that we will still update the status with that timeout error.
	reconcileCtx, cancel := controllerutils.GetMainReconciliationContext(reconcileCtx, r.Config.Controllers.GardenCare.SyncPeriod.Duration)
	defer cancel()

	garden := &operatorv1alpha1.Garden{}
	if err := r.RuntimeClient.Get(reconcileCtx, req.NamespacedName, garden); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if !r.managedResourceWatchRegistered && r.registerManagedResourceWatchFunc != nil {
		if err := r.registerManagedResourceWatchFunc(); err != nil {
			log.Error(err, "Failed registering watch for resources.gardener.cloud/v1alpha1.ManagedResource now that a operator.gardener.cloud/v1alpha1.Garden object has been created")
		} else {
			r.managedResourceWatchRegistered = true
		}
	}

	ctx, cancel := controllerutils.GetChildReconciliationContext(reconcileCtx, r.Config.Controllers.GardenCare.SyncPeriod.Duration)
	defer cancel()

	log.V(1).Info("Starting garden care")

	// Initialize conditions based on the current status.
	gardenConditions := NewGardenConditions(r.Clock, garden.Status)

	gardenClientSet, err := r.GardenClientMap.GetClient(reconcileCtx, keys.ForGarden(garden))
	if err != nil {
		log.V(1).Info("Could not get garden client", "error", err)
	}

	updatedConditions := NewHealthCheck(
		garden,
		r.RuntimeClient,
		gardenClientSet,
		r.Clock,
		r.conditionThresholdsToProgressingMapping(),
		r.GardenNamespace,
	).Check(
		ctx,
		gardenConditions,
	)

	// Update Garden status conditions if necessary
	if v1beta1helper.ConditionsNeedUpdate(gardenConditions.ConvertToSlice(), updatedConditions) {
		log.Info("Updating garden status conditions")
		patch := client.MergeFrom(garden.DeepCopy())
		// Rebuild garden conditions to ensure that only the conditions with the
		// correct types will be updated, and any other conditions will remain intact
		garden.Status.Conditions = v1beta1helper.BuildConditions(garden.Status.Conditions, updatedConditions, gardenConditions.ConditionTypes())

		if err := r.RuntimeClient.Status().Patch(reconcileCtx, garden, patch); err != nil {
			log.Error(err, "Could not update garden status")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.GardenCare.SyncPeriod.Duration}, nil
}

func (r *Reconciler) conditionThresholdsToProgressingMapping() map[gardencorev1beta1.ConditionType]time.Duration {
	conditions := map[gardencorev1beta1.ConditionType]time.Duration{}
	for _, condition := range r.Config.Controllers.GardenCare.ConditionThresholds {
		conditions[gardencorev1beta1.ConditionType(condition.Type)] = condition.Duration.Duration
	}
	return conditions
}
