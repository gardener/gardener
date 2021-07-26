// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ShootRetryControllerName is the name of the shoot-retry controller.
	ShootRetryControllerName = "shoot-retry"
)

func addShootRetryController(
	ctx context.Context,
	mgr manager.Manager,
	config *config.ShootRetryControllerConfiguration,
) error {
	logger := mgr.GetLogger()
	gardenClient := mgr.GetClient()

	ctrlOptions := controller.Options{
		Reconciler:              NewShootRetryReconciler(logger, gardenClient, config),
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(ShootRetryControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := c.Watch(&source.Kind{Type: shoot}, &handler.EnqueueRequestForObject{}); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", shoot, err)
	}

	return nil
}

// NewShootRetryReconciler creates a new instance of a reconciler which retries certain failed Shoots.
func NewShootRetryReconciler(l logr.Logger, gardenClient client.Client, config *config.ShootRetryControllerConfiguration) reconcile.Reconciler {
	return &shootRetryReconciler{
		logger:       l,
		gardenClient: gardenClient,
		config:       config,
	}
}

type shootRetryReconciler struct {
	logger       logr.Logger
	gardenClient client.Client
	config       *config.ShootRetryControllerConfiguration
}

func (r *shootRetryReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.WithValues("shoot", request)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		logger.Error(err, "Unable to retrieve object from store")
		return reconcile.Result{}, err
	}

	if !shootFailedDueToRateLimits(shoot) {
		return reconcile.Result{}, nil
	}

	mustRetry, requeueAfter := mustRetryNow(shoot, *r.config.RetryPeriod)
	if !mustRetry {
		logger.WithValues("retry", requeueAfter.Round(time.Minute)).Info("Scheduled retry")
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	logger.Info("Retrying a failed Shoot")

	shootCopy := shoot.DeepCopy()
	metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRetry)
	if err := r.gardenClient.Patch(ctx, shoot, client.MergeFrom(shootCopy)); err != nil {
		return reconcile.Result{}, err
	}

	logger.Info("Shoot was successfully retried")

	return reconcile.Result{}, nil
}

func shootFailedDueToRateLimits(shoot *gardencorev1beta1.Shoot) bool {
	return isShootFailed(shoot) && gardencorev1beta1helper.HasErrorCode(shoot.Status.LastErrors, gardencorev1beta1.ErrorInfraRateLimitsExceeded)
}

func isShootFailed(shoot *gardencorev1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation

	return lastOperation != nil && lastOperation.State == gardencorev1beta1.LastOperationStateFailed &&
		shoot.Generation == shoot.Status.ObservedGeneration
}

func mustRetryNow(shoot *gardencorev1beta1.Shoot, retryPeriod metav1.Duration) (bool, time.Duration) {
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

	return false, lastReconciliationPlusRetryPeriod.Sub(now)
}
