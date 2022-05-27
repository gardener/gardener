// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const seedCareReconcilerName = "seed-care"

// NewCareReconciler returns an implementation of reconcile.Reconciler which is dedicated to execute care operations
func NewCareReconciler(
	clientMap clientmap.ClientMap,
	seedCareControllerConfig gardenletconfig.SeedCareControllerConfiguration,
) reconcile.Reconciler {
	return &careReconciler{
		clientMap:                clientMap,
		seedCareControllerConfig: seedCareControllerConfig,
	}
}

type careReconciler struct {
	clientMap                clientmap.ClientMap
	seedCareControllerConfig gardenletconfig.SeedCareControllerConfiguration
}

func (r *careReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	seed := &gardencorev1beta1.Seed{}
	if err := gardenClient.Client().Get(ctx, req.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Stopping care operations for Seed since it has been deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if err := r.care(ctx, gardenClient.Client(), seed, log); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.seedCareControllerConfig.SyncPeriod.Duration}, nil
}

var (
	// NewSeed is used to create a new `operation.Operation` instance.
	NewSeed = defaultNewSeedObjectFunc
	// NewHealthCheck is used to create a new Health check instance.
	NewHealthCheck = defaultNewHealthCheck
)

func (r *careReconciler) care(ctx context.Context, gardenClientSet client.Client, seed *gardencorev1beta1.Seed, log logr.Logger) error {
	careCtx, cancel := context.WithTimeout(ctx, r.seedCareControllerConfig.SyncPeriod.Duration)
	defer cancel()

	log.V(1).Info("Starting seed care")

	// Initialize conditions based on the current status.
	conditionTypes := []gardencorev1beta1.ConditionType{
		gardencorev1beta1.SeedSystemComponentsHealthy,
	}
	var conditions []gardencorev1beta1.Condition
	for _, cond := range conditionTypes {
		conditions = append(conditions, gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, cond))
	}

	seedClient, err := r.clientMap.GetClient(careCtx, keys.ForSeed(seed))
	if err != nil {
		log.Error(err, "SeedClient cannot be constructed")

		if err := careSetupFailure(ctx, gardenClientSet, seed, "Precondition failed: seed client cannot be constructed", conditions); err != nil {
			log.Error(err, "Unable to create error condition")
		}

		return nil // We do not want to run in the exponential backoff for the condition checks.
	}

	// Trigger health check
	seedHealth := NewHealthCheck(seed, seedClient.Client())
	updatedConditions := seedHealth.CheckSeed(
		careCtx,
		conditions,
		r.conditionThresholdsToProgressingMapping(),
	)

	// Update Seed status conditions if necessary
	if gardencorev1beta1helper.ConditionsNeedUpdate(conditions, updatedConditions) {
		// Rebuild seed conditions to ensure that only the conditions with the
		// correct types will be updated, and any other conditions will remain intact
		conditions := buildSeedConditions(seed.Status.Conditions, updatedConditions, conditionTypes)
		log.Info("Updating seed status conditions")
		if err := patchSeedStatus(ctx, gardenClientSet, seed, conditions); err != nil {
			log.Error(err, "Could not update Seed status")
			return nil // We do not want to run in the exponential backoff for the condition checks.
		}
	}
	return nil
}

func careSetupFailure(ctx context.Context, gardenClient client.Client, seed *gardencorev1beta1.Seed, message string, conditions []gardencorev1beta1.Condition) error {
	updatedConditions := make([]gardencorev1beta1.Condition, 0, len(conditions))
	for _, cond := range conditions {
		updatedConditions = append(updatedConditions, gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(cond, message))
	}

	if !gardencorev1beta1helper.ConditionsNeedUpdate(conditions, updatedConditions) {
		return nil
	}

	return patchSeedStatus(ctx, gardenClient, seed, updatedConditions)
}

// buildSeedConditions builds and returns the seed conditions using the given seed conditions as a base,
// by first removing all conditions with the given types and then merging the given conditions (which must be of the same types).
func buildSeedConditions(seedConditions []gardencorev1beta1.Condition, conditions []gardencorev1beta1.Condition, conditionTypes []gardencorev1beta1.ConditionType) []gardencorev1beta1.Condition {
	result := gardencorev1beta1helper.RemoveConditions(seedConditions, conditionTypes...)
	result = gardencorev1beta1helper.MergeConditions(result, conditions...)
	return result
}

func patchSeedStatus(ctx context.Context, c client.StatusClient, seed *gardencorev1beta1.Seed, conditions []gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(seed.DeepCopy())
	seed.Status.Conditions = conditions
	return c.Status().Patch(ctx, seed, patch)
}

func (r *careReconciler) conditionThresholdsToProgressingMapping() map[gardencorev1beta1.ConditionType]time.Duration {
	out := make(map[gardencorev1beta1.ConditionType]time.Duration)
	for _, threshold := range r.seedCareControllerConfig.ConditionThresholds {
		out[gardencorev1beta1.ConditionType(threshold.Type)] = threshold.Duration.Duration
	}
	return out
}

func (c *Controller) seedCareAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.seedCareQueue.Add(key)
}

func (c *Controller) seedCareUpdate(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		return
	}

	oldSeed, ok := oldObj.(*gardencorev1beta1.Seed)
	if !ok {
		return
	}

	newSeed, ok := newObj.(*gardencorev1beta1.Seed)
	if !ok {
		return
	}

	// re-evaluate seed health status right after seed was successfully bootstrapped
	if seedBootstrappedSuccessfully(oldSeed, newSeed) {
		c.seedCareQueue.Add(key)
	}
}

func seedBootstrappedSuccessfully(oldSeed, newSeed *gardencorev1beta1.Seed) bool {
	oldBootstrappedCondition := gardencorev1beta1helper.GetCondition(oldSeed.Status.Conditions, gardencorev1beta1.SeedBootstrapped)
	newBootstrappedCondition := gardencorev1beta1helper.GetCondition(newSeed.Status.Conditions, gardencorev1beta1.SeedBootstrapped)

	if newBootstrappedCondition != nil &&
		newBootstrappedCondition.Status == gardencorev1beta1.ConditionTrue &&
		(oldBootstrappedCondition == nil ||
			oldBootstrappedCondition.Status != gardencorev1beta1.ConditionTrue) {
		return true
	}
	return false
}
