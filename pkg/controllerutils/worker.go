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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// WorkerOptions are options for a controller's worker.
type WorkerOptions struct {
	logger logr.Logger
}

// WorkerOption is a func that mutates WorkerOptions.
type WorkerOption func(*WorkerOptions)

// WithLogger configures the logr.Logger to use for a controller worker.
// If set, a logger preconfigured with the name and namespace field will be injected into the context.Context on
// each Reconcile call. It can be retrieved via log.FromContext.
func WithLogger(logger logr.Logger) WorkerOption {
	return func(options *WorkerOptions) {
		options.logger = logger
	}
}

// CreateWorker creates and runs a worker goroutine that just processes items in the
// specified queue. The worker will run until ctx is cancelled. The worker will be
// added to the wait group when started and marked done when finished.
func CreateWorker(ctx context.Context, queue workqueue.RateLimitingInterface, resourceType string, reconciler reconcile.Reconciler, waitGroup *sync.WaitGroup, workerCh chan<- int, opts ...WorkerOption) {
	options := WorkerOptions{}
	for _, o := range opts {
		o(&options)
	}

	waitGroup.Add(1)
	workerCh <- 1
	go func() {
		wait.UntilWithContext(ctx, func(ctx context.Context) {
			worker(ctx, queue, resourceType, reconciler, options)
		}, time.Second)
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
func worker(ctx context.Context, queue workqueue.RateLimitingInterface, resourceType string, reconciler reconcile.Reconciler, opts WorkerOptions) {
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
				opts.logger.Error(err, "Cannot obtain request from key")
				queue.Forget(key)
				return false
			}

			reconcileLogger := opts.logger.WithValues("name", req.Name, "namespace", req.Namespace)
			reconcileCtx := logf.IntoContext(ctx, reconcileLogger)

			res, err := reconciler.Reconcile(reconcileCtx, req)
			if err != nil {
				// resource type is not added to logger here. Ideally, each controller sets WithName and it is clear from
				// which controller the error log comes.
				reconcileLogger.Error(err, "Error reconciling request")
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
