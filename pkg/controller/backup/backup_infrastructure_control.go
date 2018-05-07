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

package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	cloudbotanistpkg "github.com/gardener/gardener/pkg/operation/cloudbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	utilerrors "github.com/gardener/gardener/pkg/operation/errors"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func (c *Controller) backupInfrastructureAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.backupInfrastructureQueue.Add(key)
}

func (c *Controller) backupInfrastructureUpdate(oldObj, newObj interface{}) {
	var (
		oldBackupInfrastructure = oldObj.(*gardenv1beta1.BackupInfrastructure)
		newBackupInfrastructure = newObj.(*gardenv1beta1.BackupInfrastructure)
		specChanged             = !apiequality.Semantic.DeepEqual(oldBackupInfrastructure.Spec, newBackupInfrastructure.Spec)
		statusChanged           = !apiequality.Semantic.DeepEqual(oldBackupInfrastructure.Status, newBackupInfrastructure.Status)
	)

	if !specChanged && statusChanged {
		return
	}
	c.backupInfrastructureAdd(newObj)
}

func (c *Controller) backupInfrastructureDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.backupInfrastructureQueue.Add(key)
}

func (c *Controller) reconcileBackupInfrastructureKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	backupInfrastructure, err := c.backupInfrastructureLister.BackupInfrastructures(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[BACKUPINFRASTRUCTURE RECONCILE] %s - skipping because BackupInfrastructure has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[BACKUPINFRASTRUCTURE RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	backupInfrastructureLogger := logger.NewBackupInfrastructureLogger(logger.Logger, backupInfrastructure.ObjectMeta.Name, backupInfrastructure.ObjectMeta.Namespace, "")
	if mustIgnoreBackupInfrastructure(backupInfrastructure.Annotations, c.config.Controllers.BackupInfrastructure.RespectSyncPeriodOverwrite) {
		backupInfrastructureLogger.Info("Skipping reconciliation because BackupInfrastructure is marked as 'to-be-ignored'.")
		return nil
	}

	if backupInfrastructure.DeletionTimestamp != nil && !sets.NewString(backupInfrastructure.Finalizers...).Has(gardenv1beta1.GardenerName) {
		backupInfrastructureLogger.Debug("Do not need to do anything as the BackupInfrastructure does not have my finalizer")
		c.backupInfrastructureQueue.Forget(key)
		return nil
	}

	needsRequeue, reconcileErr := c.control.ReconcileBackupInfrastructure(backupInfrastructure, key)
	if wantsResync, durationToNextSync := scheduleNextSync(backupInfrastructure.ObjectMeta, reconcileErr != nil, c.config.Controllers.BackupInfrastructure); wantsResync && needsRequeue {
		c.backupInfrastructureQueue.AddAfter(key, durationToNextSync)
		backupInfrastructureLogger.Infof("Scheduled next reconciliation for BackupInfrastructure '%s' in %s", key, durationToNextSync)
	}
	return nil
}

func mustIgnoreBackupInfrastructure(annotations map[string]string, respectSyncPeriodOverwrite *bool) bool {
	_, ignore := annotations[common.ShootIgnore]
	return respectSyncPeriodOverwrite != nil && ignore && *respectSyncPeriodOverwrite
}

func scheduleNextSync(objectMeta metav1.ObjectMeta, errorOccured bool, config componentconfig.BackupInfrastructureControllerConfiguration) (bool, time.Duration) {
	if errorOccured {
		return true, (*config.RetrySyncPeriod).Duration
	}

	var (
		syncPeriod                 = config.SyncPeriod
		respectSyncPeriodOverwrite = *config.RespectSyncPeriodOverwrite

		currentTimeNano  = time.Now().UnixNano()
		creationTimeNano = objectMeta.CreationTimestamp.UnixNano()
	)

	if syncPeriodOverwrite, ok := objectMeta.Annotations[common.ShootSyncPeriod]; ok && respectSyncPeriodOverwrite {
		if syncPeriodDuration, err := time.ParseDuration(syncPeriodOverwrite); err == nil {
			if syncPeriodDuration.Nanoseconds() == 0 {
				return false, 0
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

	return true, time.Duration(nextSyncNano - currentTimeNano)
}

// ControlInterface implements the control logic for updating BackupInfrastructures. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	// ReconcileBackupInfrastructure implements the control logic for BackupInfrastructure creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	// The bool return value determines whether the BackupInfrastructure should be automatically requeued for reconciliation.
	ReconcileBackupInfrastructure(backupInfrastructure *gardenv1beta1.BackupInfrastructure, key string) (bool, error)
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for BackupInfrastructures. You should use an instance returned from NewDefaultControl() for any
// scenario other than testing.
func NewDefaultControl(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, identity *gardenv1beta1.Gardener, config *componentconfig.ControllerManagerConfiguration, recorder record.EventRecorder, updater UpdaterInterface) ControlInterface {
	return &defaultControl{k8sGardenClient, k8sGardenInformers, secrets, imageVector, identity, config, recorder, updater}
}

type defaultControl struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.Interface
	secrets            map[string]*corev1.Secret
	imageVector        imagevector.ImageVector
	identity           *gardenv1beta1.Gardener
	config             *componentconfig.ControllerManagerConfiguration
	recorder           record.EventRecorder
	updater            UpdaterInterface
}

func (c *defaultControl) ReconcileBackupInfrastructure(obj *gardenv1beta1.BackupInfrastructure, key string) (bool, error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return true, err
	}

	var (
		backupInfrastructure       = obj.DeepCopy()
		backupInfrastructureLogger = logger.NewFieldLogger(logger.Logger, "backupinfrastructure", fmt.Sprintf("%s/%s", backupInfrastructure.Namespace, backupInfrastructure.Name))
		phase                      = backupInfrastructure.Status.Phase
		operationID                = utils.GenerateRandomString(8)
	)

	logger.Logger.Infof("[BACKUPINFRASTRUCTURE RECONCILE] %s", key)
	backupInfrastructureJSON, _ := json.Marshal(backupInfrastructure)
	backupInfrastructureLogger.Debugf(string(backupInfrastructureJSON))

	op, err := operation.NewWithBackupInfrastructure(backupInfrastructure, backupInfrastructureLogger, c.k8sGardenClient, c.k8sGardenInformers, c.identity, c.secrets, c.imageVector)
	if err != nil {
		backupInfrastructureLogger.Errorf("Could not initialize a new operation: %s", err.Error())
		return true, err
	}

	// We check whether the BackupInfrastructure's last operation status field indicates that the last operation failed (i.e. the operation
	// will not be retried unless the backupInfrastructure generation changes).
	if phase != nil && *phase == gardenv1beta1.PhaseFailed && backupInfrastructure.Generation == backupInfrastructure.Status.ObservedGeneration {
		backupInfrastructureLogger.Infof("Will not reconcile as the phase has been set to '%s' and the generation has not changed since then.", gardenv1beta1.PhaseFailed)
		return false, nil
	}
	// The deletionTimestamp labels a BackupInfrastructure as intended to get deleted. Before deletion,
	// it has to be ensured that no Infrastructure resources are depending on the BackupInfrastructure anymore.
	// When this happens the controller will remove the finalizer from the BackupInfrastructure so that it can be garbage collected.
	if backupInfrastructure.DeletionTimestamp != nil {
		if !sets.NewString(backupInfrastructure.Finalizers...).Has(gardenv1beta1.GardenerName) {
			return false, nil
		}
		// interpret gracePeriod in spec as number of days
		gracePeriod := time.Minute * time.Duration(*backupInfrastructure.Spec.GracePeriod) //TODO update it for per day
		if time.Now().Sub(backupInfrastructure.DeletionTimestamp.Time) > gracePeriod {

			c.recorder.Eventf(backupInfrastructure, corev1.EventTypeNormal, gardenv1beta1.BackupInfrastructureEventDeleting, "[%s] Deleting Backup Infrastructure", operationID)
			if _, updateErr := c.updateBackupInfrastructureStatus(op, gardenv1beta1.PhaseDeleting, nil); updateErr != nil {
				backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after deletion start: %+v", updateErr)
				return true, updateErr
			}

			if deleteErr := c.deleteBackupInfrastructure(op); deleteErr != nil {
				c.recorder.Eventf(backupInfrastructure, corev1.EventTypeWarning, gardenv1beta1.BackupInfrastructureEventDeleteError, "[%s] %s", operationID, deleteErr.Description)
				if state, updateErr := c.updateBackupInfrastructureStatus(op, gardenv1beta1.PhaseError, deleteErr); updateErr != nil {
					backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after deletion error: %+v", updateErr)
					return state != gardenv1beta1.PhaseFailed, updateErr
				}
				return true, errors.New(deleteErr.Description)
			}
			c.recorder.Eventf(backupInfrastructure, corev1.EventTypeNormal, gardenv1beta1.BackupInfrastructureEventDeleted, "[%s] Deleted Backup Infrastructure", operationID)
			if _, updateErr := c.updateBackupInfrastructureStatus(op, gardenv1beta1.PhaseDeleted, nil); updateErr != nil {
				backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after deletion successful: %+v", updateErr)
				return true, updateErr
			}
			// Remove finalizer from BackupInfrastructure
			backupInfrastructureFinalizers := sets.NewString(op.BackupInfrastructure.Finalizers...)
			backupInfrastructureFinalizers.Delete(gardenv1beta1.GardenerName)
			op.BackupInfrastructure.Finalizers = backupInfrastructureFinalizers.UnsortedList()

			newBackupInfrastructure, err := c.k8sGardenClient.GardenClientset().GardenV1beta1().BackupInfrastructures(op.BackupInfrastructure.Namespace).Update(op.BackupInfrastructure)
			if err != nil {
				backupInfrastructureLogger.Errorf("Could not remove finalizer of the BackupInfrastructure: %+v", err.Error())
				return true, err
			}
			op.BackupInfrastructure = newBackupInfrastructure
			// Wait until the above modifications are reflected in the cache to prevent unwanted reconcile
			// operations (sometimes the cache is not synced fast enough).
			err = wait.PollImmediate(time.Second, 30*time.Second, func() (bool, error) {
				backupInfrastructure, err := c.k8sGardenInformers.BackupInfrastructures().Lister().BackupInfrastructures(op.BackupInfrastructure.Namespace).Get(op.BackupInfrastructure.Name)
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				if err != nil {
					return false, err
				}
				if !sets.NewString(backupInfrastructure.Finalizers...).Has(gardenv1beta1.GardenerName) {
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				return true, err
			}
			return false, nil
		}
	}

	// When a BackupInfrastructure deletion timestamp is not set we need to create/reconcile the backup infrastructure.
	c.recorder.Eventf(backupInfrastructure, corev1.EventTypeNormal, gardenv1beta1.BackupInfrastructureEventReconciling, "[%s] Reconciling Backup Infrastructure state", operationID)
	if _, updateErr := c.updateBackupInfrastructureStatus(op, gardenv1beta1.PhaseReconciling, nil); updateErr != nil {
		backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after reconciliation start: %+v", updateErr)
		return true, updateErr
	}
	if reconcileErr := c.reconcileBackupInfrastructure(op); reconcileErr != nil {
		c.recorder.Eventf(backupInfrastructure, corev1.EventTypeWarning, gardenv1beta1.BackupInfrastructureEventReconcileError, "[%s] %s", operationID, reconcileErr.Description)
		if state, updateErr := c.updateBackupInfrastructureStatus(op, gardenv1beta1.PhaseError, reconcileErr); updateErr != nil {
			backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after reconciliation error: %+v", updateErr)
			return state != gardenv1beta1.PhaseFailed, updateErr
		}
		return true, errors.New(reconcileErr.Description)
	}
	c.recorder.Eventf(backupInfrastructure, corev1.EventTypeNormal, gardenv1beta1.BackupInfrastructureEventReconciled, "[%s] Reconciled Backup Infrastructure state", operationID)
	if _, updateErr := c.updateBackupInfrastructureStatus(op, gardenv1beta1.PhaseReconciled, nil); updateErr != nil {
		backupInfrastructureLogger.Errorf("Could not update the Shoot status after reconciliation success: %+v", updateErr)
		return true, updateErr
	}

	return true, nil
}

// reconcileBackupInfrastructure reconciles a BackupInfrastructure state.
func (c *defaultControl) reconcileBackupInfrastructure(o *operation.Operation) *gardenv1beta1.LastError {

	// We create botanists (which will do the actual work).
	botanist, err := botanistpkg.New(o)
	if err != nil {
		return formatError("Failed to create a Botanist", err)
	}
	seedCloudBotanist, err := cloudbotanistpkg.New(o, common.CloudPurposeSeed)
	if err != nil {
		return formatError("Failed to create a Seed CloudBotanist", err)
	}

	var (
		backupInfrastructure = o.BackupInfrastructure
		defaultRetry         = 30 * time.Second
	)

	// Deploy namespace object
	if err := utils.Retry(o.Logger, defaultRetry, func() (bool, error) {
		if err := DeployBackupNamespace(botanist, backupInfrastructure); err != nil {
			return false, err
		}
		return true, nil
	}); err != nil {
		e := utilerrors.New(err)
		lastError := &gardenv1beta1.LastError{
			Codes:       []gardenv1beta1.ErrorCode{*e.Code},
			Description: fmt.Sprintf("Failed to create Backup Namespace: %s", e.Description),
		}
		return lastError
	}

	// Deploy cloud resources
	if err := seedCloudBotanist.DeployBackupInfrastructure(); err != nil {
		e := utilerrors.New(err)
		lastError := &gardenv1beta1.LastError{
			Codes:       []gardenv1beta1.ErrorCode{*e.Code},
			Description: fmt.Sprintf("Failed to deploy Backup Infrastructure: %s", e.Description),
		}
		return lastError
	}

	o.Logger.Infof("Successfully reconciled Backup Infrastructure state '%s'", backupInfrastructure.Name)
	return nil
}

// DeployBackupNamespace creates a namespace in the Seed cluster which is used to deploy all the backup infrastructure
// realted resources for shoot cluster. Moreover, the cloud provider configuration and all the secrets will be
// stored as ConfigMaps/Secrets.
func DeployBackupNamespace(b *botanistpkg.Botanist, backupInfrastructure *gardenv1beta1.BackupInfrastructure) error {
	namespace, err := b.K8sSeedClient.CreateNamespace(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.GenerateBackupNamespaceName(backupInfrastructure.Name),
			Labels: map[string]string{
				common.GardenRole: common.GardenRoleBackup,
			},
		},
	}, true)
	if err != nil {
		return err
	}
	b.SeedNamespaceObject = namespace
	return nil
}

// deleteBackupInfrastructure deletes a BackupInfrastructure entirely.
func (c *defaultControl) deleteBackupInfrastructure(o *operation.Operation) *gardenv1beta1.LastError {

	// We create botanists (which will do the actual work).
	botanist, err := botanistpkg.New(o)
	if err != nil {
		return formatError("Failed to create a Botanist", err)
	}

	// We first check whether the namespace in the Seed cluster does exist - if it does not, then we assume that
	// all resources have already been deleted. We can delete the BackupInfrastructure resource as a consequence.
	namespace, err := botanist.K8sSeedClient.GetNamespace(common.GenerateBackupNamespaceName(o.BackupInfrastructure.Name))
	if apierrors.IsNotFound(err) {
		o.Logger.Infof("Did not find '%s' namespace in the Seed cluster - nothing to be done", common.GenerateBackupNamespaceName(o.BackupInfrastructure.Name))
		return nil
	}
	if err != nil {
		return formatError("Failed to retrieve the Shoot backup namespace in the Seed cluster", err)
	}

	seedCloudBotanist, err := cloudbotanistpkg.New(o, common.CloudPurposeSeed)
	if err != nil {
		return formatError("Failed to create a Seed CloudBotanist", err)
	}

	// We check whether the Backup namespace in the Seed cluster is already in a terminating state, i.e. whether
	// we have tried to delete it in a previous run. In that case, we do not need to cleanup backup infrastructure resource because
	// that would have already been done.
	var (
		cleanupBackupInfrastructureResources = namespace.Status.Phase != corev1.NamespaceTerminating
		defaultRetry                         = 30 * time.Second
	)

	// Destroy cloud resources
	if cleanupBackupInfrastructureResources {
		if err := seedCloudBotanist.DestroyBackupInfrastructure(); err != nil {
			e := utilerrors.New(err)
			lastError := &gardenv1beta1.LastError{
				Codes:       []gardenv1beta1.ErrorCode{*e.Code},
				Description: fmt.Sprintf("Failed to delete Backup Infrastructure: %s", e.Description),
			}
			return lastError
		}

		// Delete namespace object
		if err := utils.Retry(o.Logger, defaultRetry, func() (bool, error) {
			if err := DeleteBackupNamespace(botanist, o.BackupInfrastructure); err != nil {
				return false, err
			}
			return true, nil
		}); err != nil {
			e := utilerrors.New(err)
			lastError := &gardenv1beta1.LastError{
				Codes:       []gardenv1beta1.ErrorCode{*e.Code},
				Description: fmt.Sprintf("Failed to delete Backup Namespace: %s", e.Description),
			}
			return lastError
		}
	}
	// Wait until namespace deletion completes
	if err := WaitUntilBackupNamespaceDeleted(botanist, o.BackupInfrastructure); err != nil {
		e := utilerrors.New(err)
		lastError := &gardenv1beta1.LastError{
			Codes:       []gardenv1beta1.ErrorCode{*e.Code},
			Description: fmt.Sprintf("Failed to delete Backup Namespace: %s", e.Description),
		}
		return lastError
	}

	o.Logger.Infof("Successfully deleted Backup Infrastructure '%s'", o.BackupInfrastructure.Name)
	return nil
}

/*
// ReportBackupInfrastructureProgress will update the phase and error in the BackupInfrastructure manifest `status` section
// by the current progress of the Flow execution.
func ReportBackupInfrastructureProgress(backupInfrastructure *gardenv1beta1.BackupInfrastructure, progress int, currentFunctions string) {
	backupInfrastructure.Status.LastOperation.Description = "Currently executing " + currentFunctions
	backupInfrastructure.Status.LastOperation.Progress = progress
	backupInfrastructure.Status.LastOperation.LastUpdateTime = metav1.Now()

	if newBackupInfrastructure, err := o.K8sGardenClient.GardenClientset().GardenV1beta1().BackupInfrastructures(backupInfrastructure.Namespace).UpdateStatus(backupInfrastructure); err == nil {
		backupInfrastructure = newBackupInfrastructure
	}
}
*/

// DeleteBackupNamespace deletes the namespace in the Seed cluster which holds the backup infrastructure state. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace.
func DeleteBackupNamespace(b *botanistpkg.Botanist, backupInfrastructure *gardenv1beta1.BackupInfrastructure) error {
	err := b.K8sSeedClient.DeleteNamespace(common.GenerateBackupNamespaceName(backupInfrastructure.Name))
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}
	return err
}

// WaitUntilBackupNamespaceDeleted waits until the namespace for the backup of Shoot cluster within the Seed cluster is deleted.
func WaitUntilBackupNamespaceDeleted(b *botanistpkg.Botanist, backupInfrastructure *gardenv1beta1.BackupInfrastructure) error {
	return wait.PollImmediate(5*time.Second, 900*time.Second, func() (bool, error) {
		_, err := b.K8sSeedClient.GetNamespace(common.GenerateBackupNamespaceName(backupInfrastructure.Name))
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		b.Logger.Info("Waiting until the Shoot backup namespace has been cleaned up and deleted in the Seed cluster...")
		return false, nil
	})
}

func (c *defaultControl) updateBackupInfrastructureStatus(o *operation.Operation, phase gardenv1beta1.BackupInfrastructurePhase, lastError *gardenv1beta1.LastError) (gardenv1beta1.BackupInfrastructurePhase, error) {

	var (
		now = metav1.Now()
	)

	if phase == gardenv1beta1.PhaseError || phase == gardenv1beta1.PhaseFailed {
		if !utils.TimeElapsed(o.BackupInfrastructure.Status.RetryCycleStartTime, c.config.Controllers.BackupInfrastructure.RetryDuration.Duration) {
			//description := " Operation will be retried."
			phase = gardenv1beta1.PhaseError
		} else {
			phase = gardenv1beta1.PhaseFailed
			o.BackupInfrastructure.Status.RetryCycleStartTime = nil
		}
	}
	if o.BackupInfrastructure.Status.RetryCycleStartTime == nil {
		o.BackupInfrastructure.Status.RetryCycleStartTime = &now
	}
	o.BackupInfrastructure.Status.Gardener = *o.GardenerInfo
	o.BackupInfrastructure.Status.Phase = &phase
	o.BackupInfrastructure.Status.LastError = lastError
	o.BackupInfrastructure.Status.ObservedGeneration = o.BackupInfrastructure.Generation

	newBackupInfrastructure, err := c.updater.UpdateBackupInfrastructureStatus(o.BackupInfrastructure)
	if err == nil {
		o.BackupInfrastructure = newBackupInfrastructure
	}

	return phase, err
}

func formatError(message string, err error) *gardenv1beta1.LastError {
	return &gardenv1beta1.LastError{
		Description: fmt.Sprintf("%s (%s)", message, err.Error()),
	}
}
