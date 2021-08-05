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

package care

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GarbageCollection contains required information for shoot and seed garbage collection.
type GarbageCollection struct {
	initializeShootClients ShootClientInit
	shoot                  *shoot.Shoot
	seedClient             client.Client
	logger                 logrus.FieldLogger
}

// NewGarbageCollection creates a new garbage collection instance.
func NewGarbageCollection(op *operation.Operation, shootClientInit ShootClientInit) *GarbageCollection {
	return &GarbageCollection{
		shoot:                  op.Shoot,
		initializeShootClients: shootClientInit,
		seedClient:             op.K8sSeedClient.Client(),
		logger:                 op.Logger,
	}
}

// Collect cleans the Seed and the Shoot cluster from no longer required
// objects. It receives a botanist object <botanist> which stores the Shoot object.
func (g *GarbageCollection) Collect(ctx context.Context) {
	var (
		qualifiedShootName = fmt.Sprintf("%s/%s", g.shoot.GetInfo().Namespace, g.shoot.GetInfo().Name)
		wg                 sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := g.performGarbageCollectionSeed(ctx); err != nil {
			g.logger.Errorf("Error during seed garbage collection: %+v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		shootClient, apiServerRunning, err := g.initializeShootClients()
		if err != nil || !apiServerRunning {
			if err != nil {
				g.logger.Errorf("Could not initialize Shoot client for garbage collection of shoot %s: %+v", qualifiedShootName, err)
			}
			return
		}
		if err := g.performGarbageCollectionShoot(ctx, shootClient.Client()); err != nil {
			g.logger.Errorf("Error during shoot garbage collection: %+v", err)
		}
	}()

	wg.Wait()
	g.logger.Debugf("Successfully performed full garbage collection for Shoot cluster %s", qualifiedShootName)
}

// PerformGarbageCollectionSeed performs garbage collection in the Shoot namespace in the Seed cluster
func (g *GarbageCollection) performGarbageCollectionSeed(ctx context.Context) error {
	podList := &corev1.PodList{}
	if err := g.seedClient.List(ctx, podList, client.InNamespace(g.shoot.SeedNamespace)); err != nil {
		return err
	}

	return g.deleteStalePods(ctx, g.seedClient, podList)
}

// PerformGarbageCollectionShoot performs garbage collection in the kube-system namespace in the Shoot
// cluster, i.e., it deletes evicted pods (mitigation for https://github.com/kubernetes/kubernetes/issues/55051).
func (g *GarbageCollection) performGarbageCollectionShoot(ctx context.Context, shootClient client.Client) error {
	namespace := metav1.NamespaceSystem
	if g.shoot.GetInfo().DeletionTimestamp != nil {
		namespace = metav1.NamespaceAll
	}

	podList := &corev1.PodList{}
	if err := shootClient.List(ctx, podList, client.InNamespace(namespace)); err != nil {
		return err
	}

	return g.deleteStalePods(ctx, shootClient, podList)
}

// GardenerDeletionGracePeriod is the default grace period for Gardener's force deletion methods.
const GardenerDeletionGracePeriod = 5 * time.Minute

func (g *GarbageCollection) deleteStalePods(ctx context.Context, c client.Client, podList *corev1.PodList) error {
	var result error

	for _, pod := range podList.Items {
		if strings.Contains(pod.Status.Reason, "Evicted") || strings.HasPrefix(pod.Status.Reason, "OutOf") {
			g.logger.Debugf("Deleting pod %s as its reason is %s.", pod.Name, pod.Status.Reason)
			if err := c.Delete(ctx, &pod, kubernetes.DefaultDeleteOptions...); client.IgnoreNotFound(err) != nil {
				result = multierror.Append(result, err)
			}
			continue
		}

		if shouldObjectBeRemoved(&pod, GardenerDeletionGracePeriod) {
			g.logger.Debugf("Deleting stuck terminating pod %q", pod.Name)
			if err := c.Delete(ctx, &pod, kubernetes.ForceDeleteOptions...); client.IgnoreNotFound(err) != nil {
				result = multierror.Append(result, err)
			}
		}
	}

	return result
}

// shouldObjectBeRemoved determines whether the given object should be gone now.
// This is calculated by first checking the deletion timestamp of an object: If the deletion timestamp
// is unset, the object should not be removed - i.e. this returns false.
// Otherwise, it is checked whether the deletionTimestamp is before the current time minus the
// grace period.
func shouldObjectBeRemoved(obj metav1.Object, gracePeriod time.Duration) bool {
	deletionTimestamp := obj.GetDeletionTimestamp()
	if deletionTimestamp == nil {
		return false
	}

	return deletionTimestamp.Time.Before(time.Now().Add(-gracePeriod))
}
