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

	utilretry "github.com/gardener/gardener/pkg/utils/retry"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
)

// deleteShoot deletes a Shoot cluster entirely.
// It receives a Garden object <garden> which stores the Shoot object.
func (c *defaultControl) deleteShoot(o *operation.Operation) *gardencorev1alpha1.LastError {
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
		return formatError("Failed to create a Botanist", err)
	}

	if err := botanist.RequiredExtensionsExist(); err != nil {
		return formatError("Failed to check whether all required extensions exist", err)
	}

	// We first check whether the namespace in the Seed cluster does exist - if it does not, then we assume that
	// all resources have already been deleted. We can delete the Shoot resource as a consequence.
	namespace, err := botanist.K8sSeedClient.GetNamespace(o.Shoot.SeedNamespace)
	if apierrors.IsNotFound(err) {
		o.Logger.Infof("Did not find '%s' namespace in the Seed cluster - nothing to be done", o.Shoot.SeedNamespace)
		return nil
	}
	if err != nil {
		return formatError("Failed to retrieve the Shoot namespace in the Seed cluster", err)
	}

	seedCloudBotanist, err := cloudbotanistpkg.New(o, common.CloudPurposeSeed)
	if err != nil {
		return formatError("Failed to create a Seed CloudBotanist", err)
	}
	shootCloudBotanist, err := cloudbotanistpkg.New(o, common.CloudPurposeShoot)
	if err != nil {
		return formatError("Failed to create a Shoot CloudBotanist", err)
	}
	hybridBotanist, err := hybridbotanistpkg.New(o, botanist, seedCloudBotanist, shootCloudBotanist)
	if err != nil {
		return formatError("Failed to create a HybridBotanist", err)
	}

	// We check whether the shoot namespace in the seed cluster is already in a terminating state, i.e. whether
	// we have tried to delete it in a previous run. In that case, we do not need to cleanup shoot resource because
	// that would have already been done.
	// We also check whether the kube-apiserver deployment exists in the shoot namespace. If it does not, then we assume
	// that it has never been deployed successfully, or that we have deleted it in a previous run because we already
	// cleaned up. We follow that no (more) resources can have been deployed in the shoot cluster, thus there is nothing
	// to delete anymore.
	kubeAPIServerDeploymentFound := true
	if err := botanist.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, common.KubeAPIServerDeploymentName), &appsv1.Deployment{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return formatError("Failed to retrieve the kube-apiserver deployment in the shoot namespace in the seed cluster", err)
		}
		kubeAPIServerDeploymentFound = false
	}

	infrastructureMigrationNeeded, err := c.needsInfrastructureMigration(o)
	if err != nil {
		return formatError("Failed to check whether infrastructure migration is needed", err)
	}
	workerMigrationNeeded, err := c.needsWorkerMigration(o)
	if err != nil {
		return formatError("Failed to check whether worker migration is needed", err)
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
			Fn:           flow.SimpleTaskFn(botanist.DeployCloudProviderSecret).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(syncClusterResourceToSeed),
		})
		refreshCloudProviderConfig = g.Add(flow.Task{
			Name:         "Refreshing cloud provider configuration",
			Fn:           flow.SimpleTaskFn(hybridBotanist.RefreshCloudProviderConfig).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret),
		})
		refreshCloudControllerManager = g.Add(flow.Task{
			Name:         "Refreshing cloud controller manager checksums",
			Fn:           flow.SimpleTaskFn(botanist.RefreshCloudControllerManagerChecksums).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, refreshCloudProviderConfig),
		})
		refreshKubeControllerManager = g.Add(flow.Task{
			Name:         "Refreshing Kubernetes controller manager checksums",
			Fn:           flow.SimpleTaskFn(botanist.RefreshKubeControllerManagerChecksums).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, refreshCloudProviderConfig),
		})
		deploySecrets = g.Add(flow.Task{
			Name: "Deploying Shoot certificates / keys",
			Fn:   flow.SimpleTaskFn(botanist.DeploySecrets).DoIf(infrastructureMigrationNeeded || workerMigrationNeeded || (cleanupShootResources && botanist.Shoot.UsesCSI())),
		})
		refreshCSIControllers = g.Add(flow.Task{
			Name:         "Refreshing CSI Controllers checksums",
			Fn:           flow.SimpleTaskFn(hybridBotanist.RefreshCSIControllersChecksums).DoIf(cleanupShootResources && botanist.Shoot.UsesCSI()).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, deploySecrets, refreshCloudProviderConfig),
		})

		wakeUpControlPlane = g.Add(flow.Task{
			Name:         "Waking up control plane to ensure proper cleanup of resources",
			Fn:           flow.TaskFn(botanist.WakeUpControlPlane).DoIf(o.Shoot.IsHibernated && cleanupShootResources),
			Dependencies: flow.NewTaskIDs(syncClusterResourceToSeed),
		})

		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.SimpleTaskFn(botanist.InitializeShootClients).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, refreshCloudProviderConfig, refreshCloudControllerManager, refreshKubeControllerManager, refreshCSIControllers, wakeUpControlPlane),
		})

		// Only needed for migration from in-tree infrastructure management to out-of-tree mgmt by an extension controller.
		// If a shoot is already marked for deletion then the new Infrastructure resources might not exist, so
		// let's just quickly deploy it. If it already existed then nothing happens, but we can make sure that the
		// extension controller cleans up. If it did not exist then the resource is just created, and can be
		// safely deleted by latter steps.
		deployInfrastructure = g.Add(flow.Task{
			Name:         "Deploying Shoot infrastructure",
			Fn:           flow.TaskFn(botanist.DeployInfrastructure).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(infrastructureMigrationNeeded),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret),
		})
		waitUntilInfrastructureReady = g.Add(flow.Task{
			Name:         "Waiting until shoot infrastructure has been reconciled",
			Fn:           flow.TaskFn(botanist.WaitUntilInfrastructureReady).DoIf(infrastructureMigrationNeeded || workerMigrationNeeded),
			Dependencies: flow.NewTaskIDs(deployInfrastructure),
		})

		// Only needed for migration from in-tree worker management to out-of-tree mgmt by an extension controller.
		// If a shoot is already marked for deletion then the new Worker resource might not exist, so
		// let's just quickly deploy it. If it already existed then nothing happens, but we can make sure that the
		// extension controller cleans up. If it did not exist then the resource is just created, and can be
		// safely deleted by latter steps.
		computeShootOSConfig = g.Add(flow.Task{
			Name:         "Computing operating system specific configuration for shoot workers",
			Fn:           flow.SimpleTaskFn(hybridBotanist.ComputeShootOperatingSystemConfig).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(workerMigrationNeeded),
			Dependencies: flow.NewTaskIDs(deploySecrets, initializeShootClients, deployInfrastructure),
		})
		deployWorker = g.Add(flow.Task{
			Name:         "Configuring shoot worker pools",
			Fn:           flow.TaskFn(botanist.DeployWorker).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(workerMigrationNeeded),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, waitUntilInfrastructureReady, initializeShootClients, computeShootOSConfig),
		})

		// Deletion of monitoring stack (to avoid false positive alerts) and kube-addon-manager (to avoid redeploying
		// resources).
		deleteSeedMonitoring = g.Add(flow.Task{
			Name:         "Deleting Shoot monitoring stack in Seed",
			Fn:           flow.SimpleTaskFn(botanist.DeleteSeedMonitoring).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		deleteKubeAddonManager = g.Add(flow.Task{
			Name:         "Deleting Kubernetes addon manager",
			Fn:           flow.SimpleTaskFn(botanist.DeleteKubeAddonManager).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		deleteExtensionResources = g.Add(flow.Task{
			Name:         "Deleting extension resources",
			Fn:           flow.TaskFn(botanist.DeleteExtensionResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		waitUntilExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been deleted",
			Fn:           flow.TaskFn(botanist.WaitUntilExtensionResourcesDeleted),
			Dependencies: flow.NewTaskIDs(deleteExtensionResources),
		})
		deleteClusterAutoscaler = g.Add(flow.Task{
			Name:         "Deleting cluster autoscaler",
			Fn:           flow.SimpleTaskFn(botanist.DeleteClusterAutoscaler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		waitUntilKubeAddonManagerDeleted = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes addon manager has been deleted",
			Fn:           botanist.WaitUntilKubeAddonManagerDeleted,
			Dependencies: flow.NewTaskIDs(deleteKubeAddonManager),
		})

		cleanupWebhooks = g.Add(flow.Task{
			Name:         "Cleaning up webhooks",
			Fn:           flow.TaskFn(botanist.CleanWebhooks).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(refreshKubeControllerManager, refreshCloudControllerManager, wakeUpControlPlane, waitUntilKubeAddonManagerDeleted),
		})
		waitForControllersToBeActive = g.Add(flow.Task{
			Name:         "Waiting until both cloud-controller-manager and kube-controller-manager are active",
			Fn:           flow.SimpleTaskFn(botanist.WaitForControllersToBeActive).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanupWebhooks),
		})
		cleanExtendedAPIs = g.Add(flow.Task{
			Name:         "Cleaning extended API groups",
			Fn:           flow.TaskFn(botanist.CleanExtendedAPIs).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAddonManagerDeleted, deleteClusterAutoscaler, waitForControllersToBeActive, waitUntilExtensionResourcesDeleted),
		})
		cleanKubernetesResources = g.Add(flow.Task{
			Name:         "Cleaning kubernetes resources",
			Fn:           flow.TaskFn(botanist.CleanKubernetesResources).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(cleanExtendedAPIs),
		})
		destroyWorker = g.Add(flow.Task{
			Name:         "Destroying Shoot workers",
			Fn:           flow.TaskFn(botanist.DestroyWorker).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, deployWorker),
		})
		waitUntilWorkerDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot worker nodes have been terminated",
			Fn:           flow.TaskFn(botanist.WaitUntilWorkerDeleted),
			Dependencies: flow.NewTaskIDs(destroyWorker),
		})
		deleteKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting Kubernetes API server",
			Fn:           flow.SimpleTaskFn(botanist.DeleteKubeAPIServer).Retry(defaultInterval),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, waitUntilWorkerDeleted),
		})

		destroyNginxIngressResources = g.Add(flow.Task{
			Name:         "Destroying ingress DNS record",
			Fn:           flow.TaskFn(botanist.DestroyIngressDNSRecord),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		destroyKube2IAMResources = g.Add(flow.Task{
			Name:         "Destroying Kube2IAM resources",
			Fn:           flow.SimpleTaskFn(shootCloudBotanist.DestroyKube2IAMResources),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		destroyInfrastructure = g.Add(flow.Task{
			Name:         "Destroying Shoot infrastructure",
			Fn:           flow.TaskFn(botanist.DestroyInfrastructure),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, waitUntilWorkerDeleted, waitUntilInfrastructureReady),
		})
		waitUntilInfrastructureDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot infrastructure has been destroyed",
			Fn:           flow.TaskFn(botanist.WaitUntilInfrastructureDeleted),
			Dependencies: flow.NewTaskIDs(destroyInfrastructure),
		})
		destroyExternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying external domain DNS record",
			Fn:           flow.TaskFn(botanist.DestroyExternalDomainDNSRecord),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		syncPoint = flow.NewTaskIDs(
			deleteSeedMonitoring,
			deleteKubeAPIServer,
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
			Fn:           flow.SimpleTaskFn(botanist.DeleteNamespace).Retry(defaultInterval),
			Dependencies: flow.NewTaskIDs(syncPoint, destroyInternalDomainDNSRecord, deleteBackupInfrastructure, deleteKubeAPIServer),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until Shoot namespace in Seed has been deleted",
			Fn:           botanist.WaitUntilSeedNamespaceDeleted,
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})
		_ = g.Add(flow.Task{
			Name:         "Deleting Garden secrets",
			Fn:           flow.SimpleTaskFn(botanist.DeleteGardenSecrets).Retry(defaultInterval),
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

		return &gardencorev1alpha1.LastError{
			Codes:       gardencorev1alpha1helper.ExtractErrorCodes(flow.Causes(err)),
			Description: gardencorev1alpha1helper.FormatLastErrDescription(err),
		}
	}

	o.Logger.Infof("Successfully deleted Shoot %q", o.Shoot.Info.Name)
	return nil
}

func (c *defaultControl) updateShootStatusDeleteStart(o *operation.Operation) error {
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

func (c *defaultControl) updateShootStatusDeleteSuccess(o *operation.Operation) error {
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
		shoot, err := c.k8sGardenInformers.Shoots().Lister().Shoots(o.Shoot.Info.Namespace).Get(o.Shoot.Info.Name)
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

func (c *defaultControl) updateShootStatusDeleteError(o *operation.Operation, lastError *gardencorev1alpha1.LastError) (gardencorev1alpha1.LastOperationState, error) {
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
	return state, err
}

func (c *defaultControl) needsInfrastructureMigration(o *operation.Operation) (bool, error) {
	if err := o.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, o.Shoot.Info.Name), &extensionsv1alpha1.Infrastructure{}); err != nil {
		if apierrors.IsNotFound(err) {
			// The infrastructure resource has not been found - we need to check whether the Terraform state does still exist.
			// If it does still exist then we need to migrate. Otherwise there are no infrastructures resources anymore that
			// need to be deleted, so no migration would be necessary.

			tf, err := o.NewShootTerraformer(common.TerraformerPurposeInfraDeprecated)
			if err != nil {
				return false, err
			}

			configExists, err := tf.ConfigExists()
			if err != nil {
				return false, err
			}

			return configExists, nil
		}
		return false, err
	}

	// The infrastructure resource has been found - no need to migrate as it already exists.
	return false, nil
}

func (c *defaultControl) needsWorkerMigration(o *operation.Operation) (bool, error) {
	if err := o.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, o.Shoot.Info.Name), &extensionsv1alpha1.Worker{}); err != nil {
		if apierrors.IsNotFound(err) {
			// The Worker resource has not been found - we need to check whether the MCM deployment does still exist.
			// If it does still exist then we need to migrate. Otherwise there are no machine resources anymore that
			// need to be deleted, so no migration would be necessary.
			machineDeployments := &machinev1alpha1.MachineDeploymentList{}
			if err := o.K8sSeedClient.Client().List(context.TODO(), machineDeployments, kutil.Limit(1)); err != nil {
				return false, err
			}
			return len(machineDeployments.Items) != 0, nil
		}

		return false, err
	}

	// The Worker resource has been found - no need to migrate as it already exists.
	return false, nil
}
