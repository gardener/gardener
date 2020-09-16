// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var nowFunc = time.Now

func (c *Controller) enqueueEvent(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.eventQueue.Add(key)
}

func (c *Controller) reconcileEvent(req reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	log := logger.NewFieldLogger(logger.Logger, "event", req.Namespace+"/"+req.Name)

	event := &corev1.Event{}
	if err := gardenClient.Client().Get(ctx, req.NamespacedName, event); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Skipping because Event has been deleted")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if isShootEvent(event) {
		return reconcile.Result{}, nil
	}

	deleteAt := event.LastTimestamp.Add(c.cfg.TTLNonShootEvents.Duration)
	timeUntilDeletion := deleteAt.Sub(nowFunc())
	if timeUntilDeletion > 0 {
		return reconcile.Result{RequeueAfter: timeUntilDeletion}, nil
	}

	err = gardenClient.Client().Delete(ctx, event)
	return reconcile.Result{}, err
}

func isShootEvent(event *corev1.Event) bool {
	if gv, err := schema.ParseGroupVersion(event.InvolvedObject.APIVersion); err != nil || gv.Group != gardencorev1beta1.GroupName {
		return false
	}
	if event.InvolvedObject.Kind == "Shoot" {
		return true
	}
	return false
}
