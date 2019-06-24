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
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	cloudbotanistpkg "github.com/gardener/gardener/pkg/operation/cloudbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	hybridbotanistpkg "github.com/gardener/gardener/pkg/operation/hybridbotanist"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilretry "github.com/gardener/gardener/pkg/utils/retry"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// runDeleteShootFlow deletes a Shoot cluster entirely.
// It receives a Garden object <garden> which stores the Shoot object.
func (c *Controller) runDeleteShootFlow(o *operation.Operation) *gardencorev1alpha1.LastError {
	// If the .status.uid field is empty, then we assume that there has never been any operation running for this Shoot
	// cluster. This implies that there can not be any resource which we have to delete.
	// We accept the deletion.
	if len(o.Shoot.Info.Status.UID) == 0 {
		o.Logger.Info("`.status.uid` is empty, assuming Shoot cluster did never exist. Deletion accepted.")
		return nil
	}
	// If the shoot has never been scheduled (this is the case e.g when the scheduler cannot find a seed for the shoot), the gardener controller manager has never reconciled it.
	// This implies that there can not be any resource which we have to delete.
	// We accept the deletion.
	if o.Shoot.Info.Spec.Cloud.Seed == nil {
		o.Logger.Info("`.spec.cloud.seed` is empty, assuming Shoot cluster has never been scheduled - thus never existed. Deletion accepted.")
		return nil
	}

	// We create botanists (which will do the actual work).
	var botanist *botanistpkg.Botanist
	if err := utilretry.UntilTimeout(context.TODO(), 10*time.Second, 10*time.Minute, func(context.Context) (done bool, err error) {
		botanist, err = botanistpkg.New(o)
		if err != nil {
			return utilretry.MinorError(err)
		}
		return utilretry.Ok()
	}); err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to create a Botanist (%s)", err.Error()))
	}

	if err := botanist.RequiredExtensionsExist(); err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to check whether all required extensions exist (%s)", err.Error()))
	}

	// We first check whether the namespace in the Seed cluster does exist - if it does not, then we assume that
	// all resources have already been deleted. We can delete the Shoot resource as a consequence.
	namespace := &corev1.Namespace{}
	err := botanist.K8sSeedClient.Client().Get(context.TODO(), client.ObjectKey{Name: o.Shoot.SeedNamespace}, namespace)
	if apierrors.IsNotFound(err) {
		o.Logger.Infof("Did not find '%s' namespace in the Seed cluster - nothing to be done", o.Shoot.SeedNamespace)
		return nil
	}
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to retrieve the Shoot namespace in the Seed cluster (%s)", err.Error()))
	}

	seedCloudBotanist, err := cloudbotanistpkg.New(o, common.CloudPurposeSeed)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to create a Seed CloudBotanist (%s)", err.Error()))
	}
	shootCloudBotanist, err := cloudbotanistpkg.New(o, common.CloudPurposeShoot)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to create a Shoot CloudBotanist (%s)", err.Error()))
	}
	hybridBotanist, err := hybridbotanistpkg.New(o, botanist, seedCloudBotanist, shootCloudBotanist)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to create a HybridBotanist (%s)", err.Error()))
	}

	// We check whether the kube-apiserver deployment exists in the shoot namespace. If it does not, then we assume
	// that it has never been deployed successfully, or that we have deleted it in a previous run because we already
	// cleaned up. We follow that no (more) resources can have been deployed in the shoot cluster, thus there is nothing
	// to delete anymore.
	kubeAPIServerDeploymentFound := true
	if err := botanist.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, common.KubeAPIServerDeploymentName), &appsv1.Deployment{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to retrieve the kube-apiserver deployment in the shoot namespace in the seed cluster (%s)", err.Error()))
		}
		kubeAPIServerDeploymentFound = false
	}

	// We check whether the kube-controller-manager deployment exists in the shoot namespace. If it does not, then we assume
	// that it has never been deployed successfully, or that we have deleted it in a previous run because we already
	// cleaned up.
	kubeControllerManagerDeploymentFound := true
	if err := botanist.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, common.KubeControllerManagerDeploymentName), &appsv1.Deployment{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to retrieve the kube-controller-manager deployment in the shoot namespace in the seed cluster (%s)", err))
		}
		kubeControllerManagerDeploymentFound = false
	}

	controlPlaneDeploymentNeeded, err := c.needsControlPlaneDeployment(o)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to check whether control plane deployment is needed (%s)", err.Error()))
	}

	var (
		nonTerminatingNamespace = namespace.Status.Phase != corev1.NamespaceTerminating
		cleanupShootResources   = nonTerminatingNamespace && kubeAPIServerDeploymentFound
		defaultInterval         = 5 * time.Second
		defaultTimeout          = 30 * time.Second

		g = flow.NewGraph("Shoot cluster deletion")

		syncClusterResourceToSeed = g.Add(flow.Task{
			Name: "Syncing shoot cluster information to seed",
			Fn:   flow.TaskFn(botanist.SyncClusterResourceToSeed).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})

		// We need to ensure that the deployed cloud provider secret is up-to-date. In case it has changed then we
		// need to redeploy the cloud provider config (containing the secrets for some cloud providers) as well as
		// restart the components using the secrets (cloud controller, controller manager). We also need to update all
		// existing machine class secrets.
		deployCloudProviderSecret = g.Add(flow.Task{
			Name:         "Deploying cloud provider account secret",
			Fn:           flow.TaskFn(botanist.DeployCloudProviderSecret).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(syncClusterResourceToSeed),
		})
		deploySecrets = g.Add(flow.Task{
			Name: "Deploying Shoot certificates / keys",
			Fn:   flow.SimpleTaskFn(botanist.DeploySecrets),
		})

		wakeUpControlPlane = g.Add(flow.Task{
			Name:         "Waking up control plane to ensure proper cleanup of resources",
			Fn:           flow.TaskFn(botanist.WakeUpControlPlane).DoIf(o.Shoot.IsHibernated && cleanupShootResources),
			Dependencies: flow.NewTaskIDs(syncClusterResourceToSeed),
		})

		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.SimpleTaskFn(botanist.InitializeShootClients).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, wakeUpControlPlane),
		})

		// Redeploy the custom control plane to make sure cloud-controller-manager is restarted if the cloud provider secret changes.
		deployControlPlane = g.Add(flow.Task{
			Name:         "Deploying Shoot control plane",
			Fn:           flow.TaskFn(botanist.DeployControlPlane).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources && controlPlaneDeploymentNeeded),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret),
		})
		waitUntilControlPlaneReady = g.Add(flow.Task{
			Name:         "Waiting until shoot control plane has been reconciled",
			Fn:           flow.TaskFn(botanist.WaitUntilControlPlaneReady).DoIf(cleanupShootResources && controlPlaneDeploymentNeeded),
			Dependencies: flow.NewTaskIDs(deployControlPlane),
		})

		// Redeploy the kube-controller-manager to make sure that it's restarted if the cloud provider secret changes.
		deployKubeControllerManager = g.Add(flow.Task{
			Name:         "Deploying Kubernetes controller manager",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DeployKubeControllerManager).DoIf(cleanupShootResources && kubeControllerManagerDeploymentFound).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, initializeShootClients),
		})

		deleteSeedMonitoring = g.Add(flow.Task{
			Name:         "Deleting Shoot monitoring stack in Seed",
			Fn:           flow.TaskFn(botanist.DeleteSeedMonitoring).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		deleteClusterAutoscaler = g.Add(flow.Task{
			Name:         "Deleting cluster autoscaler",
			Fn:           flow.TaskFn(botanist.DeleteClusterAutoscaler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		deleteExtensionResources = g.Add(flow.Task{
			Name:         "Deleting extension resources",
			Fn:           flow.TaskFn(botanist.DeleteExtensionResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		waitUntilExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been deleted",
			Fn:           botanist.WaitUntilExtensionResourcesDeleted,
			Dependencies: flow.NewTaskIDs(deleteExtensionResources),
		})

		cleanupWebhooks = g.Add(flow.Task{
			Name:         "Cleaning up webhooks",
			Fn:           flow.TaskFn(botanist.CleanWebhooks).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(initializeShootClients, wakeUpControlPlane),
		})
		waitForControllersToBeActive = g.Add(flow.Task{
			Name:         "Waiting until kube-controller-manager is active",
			Fn:           flow.TaskFn(botanist.WaitForControllersToBeActive).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients, cleanupWebhooks, waitUntilControlPlaneReady, deployKubeControllerManager),
		})
		cleanExtendedAPIs = g.Add(flow.Task{
			Name:         "Cleaning extended API groups",
			Fn:           flow.TaskFn(botanist.CleanExtendedAPIs).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deleteClusterAutoscaler, waitForControllersToBeActive, waitUntilExtensionResourcesDeleted),
		})
		cleanKubernetesResources = g.Add(flow.Task{
			Name:         "Cleaning kubernetes resources",
			Fn:           flow.TaskFn(botanist.CleanKubernetesResources).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(initializeShootClients, cleanExtendedAPIs),
		})
		destroyWorker = g.Add(flow.Task{
			Name:         "Destroying Shoot workers",
			Fn:           flow.TaskFn(botanist.DestroyWorker).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		waitUntilWorkerDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot worker nodes have been terminated",
			Fn:           botanist.WaitUntilWorkerDeleted,
			Dependencies: flow.NewTaskIDs(destroyWorker),
		})
		deleteManagedResources = g.Add(flow.Task{
			Name:         "Deleting managed resources",
			Fn:           flow.TaskFn(botanist.DeleteManagedResources).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, waitUntilWorkerDeleted),
		})
		waitUntilManagedResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until managed resources have been deleted",
			Fn:           botanist.WaitUntilManagedResourcesDeleted,
			Dependencies: flow.NewTaskIDs(deleteManagedResources),
		})

		syncPointCleaned = flow.NewTaskIDs(
			cleanupWebhooks,
			cleanExtendedAPIs,
			destroyWorker,
			waitUntilWorkerDeleted,
			deleteManagedResources,
			waitUntilManagedResourcesDeleted,
		)

		deleteKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting Kubernetes API server",
			Fn:           flow.TaskFn(botanist.DeleteKubeAPIServer).Retry(defaultInterval),
			Dependencies: flow.NewTaskIDs(syncPointCleaned, waitUntilWorkerDeleted),
		})
		destroyControlPlane = g.Add(flow.Task{
			Name:         "Destroying Shoot control plane",
			Fn:           flow.TaskFn(botanist.DestroyControlPlane),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, deleteKubeAPIServer),
		})
		waitUntilControlPlaneDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot control plane has been destroyed",
			Fn:           flow.TaskFn(botanist.WaitUntilControlPlaneDeleted),
			Dependencies: flow.NewTaskIDs(destroyControlPlane),
		})

		destroyNginxIngressResources = g.Add(flow.Task{
			Name:         "Destroying ingress DNS record",
			Fn:           botanist.DestroyIngressDNSRecord,
			Dependencies: flow.NewTaskIDs(syncPointCleaned),
		})
		destroyKube2IAMResources = g.Add(flow.Task{
			Name:         "Destroying Kube2IAM resources",
			Fn:           flow.SimpleTaskFn(shootCloudBotanist.DestroyKube2IAMResources),
			Dependencies: flow.NewTaskIDs(syncPointCleaned),
		})
		destroyInfrastructure = g.Add(flow.Task{
			Name:         "Destroying Shoot infrastructure",
			Fn:           flow.TaskFn(botanist.DestroyInfrastructure).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointCleaned, waitUntilWorkerDeleted, waitUntilControlPlaneDeleted),
		})
		waitUntilInfrastructureDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot infrastructure has been destroyed",
			Fn:           botanist.WaitUntilInfrastructureDeleted,
			Dependencies: flow.NewTaskIDs(destroyInfrastructure),
		})
		destroyExternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying external domain DNS record",
			Fn:           botanist.DestroyExternalDomainDNSRecord,
			Dependencies: flow.NewTaskIDs(syncPointCleaned),
		})

		syncPoint = flow.NewTaskIDs(
			deleteSeedMonitoring,
			deleteKubeAPIServer,
			waitUntilControlPlaneDeleted,
			destroyNginxIngressResources,
			destroyKube2IAMResources,
			destroyExternalDomainDNSRecord,
			waitUntilInfrastructureDeleted,
		)

		deleteBackupInfrastructure = g.Add(flow.Task{
			Name:         "Deleting backup infrastructure",
			Fn:           flow.SimpleTaskFn(botanist.DeleteBackupInfrastructure),
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		destroyInternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying internal domain DNS record",
			Fn:           botanist.DestroyInternalDomainDNSRecord,
			Dependencies: flow.NewTaskIDs(syncPoint),
		})
		deleteNamespace = g.Add(flow.Task{
			Name:         "Deleting Shoot namespace in Seed",
			Fn:           flow.TaskFn(botanist.DeleteNamespace).Retry(defaultInterval),
			Dependencies: flow.NewTaskIDs(syncPoint, destroyInternalDomainDNSRecord, deleteBackupInfrastructure, deleteKubeAPIServer),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until Shoot namespace in Seed has been deleted",
			Fn:           botanist.WaitUntilSeedNamespaceDeleted,
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})
		_ = g.Add(flow.Task{
			Name:         "Deleting Garden secrets",
			Fn:           flow.TaskFn(botanist.DeleteGardenSecrets).Retry(defaultInterval),
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})

		f = g.Compile()
	)
	err = f.Run(flow.Opts{
		Logger:           o.Logger,
		ProgressReporter: o.ReportShootProgress,
	})
	if err != nil {
		o.Logger.Errorf("Error deleting Shoot %q: %+v", o.Shoot.Info.Name, err)
		return gardencorev1alpha1helper.LastError(gardencorev1alpha1helper.FormatLastErrDescription(err), gardencorev1alpha1helper.ExtractErrorCodes(flow.Causes(err))...)
	}

	o.Logger.Infof("Successfully deleted Shoot %q", o.Shoot.Info.Name)
	return nil
}

func (c *Controller) updateShootStatusDeleteStart(o *operation.Operation) error {
	var (
		status = o.Shoot.Info.Status
		now    = metav1.Now()
	)

	newShoot, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.Garden(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			if status.RetryCycleStartTime == nil || (status.LastOperation != nil && status.LastOperation.Type != gardencorev1alpha1.LastOperationTypeDelete) {
				shoot.Status.RetryCycleStartTime = &now
			}
			if len(status.TechnicalID) == 0 {
				shoot.Status.TechnicalID = o.Shoot.SeedNamespace
			}

			shoot.Status.Gardener = *o.GardenerInfo
			shoot.Status.ObservedGeneration = o.Shoot.Info.Generation
			shoot.Status.LastOperation = &gardencorev1alpha1.LastOperation{
				Type:           gardencorev1alpha1.LastOperationTypeDelete,
				State:          gardencorev1alpha1.LastOperationStateProcessing,
				Progress:       1,
				Description:    "Deletion of Shoot cluster in progress.",
				LastUpdateTime: now,
			}
			return shoot, nil
		})
	if err == nil {
		o.Shoot.Info = newShoot
	}
	return err
}

func (c *Controller) updateShootStatusDeleteSuccess(o *operation.Operation) error {
	newShoot, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.Garden(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			shoot.Status.RetryCycleStartTime = nil
			shoot.Status.LastError = nil
			shoot.Status.LastOperation = &gardencorev1alpha1.LastOperation{
				Type:           gardencorev1alpha1.LastOperationTypeDelete,
				State:          gardencorev1alpha1.LastOperationStateSucceeded,
				Progress:       100,
				Description:    "Shoot cluster has been successfully deleted.",
				LastUpdateTime: metav1.Now(),
			}
			return shoot, nil
		})
	if err != nil {
		return err
	}
	o.Shoot.Info = newShoot

	// Remove finalizer
	finalizers := sets.NewString(o.Shoot.Info.Finalizers...)
	finalizers.Delete(gardenv1beta1.GardenerName)
	o.Shoot.Info.Finalizers = finalizers.List()
	newShoot, err = c.k8sGardenClient.Garden().GardenV1beta1().Shoots(o.Shoot.Info.Namespace).Update(o.Shoot.Info)
	if err != nil {
		return err
	}
	o.Shoot.Info = newShoot

	// Wait until the above modifications are reflected in the cache to prevent unwanted reconcile
	// operations (sometimes the cache is not synced fast enough).
	return utilretry.UntilTimeout(context.TODO(), time.Second, 30*time.Second, func(context.Context) (done bool, err error) {
		shoot, err := c.shootLister.Shoots(o.Shoot.Info.Namespace).Get(o.Shoot.Info.Name)
		if apierrors.IsNotFound(err) {
			return utilretry.Ok()
		}
		if err != nil {
			return utilretry.SevereError(err)
		}
		lastOperation := shoot.Status.LastOperation
		if !sets.NewString(shoot.Finalizers...).Has(gardenv1beta1.GardenerName) && lastOperation != nil && lastOperation.Type == gardencorev1alpha1.LastOperationTypeDelete && lastOperation.State == gardencorev1alpha1.LastOperationStateSucceeded {
			return utilretry.Ok()
		}
		return utilretry.MinorError(fmt.Errorf("shoot still has finalizer %s", gardenv1beta1.GardenerName))
	})
}

func (c *Controller) updateShootStatusDeleteError(o *operation.Operation, lastError *gardencorev1alpha1.LastError) error {
	var (
		state       = gardencorev1alpha1.LastOperationStateFailed
		description = lastError.Description
	)

	newShoot, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.Garden(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			if !utils.TimeElapsed(shoot.Status.RetryCycleStartTime, c.config.Controllers.Shoot.RetryDuration.Duration) {
				description += " Operation will be retried."
				state = gardencorev1alpha1.LastOperationStateError
			} else {
				shoot.Status.RetryCycleStartTime = nil
			}

			shoot.Status.Gardener = *o.GardenerInfo
			shoot.Status.LastError = lastError
			shoot.Status.LastOperation.Type = gardencorev1alpha1.LastOperationTypeDelete
			shoot.Status.LastOperation.State = state
			shoot.Status.LastOperation.Description = description
			shoot.Status.LastOperation.LastUpdateTime = metav1.Now()
			return shoot, nil
		},
	)
	if err == nil {
		o.Shoot.Info = newShoot
	}
	o.Logger.Error(description)

	newShootAfterLabel, err := kutil.TryUpdateShootLabels(c.k8sGardenClient.Garden(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta, StatusLabelTransform(StatusUnhealthy))
	if err == nil {
		o.Shoot.Info = newShootAfterLabel
	}
	return err
}

func (c *Controller) needsControlPlaneDeployment(o *operation.Operation) (bool, error) {
	// Get the infrastructure resource
	infrastructure := &extensionsv1alpha1.Infrastructure{}
	if err := o.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, o.Shoot.Info.Name), infrastructure); err != nil {
		if apierrors.IsNotFound(err) {
			// The infrastructure resource has not been found, no need to redeploy the control plane
			return false, nil
		}
		return false, err
	}

	if providerStatus := infrastructure.Status.ProviderStatus; providerStatus != nil {
		// The infrastructure resource has been found with a non-nil provider status, so redeploy the control plane
		o.Shoot.InfrastructureStatus = providerStatus.Raw
		return true, nil
	}

	// The infrastructure resource has been found, but its provider status is nil
	// In this case the control plane could not have been created at all, so no need to redeploy it
	return false, nil
}
