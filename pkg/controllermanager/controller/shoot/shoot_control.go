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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/reconcilescheduler"
	"github.com/gardener/gardener/pkg/version"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
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
		shootLogger     = logger.NewShootLogger(logger.Logger, newShoot.ObjectMeta.Name, newShoot.ObjectMeta.Namespace, "")
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

func (c *Controller) newShootElement(shoot *gardenv1beta1.Shoot) (*shootElement, error) {
	shootedSeed, err := c.usesShootedSeed(shoot)
	if err != nil {
		logger.Logger.Errorf("Error while trying to identify whether shoot %q uses a shoot seed: %+v", shoot.Name, err)
		return nil, err
	}

	var p id
	switch {
	case shootedSeed != nil:
		p = newID(shootedSeed.Namespace, shootedSeed.Name)
	case shoot.Spec.Cloud.Seed != nil:
		p = newID("", *shoot.Spec.Cloud.Seed)
	default:
		return nil, fmt.Errorf("Could not identify seed for shoot %q", shoot.Name)
	}

	return &shootElement{
		shoot: newID(shoot.Namespace, shoot.Name),
		seed:  p,
	}, nil
}

func (c *Controller) reconcileShootKey(key string) error {
	shootID, err := newIDFromString(key)
	if err != nil {
		return err
	}

	shoot, err := c.shootLister.Shoots(shootID.GetNamespace()).Get(shootID.GetName())
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SHOOT RECONCILE] %s - skipping because Shoot has been deleted", key)
		c.scheduler.Delete(shootID)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SHOOT RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	var (
		shootLogger  = logger.NewShootLogger(logger.Logger, shoot.ObjectMeta.Name, shoot.ObjectMeta.Namespace, "")
		needsRequeue = true
		reconcileErr error
	)

	// Ignore Shoots which do not have the gardener finalizer.
	if shoot.DeletionTimestamp != nil && !sets.NewString(shoot.Finalizers...).Has(gardenv1beta1.GardenerName) {
		shootLogger.Debug("Do not need to do anything as the Shoot does not have my finalizer")
		c.scheduler.Delete(shootID)
		c.getShootQueue(shoot).Forget(key)
		return nil
	}

	shootElement, err := c.newShootElement(shoot)
	if err != nil {
		return err
	}

	var (
		mayReconcile bool
		reason       *reconcilescheduler.Reason
	)

	// Check whether the shoot may be reconciled. We always allow reconciliation if the shoot shall be deleted or
	// if the shoot shall be created.
	switch {
	case shoot.DeletionTimestamp != nil:
		c.scheduler.Delete(shootID)
		mayReconcile, reason = true, reconcilescheduler.NewReason(reconcilescheduler.CodeOther, "shoot shall be deleted")
	case shoot.Status.LastOperation != nil && shoot.Status.LastOperation.Type == gardencorev1alpha1.LastOperationTypeCreate:
		mayReconcile, reason = true, reconcilescheduler.NewReason(reconcilescheduler.CodeOther, "shoot shall be created")
	default:
		mayReconcile, reason = c.scheduler.TestAndActivate(shootElement, shoot.Generation != shoot.Status.ObservedGeneration, shootIsSeed(shoot))
	}

	switch {
	case mustIgnoreShoot(shoot.Annotations, c.config.Controllers.Shoot.RespectSyncPeriodOverwrite):
		// Check whether the shoot has been marked as "never reconcile".
		shootLogger.Info("Skipping reconciliation because Shoot is marked as 'to-be-ignored'.")

	case !mayReconcile:
		// If the shoot may not be reconciled (due to above decision) then we mark it as pending in case it was not failed
		message := fmt.Sprintf("May not yet reconcile shoot %q: %s", shoot.Name, reason)
		if !shootIsFailed(shoot) {
			shootLogger.Debugf(message)
			c.recorder.Eventf(shoot, corev1.EventTypeNormal, "OperationPending", message)
			c.updateShootStatusPending(shoot, message)
		}

	default:
		// Otherwise (i.e., shoot is not ignored and may be reconciled) we start the reconcile operation).
		needsRequeue, reconcileErr = c.control.ReconcileShoot(shoot, key)
	}
	c.scheduler.Done(shootElement.GetID())

	if durationToNextSync := scheduleNextSync(c.config.Controllers.Shoot, reconcileErr != nil, shoot.ObjectMeta, reason); durationToNextSync > 0 && needsRequeue {
		c.getShootQueue(shoot).AddAfter(key, durationToNextSync)
		message := fmt.Sprintf("Scheduled next queuing time for Shoot '%s' in %s (%s)", key, durationToNextSync, time.Now().UTC().Add(durationToNextSync))
		shootLogger.Infof(message)
		c.recorder.Eventf(shoot, corev1.EventTypeNormal, "ScheduledNextSync", message)
	}

	return nil
}

func (c *Controller) updateShootStatusPending(shoot *gardenv1beta1.Shoot, message string) error {
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

func scheduleNextSync(config config.ShootControllerConfiguration, errorOccurred bool, objectMeta metav1.ObjectMeta, reason *reconcilescheduler.Reason) time.Duration {
	switch {
	case reason == nil, reason.Code() == reconcilescheduler.CodeOther, reason.Code() == reconcilescheduler.CodeActivated:
	case reason.Code() == reconcilescheduler.CodeParentUnknown:
		return time.Second
	default:
		return 10 * time.Second
	}

	if errorOccurred {
		return (*config.RetrySyncPeriod).Duration
	}

	var (
		syncPeriod                 = config.SyncPeriod
		respectSyncPeriodOverwrite = *config.RespectSyncPeriodOverwrite

		currentTimeNano  = time.Now().UnixNano()
		creationTimeNano = objectMeta.CreationTimestamp.UnixNano()
	)

	if syncPeriodOverwrite, ok := objectMeta.Annotations[common.ShootSyncPeriod]; ok && (respectSyncPeriodOverwrite || objectMeta.Namespace == common.GardenNamespace) {
		if syncPeriodDuration, err := time.ParseDuration(syncPeriodOverwrite); err == nil {
			if syncPeriodDuration.Nanoseconds() == 0 {
				return 0
			}
			if syncPeriodDuration >= time.Minute {
				syncPeriod = metav1.Duration{Duration: syncPeriodDuration}
			}
		}
	}

	var (
		syncPeriodNano = syncPeriod.Nanoseconds()
		nextSyncNano   = currentTimeNano - (currentTimeNano-creationTimeNano)%syncPeriodNano + syncPeriodNano
	)

	return time.Duration(nextSyncNano - currentTimeNano)
}

// ControlInterface implements the control logic for updating Shoots. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	// ReconcileShoot implements the control logic for Shoot creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	// The bool return value determines whether the Shoot should be automatically requeued for reconciliation.
	ReconcileShoot(shoot *gardenv1beta1.Shoot, key string) (bool, error)
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Shoots. updater is the UpdaterInterface used
// to update the status of Shoots. You should use an instance returned from NewDefaultControl() for any
// scenario other than testing.
func NewDefaultControl(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, identity *gardenv1beta1.Gardener, config *config.ControllerManagerConfiguration, gardenerNamespace string, recorder record.EventRecorder) ControlInterface {
	return &defaultControl{k8sGardenClient, k8sGardenInformers, secrets, imageVector, identity, config, gardenerNamespace, recorder}
}

type defaultControl struct {
	k8sGardenClient    kubernetes.Interface
	k8sGardenInformers gardeninformers.Interface
	secrets            map[string]*corev1.Secret
	imageVector        imagevector.ImageVector
	identity           *gardenv1beta1.Gardener
	config             *config.ControllerManagerConfiguration
	gardenerNamespace  string
	recorder           record.EventRecorder
}

func (c *defaultControl) ReconcileShoot(shootObj *gardenv1beta1.Shoot, key string) (bool, error) {
	key, err := cache.MetaNamespaceKeyFunc(shootObj)
	if err != nil {
		return true, err
	}
	operationID, err := utils.GenerateRandomString(8)
	if err != nil {
		return true, err
	}

	var (
		shoot         = shootObj.DeepCopy()
		shootLogger   = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace, operationID)
		operationType = gardencorev1alpha1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation)
	)

	logger.Logger.Infof("[SHOOT RECONCILE] %s", key)
	shootJSON, _ := json.Marshal(shoot)
	shootLogger.Debugf(string(shootJSON))

	operation, err := operation.New(shoot, shootLogger, c.k8sGardenClient, c.k8sGardenInformers, c.identity, c.secrets, c.imageVector, c.config.ShootBackup)
	if err != nil {
		shootLogger.Errorf("Could not initialize a new operation: %s", err.Error())
		return true, err
	}

	// We check whether the Shoot's last operation status field indicates that the last operation failed (i.e. the operation
	// will not be retried unless the shoot generation changes).
	if shootIsFailed(shoot) {
		if shoot.Status.Gardener.Version == version.Get().GitVersion {
			shootLogger.Infof("Will not reconcile as the last operation has been set to '%s' and the generation has not changed since then.", gardencorev1alpha1.LastOperationStateFailed)
			return false, nil
		}

		if updateErr := c.updateShootStatusResetRetry(operation, operationType); err != nil {
			shootLogger.Errorf("Could not reschedule failed shoot due to Gardener version update: %+v", updateErr)
			return true, updateErr
		}

		shootLogger.Infof("Successfully rescheduled failed shoot %q for reconciliation due to Gardener version update", shoot.Name)
	}

	// When a Shoot clusters deletion timestamp is set we need to delete the cluster and must not trigger a new reconciliation operation.
	if shoot.DeletionTimestamp != nil {
		c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardenv1beta1.EventDeleting, "[%s] Deleting Shoot cluster", operationID)
		if updateErr := c.updateShootStatusDeleteStart(operation); updateErr != nil {
			shootLogger.Errorf("Could not update the Shoot status after deletion start: %+v", updateErr)
			return true, updateErr
		}
		if deleteErr := c.deleteShoot(operation); deleteErr != nil {
			c.recorder.Eventf(shoot, corev1.EventTypeWarning, gardenv1beta1.EventDeleteError, "[%s] %s", operationID, deleteErr.Description)
			if state, updateErr := c.updateShootStatusDeleteError(operation, deleteErr); updateErr != nil {
				shootLogger.Errorf("Could not update the Shoot status after deletion error: %+v", updateErr)
				return state != gardencorev1alpha1.LastOperationStateFailed, updateErr
			}
			return true, errors.New(deleteErr.Description)
		}
		c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardenv1beta1.EventDeleted, "[%s] Deleted Shoot cluster", operationID)
		if updateErr := c.updateShootStatusDeleteSuccess(operation); updateErr != nil {
			shootLogger.Errorf("Could not update the Shoot status after deletion success: %+v", updateErr)
			return true, updateErr
		}
		return false, nil
	}

	// When a Shoot clusters deletion timestamp is not set we need to create/reconcile the cluster.
	c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardenv1beta1.EventReconciling, "[%s] Reconciling Shoot cluster state", operationID)
	if updateErr := c.updateShootStatusReconcileStart(operation, operationType); updateErr != nil {
		shootLogger.Errorf("Could not update the Shoot status after reconciliation start: %+v", updateErr)
		return true, updateErr
	}
	if reconcileErr := c.reconcileShoot(operation, operationType); reconcileErr != nil {
		c.recorder.Eventf(shoot, corev1.EventTypeWarning, gardenv1beta1.EventReconcileError, "[%s] %s", operationID, reconcileErr.Description)
		if state, updateErr := c.updateShootStatusReconcileError(operation, operationType, reconcileErr); updateErr != nil {
			shootLogger.Errorf("Could not update the Shoot status after reconciliation error: %+v", updateErr)
			return state != gardencorev1alpha1.LastOperationStateFailed, updateErr
		}
		return true, errors.New(reconcileErr.Description)
	}
	c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardenv1beta1.EventReconciled, "[%s] Reconciled Shoot cluster state", operationID)
	if updateErr := c.updateShootStatusReconcileSuccess(operation, operationType); updateErr != nil {
		shootLogger.Errorf("Could not update the Shoot status after reconciliation success: %+v", updateErr)
		return true, updateErr
	}
	return true, nil
}

// usesShootedSeed checks whether the seed used by given <shoot> is a shoot cluster itself. If yes it
// will return true and the shoot object of the shooted seed, otherwise false.
func (c *Controller) usesShootedSeed(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
	seed := shoot.Spec.Cloud.Seed
	if seed == nil {
		return nil, fmt.Errorf("shoot does not specify a seed (.spec.cloud.seed=nil)")
	}

	seedObj, err := c.seedLister.Get(*seed)
	if err != nil {
		return nil, err
	}

	if hasOwnerReferences, shootName := seedHasShootOwnerReference(seedObj.ObjectMeta); hasOwnerReferences {
		shootObj, err := c.shootLister.Shoots(common.GardenNamespace).Get(shootName)
		if err != nil {
			return nil, err
		}
		return shootObj, nil
	}

	return nil, nil
}

func seedHasShootOwnerReference(meta metav1.ObjectMeta) (bool, string) {
	gvk := gardenv1beta1.SchemeGroupVersion.WithKind("Shoot")

	for _, ownerReference := range meta.OwnerReferences {
		if ownerReference.APIVersion == gvk.GroupVersion().String() && ownerReference.Kind == gvk.Kind {
			return true, ownerReference.Name
		}
	}

	return false, ""
}
