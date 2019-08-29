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

package shoot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) shootAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.getShootQueue(obj).Add(key)
}

func (c *Controller) shootUpdate(oldObj, newObj interface{}) {
	var (
		oldShoot        = oldObj.(*gardenv1beta1.Shoot)
		newShoot        = newObj.(*gardenv1beta1.Shoot)
		oldShootJSON, _ = json.Marshal(oldShoot)
		newShootJSON, _ = json.Marshal(newShoot)
		shootLogger     = logger.NewShootLogger(logger.Logger, newShoot.ObjectMeta.Name, newShoot.ObjectMeta.Namespace)
	)
	shootLogger.Debugf(string(oldShootJSON))
	shootLogger.Debugf(string(newShootJSON))

	// If the generation did not change for an update event (i.e., no changes to the .spec section have
	// been made), we do not want to add the Shoot to the queue. The period reconciliation is handled
	// elsewhere by adding the Shoot to the queue to dedicated times.
	if newShoot.Generation == newShoot.Status.ObservedGeneration {
		shootLogger.Debug("Do not need to do anything as the Update event occurred due to .status field changes")
		return
	}

	c.shootAdd(newObj)
}

func (c *Controller) shootDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	c.getShootQueue(obj).Add(key)
}

func (c *Controller) reconcileInMaintenanceOnly() bool {
	return utils.BoolPtrDerefOr(c.config.Controllers.Shoot.ReconcileInMaintenanceOnly, false)
}

func (c *Controller) respectSyncPeriodOverwrite() bool {
	return utils.BoolPtrDerefOr(c.config.Controllers.Shoot.RespectSyncPeriodOverwrite, false)
}

func (c *Controller) checkSeedAndSyncClusterResource(shoot *gardenv1beta1.Shoot, o *operation.Operation) error {
	seedName := shoot.Spec.Cloud.Seed
	if seedName == nil {
		return nil
	}

	seed, err := c.seedLister.Get(*seedName)
	if err != nil {
		return fmt.Errorf("could not find seed %s: %v", *seedName, err)
	}

	if err := health.CheckSeed(seed, c.identity); err != nil {
		return fmt.Errorf("seed is not yet ready: %v", err)
	}

	if err := o.SyncClusterResourceToSeed(context.TODO()); err != nil {
		return fmt.Errorf("could not sync cluster resource to seed: %v", err)
	}

	return nil
}

func (c *Controller) reconcileShootRequest(req reconcile.Request) (reconcile.Result, error) {
	log := logger.NewShootLogger(logger.Logger, req.Name, req.Namespace).WithField("operation", "reconcile")

	shoot, err := c.shootLister.Shoots(req.Namespace).Get(req.Name)
	if apierrors.IsNotFound(err) {
		log.Debug("Skipping because Shoot has been deleted")
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	o, err := operation.New(shoot, c.config, log, c.k8sGardenClient, c.k8sGardenInformers.Garden().V1beta1(), c.identity, c.secrets, c.imageVector, c.config.ShootBackup)
	if err != nil {
		return reconcile.Result{}, err
	}

	if shoot.DeletionTimestamp != nil {
		return c.deleteShoot(shoot, o)
	}
	return c.reconcileShoot(shoot, o)
}

func (c *Controller) updateShootStatusProcessing(shoot *gardenv1beta1.Shoot, message string) error {
	_, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.Garden(), retry.DefaultRetry, shoot.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			shoot.Status.LastOperation = &gardencorev1alpha1.LastOperation{
				Type:           gardencorev1alpha1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation),
				State:          gardencorev1alpha1.LastOperationStateProcessing,
				Progress:       0,
				Description:    message,
				LastUpdateTime: metav1.Now(),
			}
			return shoot, nil
		})
	return err
}

func (c *Controller) durationUntilNextShootSync(shoot *gardenv1beta1.Shoot) time.Duration {
	syncPeriod := common.SyncPeriodOfShoot(c.respectSyncPeriodOverwrite(), c.config.Controllers.Shoot.SyncPeriod.Duration, shoot)
	if !c.reconcileInMaintenanceOnly() {
		return syncPeriod
	}

	now := time.Now()
	window := common.EffectiveShootMaintenanceTimeWindow(shoot)

	if !window.Contains(now.Add(syncPeriod)) {
		return window.RandomDurationUntilNext(now)
	}
	return syncPeriod
}

func (c *Controller) deleteShoot(shoot *gardenv1beta1.Shoot, o *operation.Operation) (reconcile.Result, error) {
	if shoot.DeletionTimestamp != nil && !sets.NewString(shoot.Finalizers...).Has(gardenv1beta1.GardenerName) {
		return reconcile.Result{}, nil
	}

	if err := c.checkSeedAndSyncClusterResource(shoot, o); err != nil {
		lastErr := gardencorev1alpha1helper.LastError(fmt.Sprintf("Could not check and sync Shoot with Seed: %v", err))
		c.recorder.Event(shoot, corev1.EventTypeWarning, gardenv1beta1.EventDeleteError, lastErr.Description)
		return reconcile.Result{}, utilerrors.WithSuppressed(err, c.updateShootStatusDeleteError(o, lastErr))
	}

	if common.IsShootFailed(shoot) {
		o.Logger.Info("Shoot is failed")
		return reconcile.Result{}, nil
	}

	if common.ShouldIgnoreShoot(c.respectSyncPeriodOverwrite(), shoot) {
		o.Logger.Info("Shoot is being ignored")
		return reconcile.Result{}, nil
	}

	// If the .status.uid field is empty, then we assume that there has never been any operation running for this Shoot
	// cluster. This implies that there can not be any resource which we have to delete.
	// We accept the deletion.
	if len(o.Shoot.Info.Status.UID) == 0 {
		o.Logger.Info("`.status.uid` is empty, assuming Shoot cluster did never exist. Deletion accepted.")
		return c.finalizeShootDeletion(shoot, o)
	}

	// If the shoot has never been scheduled (this is the case e.g when the scheduler cannot find a seed for the shoot),
	// the gardener controller manager has never reconciled it. This implies that there can not be any resource which we
	// have to delete. We accept the deletion.
	if o.Shoot.Info.Spec.Cloud.Seed == nil {
		o.Logger.Info("`.spec.cloud.seed` is empty, assuming Shoot cluster has never been scheduled - thus never existed. Deletion accepted.")
		return c.finalizeShootDeletion(shoot, o)
	}

	// Trigger regular shoot deletion flow.
	c.recorder.Event(shoot, corev1.EventTypeNormal, gardenv1beta1.EventDeleting, "Deleting Shoot cluster")
	if err := c.updateShootStatusDeleteStart(o); err != nil {
		return reconcile.Result{}, err
	}

	if err := c.runDeleteShootFlow(o); err != nil {
		c.recorder.Event(shoot, corev1.EventTypeWarning, gardenv1beta1.EventDeleteError, err.Description)
		return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(err.Description), c.updateShootStatusDeleteError(o, err))
	}

	c.recorder.Event(shoot, corev1.EventTypeNormal, gardenv1beta1.EventDeleted, "Deleted Shoot cluster")
	return c.finalizeShootDeletion(shoot, o)
}

func (c *Controller) finalizeShootDeletion(shoot *gardenv1beta1.Shoot, o *operation.Operation) (reconcile.Result, error) {
	if err := o.DeleteClusterResourceFromSeed(context.TODO()); err != nil {
		lastErr := gardencorev1alpha1helper.LastError(fmt.Sprintf("Could not delete Cluster resource in seed: %s", err))
		c.recorder.Event(shoot, corev1.EventTypeWarning, gardenv1beta1.EventDeleteError, lastErr.Description)
		return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(lastErr.Description), c.updateShootStatusDeleteError(o, lastErr))
	}

	return reconcile.Result{}, c.updateShootStatusDeleteSuccess(o)
}

func (c *Controller) reconcileShoot(shoot *gardenv1beta1.Shoot, o *operation.Operation) (reconcile.Result, error) {
	var (
		operationType                              = gardencorev1alpha1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation)
		respectSyncPeriodOverwrite                 = c.respectSyncPeriodOverwrite()
		failed                                     = common.IsShootFailed(shoot)
		ignored                                    = common.ShouldIgnoreShoot(respectSyncPeriodOverwrite, shoot)
		failedOrIgnored                            = failed || ignored
		reconcileInMaintenanceOnly                 = c.reconcileInMaintenanceOnly()
		isUpToDate                                 = common.IsObservedAtLatestGenerationAndSucceeded(shoot)
		isNowInEffectiveShootMaintenanceTimeWindow = common.IsNowInEffectiveShootMaintenanceTimeWindow(shoot)
		reconcileAllowed                           = !reconcileInMaintenanceOnly || !isUpToDate || isNowInEffectiveShootMaintenanceTimeWindow
		allowedToUpdate                            = !failedOrIgnored && reconcileAllowed
	)
	o.Logger.WithFields(logrus.Fields{
		"operationType":              operationType,
		"respectSyncPeriodOverwrite": respectSyncPeriodOverwrite,
		"failed":                     failed,
		"ignored":                    ignored,
		"failedOrIgnored":            failedOrIgnored,
		"reconcileInMaintenanceOnly": reconcileInMaintenanceOnly,
		"isUpToDate":                 isUpToDate,
		"isNowInEffectiveShootMaintenanceTimeWindow": isNowInEffectiveShootMaintenanceTimeWindow,
		"reconcileAllowed":                           reconcileAllowed,
		"allowedToUpdate":                            allowedToUpdate,
	}).Info("Checking if Shoot can be reconciled")

	if err := c.checkSeedAndSyncClusterResource(shoot, o); err != nil {
		message := fmt.Sprintf("Shoot cannot be synced with Seed: %v", err)
		c.recorder.Event(shoot, corev1.EventTypeNormal, gardenv1beta1.EventOperationPending, message)
		if !allowedToUpdate {
			o.Logger.WithError(err).Infof("Not allowed to update shoot with error")
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, utilerrors.WithSuppressed(err, c.updateShootStatusProcessing(shoot, message))
	}

	if failedOrIgnored {
		o.Logger.Info("Shoot is failed or ignored")
		return reconcile.Result{}, nil
	}

	if !reconcileAllowed {
		durationUntilNextSync := c.durationUntilNextShootSync(shoot)
		message := fmt.Sprintf("Scheduled next queuing time for Shoot in %s (%s)", durationUntilNextSync, time.Now().UTC().Add(durationUntilNextSync))
		c.recorder.Event(shoot, corev1.EventTypeNormal, "ScheduledNextSync", message)
		return reconcile.Result{RequeueAfter: durationUntilNextSync}, nil
	}

	if shoot.Spec.Cloud.Seed == nil {
		message := "Cannot reconcile Shoot: Waiting for Shoot to get assigned to a Seed"
		c.recorder.Event(shoot, corev1.EventTypeWarning, "OperationPending", message)
		return reconcile.Result{}, utilerrors.WithSuppressed(fmt.Errorf("shoot %s/%s has not yet been scheduled on a Seed", shoot.Namespace, shoot.Name), c.updateShootStatusProcessing(shoot, message))
	}

	c.recorder.Event(shoot, corev1.EventTypeNormal, gardenv1beta1.EventReconciling, "Reconciling Shoot cluster state")
	if err := c.updateShootStatusReconcileStart(o, operationType); err != nil {
		return reconcile.Result{}, err
	}

	if err := c.runReconcileShootFlow(o, operationType); err != nil {
		c.recorder.Event(shoot, corev1.EventTypeWarning, gardenv1beta1.EventReconcileError, err.Description)
		return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(err.Description), c.updateShootStatusReconcileError(o, operationType, err))
	}

	c.recorder.Event(shoot, corev1.EventTypeNormal, gardenv1beta1.EventReconciled, "Reconciled Shoot cluster state")
	if err := c.updateShootStatusReconcileSuccess(o, operationType); err != nil {
		return reconcile.Result{}, err
	}

	durationUntilNextSync := c.durationUntilNextShootSync(shoot)
	message := fmt.Sprintf("Scheduled next queuing time for Shoot in %s (%s)", durationUntilNextSync, time.Now().UTC().Add(durationUntilNextSync))
	c.recorder.Event(shoot, corev1.EventTypeNormal, "ScheduledNextSync", message)
	return reconcile.Result{RequeueAfter: durationUntilNextSync}, nil
}
