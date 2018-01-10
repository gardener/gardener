// Copyright 2018 The Gardener Authors.
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

package controller

import (
	"reflect"
	"sync"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

// CreateWorker creates and runs a worker thread that just processes items in the
// specified queue. The worker will run until stopCh is closed. The worker will be
// added to the wait group when started and marked done when finished.
func CreateWorker(queue workqueue.RateLimitingInterface, resourceType string, reconciler func(key string) error, stopCh <-chan struct{}, waitGroup *sync.WaitGroup, workerCh chan int) {
	waitGroup.Add(1)
	workerCh <- 1
	go func() {
		wait.Until(worker(queue, resourceType, reconciler), time.Second, stopCh)
		workerCh <- -1
		waitGroup.Done()
	}()
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the reconciler is never invoked concurrently with the same key.
func worker(queue workqueue.RateLimitingInterface, resourceType string, reconciler func(key string) error) func() {
	return func() {
		exit := false
		for !exit {
			exit = func() bool {
				key, quit := queue.Get()
				if quit {
					return true
				}
				defer queue.Done(key)

				err := reconciler(key.(string))
				if err == nil {
					queue.Forget(key)
					return false
				}

				logger.Logger.Infof("Error syncing %s %v: %v", resourceType, key, err)
				queue.AddRateLimited(key)
				return false
			}()
		}
	}
}

// CheckIfResourceVersionChanged returns true if the .metadata.resourceVersion fields of oldShoot and newShoot
// differ, and false otherwise.
func CheckIfResourceVersionChanged(oldShoot *gardenv1beta1.Shoot, newShoot *gardenv1beta1.Shoot) bool {
	return oldShoot.ObjectMeta.ResourceVersion != newShoot.ObjectMeta.ResourceVersion
}

// CheckIfSpecChanged returns true if the .spec fields of oldShoot and newShoot differ, and false otherwise.
func CheckIfSpecChanged(oldShoot *gardenv1beta1.Shoot, newShoot *gardenv1beta1.Shoot) bool {
	return !reflect.DeepEqual(oldShoot.Spec, newShoot.Spec)
}

// CheckIfStatusChanged returns true if the .status fields of oldShoot and newShoot differ, and false otherwise.
func CheckIfStatusChanged(oldShoot *gardenv1beta1.Shoot, newShoot *gardenv1beta1.Shoot) bool {
	return !reflect.DeepEqual(oldShoot.Status, newShoot.Status)
}
