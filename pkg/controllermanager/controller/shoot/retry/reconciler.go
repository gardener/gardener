// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package retry

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
)

// Reconciler reconciles failed Shoots and retries them.
type Reconciler struct {
	Client client.Client
	Config config.ShootRetryControllerConfiguration
}

// Reconcile reconciles failed Shoots and retries them.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.Client.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if !shootFailedDueToRateLimits(shoot) {
		return reconcile.Result{}, nil
	}

	mustRetry, requeueAfter := mustRetryNow(shoot, *r.Config.RetryPeriod, r.Config.RetryJitterPeriod)
	if !mustRetry {
		if requeueAfter == 0 {
			return reconcile.Result{}, nil
		}
		log.V(1).Info("Scheduling retry for Shoot", "requeueAfter", requeueAfter.Round(time.Second))
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	log.Info("Retrying failed Shoot")

	patch := client.MergeFrom(shoot.DeepCopy())
	metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRetry)
	return reconcile.Result{}, r.Client.Patch(ctx, shoot, patch)
}

func mustRetryNow(shoot *gardencorev1beta1.Shoot, retryPeriod metav1.Duration, jitterPeriod *metav1.Duration) (bool, time.Duration) {
	if shoot.Status.LastOperation == nil {
		return false, 0
	}
	var (
		lastReconciliation                = shoot.Status.LastOperation.LastUpdateTime.Time.UTC()
		lastReconciliationPlusRetryPeriod = lastReconciliation.Add(retryPeriod.Duration)
		now                               = time.Now().UTC()
	)

	if now.After(lastReconciliationPlusRetryPeriod) {
		return true, 0
	}

	return false, lastReconciliationPlusRetryPeriod.Add(utils.RandomDurationWithMetaDuration(jitterPeriod)).Sub(now)
}
