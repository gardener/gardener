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
	gardenlogger "github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) shootRetryAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		gardenlogger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.shootRetryQueue.Add(key)
}

func (c *Controller) shootRetryUpdate(oldObj, newObj interface{}) {
	var (
		oldShoot = oldObj.(*gardencorev1beta1.Shoot)
		newShoot = newObj.(*gardencorev1beta1.Shoot)
	)

	if shootFailedDueToRateLimits(newShoot) && !isShootFailed(oldShoot) {
		key, err := cache.MetaNamespaceKeyFunc(newObj)
		if err != nil {
			gardenlogger.Logger.Errorf("Couldn't get key for object %+v: %v", newObj, err)
			return
		}
		c.shootRetryQueue.Add(key)
	}
}

// NewShootRetryReconciler creates a new instance of a reconciler which retries certain failed Shoots.
func NewShootRetryReconciler(l logrus.FieldLogger, gardenClient client.Client, config *config.ShootRetryControllerConfiguration) reconcile.Reconciler {
	return &shootRetryReconciler{
		logger:       l,
		gardenClient: gardenClient,
		config:       config,
	}
}

type shootRetryReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
	config       *config.ShootRetryControllerConfiguration
}

func (r *shootRetryReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	key := fmt.Sprintf("%s/%s", shoot.Namespace, shoot.Name)
	shootLogger := r.logger.WithField("shoot", key)

	if !shootFailedDueToRateLimits(shoot) {
		return reconcile.Result{}, nil
	}

	mustRetry, requeueAfter := mustRetryNow(shoot, *r.config.RetryPeriod)
	if !mustRetry {
		shootLogger.Infof("[SHOOT RETRY] Scheduled retry in %s", requeueAfter.Round(time.Minute))
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	shootLogger.Info("[SHOOT RETRY] Retrying a failed Shoot")

	shootCopy := shoot.DeepCopy()
	metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRetry)
	if err := r.gardenClient.Patch(ctx, shoot, client.MergeFrom(shootCopy)); err != nil {
		return reconcile.Result{}, err
	}

	shootLogger.Info("[SHOOT RETRY] Shoot was successfully retried")

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
