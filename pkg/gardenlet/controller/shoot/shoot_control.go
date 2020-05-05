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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
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

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	return controllerutils.BoolPtrDerefOr(c.config.Controllers.Shoot.ReconcileInMaintenanceOnly, false)
}

func (c *Controller) respectSyncPeriodOverwrite() bool {
	return controllerutils.BoolPtrDerefOr(c.config.Controllers.Shoot.RespectSyncPeriodOverwrite, false)
}

func confineSpecUpdateRollout(maintenance *gardencorev1beta1.Maintenance) bool {
	return maintenance != nil && maintenance.ConfineSpecUpdateRollout != nil && *maintenance.ConfineSpecUpdateRollout
}

// SyncClusterResourceToSeed creates or updates the `Cluster` extension resource for the shoot in the seed cluster.
// It contains the shoot, seed, and cloudprofile specification.
func (c *Controller) syncClusterResourceToSeed(ctx context.Context, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) error {
	if shoot.Spec.SeedName == nil {
		return nil
	}

	k8sSeedClient, err := seedpkg.GetSeedClient(ctx, c.k8sGardenClient.Client(), c.config.SeedClientConnection.ClientConnectionConfiguration, c.config.SeedSelector == nil, *shoot.Spec.SeedName)
	if err != nil {
		return fmt.Errorf("could not initialize a new Kubernetes client for the seed cluster: %+v", err)
	}

	var (
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootpkg.ComputeTechnicalID(project.Name, shoot),
			},
		}

		cloudProfileObj = cloudProfile.DeepCopy()
		seedObj         = seed.DeepCopy()
		shootObj        = shoot.DeepCopy()
	)

	cloudProfileObj.TypeMeta = metav1.TypeMeta{
		APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
		Kind:       "CloudProfile",
	}
	seedObj.TypeMeta = metav1.TypeMeta{
		APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
		Kind:       "Seed",
	}
	shootObj.TypeMeta = metav1.TypeMeta{
		APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
		Kind:       "Shoot",
	}

	// TODO: Workaround for the issue that was fixed with https://github.com/gardener/gardener/pull/2265. It adds a
	//       fake "observed generation" and a fake "last operation" and in case it is not set yet. This prevents the
	//       ShootNotFailed predicate in the extensions library from reacting false negatively. This fake status is only
	//       internally and will not be reported in the Shoot object in the garden cluster.
	//       This code can be removed in a future version after giving extension controllers enough time to revendor
	//       Gardener's extensions library.
	shootObj.Status.ObservedGeneration = shootObj.Generation
	if shootObj.Status.LastOperation == nil {
		shootObj.Status.LastOperation = &gardencorev1beta1.LastOperation{
			Type:  gardencorev1beta1.LastOperationTypeCreate,
			State: gardencorev1beta1.LastOperationStateSucceeded,
		}
	}

	_, err = controllerutil.CreateOrUpdate(ctx, k8sSeedClient.Client(), cluster, func() error {
		cluster.Spec.CloudProfile = runtime.RawExtension{Object: cloudProfileObj}
		cluster.Spec.Seed = runtime.RawExtension{Object: seedObj}
		cluster.Spec.Shoot = runtime.RawExtension{Object: shootObj}
		return nil
	})
	return err
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

	k8sSeedClient, err := seedpkg.GetSeedClient(ctx, c.k8sGardenClient.Client(), c.config.SeedClientConnection.ClientConnectionConfiguration, c.config.SeedSelector == nil, *shoot.Spec.SeedName)
	if err != nil {
		return fmt.Errorf("could not initialize a new Kubernetes client for the seed cluster: %s", err.Error())
	}

	cluster := &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: shootpkg.ComputeTechnicalID(projectName, shoot),
		},
	}

	return client.IgnoreNotFound(k8sSeedClient.Client().Delete(ctx, cluster))
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

	if shoot.DeletionTimestamp != nil {
		return c.deleteShoot(log, shoot, project, cloudProfile, seed)
	}
	return c.reconcileShoot(log, shoot, project, cloudProfile, seed)
}

func (c *Controller) updateShootStatusError(shoot *gardencorev1beta1.Shoot, message string) error {
	_, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.GardenCore(), retry.DefaultRetry, shoot.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:           gardencorev1beta1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation),
				State:          gardencorev1beta1.LastOperationStateError,
				Progress:       0,
				Description:    message,
				LastUpdateTime: metav1.Now(),
			}
			return shoot, nil
		})
	return err
}

func (c *Controller) updateShootStatusProcessing(shoot *gardencorev1beta1.Shoot, message string) error {
	_, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.GardenCore(), retry.DefaultRetry, shoot.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:           gardencorev1beta1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation),
				State:          gardencorev1beta1.LastOperationStateProcessing,
				Progress:       0,
				Description:    message,
				LastUpdateTime: metav1.Now(),
			}
			return shoot, nil
		})
	return err
}

func (c *Controller) initializeOperation(ctx context.Context, logger *logrus.Entry, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) (*operation.Operation, error) {
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
		WithSeedSecretFromClient(ctx, c.k8sGardenClient.Client()).
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
		WithDisableDNS(gardencorev1beta1helper.TaintsHave(seedObj.Info.Spec.Taints, gardencorev1beta1.SeedTaintDisableDNS)).
		WithInternalDomain(gardenObj.InternalDomain).
		WithDefaultDomains(gardenObj.DefaultDomains).
		Build(ctx, c.k8sGardenClient.Client())
	if err != nil {
		return nil, err
	}

	return operation.
		NewBuilder().
		WithLogger(logger).
		WithConfig(c.config).
		WithGardenerInfo(c.identity).
		WithSecrets(c.secrets).
		WithImageVector(c.imageVector).
		WithGarden(gardenObj).
		WithSeed(seedObj).
		WithShoot(shootObj).
		Build(ctx, c.k8sGardenClient)
}

func (c *Controller) deleteShoot(logger *logrus.Entry, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) (reconcile.Result, error) {
	var (
		ctx = context.TODO()

		respectSyncPeriodOverwrite = c.respectSyncPeriodOverwrite()
		failed                     = common.IsShootFailed(shoot)
		ignored                    = common.ShouldIgnoreShoot(respectSyncPeriodOverwrite, shoot)
		failedOrIgnored            = failed || ignored
	)

	if !controllerutils.HasFinalizer(shoot, gardencorev1beta1.GardenerName) {
		return reconcile.Result{}, nil
	}

	// If the .status.uid field is empty, then we assume that there has never been any operation running for this Shoot
	// cluster. This implies that there can not be any resource which we have to delete.
	// We accept the deletion.
	if len(shoot.Status.UID) == 0 {
		logger.Info("`.status.uid` is empty, assuming Shoot cluster did never exist. Deletion accepted.")
		c.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventDeleted, "Deleted Shoot cluster")
		return c.finalizeShootDeletion(ctx, logger, shoot, project.Name)
	}

	// If shoot is failed or ignored then sync the Cluster resource so that extension controllers running in the seed
	// get to know of the shoot's status.
	if failedOrIgnored {
		if err := c.syncClusterResourceToSeed(ctx, shoot, project, cloudProfile, seed); err != nil {
			logger.WithError(err).Infof("Not allowed to update Shoot with error, trying to sync Cluster resource again")
			return reconcile.Result{}, utilerrors.WithSuppressed(err, c.updateShootStatusDeleteError(logger, shoot, err.Error(), shoot.Status.LastErrors...))
		}

		logger.Info("Shoot is failed or ignored")
		return reconcile.Result{}, nil
	}

	o, err := c.initializeOperation(ctx, logger, shoot, project, cloudProfile, seed)
	if err != nil {
		return reconcile.Result{}, utilerrors.WithSuppressed(err, c.updateShootStatusError(shoot, fmt.Sprintf("Could not initialize a new operation for Shoot deletion: %s", err.Error())))
	}

	// Trigger regular shoot deletion flow.
	c.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting Shoot cluster")
	if err := c.updateShootStatusDeleteStart(o); err != nil {
		return reconcile.Result{}, err
	}

	// At this point the deletion is allowed, hence, check if the seed is up-to-date, then sync the Cluster resource
	// initialize a new operation and, eventually, start the deletion flow.
	if err := c.checkSeedAndSyncClusterResource(ctx, shoot, project, cloudProfile, seed); err != nil {
		message := fmt.Sprintf("Shoot cannot be synced with Seed: %v", err)
		c.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventOperationPending, message)

		if err := c.updateShootStatusProcessing(o.Shoot.Info, message); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: 15 * time.Second, // prevent ddos-ing the seed
		}, nil
	}

	if err := c.runDeleteShootFlow(o); err != nil {
		c.recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Description)
		return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(err.Description), c.updateShootStatusDeleteError(logger, o.Shoot.Info, err.Description, err.LastErrors...))
	}

	c.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventDeleted, "Deleted Shoot cluster")
	return c.finalizeShootDeletion(ctx, logger, o.Shoot.Info, project.Name)
}

func (c *Controller) finalizeShootDeletion(ctx context.Context, logger *logrus.Entry, shoot *gardencorev1beta1.Shoot, projectName string) (reconcile.Result, error) {
	if err := c.deleteClusterResourceFromSeed(ctx, shoot, projectName); err != nil {
		lastErr := gardencorev1beta1helper.LastError(fmt.Sprintf("Could not delete Cluster resource in seed: %s", err))
		c.recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, lastErr.Description)
		return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(lastErr.Description), c.updateShootStatusDeleteError(logger, shoot, lastErr.Description, *lastErr))
	}

	return reconcile.Result{}, c.updateShootStatusDeleteSuccess(shoot)
}

func (c *Controller) reconcileShoot(logger *logrus.Entry, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) (reconcile.Result, error) {
	var (
		ctx = context.TODO()

		operationType                              = gardencorev1beta1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation)
		respectSyncPeriodOverwrite                 = c.respectSyncPeriodOverwrite()
		failed                                     = common.IsShootFailed(shoot)
		ignored                                    = common.ShouldIgnoreShoot(respectSyncPeriodOverwrite, shoot)
		failedOrIgnored                            = failed || ignored
		reconcileInMaintenanceOnly                 = c.reconcileInMaintenanceOnly()
		isUpToDate                                 = common.IsObservedAtLatestGenerationAndSucceeded(shoot)
		isNowInEffectiveShootMaintenanceTimeWindow = common.IsNowInEffectiveShootMaintenanceTimeWindow(shoot)
		reconcileAllowed                           = !failedOrIgnored && ((!reconcileInMaintenanceOnly && !confineSpecUpdateRollout(shoot.Spec.Maintenance)) || !isUpToDate || isNowInEffectiveShootMaintenanceTimeWindow)
	)

	// need retry logic, because the scheduler is acting on it at the same time and cached object might not be up to date
	updatedShoot, err := kutil.TryUpdateShoot(c.k8sGardenClient.GardenCore(), retry.DefaultBackoff, shoot.ObjectMeta, func(curShoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
		finalizers := sets.NewString(curShoot.Finalizers...)
		if finalizers.Has(gardencorev1beta1.GardenerName) {
			return curShoot, nil
		}

		finalizers.Insert(gardencorev1beta1.GardenerName)
		curShoot.Finalizers = finalizers.UnsortedList()

		return curShoot, nil
	})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not add finalizer to Shoot: %s", err.Error())
	}
	shoot = updatedShoot

	logger.WithFields(logrus.Fields{
		"operationType":              operationType,
		"respectSyncPeriodOverwrite": respectSyncPeriodOverwrite,
		"failed":                     failed,
		"ignored":                    ignored,
		"failedOrIgnored":            failedOrIgnored,
		"reconcileInMaintenanceOnly": reconcileInMaintenanceOnly,
		"isUpToDate":                 isUpToDate,
		"isNowInEffectiveShootMaintenanceTimeWindow": isNowInEffectiveShootMaintenanceTimeWindow,
		"reconcileAllowed":                           reconcileAllowed,
	}).Info("Checking if Shoot can be reconciled")

	// If shoot is failed or ignored then sync the Cluster resource so that extension controllers running in the seed
	// get to know of the shoot's status.
	if failedOrIgnored {
		if err := c.syncClusterResourceToSeed(ctx, shoot, project, cloudProfile, seed); err != nil {
			logger.WithError(err).Infof("Not allowed to update Shoot with error, trying to sync Cluster resource again")
			return reconcile.Result{}, utilerrors.WithSuppressed(err, c.updateShootStatusReconcileError(shoot, operationType, err.Error(), shoot.Status.LastErrors...))
		}

		logger.Info("Shoot is failed or ignored")
		return reconcile.Result{}, nil
	}

	// If reconciliation is not allowed then compute the duration until the next sync and requeue.
	if !reconcileAllowed {
		durationUntilNextSync := c.durationUntilNextShootSync(shoot)
		message := fmt.Sprintf("Reconciliation not allowed, scheduling next sync in %s (%s)", durationUntilNextSync, time.Now().UTC().Add(durationUntilNextSync))
		c.recorder.Event(shoot, corev1.EventTypeNormal, "ScheduledNextSync", message)
		logger.Infof(message)
		return reconcile.Result{RequeueAfter: durationUntilNextSync}, nil
	}

	o, err := c.initializeOperation(ctx, logger, shoot, project, cloudProfile, seed)
	if err != nil {
		return reconcile.Result{}, utilerrors.WithSuppressed(err, c.updateShootStatusError(shoot, fmt.Sprintf("Could not initialize a new operation for Shoot reconciliation: %s", err.Error())))
	}

	// write UID to status when operation was created successfully once
	if len(shoot.Status.UID) == 0 {
		_, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.GardenCore(), retry.DefaultRetry, shoot.ObjectMeta,
			func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
				shoot.Status.UID = shoot.UID
				return shoot, nil
			},
		)
		return reconcile.Result{}, err
	}

	c.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling Shoot cluster state")
	if err := c.updateShootStatusReconcileStart(o, operationType); err != nil {
		return reconcile.Result{}, err
	}

	// At this point the reconciliation is allowed, hence, check if the seed is up-to-date, then sync the Cluster resource
	// initialize a new operation and, eventually, start the reconciliation flow.
	if err := c.checkSeedAndSyncClusterResource(ctx, shoot, project, cloudProfile, seed); err != nil {
		message := fmt.Sprintf("Shoot cannot be synced with Seed: %v", err)
		c.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventOperationPending, message)

		if err := c.updateShootStatusProcessing(o.Shoot.Info, message); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: 15 * time.Second, // prevent ddos-ing the seed
		}, nil
	}

	var dnsEnabled = !o.Shoot.DisableDNS

	// TODO: timuthy - Only required for migration and can be removed in a future version once admission plugin
	// forbids to create functionless DNS providers.
	if dnsEnabled && o.Shoot.Info.Spec.DNS != nil {
		updated, err := migrateDNSProviders(ctx, c.k8sGardenClient.Client(), o)
		if err != nil {
			message := "Cannot reconcile Shoot: Migrating DNS providers failed"
			c.recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, message)
			return reconcile.Result{}, utilerrors.WithSuppressed(fmt.Errorf("migrating dns providers failed: %v", err), c.updateShootStatusReconcileError(o.Shoot.Info, operationType, err.Error(), shoot.Status.LastErrors...))
		}
		if updated {
			message := "Requeue Shoot after migrating DNS providers"
			o.Logger.Info(message)
			c.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, message)
			return reconcile.Result{}, nil
		}
	}

	if err := c.runReconcileShootFlow(o); err != nil {
		c.recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Description)
		return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(err.Description), c.updateShootStatusReconcileError(o.Shoot.Info, operationType, err.Description, err.LastErrors...))
	}

	c.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventReconciled, "Reconciled Shoot cluster state")
	if err := c.updateShootStatusReconcileSuccess(o, operationType); err != nil {
		return reconcile.Result{}, err
	}

	if err := c.syncClusterResourceToSeed(ctx, o.Shoot.Info, project, cloudProfile, seed); err != nil {
		logger.WithError(err).Infof("Cluster resource sync to shoot failed")
		return reconcile.Result{}, utilerrors.WithSuppressed(err, c.updateShootStatusReconcileError(o.Shoot.Info, operationType, err.Error(), shoot.Status.LastErrors...))
	}

	durationUntilNextSync := c.durationUntilNextShootSync(shoot)
	message := fmt.Sprintf("Scheduled next queuing time for Shoot in %s (%s)", durationUntilNextSync, time.Now().UTC().Add(durationUntilNextSync))
	c.recorder.Event(shoot, corev1.EventTypeNormal, "ScheduledNextSync", message)
	return reconcile.Result{RequeueAfter: durationUntilNextSync}, nil
}

func (c *Controller) durationUntilNextShootSync(shoot *gardencorev1beta1.Shoot) time.Duration {
	syncPeriod := common.SyncPeriodOfShoot(c.respectSyncPeriodOverwrite(), c.config.Controllers.Shoot.SyncPeriod.Duration, shoot)
	if !c.reconcileInMaintenanceOnly() && !confineSpecUpdateRollout(shoot.Spec.Maintenance) {
		return syncPeriod
	}

	now := time.Now()
	window := common.EffectiveShootMaintenanceTimeWindow(shoot)

	if !window.Contains(now.Add(syncPeriod)) {
		return window.RandomDurationUntilNext(now)
	}
	return syncPeriod
}

func migrateDNSProviders(ctx context.Context, c client.Client, o *operation.Operation) (bool, error) {
	o.Logger.Info("Migration step - DNS providers")
	if err := kutil.TryUpdate(ctx, retry.DefaultBackoff, c, o.Shoot.Info, func() error {
		var (
			dns                = o.Shoot.Info.Spec.DNS
			primaryDNSProvider = gardencorev1beta1helper.FindPrimaryDNSProvider(dns.Providers)
			usesDefaultDomain  = o.Shoot.ExternalClusterDomain != nil && garden.DomainIsDefaultDomain(*o.Shoot.ExternalClusterDomain, o.Garden.DefaultDomains) != nil
		)
		// Set primary DNS provider field
		if !usesDefaultDomain && primaryDNSProvider != nil && primaryDNSProvider.Primary == nil {
			for i, provider := range dns.Providers {
				if provider.Type == primaryDNSProvider.Type && provider.SecretName == primaryDNSProvider.SecretName {
					dns.Providers[i].Primary = pointer.BoolPtr(true)
					break
				}
			}
		}

		// Remove functionless DNS providers
		// TODO: timuhty - Validation should forbid to create functionless DNS providers in the future.
		var providers []gardencorev1beta1.DNSProvider
		for _, provider := range dns.Providers {
			if utils.IsTrue(provider.Primary) || (provider.Type != nil && provider.SecretName != nil) {
				providers = append(providers, provider)
			}
		}
		dns.Providers = providers
		return nil
	}); err != nil {
		return false, nil
	}
	updated := o.Shoot.Info.ObjectMeta.Generation > o.Shoot.Info.Status.ObservedGeneration
	return updated, nil
}
