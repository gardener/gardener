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

package project

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/tools/cache"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const projectActivityReconcilerName = "project-activity"

// NewActivityReconciler creates a new instance of a reconciler which reconciles the LastActivityTimestamp of a project, then it calls the stale reconciler.
func NewActivityReconciler(gardenClient client.Client, clock clock.Clock) reconcile.Reconciler {
	return &projectActivityReconciler{
		gardenClient: gardenClient,
		clock:        clock,
	}
}

type projectActivityReconciler struct {
	gardenClient client.Client
	clock        clock.Clock
}

func (r *projectActivityReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	project := &gardencorev1beta1.Project{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	now := r.clock.Now()
	log.Info("Updating Project's lastActivityTimestamp", "lastActivityTimestamp", now)
	if err := updateStatus(ctx, r.gardenClient, project, func() {
		project.Status.LastActivityTimestamp = &metav1.Time{Time: now}
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed updating Project's lastActivityTimestamp: %w", err)
	}

	return reconcile.Result{}, nil
}

func (c *Controller) projectActivityBackupEntryAdd(ctx context.Context, obj interface{}) {
	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return
	}

	// If the creationTimestamp of the object is less than 1 hour (experimental) from current time,
	// skip it. This is to prevent unnecessary reconciliations in case of GCM restart.
	if backupEntry.GetCreationTimestamp().Add(time.Hour).UTC().Before(c.clock.Now().UTC()) {
		return
	}

	key := c.getProjectKey(ctx, backupEntry.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) projectActivityBackupEntryUpdate(ctx context.Context, oldObj, newObj interface{}) {
	oldBackupEntry, ok := oldObj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return
	}
	newBackupEntry, ok := newObj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return
	}

	if apiequality.Semantic.DeepEqual(newBackupEntry.Spec, oldBackupEntry.Spec) {
		return
	}

	key := c.getProjectKey(ctx, newBackupEntry.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) projectActivityPlantAdd(ctx context.Context, obj interface{}) {
	plant, ok := obj.(*gardencorev1beta1.Plant)
	if !ok {
		return
	}

	// If the creationTimestamp of the object is less than 1 hour (experimental) from current time,
	// skip it. This is to prevent unnecessary reconciliations in case of GCM restart.
	if plant.GetCreationTimestamp().Add(time.Hour).UTC().Before(c.clock.Now().UTC()) {
		return
	}

	key := c.getProjectKey(ctx, plant.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) projectActivityPlantUpdate(ctx context.Context, oldObj, newObj interface{}) {
	oldPlant, ok := oldObj.(*gardencorev1beta1.Plant)
	if !ok {
		return
	}
	newPlant, ok := newObj.(*gardencorev1beta1.Plant)
	if !ok {
		return
	}

	if apiequality.Semantic.DeepEqual(newPlant.Spec, oldPlant.Spec) {
		return
	}

	key := c.getProjectKey(ctx, newPlant.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) projectActivityQuotaAdd(ctx context.Context, obj interface{}) {
	quota, ok := obj.(*gardencorev1beta1.Quota)
	if !ok {
		return
	}

	// If the creationTimestamp of the object is less than 1 hour (experimental) from current time,
	// skip it. This is to prevent unnecessary reconciliations in case of GCM restart.
	if quota.GetCreationTimestamp().Add(time.Hour).UTC().Before(c.clock.Now().UTC()) {
		return
	}

	key := c.getProjectKey(ctx, quota.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) projectActivityQuotaUpdate(ctx context.Context, oldObj, newObj interface{}) {
	oldQuota, ok := oldObj.(*gardencorev1beta1.Quota)
	if !ok {
		return
	}
	newQuota, ok := newObj.(*gardencorev1beta1.Quota)
	if !ok {
		return
	}

	if apiequality.Semantic.DeepEqual(newQuota.Spec, oldQuota.Spec) {
		return
	}

	key := c.getProjectKey(ctx, newQuota.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) projectActivitySecretAdd(ctx context.Context, obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return
	}

	// skip queueing if the secret doesn't have the "referred by a secretbinding" label
	if !metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
		return
	}

	// If the creationTimestamp of the object is less than 1 hour (experimental) from current time,
	// skip it. This is to prevent unnecessary reconciliations in case of GCM restart.
	if secret.GetCreationTimestamp().Add(time.Hour).UTC().Before(c.clock.Now().UTC()) {
		return
	}

	key := c.getProjectKey(ctx, secret.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) projectActivitySecretUpdate(ctx context.Context, oldObj, newObj interface{}) {
	oldSecret, ok := oldObj.(*corev1.Secret)
	if !ok {
		return
	}
	newSecret, ok := newObj.(*corev1.Secret)
	if !ok {
		return
	}

	// skip queueing if the secret doesn't have the "referred by a secretbinding" label
	if !metav1.HasLabel(newSecret.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
		return
	}

	if apiequality.Semantic.DeepEqual(newSecret.Data, oldSecret.Data) {
		return
	}

	key := c.getProjectKey(ctx, newSecret.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) projectActivityShootAdd(ctx context.Context, obj interface{}) {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}

	// If the creationTimestamp of the object is less than 1 hour (experimental) from current time,
	// skip it. This is to prevent unnecessary reconciliations in case of GCM restart.
	if shoot.GetCreationTimestamp().Add(time.Hour).UTC().Before(c.clock.Now().UTC()) {
		return
	}

	key := c.getProjectKey(ctx, shoot.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) projectActivityShootUpdate(ctx context.Context, oldObj, newObj interface{}) {
	oldShoot, ok := oldObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}
	newShoot, ok := newObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}

	if apiequality.Semantic.DeepEqual(newShoot.Spec, oldShoot.Spec) {
		return
	}

	key := c.getProjectKey(ctx, newShoot.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) getProjectKey(ctx context.Context, namespace string) string {
	project, err := gutil.ProjectForNamespaceFromReader(ctx, c.gardenClient, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// project is gone, nothing to do here
			return ""
		}
		c.log.Error(err, "Failed to get project for namespace", "namespace", namespace)
		return ""
	}

	key, err := cache.MetaNamespaceKeyFunc(project)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", project)
		return ""
	}

	return key
}
