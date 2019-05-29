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

package backupinfrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	utilretry "github.com/gardener/gardener/pkg/utils/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	cloudbotanistpkg "github.com/gardener/gardener/pkg/operation/cloudbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
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
		newBackupInfrastructure    = newObj.(*gardenv1beta1.BackupInfrastructure)
		backupInfrastructureLogger = logger.NewFieldLogger(logger.Logger, "backupinfrastructure", fmt.Sprintf("%s/%s", newBackupInfrastructure.Namespace, newBackupInfrastructure.Name))
	)

	// If the generation did not change for an update event (i.e., no changes to the .spec section have
	// been made), we do not want to add the BackupInfrastructure to the queue. The periodic reconciliation is handled
	// elsewhere by adding the BackupInfrastructure to the queue to dedicated times.
	if newBackupInfrastructure.Generation == newBackupInfrastructure.Status.ObservedGeneration {
		backupInfrastructureLogger.Debug("Do not need to do anything as the Update event occurred due to .status field changes")
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

	backupInfrastructureLogger := logger.NewFieldLogger(logger.Logger, "backupinfrastructure", fmt.Sprintf("%s/%s", backupInfrastructure.Namespace, backupInfrastructure.Name))

	if backupInfrastructure.DeletionTimestamp != nil && !sets.NewString(backupInfrastructure.Finalizers...).Has(gardenv1beta1.GardenerName) {
		backupInfrastructureLogger.Debug("Do not need to do anything as the BackupInfrastructure does not have my finalizer")
		c.backupInfrastructureQueue.Forget(key)
		return nil
	}

	durationToNextSync := c.config.Controllers.BackupInfrastructure.SyncPeriod.Duration
	deleted, reconcileErr := c.control.ReconcileBackupInfrastructure(backupInfrastructure, key)
	if reconcileErr != nil {
		durationToNextSync = 15 * time.Second
	} else if deleted {
		gracePeriod := computeGracePeriod(c.config, backupInfrastructure)
		durationToActualDeletion := gracePeriod - time.Since(backupInfrastructure.DeletionTimestamp.Local())
		// We don't set durationToNextSync directly to durationToActualDeletion since,
		// we want reconciliation to update status as per sync period. This will help in adjusting
		// the next sync time in case deletionGracePeriod is reduced from GCM config.
		if durationToNextSync > durationToActualDeletion {
			durationToNextSync = durationToActualDeletion
		}
	}
	c.backupInfrastructureQueue.AddAfter(key, durationToNextSync)
	backupInfrastructureLogger.Infof("Scheduled next reconciliation for BackupInfrastructure '%s' in %s", key, durationToNextSync)
	return nil
}

// ControlInterface implements the control logic for updating BackupInfrastructures. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	// ReconcileBackupInfrastructure implements the control logic for BackupInfrastructure creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	// It returns boolean indicating whether the next sync period should be adjusted as per deletionTimestamp.
	ReconcileBackupInfrastructure(backupInfrastructure *gardenv1beta1.BackupInfrastructure, key string) (bool, error)
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for BackupInfrastructures. You should use an instance returned from NewDefaultControl() for any
// scenario other than testing.
func NewDefaultControl(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, identity *gardenv1beta1.Gardener, config *config.ControllerManagerConfiguration, recorder record.EventRecorder) ControlInterface {
	return &defaultControl{k8sGardenClient, k8sGardenInformers, secrets, imageVector, identity, config, recorder}
}

type defaultControl struct {
	k8sGardenClient    kubernetes.Interface
	k8sGardenInformers gardeninformers.Interface
	secrets            map[string]*corev1.Secret
	imageVector        imagevector.ImageVector
	identity           *gardenv1beta1.Gardener
	config             *config.ControllerManagerConfiguration
	recorder           record.EventRecorder
}

func (c *defaultControl) ReconcileBackupInfrastructure(obj *gardenv1beta1.BackupInfrastructure, key string) (bool, error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return false, err
	}

	var (
		backupInfrastructure       = obj.DeepCopy()
		backupInfrastructureLogger = logger.NewFieldLogger(logger.Logger, "backupinfrastructure", fmt.Sprintf("%s/%s", backupInfrastructure.Namespace, backupInfrastructure.Name))
		lastOperation              = backupInfrastructure.Status.LastOperation
		operationType              = gardencorev1alpha1helper.ComputeOperationType(obj.ObjectMeta, lastOperation)
	)

	logger.Logger.Infof("[BACKUPINFRASTRUCTURE RECONCILE] %s", key)

	// Skip further logic if the last successful reconciliation happened less than the specified syncPeriod ago
	// and the object does not have an explicit reconcile instruction in its annotations.
	syncPeriod := c.config.Controllers.BackupInfrastructure.SyncPeriod.Duration
	if backupInfrastructure.DeletionTimestamp == nil &&
		!nextReconcileScheduleReached(obj, syncPeriod) &&
		!kutil.HasMetaDataAnnotation(&obj.ObjectMeta, common.BackupInfrastructureOperation, common.BackupInfrastructureReconcile) {
		logger.Logger.Infof("Skip reconciliation for BackupInfrastructure %s. Last successful operation happened less than %q ago and reconcile annotation is not set.", key, syncPeriod)
		return false, nil
	}

	backupInfrastructureJSON, _ := json.Marshal(backupInfrastructure)
	backupInfrastructureLogger.Debugf(string(backupInfrastructureJSON))

	op, err := operation.NewWithBackupInfrastructure(backupInfrastructure, c.config, backupInfrastructureLogger, c.k8sGardenClient, c.k8sGardenInformers, c.identity, c.secrets, c.imageVector)
	if err != nil {
		backupInfrastructureLogger.Errorf("Could not initialize a new operation: %s", err.Error())
		return false, err
	}

	// The deletionTimestamp labels a BackupInfrastructure as intended to get deleted. Before deletion,
	// it has to be ensured that no infrastructure resources are depending on the BackupInfrastructure anymore.
	// When this happens the controller will remove the finalizer from the BackupInfrastructure so that it can be garbage collected.
	if backupInfrastructure.DeletionTimestamp != nil {
		gracePeriod := computeGracePeriod(c.config, backupInfrastructure)
		present, _ := strconv.ParseBool(backupInfrastructure.ObjectMeta.Annotations[common.BackupInfrastructureForceDeletion])
		if present || time.Since(backupInfrastructure.DeletionTimestamp.Time) > gracePeriod {
			if updateErr := c.updateBackupInfrastructureStatus(op, gardencorev1alpha1.LastOperationStateProcessing, operationType, "Deletion of Backup Infrastructure in progress.", 1, nil); updateErr != nil {
				backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after deletion start: %+v", updateErr)
				return false, updateErr
			}

			if deleteErr := c.deleteBackupInfrastructure(op); deleteErr != nil {
				c.recorder.Eventf(backupInfrastructure, corev1.EventTypeWarning, gardenv1beta1.EventDeleteError, "%s", deleteErr.Description)
				if updateErr := c.updateBackupInfrastructureStatus(op, gardencorev1alpha1.LastOperationStateError, operationType, deleteErr.Description+" Operation will be retried.", 1, deleteErr); updateErr != nil {
					backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after deletion error: %+v", updateErr)
					return false, updateErr
				}
				return false, errors.New(deleteErr.Description)
			}
			if updateErr := c.updateBackupInfrastructureStatus(op, gardencorev1alpha1.LastOperationStateSucceeded, operationType, "Backup Infrastructure has been successfully deleted.", 100, nil); updateErr != nil {
				backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after deletion successful: %+v", updateErr)
				return false, updateErr
			}
			return false, c.removeFinalizer(op)
		}

		if updateErr := c.updateBackupInfrastructureStatus(op, gardencorev1alpha1.LastOperationStatePending, operationType, fmt.Sprintf("Deletion of backup infrastructure is scheduled for %s", backupInfrastructure.DeletionTimestamp.Time.Add(gracePeriod)), 1, nil); updateErr != nil {
			backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after suspending deletion: %+v", updateErr)
			return true, updateErr
		}
		return true, nil
	}

	// When a BackupInfrastructure deletion timestamp is not set we need to create/reconcile the backup infrastructure.
	if updateErr := c.updateBackupInfrastructureStatus(op, gardencorev1alpha1.LastOperationStateProcessing, operationType, "Reconciliation of Backup Infrastructure state in progress.", 1, nil); updateErr != nil {
		backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after reconciliation start: %+v", updateErr)
		return false, updateErr
	}
	if reconcileErr := c.reconcileBackupInfrastructure(op); reconcileErr != nil {
		c.recorder.Eventf(backupInfrastructure, corev1.EventTypeWarning, gardenv1beta1.EventReconcileError, "%s", reconcileErr.Description)
		if updateErr := c.updateBackupInfrastructureStatus(op, gardencorev1alpha1.LastOperationStateError, operationType, reconcileErr.Description+" Operation will be retried.", 1, reconcileErr); updateErr != nil {
			backupInfrastructureLogger.Errorf("Could not update the BackupInfrastructure status after reconciliation error: %+v", updateErr)
			return false, updateErr
		}
		return false, errors.New(reconcileErr.Description)
	}
	if updateErr := c.updateBackupInfrastructureStatus(op, gardencorev1alpha1.LastOperationStateSucceeded, operationType, "Backup Infrastructure has been successfully reconciled.", 100, nil); updateErr != nil {
		backupInfrastructureLogger.Errorf("Could not update the Shoot status after reconciliation success: %+v", updateErr)
		return false, updateErr
	}

	if _, updateErr := kutil.TryUpdateBackupInfrastructureAnnotations(op.K8sGardenClient.Garden(), retry.DefaultRetry, obj.ObjectMeta,
		func(backupInfrastructure *gardenv1beta1.BackupInfrastructure) (*gardenv1beta1.BackupInfrastructure, error) {
			delete(backupInfrastructure.Annotations, common.BackupInfrastructureOperation)
			return backupInfrastructure, nil
		}); updateErr != nil {
		backupInfrastructureLogger.Errorf("Could not remove %q annotation: %+v", common.BackupInfrastructureOperation, updateErr)
		return false, updateErr
	}

	return false, nil
}

// reconcileBackupInfrastructure reconciles a BackupInfrastructure state.
func (c *defaultControl) reconcileBackupInfrastructure(o *operation.Operation) *gardencorev1alpha1.LastError {
	// We create botanists (which will do the actual work).
	botanist, err := botanistpkg.New(o)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to create a Botanist (%s)", err.Error()))
	}
	seedCloudBotanist, err := cloudbotanistpkg.New(o, common.CloudPurposeSeed)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to create a Seed CloudBotanist (%s)", err.Error()))
	}

	var (
		defaultTimeout  = 30 * time.Second
		defaultInterval = 5 * time.Second

		g = flow.NewGraph("Backup Infrastructure Creation")

		deployBackupNamespace = g.Add(flow.Task{
			Name: "Deploying backup namespace",
			Fn:   flow.TaskFn(botanist.DeployBackupNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})

		_ = g.Add(flow.Task{
			Name:         "Deploying backup infrastructure",
			Fn:           flow.SimpleTaskFn(seedCloudBotanist.DeployBackupInfrastructure),
			Dependencies: flow.NewTaskIDs(deployBackupNamespace),
		})

		f = g.Compile()
	)
	err = f.Run(flow.Opts{
		Logger:           o.Logger,
		ProgressReporter: o.ReportBackupInfrastructureProgress,
	})
	if err != nil {
		o.Logger.Errorf("Failed to reconcile backup infrastructure %q: %+v", o.BackupInfrastructure.Name, err)
		return gardencorev1alpha1helper.LastError(gardencorev1alpha1helper.FormatLastErrDescription(err), gardencorev1alpha1helper.ExtractErrorCodes(flow.Causes(err))...)
	}

	o.Logger.Infof("Successfully reconciled backup infrastructure %q", o.BackupInfrastructure.Name)
	return nil
}

// deleteBackupInfrastructure deletes a BackupInfrastructure entirely.
func (c *defaultControl) deleteBackupInfrastructure(o *operation.Operation) *gardencorev1alpha1.LastError {
	// We create botanists (which will do the actual work).
	botanist, err := botanistpkg.New(o)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to create a Botanist (%s)", err.Error()))
	}

	// We first check whether the namespace in the Seed cluster does exist - if it does not, then we assume that
	// all resources have already been deleted. We can delete the BackupInfrastructure resource as a consequence.
	namespace := &corev1.Namespace{}
	namespaceName := common.GenerateBackupNamespaceName(o.BackupInfrastructure.Name)
	err = botanist.K8sSeedClient.Client().Get(context.TODO(), client.ObjectKey{Name: namespaceName}, namespace)
	if apierrors.IsNotFound(err) {
		o.Logger.Infof("Did not find '%s' namespace in the Seed cluster - nothing to be done", common.GenerateBackupNamespaceName(o.BackupInfrastructure.Name))
		return nil
	}
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to retrieve the backup namespace in the Seed cluster (%s)", err.Error()))
	}

	seedCloudBotanist, err := cloudbotanistpkg.New(o, common.CloudPurposeSeed)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to create a Seed CloudBotanist (%s)", err.Error()))
	}

	// We check whether the Backup namespace in the Seed cluster is already in a terminating state, i.e. whether
	// we have tried to delete it in a previous run. In that case, we do not need to cleanup backup infrastructure resource because
	// that would have already been done.
	var (
		cleanupBackupInfrastructureResources = namespace.Status.Phase != corev1.NamespaceTerminating
		defaultInterval                      = 5 * time.Second
		defaultTimeout                       = 30 * time.Second

		g                           = flow.NewGraph("Backup infrastructure deletion")
		destroyBackupInfrastructure = g.Add(flow.Task{
			Name: "Destroying backup infrastructure",
			Fn:   flow.SimpleTaskFn(seedCloudBotanist.DestroyBackupInfrastructure).DoIf(cleanupBackupInfrastructureResources),
		})
		deleteBackupNamespace = g.Add(flow.Task{
			Name:         "Deleting backup namespace",
			Fn:           flow.TaskFn(botanist.DeleteBackupNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(destroyBackupInfrastructure),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until backup namespace is deleted",
			Fn:           botanist.WaitUntilBackupNamespaceDeleted,
			Dependencies: flow.NewTaskIDs(deleteBackupNamespace),
		})
		f = g.Compile()
	)
	err = f.Run(flow.Opts{
		Logger:           o.Logger,
		ProgressReporter: o.ReportBackupInfrastructureProgress,
	})
	if err != nil {
		o.Logger.Errorf("Failed to delete backup infrastructure %q: %+v", o.BackupInfrastructure.Name, err)
		return gardencorev1alpha1helper.LastError(err.Error(), gardencorev1alpha1helper.ExtractErrorCodes(err)...)
	}

	o.Logger.Infof("Successfully deleted backup infrastructure %q", o.BackupInfrastructure.Name)
	return nil
}

func (c *defaultControl) updateBackupInfrastructureStatus(o *operation.Operation, state gardencorev1alpha1.LastOperationState, operationType gardencorev1alpha1.LastOperationType, operationDescription string, progress int, lastError *gardencorev1alpha1.LastError) error {
	if state == gardencorev1alpha1.LastOperationStateError && o.BackupInfrastructure.Status.LastOperation != nil {
		progress = o.BackupInfrastructure.Status.LastOperation.Progress
	}
	lastOperation := &gardencorev1alpha1.LastOperation{
		Type:           operationType,
		State:          state,
		Progress:       progress,
		Description:    operationDescription,
		LastUpdateTime: metav1.Now(),
	}

	newBackupInfrastructure, err := kutil.TryUpdateBackupInfrastructureStatus(c.k8sGardenClient.Garden(), retry.DefaultRetry, o.BackupInfrastructure.ObjectMeta,
		func(backupInfrastructure *gardenv1beta1.BackupInfrastructure) (*gardenv1beta1.BackupInfrastructure, error) {
			backupInfrastructure.Status.LastOperation = lastOperation
			backupInfrastructure.Status.LastError = lastError
			backupInfrastructure.Status.ObservedGeneration = backupInfrastructure.Generation
			return backupInfrastructure, nil
		})
	if err == nil {
		o.BackupInfrastructure = newBackupInfrastructure
	}
	return err
}

func (c *defaultControl) removeFinalizer(op *operation.Operation) error {
	backupInfrastructureFinalizers := sets.NewString(op.BackupInfrastructure.Finalizers...)
	backupInfrastructureFinalizers.Delete(gardenv1beta1.GardenerName)
	op.BackupInfrastructure.Finalizers = backupInfrastructureFinalizers.UnsortedList()

	newBackupInfrastructure, err := c.k8sGardenClient.Garden().GardenV1beta1().BackupInfrastructures(op.BackupInfrastructure.Namespace).Update(op.BackupInfrastructure)
	if err != nil {
		op.Logger.Errorf("Could not remove finalizer of the BackupInfrastructure: %+v", err.Error())
		return err
	}
	op.BackupInfrastructure = newBackupInfrastructure

	// Wait until the above modifications are reflected in the cache to prevent unwanted reconcile
	// operations (sometimes the cache is not synced fast enough).
	return utilretry.UntilTimeout(context.TODO(), time.Second, 30*time.Second, func(context.Context) (done bool, err error) {
		backupInfrastructure, err := c.k8sGardenInformers.BackupInfrastructures().Lister().BackupInfrastructures(op.BackupInfrastructure.Namespace).Get(op.BackupInfrastructure.Name)
		if apierrors.IsNotFound(err) {
			return utilretry.Ok()
		}
		if err != nil {
			return utilretry.SevereError(err)
		}
		if !sets.NewString(backupInfrastructure.Finalizers...).Has(gardenv1beta1.GardenerName) {
			return utilretry.Ok()
		}
		return utilretry.MinorError(fmt.Errorf("backup infrastructure still has finalizer %s", gardenv1beta1.GardenerName))
	})
}

func computeGracePeriod(config *config.ControllerManagerConfiguration, backupInfrastructure *gardenv1beta1.BackupInfrastructure) time.Duration {
	var (
		shootPurpose                      = backupInfrastructure.ObjectMeta.Annotations[gardencorev1alpha1.GardenPurpose]
		deletionGracePeriodHoursByPurpose = config.Controllers.BackupInfrastructure.DeletionGracePeriodHoursByPurpose
	)

	if deletionGracePeriodHours, ok := deletionGracePeriodHoursByPurpose[shootPurpose]; ok {
		return time.Hour * time.Duration(deletionGracePeriodHours)
	}

	return time.Hour * time.Duration(*config.Controllers.BackupInfrastructure.DeletionGracePeriodHours)
}

func nextReconcileScheduleReached(obj *gardenv1beta1.BackupInfrastructure, syncPeriod time.Duration) bool {
	lastOperation := obj.Status.LastOperation

	if lastOperation != nil &&
		lastOperation.Type == gardencorev1alpha1.LastOperationTypeReconcile &&
		lastOperation.State == gardencorev1alpha1.LastOperationStateSucceeded {

		earliestNextReconcile := lastOperation.LastUpdateTime.Add(syncPeriod)
		return time.Now().After(earliestNextReconcile)
	}
	return true
}
