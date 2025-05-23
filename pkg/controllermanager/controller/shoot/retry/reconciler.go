// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
)

// Reconciler reconciles failed Shoots and retries them.
type Reconciler struct {
	Client client.Client
	Config controllermanagerconfigv1alpha1.ShootRetryControllerConfiguration
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
		lastReconciliation                = shoot.Status.LastOperation.LastUpdateTime.UTC()
		lastReconciliationPlusRetryPeriod = lastReconciliation.Add(retryPeriod.Duration)
		now                               = time.Now().UTC()
	)

	if now.After(lastReconciliationPlusRetryPeriod) {
		return true, 0
	}

	return false, lastReconciliationPlusRetryPeriod.Add(utils.RandomDurationWithMetaDuration(jitterPeriod)).Sub(now)
}
