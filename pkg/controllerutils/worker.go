// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerutils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DeprecatedCreateWorker creates and runs a worker thread that just processes items in the
// specified queue. The worker will run until stopCh is closed. The worker will be
// added to the wait group when started and marked done when finished.
// Deprecated: Use CreateWorker instead.
func DeprecatedCreateWorker(ctx context.Context, queue workqueue.RateLimitingInterface, resourceType string, reconciler func(key string) error, waitGroup *sync.WaitGroup, workerCh chan int) {
	CreateWorker(ctx, queue, resourceType, reconcile.Func(func(req reconcile.Request) (reconcile.Result, error) {
		meta := kutil.ObjectMeta(req.Namespace, req.Name)
		key, err := cache.MetaNamespaceKeyFunc(&meta)
		if err != nil {
			logger.Logger.WithError(err).Error("Could not create key from meta")
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, reconciler(key)
	}), waitGroup, workerCh)
}

// CreateWorker creates and runs a worker thread that just processes items in the
// specified queue. The worker will run until stopCh is closed. The worker will be
// added to the wait group when started and marked done when finished.
func CreateWorker(ctx context.Context, queue workqueue.RateLimitingInterface, resourceType string, reconciler reconcile.Reconciler, waitGroup *sync.WaitGroup, workerCh chan int) {
	waitGroup.Add(1)
	workerCh <- 1
	go func() {
		wait.Until(worker(queue, resourceType, reconciler), time.Second, ctx.Done())
		workerCh <- -1
		waitGroup.Done()
	}()
}

func requestFromKey(key interface{}) (reconcile.Request, error) {
	switch v := key.(type) {
	case string:
		namespace, name, err := cache.SplitMetaNamespaceKey(key.(string))
		if err != nil {
			return reconcile.Request{}, err
		}

		return reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}}, nil
	case reconcile.Request:
		return v, nil
	default:
		return reconcile.Request{}, fmt.Errorf("unknown key type %T", key)
	}
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the reconciler is never invoked concurrently with the same key.
func worker(queue workqueue.RateLimitingInterface, resourceType string, reconciler reconcile.Reconciler) func() {
	return func() {
		exit := false
		for !exit {
			exit = func() bool {
				key, quit := queue.Get()
				if quit {
					return true
				}
				defer queue.Done(key)

				req, err := requestFromKey(key)
				if err != nil {
					logger.Logger.WithError(err).Error("Cannot obtain request from key")
					queue.Forget(key)
					return false
				}

				res, err := reconciler.Reconcile(req)
				if err != nil {
					logger.Logger.Infof("Error syncing %s %v: %v", resourceType, key, err)
					queue.AddRateLimited(key)
					return false
				}

				if res.RequeueAfter > 0 {
					queue.AddAfter(key, res.RequeueAfter)
					return false
				}
				if res.Requeue {
					queue.AddRateLimited(key)
					return false
				}
				queue.Forget(key)
				return false
			}()
		}
	}
}
