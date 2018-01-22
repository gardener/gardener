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

package shoot

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func (c *Controller) shootAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.shootQueue.Add(key)
}

func (c *Controller) shootUpdate(oldObj, newObj interface{}) {
	var (
		oldShoot        = oldObj.(*gardenv1beta1.Shoot)
		newShoot        = newObj.(*gardenv1beta1.Shoot)
		oldShootJSON, _ = json.Marshal(oldShoot)
		newShootJSON, _ = json.Marshal(newShoot)
		shoot           = newShoot.DeepCopy()
		shootLogger     = logger.NewShootLogger(logger.Logger, newShoot.ObjectMeta.Name, newShoot.ObjectMeta.Namespace, "")
		specChanged     = !apiequality.Semantic.DeepEqual(oldShoot.Spec, newShoot.Spec)
		statusChanged   = !apiequality.Semantic.DeepEqual(oldShoot.Status, newShoot.Status)
	)
	shootLogger.Debugf(string(oldShootJSON))
	shootLogger.Debugf(string(newShootJSON))

	// If the .spec field has not changed, but the .status field has, then the Update event was triggerd by the
	// Gardener itself as it has updated the .status field. In this case, we do not need to do anything.
	if !specChanged && statusChanged {
		shootLogger.Debug("Do not need to do anything as the Update event occurred due to .status field changes")
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", newObj, err)
		return
	}

	if !sets.NewString(shoot.Finalizers...).Has(gardenv1beta1.GardenerName) {
		shootLogger.Debug("Do not need to do anything as the Shoot does not have my finalizer")
		c.shootQueue.Forget(key)
		return
	}

	c.shootQueue.Add(key)
}

func (c *Controller) shootDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	c.shootQueue.Add(key)
}

func (c *Controller) reconcileShootKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	shoot, err := c.shootLister.Shoots(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SHOOT RECONCILE] %s - skipping because Shoot has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SHOOT RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	// We check whether the Shoot's last operation status field indicates that the cluster has been successfully deleted.
	lastOperation := shoot.Status.LastOperation
	if lastOperation != nil && lastOperation.Type == gardenv1beta1.ShootLastOperationTypeDelete && lastOperation.State == gardenv1beta1.ShootLastOperationStateSucceeded {
		return nil
	}

	err = c.control.ReconcileShoot(shoot, key)
	if err != nil {
		c.shootQueue.AddAfter(key, 15*time.Second)
	}
	return nil
}

// ControlInterface implements the control logic for updating Shoots. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	// ReconcileShoot implements the control logic for Shoot creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	ReconcileShoot(shoot *gardenv1beta1.Shoot, key string) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Shoots. updater is the UpdaterInterface used
// to update the status of Shoots. You should use an instance returned from NewDefaultControl() for any
// scenario other than testing.
func NewDefaultControl(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, identity *gardenv1beta1.Gardener, config *componentconfig.ControllerManagerConfiguration, gardenerNamespace string, recorder record.EventRecorder, updater UpdaterInterface) ControlInterface {
	return &defaultControl{k8sGardenClient, k8sGardenInformers, secrets, imageVector, identity, config, gardenerNamespace, recorder, updater}
}

type defaultControl struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.Interface
	secrets            map[string]*corev1.Secret
	imageVector        imagevector.ImageVector
	identity           *gardenv1beta1.Gardener
	config             *componentconfig.ControllerManagerConfiguration
	gardenerNamespace  string
	recorder           record.EventRecorder
	updater            UpdaterInterface
}

func (c *defaultControl) ReconcileShoot(shootObj *gardenv1beta1.Shoot, key string) error {
	key, err := cache.MetaNamespaceKeyFunc(shootObj)
	if err != nil {
		return err
	}

	var (
		shoot                              = shootObj.DeepCopy()
		operationID                        = utils.GenerateRandomString(8)
		shootLogger                        = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace, operationID)
		confirmationDeletionTimestampFound = checkConfirmationDeletionTimestamp(shoot)
		confirmationDeletionTimestampValid = checkConfirmationDeletionTimestampValid(shoot)
		triggerDeletion                    = sets.NewString(shoot.Finalizers...).Has(gardenv1beta1.GardenerName) && confirmationDeletionTimestampValid
		lastOperation                      = shoot.Status.LastOperation
	)

	logger.Logger.Infof("[SHOOT RECONCILE] %s", key)
	shootJSON, _ := json.Marshal(shoot)
	shootLogger.Debugf(string(shootJSON))

	operation, err := operation.New(shoot, shootLogger, c.k8sGardenClient, c.k8sGardenInformers, c.identity, c.secrets, c.imageVector)
	if err != nil {
		shootLogger.Errorf("could not initialize a new operation: %s", err.Error())
		return err
	}

	// We check whether the Shoot's annotations contains the a key stating the Garden action equals to 'delete' which means that the
	// user has confirmed the deletion via the Gardener UI. In this case we have to trigger the deletion of the Shoot cluster.
	if shoot.DeletionTimestamp != nil && !confirmationDeletionTimestampFound {
		shootLogger.Infof("Shoot cluster's deletionTimestamp is set but the confirmation annotation '%s' is missing. Skipping.", common.ConfirmationDeletionTimestamp)
		return nil
	}
	if triggerDeletion {
		c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardenv1beta1.ShootEventDeleting, "[%s] Deleting Shoot cluster", operationID)
		shootLogger.Infof("Detected valid annotation '%s' confirming the Shoot deletion.", common.ConfirmationDeletionTimestamp)
		if updateErr := c.updateShootStatusDeleteStart(operation); updateErr != nil {
			shootLogger.Errorf("Could not update the Shoot status after deletion start: %+v", updateErr)
			return updateErr
		}
		deleteErr := c.deleteShoot(operation)
		if deleteErr != nil {
			c.recorder.Eventf(shoot, corev1.EventTypeWarning, gardenv1beta1.ShootEventDeleteError, "[%s] %s", operationID, deleteErr.Description)
			if updateErr := c.updateShootStatusDeleteError(operation, deleteErr); updateErr != nil {
				shootLogger.Errorf("Could not update the Shoot status after deletion error: %+v", updateErr)
				return updateErr
			}
			return errors.New(deleteErr.Description)
		}
		c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardenv1beta1.ShootEventDeleted, "[%s] Deleted Shoot cluster", operationID)
		if updateErr := c.updateShootStatusDeleteSuccess(operation); updateErr != nil {
			shootLogger.Errorf("Could not update the Shoot status after deletion success: %+v", updateErr)
			return updateErr
		}
		return nil
	}

	// Re-enable reconciliation on a failed Shoot cluster resource because the specifcation has been changed.
	if shoot.Generation != shoot.Status.ObservedGeneration {
		if lastOperation != nil && lastOperation.State == gardenv1beta1.ShootLastOperationStateFailed {
			shoot.Status.LastOperation.State = gardenv1beta1.ShootLastOperationStateError
		}
		// Set `.status.operationStartTime to nil because a new specification has been applied.
		operation.Shoot.Info.Status.OperationStartTime = nil
	}

	// We check whether the Shoot's last operation status field indicates that the last operation failed (i.e. will not be retried).
	if lastOperation != nil && lastOperation.State == gardenv1beta1.ShootLastOperationStateFailed {
		shootLogger.Infof("Will not reconcile as the last operation has been set to '%s'.", gardenv1beta1.ShootLastOperationStateFailed)
		return nil
	}

	operationType := gardenv1beta1.ShootLastOperationTypeReconcile
	if lastOperation == nil || (lastOperation.Type == gardenv1beta1.ShootLastOperationTypeCreate && lastOperation.State != gardenv1beta1.ShootLastOperationStateSucceeded) {
		operationType = gardenv1beta1.ShootLastOperationTypeCreate
	}

	c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardenv1beta1.ShootEventReconciling, "[%s] Reconciling Shoot cluster state", operationID)
	if updateErr := c.updateShootStatusReconcileStart(operation, operationType); updateErr != nil {
		shootLogger.Errorf("Could not update the Shoot status after reconciliation start: %+v", updateErr)
		return updateErr
	}
	reconcileErr := c.reconcileShoot(operation, operationType, c.updater)
	if reconcileErr != nil {
		c.recorder.Eventf(shoot, corev1.EventTypeWarning, gardenv1beta1.ShootEventReconcileError, "[%s] %s", operationID, reconcileErr.Description)
		if updateErr := c.updateShootStatusReconcileError(operation, operationType, reconcileErr); updateErr != nil {
			shootLogger.Errorf("Could not update the Shoot status after reconciliation error: %+v", updateErr)
			return updateErr
		}
		return errors.New(reconcileErr.Description)
	}
	c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardenv1beta1.ShootEventReconciled, "[%s] Reconciled Shoot cluster state", operationID)
	if updateErr := c.updateShootStatusReconcileSuccess(operation, operationType); updateErr != nil {
		shootLogger.Errorf("Could not update the Shoot status after reconciliation success: %+v", updateErr)
		return updateErr
	}
	return nil
}
