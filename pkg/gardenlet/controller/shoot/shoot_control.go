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
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardencore "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/version"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) shootAdd(obj interface{}, resetRateLimiting bool) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	if resetRateLimiting {
		c.getShootQueue(obj).Forget(key)
	}
	c.getShootQueue(obj).Add(key)
}

func (c *Controller) shootUpdate(oldObj, newObj interface{}) {
	var (
		oldShoot        = oldObj.(*gardencorev1beta1.Shoot)
		newShoot        = newObj.(*gardencorev1beta1.Shoot)
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

	// If the shoot's deletion timestamp is set then we want to forget about the potentially established exponential
	// backoff and enqueue it faster.
	resetRateLimiting := oldShoot.DeletionTimestamp == nil && newShoot.DeletionTimestamp != nil

	c.shootAdd(newObj, resetRateLimiting)
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
	return controllerutils.BoolPtrDerefOr(c.config.Controllers.Shoot.ReconcileInMaintenanceOnly, false)
}

func (c *Controller) respectSyncPeriodOverwrite() bool {
	return controllerutils.BoolPtrDerefOr(c.config.Controllers.Shoot.RespectSyncPeriodOverwrite, false)
}

func confineSpecUpdateRollout(maintenance *gardencorev1beta1.Maintenance) bool {
	return maintenance != nil && maintenance.ConfineSpecUpdateRollout != nil && *maintenance.ConfineSpecUpdateRollout
}

func (c *Controller) syncClusterResourceToSeed(ctx context.Context, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) error {
	clusterName := shootpkg.ComputeTechnicalID(project.Name, shoot)

	seedClient, err := c.clientMap.GetClient(ctx, keys.ForSeedWithName(*shoot.Spec.SeedName))
	if err != nil {
		return fmt.Errorf("could not initialize a new Kubernetes client for the seed cluster: %+v", err)
	}

	return common.SyncClusterResourceToSeed(ctx, seedClient.Client(), clusterName, shoot, cloudProfile, seed)
}

func (c *Controller) checkSeedAndSyncClusterResource(ctx context.Context, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) error {
	seedName := shoot.Spec.SeedName
	if seedName == nil || seed == nil {
		return nil
	}

	seed, err := c.seedLister.Get(*seedName)
	if err != nil {
		return fmt.Errorf("could not find seed %s: %v", *seedName, err)
	}

	// Don't wait for the Seed to be ready if it is already marked for deletion. In this case
	// it will never get ready because the bootstrap loop is never executed again.
	// Don't block the Shoot deletion flow in this case to allow proper cleanup.
	if seed.DeletionTimestamp == nil {
		if err := health.CheckSeed(seed, c.identity); err != nil {
			return fmt.Errorf("seed is not yet ready: %v", err)
		}
	}

	if err := c.syncClusterResourceToSeed(ctx, shoot, project, cloudProfile, seed); err != nil {
		return fmt.Errorf("could not sync cluster resource to seed: %v", err)
	}

	return nil
}

// deleteClusterResourceFromSeed deletes the `Cluster` extension resource for the shoot in the seed cluster.
func (c *Controller) deleteClusterResourceFromSeed(ctx context.Context, shoot *gardencorev1beta1.Shoot, projectName string) error {
	if shoot.Spec.SeedName == nil {
		return nil
	}

	seedClient, err := c.clientMap.GetClient(ctx, keys.ForSeedWithName(*shoot.Spec.SeedName))
	if err != nil {
		return fmt.Errorf("could not initialize a new Kubernetes client for the seed cluster: %+v", err)
	}

	cluster := &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: shootpkg.ComputeTechnicalID(projectName, shoot),
		},
	}

	return client.IgnoreNotFound(seedClient.Client().Delete(ctx, cluster))
}

func (c *Controller) reconcileShootRequest(req reconcile.Request) (reconcile.Result, error) {
	log := logger.NewShootLogger(logger.Logger, req.Name, req.Namespace)

	shoot, err := c.shootLister.Shoots(req.Namespace).Get(req.Name)
	if apierrors.IsNotFound(err) {
		log.Debug("Skipping because Shoot has been deleted")
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	// fetch related objects required for shoot operation
	project, err := common.ProjectForNamespace(c.k8sGardenCoreInformers.Core().V1beta1().Projects().Lister(), shoot.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}
	cloudProfile, err := c.k8sGardenCoreInformers.Core().V1beta1().CloudProfiles().Lister().Get(shoot.Spec.CloudProfileName)
	if err != nil {
		return reconcile.Result{}, err
	}
	seed, err := c.k8sGardenCoreInformers.Core().V1beta1().Seeds().Lister().Get(*shoot.Spec.SeedName)
	if err != nil {
		return reconcile.Result{}, err
	}

	key := shootKey(shoot)
	if shoot.DeletionTimestamp != nil {
		log = log.WithField("operation", "delete")
		c.shootReconciliationDueTracker.off(key)
		return c.deleteShoot(log, shoot, project, cloudProfile, seed)
	}

	if shouldPrepareShootForMigration(shoot) {
		log = log.WithField("operation", "migrate")
		c.shootReconciliationDueTracker.off(key)

		if err := c.isSeedReadyForMigration(seed); err != nil {
			return reconcile.Result{}, fmt.Errorf("target Seed is not available to host the Control Plane of Shoot %s: %v", shoot.GetName(), err)
		}

		sourceSeed, err := c.k8sGardenCoreInformers.Core().V1beta1().Seeds().Lister().Get(*shoot.Status.SeedName)
		if err != nil {
			return reconcile.Result{}, err
		}

		return c.prepareShootForMigration(log, shoot, project, cloudProfile, sourceSeed)
	}

	// if shoot is no longer managed by this gardenlet (e.g., due to migration to another seed) then don't requeue
	if !controllerutils.ShootIsManagedByThisGardenlet(shoot, c.config, c.seedLister) {
		log.Debugf("Skipping because Shoot is not managed by this gardenlet in seed %s", *shoot.Spec.SeedName)
		return reconcile.Result{}, nil
	}

	log = log.WithField("operation", "reconcile")
	return c.reconcileShoot(log, shoot, project, cloudProfile, seed)
}

func shouldPrepareShootForMigration(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Status.SeedName != nil && shoot.Spec.SeedName != nil && *shoot.Spec.SeedName != *shoot.Status.SeedName
}

const taskID = "initializeOperation"

func (c *Controller) initializeOperation(ctx context.Context, logger *logrus.Entry, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) (*operation.Operation, error) {
	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("failed to get garden client: %w", err)
	}

	gardenObj, err := garden.
		NewBuilder().
		WithProject(project).
		WithInternalDomainFromSecrets(c.secrets).
		WithDefaultDomainsFromSecrets(c.secrets).
		Build()
	if err != nil {
		return nil, err
	}

	seedObj, err := seedpkg.
		NewBuilder().
		WithSeedObject(seed).
		Build()
	if err != nil {
		return nil, err
	}

	shootObj, err := shootpkg.
		NewBuilder().
		WithShootObject(shoot).
		WithCloudProfileObject(cloudProfile).
		WithShootSecretFromSecretBindingLister(c.k8sGardenCoreInformers.Core().V1beta1().SecretBindings().Lister()).
		WithProjectName(project.Name).
		WithDisableDNS(!seedObj.Info.Spec.Settings.ShootDNS.Enabled).
		WithInternalDomain(gardenObj.InternalDomain).
		WithDefaultDomains(gardenObj.DefaultDomains).
		Build(ctx, gardenClient.Client())
	if err != nil {
		return nil, err
	}

	return operation.
		NewBuilder().
		WithLogger(logger).
		WithConfig(c.config).
		WithGardenerInfo(c.identity).
		WithGardenClusterIdentity(c.gardenClusterIdentity).
		WithSecrets(c.secrets).
		WithImageVector(c.imageVector).
		WithGarden(gardenObj).
		WithSeed(seedObj).
		WithShoot(shootObj).
		Build(ctx, c.clientMap)
}

func (c *Controller) deleteShoot(logger *logrus.Entry, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) (reconcile.Result, error) {
	var (
		ctx = context.TODO()
		err error

		operationType              = gardencorev1beta1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation)
		respectSyncPeriodOverwrite = c.respectSyncPeriodOverwrite()
		failed                     = common.IsShootFailed(shoot)
		ignored                    = common.ShouldIgnoreShoot(respectSyncPeriodOverwrite, shoot)
		failedOrIgnored            = failed || ignored
	)

	if !controllerutils.HasFinalizer(shoot, gardencorev1beta1.GardenerName) {
		return reconcile.Result{}, nil
	}

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	// If the .status.uid field is empty, then we assume that there has never been any operation running for this Shoot
	// cluster. This implies that there can not be any resource which we have to delete.
	// We accept the deletion.
	if len(shoot.Status.UID) == 0 {
		logger.Info("`.status.uid` is empty, assuming Shoot cluster did never exist. Deletion accepted.")
		return c.finalizeShootDeletion(ctx, gardenClient, shoot, project.Name)
	}

	// If the .status.lastOperation already indicates that the deletion is successful then we finalize it immediately.
	if shoot.Status.LastOperation != nil && shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeDelete && shoot.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
		logger.Info("`.status.lastOperation` indicates a successful deletion. Deletion accepted.")
		return c.finalizeShootDeletion(ctx, gardenClient, shoot, project.Name)
	}

	// If shoot is failed or ignored then sync the Cluster resource so that extension controllers running in the seed
	// get to know of the shoot's status.
	if failedOrIgnored {
		if syncErr := c.syncClusterResourceToSeed(ctx, shoot, project, cloudProfile, seed); syncErr != nil {
			logger.WithError(syncErr).Infof("Not allowed to update Shoot with error, trying to sync Cluster resource again")
			_, updateErr := c.updateShootStatusOperationError(ctx, gardenClient.GardenCore(), shoot, syncErr.Error(), operationType, shoot.Status.LastErrors...)
			return reconcile.Result{}, utilerrors.WithSuppressed(syncErr, updateErr)
		}

		logger.Info("Shoot is failed or ignored")
		return reconcile.Result{}, nil
	}

	// Trigger regular shoot deletion flow.
	c.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting Shoot cluster")
	shootNamespace := shootpkg.ComputeTechnicalID(project.Name, shoot)
	shoot, err = c.updateShootStatusOperationStart(ctx, gardenClient.GardenCore(), shoot, shootNamespace, operationType)
	if err != nil {
		return reconcile.Result{}, err
	}

	o, operationErr := c.initializeOperation(ctx, logger, shoot, project, cloudProfile, seed)
	if operationErr != nil {
		_, updateErr := c.updateShootStatusOperationError(ctx, gardenClient.GardenCore(), shoot, fmt.Sprintf("Could not initialize a new operation for Shoot deletion: %s", operationErr.Error()), operationType, lastErrorsOperationInitializationFailure(shoot.Status.LastErrors, operationErr)...)
		return reconcile.Result{}, utilerrors.WithSuppressed(operationErr, updateErr)
	}

	// At this point the deletion is allowed, hence, check if the seed is up-to-date, then sync the Cluster resource
	// initialize a new operation and, eventually, start the deletion flow.
	if err := c.checkSeedAndSyncClusterResource(ctx, o.Shoot.Info, project, cloudProfile, seed); err != nil {
		return c.updateShootStatusAndRequeueOnSyncError(ctx, gardenClient.GardenCore(), o.Shoot.Info, logger, err)
	}

	if flowErr := c.runDeleteShootFlow(ctx, o); flowErr != nil {
		c.recorder.Event(o.Shoot.Info, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, flowErr.Description)
		_, updateErr := c.updateShootStatusOperationError(ctx, gardenClient.GardenCore(), o.Shoot.Info, flowErr.Description, operationType, flowErr.LastErrors...)
		return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(flowErr.Description), updateErr)
	}

	c.recorder.Event(o.Shoot.Info, corev1.EventTypeNormal, gardencorev1beta1.EventDeleted, "Deleted Shoot cluster")
	return c.finalizeShootDeletion(ctx, gardenClient, o.Shoot.Info, project.Name)
}

func (c *Controller) finalizeShootDeletion(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, projectName string) (reconcile.Result, error) {
	if cleanErr := c.deleteClusterResourceFromSeed(ctx, shoot, projectName); cleanErr != nil {
		lastErr := gardencorev1beta1helper.LastError(fmt.Sprintf("Could not delete Cluster resource in seed: %s", cleanErr))
		_, updateErr := c.updateShootStatusOperationError(ctx, gardenClient.GardenCore(), shoot, lastErr.Description, gardencorev1beta1.LastOperationTypeDelete, *lastErr)
		c.recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, lastErr.Description)
		return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(lastErr.Description), updateErr)
	}

	return reconcile.Result{}, c.removeFinalizerFrom(ctx, gardenClient, shoot)
}

func (c *Controller) isSeedReadyForMigration(seed *gardencorev1beta1.Seed) error {
	if seed.DeletionTimestamp != nil {
		return fmt.Errorf("seed is set for deletion")
	}

	return health.CheckSeedForMigration(seed, c.identity)
}

func (c *Controller) reconcileShoot(logger *logrus.Entry, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) (reconcile.Result, error) {
	var (
		ctx = context.TODO()

		key                                        = shootKey(shoot)
		operationType                              = gardencorev1beta1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation)
		respectSyncPeriodOverwrite                 = c.respectSyncPeriodOverwrite()
		failed                                     = common.IsShootFailed(shoot)
		ignored                                    = common.ShouldIgnoreShoot(respectSyncPeriodOverwrite, shoot)
		failedOrIgnored                            = failed || ignored
		reconcileInMaintenanceOnly                 = c.reconcileInMaintenanceOnly()
		isUpToDate                                 = common.IsObservedAtLatestGenerationAndSucceeded(shoot)
		isNowInEffectiveShootMaintenanceTimeWindow = common.IsNowInEffectiveShootMaintenanceTimeWindow(shoot)
		alreadyReconciledDuringThisTimeWindow      = common.LastReconciliationDuringThisTimeWindow(shoot)
		regularReconciliationIsDue                 = isUpToDate && isNowInEffectiveShootMaintenanceTimeWindow && !alreadyReconciledDuringThisTimeWindow
		reconcileAllowed                           = !failedOrIgnored && ((!reconcileInMaintenanceOnly && !confineSpecUpdateRollout(shoot.Spec.Maintenance)) || !isUpToDate || (isNowInEffectiveShootMaintenanceTimeWindow && !alreadyReconciledDuringThisTimeWindow))
	)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	if !controllerutils.HasFinalizer(shoot, gardencorev1beta1.GardenerName) {
		if err := controllerutils.EnsureFinalizer(ctx, gardenClient.Client(), shoot.DeepCopy(), gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not add finalizer to Shoot: %s", err.Error())
		}
		return reconcile.Result{}, nil
	}

	logger.WithFields(logrus.Fields{
		"operationType":              operationType,
		"respectSyncPeriodOverwrite": respectSyncPeriodOverwrite,
		"failed":                     failed,
		"ignored":                    ignored,
		"failedOrIgnored":            failedOrIgnored,
		"reconcileInMaintenanceOnly": reconcileInMaintenanceOnly,
		"isUpToDate":                 isUpToDate,
		"isNowInEffectiveShootMaintenanceTimeWindow": isNowInEffectiveShootMaintenanceTimeWindow,
		"alreadyReconciledDuringThisTimeWindow":      alreadyReconciledDuringThisTimeWindow,
		"regularReconciliationIsDue":                 regularReconciliationIsDue,
		"reconcileAllowed":                           reconcileAllowed,
	}).Info("Checking if Shoot can be reconciled")

	// If shoot is failed or ignored then sync the Cluster resource so that extension controllers running in the seed
	// get to know of the shoot's status.
	if failedOrIgnored {
		if syncErr := c.syncClusterResourceToSeed(ctx, shoot, project, cloudProfile, seed); syncErr != nil {
			logger.WithError(syncErr).Infof("Not allowed to update Shoot with error, trying to sync Cluster resource again")
			_, updateErr := c.updateShootStatusOperationError(ctx, gardenClient.GardenCore(), shoot, syncErr.Error(), operationType, shoot.Status.LastErrors...)
			return reconcile.Result{}, utilerrors.WithSuppressed(syncErr, updateErr)
		}

		logger.Info("Shoot is failed or ignored")
		return reconcile.Result{}, nil
	}

	// If reconciliation is not allowed then compute the duration until the next sync and requeue.
	if !reconcileAllowed {
		return c.scheduleNextSync(logger, shoot, regularReconciliationIsDue, "Reconciliation is not allowed"), nil
	}

	if regularReconciliationIsDue && !c.shootReconciliationDueTracker.tracked(key) {
		c.shootReconciliationDueTracker.on(key)
		return c.scheduleNextSync(logger, shoot, regularReconciliationIsDue, "Reconciliation allowed and due but not yet tracked (jittering next sync)"), nil
	}

	c.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling Shoot cluster state")
	shootNamespace := shootpkg.ComputeTechnicalID(project.Name, shoot)
	shoot, err = c.updateShootStatusOperationStart(ctx, gardenClient.GardenCore(), shoot, shootNamespace, operationType)
	if err != nil {
		return reconcile.Result{}, err
	}

	o, operationErr := c.initializeOperation(ctx, logger, shoot, project, cloudProfile, seed)
	if operationErr != nil {
		_, updateErr := c.updateShootStatusOperationError(ctx, gardenClient.GardenCore(), shoot, fmt.Sprintf("Could not initialize a new operation for Shoot reconciliation: %s", operationErr.Error()), operationType, lastErrorsOperationInitializationFailure(shoot.Status.LastErrors, operationErr)...)
		return reconcile.Result{}, utilerrors.WithSuppressed(operationErr, updateErr)
	}

	// write UID to status when operation was created successfully once
	if len(o.Shoot.Info.Status.UID) == 0 {
		if _, err := kutil.TryUpdateShootStatus(ctx, gardenClient.GardenCore(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
			func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
				shoot.Status.UID = shoot.UID
				return shoot, nil
			},
		); err != nil {
			return reconcile.Result{}, err
		}
	}

	// At this point the reconciliation is allowed, hence, check if the seed is up-to-date, then sync the Cluster resource
	// initialize a new operation and, eventually, start the reconciliation flow.
	if err := c.checkSeedAndSyncClusterResource(ctx, o.Shoot.Info, project, cloudProfile, seed); err != nil {
		return c.updateShootStatusAndRequeueOnSyncError(ctx, gardenClient.GardenCore(), o.Shoot.Info, logger, err)
	}

	c.shootReconciliationDueTracker.off(key)

	if flowErr := c.runReconcileShootFlow(o); flowErr != nil {
		c.recorder.Event(o.Shoot.Info, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, flowErr.Description)
		_, updateErr := c.updateShootStatusOperationError(ctx, gardenClient.GardenCore(), o.Shoot.Info, flowErr.Description, operationType, flowErr.LastErrors...)
		return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(flowErr.Description), updateErr)
	}

	c.recorder.Event(o.Shoot.Info, corev1.EventTypeNormal, gardencorev1beta1.EventReconciled, "Reconciled Shoot cluster state")
	o.Shoot.Info, err = c.updateShootStatusOperationSuccess(ctx, gardenClient.GardenCore(), o.Shoot.Info, o.Shoot.SeedNamespace, &o.Seed.Info.Name, operationType)
	if err != nil {
		return reconcile.Result{}, err
	}

	if syncErr := c.syncClusterResourceToSeed(ctx, o.Shoot.Info, project, cloudProfile, seed); syncErr != nil {
		logger.WithError(syncErr).Infof("Cluster resource sync to seed failed")
		_, updateErr := c.updateShootStatusOperationError(ctx, gardenClient.GardenCore(), o.Shoot.Info, syncErr.Error(), operationType, o.Shoot.Info.Status.LastErrors...)
		return reconcile.Result{}, utilerrors.WithSuppressed(syncErr, updateErr)
	}

	c.shootReconciliationDueTracker.on(key)
	return c.scheduleNextSync(logger, o.Shoot.Info, false, "Reconciliation finished successfully"), nil
}

func (c *Controller) scheduleNextSync(logger logrus.FieldLogger, shoot *gardencorev1beta1.Shoot, regularReconciliationIsDue bool, reason string) reconcile.Result {
	durationUntilNextSync := c.durationUntilNextShootSync(shoot, regularReconciliationIsDue)
	logger.Infof("%s - scheduling next queuing time in %s (%s)", reason, durationUntilNextSync, time.Now().UTC().Add(durationUntilNextSync))
	return reconcile.Result{RequeueAfter: durationUntilNextSync}
}

func (c *Controller) durationUntilNextShootSync(shoot *gardencorev1beta1.Shoot, regularReconciliationIsDue bool) time.Duration {
	syncPeriod := common.SyncPeriodOfShoot(c.respectSyncPeriodOverwrite(), c.config.Controllers.Shoot.SyncPeriod.Duration, shoot)
	if !c.reconcileInMaintenanceOnly() && !confineSpecUpdateRollout(shoot.Spec.Maintenance) {
		return syncPeriod
	}

	now := time.Now()
	window := common.EffectiveShootMaintenanceTimeWindow(shoot)

	return window.RandomDurationUntilNext(now, regularReconciliationIsDue)
}

func (c *Controller) updateShootStatusAndRequeueOnSyncError(ctx context.Context, g gardencore.Interface, shoot *gardencorev1beta1.Shoot, logger *logrus.Entry, err error) (reconcile.Result, error) {
	msg := fmt.Sprintf("Shoot cannot be synced with Seed: %v", err)
	logger.Error(err)

	_, updateErr := kutil.TryUpdateShootStatus(ctx, g, retry.DefaultRetry, shoot.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			shoot.Status.LastOperation.Description = msg
			shoot.Status.LastOperation.LastUpdateTime = metav1.NewTime(time.Now().UTC())
			return shoot, nil
		},
	)
	if updateErr != nil {
		return reconcile.Result{}, utilerrors.WithSuppressed(err, updateErr)
	}

	return reconcile.Result{
		RequeueAfter: 15 * time.Second, // prevent ddos-ing the seed
	}, nil
}

func (c *Controller) updateShootStatusOperationStart(ctx context.Context, g gardencore.Interface, shoot *gardencorev1beta1.Shoot, shootNamespace string, operationType gardencorev1beta1.LastOperationType) (*gardencorev1beta1.Shoot, error) {
	var (
		now                   = metav1.NewTime(time.Now().UTC())
		operationTypeSwitched bool
		description           string
	)

	switch operationType {
	case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile:
		description = "Reconciliation of Shoot cluster state initialized."
		operationTypeSwitched = false

	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Deletion of Shoot cluster in progress."
		operationTypeSwitched = shoot.Status.LastOperation != nil && shoot.Status.LastOperation.Type != gardencorev1beta1.LastOperationTypeDelete
	}

	return kutil.TryUpdateShootStatus(ctx, g, retry.DefaultRetry, shoot.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			if shoot.Status.RetryCycleStartTime == nil ||
				shoot.Generation != shoot.Status.ObservedGeneration ||
				shoot.Status.Gardener.Version != version.Get().GitVersion ||
				operationTypeSwitched {

				shoot.Status.RetryCycleStartTime = &now
			}

			if len(shoot.Status.TechnicalID) == 0 {
				shoot.Status.TechnicalID = shootNamespace
			}

			shoot.Status.LastErrors = gardencorev1beta1helper.DeleteLastErrorByTaskID(shoot.Status.LastErrors, taskID)
			shoot.Status.Gardener = *c.identity
			shoot.Status.ObservedGeneration = shoot.Generation
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:           operationType,
				State:          gardencorev1beta1.LastOperationStateProcessing,
				Progress:       0,
				Description:    description,
				LastUpdateTime: now,
			}
			return shoot, nil
		})
}

func (c *Controller) updateShootStatusOperationSuccess(ctx context.Context, g gardencore.Interface, shoot *gardencorev1beta1.Shoot, shootSeedNamespace string, seedName *string, operationType gardencorev1beta1.LastOperationType) (*gardencorev1beta1.Shoot, error) {
	var (
		now                        = metav1.NewTime(time.Now().UTC())
		description                string
		setConditionsToProgressing bool
		err                        error
		isHibernated               = gardencorev1beta1helper.HibernationIsEnabled(shoot)
	)

	switch operationType {
	case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile:
		description = "Shoot cluster state has been successfully reconciled."
		setConditionsToProgressing = true

	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Shoot cluster has been successfully deleted."
		setConditionsToProgressing = false
	}

	if len(shootSeedNamespace) > 0 {
		isHibernated, err = c.isHibernationActive(ctx, shootSeedNamespace, seedName)
		if err != nil {
			return nil, fmt.Errorf("error updating Shoot (%s/%s) after successful reconciliation when checking for active hibernation: %v", shoot.Namespace, shoot.Name, err)
		}
	}

	return kutil.TryUpdateShootStatus(ctx, g, retry.DefaultRetry, shoot.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			if setConditionsToProgressing {
				for i, cond := range shoot.Status.Conditions {
					switch cond.Type {
					case gardencorev1beta1.ShootAPIServerAvailable,
						gardencorev1beta1.ShootControlPlaneHealthy,
						gardencorev1beta1.ShootEveryNodeReady,
						gardencorev1beta1.ShootSystemComponentsHealthy:
						if cond.Status != gardencorev1beta1.ConditionFalse {
							shoot.Status.Conditions[i].Status = gardencorev1beta1.ConditionProgressing
							shoot.Status.Conditions[i].LastUpdateTime = metav1.Now()
						}
					}
				}
				for i, constr := range shoot.Status.Constraints {
					switch constr.Type {
					case gardencorev1beta1.ShootHibernationPossible,
						gardencorev1beta1.ShootMaintenancePreconditionsSatisfied:
						if constr.Status != gardencorev1beta1.ConditionFalse {
							shoot.Status.Constraints[i].Status = gardencorev1beta1.ConditionProgressing
							shoot.Status.Conditions[i].LastUpdateTime = metav1.Now()
						}
					}
				}
			}

			shoot.Status.SeedName = seedName
			shoot.Status.IsHibernated = isHibernated
			shoot.Status.RetryCycleStartTime = nil
			shoot.Status.LastErrors = nil
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:           operationType,
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				Progress:       100,
				Description:    description,
				LastUpdateTime: now,
			}
			return shoot, nil
		},
	)
}

func (c *Controller) updateShootStatusOperationError(ctx context.Context, g gardencore.Interface, shoot *gardencorev1beta1.Shoot, description string, operationType gardencorev1beta1.LastOperationType, lastErrors ...gardencorev1beta1.LastError) (*gardencorev1beta1.Shoot, error) {
	var (
		now          = metav1.NewTime(time.Now().UTC())
		state        = gardencorev1beta1.LastOperationStateError
		willNotRetry = gardencorev1beta1helper.HasNonRetryableErrorCode(lastErrors...) || utils.TimeElapsed(shoot.Status.RetryCycleStartTime, c.config.Controllers.Shoot.RetryDuration.Duration)
	)

	newShoot, err := kutil.TryUpdateShootStatus(ctx, g, retry.DefaultRetry, shoot.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			if willNotRetry {
				state = gardencorev1beta1.LastOperationStateFailed
				shoot.Status.RetryCycleStartTime = nil
			} else {
				description += " Operation will be retried."
			}

			shoot.Status.Gardener = *c.identity
			shoot.Status.LastErrors = lastErrors

			if shoot.Status.LastOperation == nil {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
			}
			shoot.Status.LastOperation.Type = operationType
			shoot.Status.LastOperation.State = state
			shoot.Status.LastOperation.Description = description
			shoot.Status.LastOperation.LastUpdateTime = now
			return shoot, nil
		})
	if err != nil {
		return nil, err
	}

	return kutil.TryUpdateShootLabels(ctx, g, retry.DefaultRetry, newShoot.ObjectMeta, shootpkg.StatusLabelTransform(shootpkg.StatusUnhealthy))
}

func lastErrorsOperationInitializationFailure(lastErrors []gardencorev1beta1.LastError, err error) []gardencorev1beta1.LastError {
	var incompleteDNSConfigError *shootpkg.IncompleteDNSConfigError

	if errors.As(err, &incompleteDNSConfigError) {
		return gardencorev1beta1helper.UpsertLastError(lastErrors, gardencorev1beta1.LastError{
			TaskID:      pointer.StringPtr(taskID),
			Description: err.Error(),
			Codes:       []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorConfigurationProblem},
		})
	}

	return lastErrors
}

// isHibernationActive uses the Cluster resource in the Seed to determine whether the Shoot is hibernated
// The Cluster contains the actual or "active" spec of the Shoot resource for this reconciliation
// as the Shoot resources field `spec.hibernation.enabled` might have changed during the reconciliation
func (c *Controller) isHibernationActive(ctx context.Context, shootSeedNamespace string, seedName *string) (bool, error) {
	if seedName == nil {
		return false, nil
	}

	seedClient, err := c.clientMap.GetClient(ctx, keys.ForSeedWithName(*seedName))
	if err != nil {
		return false, err
	}

	cluster, err := gardenerextensions.GetCluster(ctx, seedClient.Client(), shootSeedNamespace)
	if err != nil {
		return false, err
	}

	shoot := cluster.Shoot
	if shoot == nil {
		return false, fmt.Errorf("shoot is missing in cluster resource: %v", err)
	}

	return gardencorev1beta1helper.HibernationIsEnabled(shoot), nil
}

func shootKey(shoot *gardencorev1beta1.Shoot) string {
	return shoot.Namespace + "/" + shoot.Name
}

type reconciliationDueTracker struct {
	lock    sync.Mutex
	tracker map[string]bool
}

func newReconciliationDueTracker() *reconciliationDueTracker {
	return &reconciliationDueTracker{tracker: make(map[string]bool)}
}

func (t *reconciliationDueTracker) on(key string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.tracker[key] = true
}

func (t *reconciliationDueTracker) off(key string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	delete(t.tracker, key)
}

func (t *reconciliationDueTracker) tracked(key string) bool {
	t.lock.Lock()
	defer t.lock.Unlock()
	v, ok := t.tracker[key]
	return ok && v
}
