// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var (
	// NewSeed is used to create a new `operation.Operation` instance.
	NewSeed = defaultNewSeedObjectFunc
	// NewHealthCheck is used to create a new Health check instance.
	NewHealthCheck = defaultNewHealthCheck
)

// Reconciler reconciles Seed resources and executes health check operations.
type Reconciler struct {
	GardenClient   client.Client
	SeedClient     client.Client
	Config         config.SeedCareControllerConfiguration
	Clock          clock.Clock
	Namespace      *string
	SeedName       string
	LoggingEnabled bool
}

// Reconcile reconciles Seed resources and executes health check operations.
func (r *Reconciler) Reconcile(reconcileCtx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(reconcileCtx)

	// Timeout for all calls (e.g. status updates), give status updates a bit of headroom if health checks
	// themselves run into timeouts, so that we will still update the status with that timeout error.
	reconcileCtx, cancel := controllerutils.GetMainReconciliationContext(reconcileCtx, r.Config.SyncPeriod.Duration)
	defer cancel()

	seed := &gardencorev1beta1.Seed{}
	if err := r.GardenClient.Get(reconcileCtx, req.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	ctx, cancel := controllerutils.GetChildReconciliationContext(reconcileCtx, r.Config.SyncPeriod.Duration)
	defer cancel()

	log.V(1).Info("Starting seed care")

	// Initialize conditions based on the current status.
	conditionTypes := []gardencorev1beta1.ConditionType{gardencorev1beta1.SeedSystemComponentsHealthy}
	var conditions []gardencorev1beta1.Condition
	for _, cond := range conditionTypes {
		conditions = append(conditions, v1beta1helper.GetOrInitConditionWithClock(r.Clock, seed.Status.Conditions, cond))
	}

	// Trigger health check
	seedIsGarden, err := gardenerutils.SeedIsGarden(ctx, r.SeedClient)
	if err != nil {
		return reconcile.Result{}, err
	}

	updatedConditions := NewHealthCheck(seed, r.SeedClient, r.Clock, r.Namespace, seedIsGarden, r.LoggingEnabled).CheckSeed(ctx, conditions, r.conditionThresholdsToProgressingMapping())

	// Update Seed status conditions if necessary
	if v1beta1helper.ConditionsNeedUpdate(conditions, updatedConditions) {
		// Rebuild seed conditions to ensure that only the conditions with the
		// correct types will be updated, and any other conditions will remain intact
		conditions = buildSeedConditions(seed.Status.Conditions, updatedConditions, conditionTypes)

		log.Info("Updating seed status conditions")
		patch := client.StrategicMergeFrom(seed.DeepCopy())
		seed.Status.Conditions = conditions
		if err := r.GardenClient.Status().Patch(ctx, seed, patch); err != nil {
			log.Error(err, "Could not update Seed status")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) conditionThresholdsToProgressingMapping() map[gardencorev1beta1.ConditionType]time.Duration {
	out := make(map[gardencorev1beta1.ConditionType]time.Duration)
	for _, threshold := range r.Config.ConditionThresholds {
		out[gardencorev1beta1.ConditionType(threshold.Type)] = threshold.Duration.Duration
	}
	return out
}

// buildSeedConditions builds and returns the seed conditions using the given seed conditions as a base,
// by first removing all conditions with the given types and then merging the given conditions (which must be of the same types).
func buildSeedConditions(seedConditions []gardencorev1beta1.Condition, conditions []gardencorev1beta1.Condition, conditionTypes []gardencorev1beta1.ConditionType) []gardencorev1beta1.Condition {
	result := v1beta1helper.RemoveConditions(seedConditions, conditionTypes...)
	result = v1beta1helper.MergeConditions(result, conditions...)
	return result
}
