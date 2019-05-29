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
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/awsbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	hybridbotanistpkg "github.com/gardener/gardener/pkg/operation/hybridbotanist"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilretry "github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/version"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// runDeleteShootFlow deletes a Shoot cluster entirely.
// It receives an Operation object <o> which stores the Shoot object.
func (c *Controller) runDeleteShootFlow(o *operation.Operation) *gardencorev1alpha1.LastError {
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
	if err := botanist.K8sSeedClient.Client().Get(context.TODO(), client.ObjectKey{Name: o.Shoot.SeedNamespace}, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			o.Logger.Infof("Did not find '%s' namespace in the Seed cluster - nothing to be done", o.Shoot.SeedNamespace)
			return nil
		}
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to retrieve the Shoot namespace in the Seed cluster (%s)", err.Error()))
	}

	// Unregister the Shoot as Seed cluster if it was annotated to be a seed and is in the garden namespace
	if o.Shoot.Info.Namespace == common.GardenNamespace && o.ShootedSeed != nil {
		if err := botanist.UnregisterAsSeed(); err != nil {
			return gardencorev1alpha1helper.LastError(fmt.Sprintf("Could not unregister Shoot %q as Seed: %+v", o.Shoot.Info.Name, err))
		}

		// wait for seed object to be deleted before going on with shoot deletion
		if err := utilretry.UntilTimeout(context.TODO(), time.Second, 300*time.Second, func(context.Context) (done bool, err error) {
			_, err = c.k8sGardenClient.Garden().GardenV1beta1().Seeds().Get(o.Shoot.Info.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return utilretry.Ok()
			}
			if err != nil {
				return utilretry.SevereError(err)
			}
			return utilretry.NotOk()
		}); err != nil {
			return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed while waiting for seed %s to be deleted, err=%s", o.Shoot.Info.Name, err.Error()))
		}
	}

	shootNamespaceInDeletion, err := kutil.HasDeletionTimestamp(namespace)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to check the deletion timestamp for the Shoot namespace (%s)", err.Error()))
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
	deploymentKubeAPIServer := &appsv1.Deployment{}
	if err := botanist.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeAPIServer), deploymentKubeAPIServer); err != nil {
		if !apierrors.IsNotFound(err) {
			return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to retrieve the kube-apiserver deployment in the shoot namespace in the seed cluster (%s)", err.Error()))
		}
		kubeAPIServerDeploymentFound = false
	}
	if deploymentKubeAPIServer.DeletionTimestamp != nil {
		kubeAPIServerDeploymentFound = false
	}

	// We check whether the kube-controller-manager deployment exists in the shoot namespace. If it does not, then we assume
	// that it has never been deployed successfully, or that we have deleted it in a previous run because we already
	// cleaned up.
	kubeControllerManagerDeploymentFound := true
	deploymentKubeControllerManager := &appsv1.Deployment{}
	if err := botanist.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeControllerManager), deploymentKubeControllerManager); err != nil {
		if !apierrors.IsNotFound(err) {
			return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to retrieve the kube-controller-manager deployment in the shoot namespace in the seed cluster (%s)", err))
		}
		kubeControllerManagerDeploymentFound = false
	}
	if deploymentKubeControllerManager.DeletionTimestamp != nil {
		kubeControllerManagerDeploymentFound = false
	}

	controlPlaneDeploymentNeeded, err := needsControlPlaneDeployment(o, kubeAPIServerDeploymentFound)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to check whether control plane deployment is needed (%s)", err.Error()))
	}
	workerDeploymentNeeded, err := needsWorkerDeployment(o)
	if err != nil {
		return gardencorev1alpha1helper.LastError(fmt.Sprintf("Failed to check whether worker deployment is needed (%s)", err.Error()))
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
			Fn:           flow.TaskFn(botanist.DeployCloudProviderSecret).DoIf(cleanupShootResources && !shootNamespaceInDeletion),
			Dependencies: flow.NewTaskIDs(syncClusterResourceToSeed),
		})
		deploySecrets = g.Add(flow.Task{
			Name: "Deploying Shoot certificates / keys",
			Fn:   flow.TaskFn(botanist.DeploySecrets).SkipIf(shootNamespaceInDeletion),
		})
		// Redeploy the control plane to make sure all components that depend on the cloud provider secret are restarted
		// in case it has changed. Also, it's needed for other control plane components like the kube-apiserver or kube-
		// controller-manager to be updateable due to provider config injection.
		deployControlPlane = g.Add(flow.Task{
			Name:         "Deploying Shoot control plane",
			Fn:           flow.TaskFn(botanist.DeployControlPlane).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources && controlPlaneDeploymentNeeded && !shootNamespaceInDeletion),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, syncClusterResourceToSeed),
		})
		waitUntilControlPlaneReady = g.Add(flow.Task{
			Name:         "Waiting until Shoot control plane has been reconciled",
			Fn:           flow.TaskFn(botanist.WaitUntilControlPlaneReady).DoIf(cleanupShootResources && controlPlaneDeploymentNeeded && !shootNamespaceInDeletion),
			Dependencies: flow.NewTaskIDs(deployControlPlane),
		})
		wakeUpControlPlane = g.Add(flow.Task{
			Name:         "Waking up control plane to ensure proper cleanup of resources",
			Fn:           flow.TaskFn(botanist.WakeUpControlPlane).DoIf((o.Shoot.Info.Status.IsHibernated != nil && *o.Shoot.Info.Status.IsHibernated || o.Shoot.Info.Status.IsHibernated == nil && o.Shoot.HibernationEnabled) && cleanupShootResources),
			Dependencies: flow.NewTaskIDs(syncClusterResourceToSeed, waitUntilControlPlaneReady),
		})
		waitUntilKubeAPIServerIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server reports readiness",
			Fn:           flow.TaskFn(botanist.WaitUntilKubeAPIServerReady).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(wakeUpControlPlane),
		})
		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.SimpleTaskFn(botanist.InitializeShootClients).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, waitUntilKubeAPIServerIsReady),
		})

		// Redeploy the worker extensions, and kube-controller-manager to make sure all components that depend on the
		// cloud provider secret are restarted in case it has changed.
		computeShootOSConfig = g.Add(flow.Task{
			Name:         "Computing operating system specific configuration for shoot workers",
			Fn:           flow.TaskFn(botanist.ComputeShootOperatingSystemConfig).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources && workerDeploymentNeeded && !shootNamespaceInDeletion),
			Dependencies: flow.NewTaskIDs(deploySecrets, waitUntilControlPlaneReady, initializeShootClients),
		})
		deployWorker = g.Add(flow.Task{
			Name:         "Configuring shoot worker pools",
			Fn:           flow.TaskFn(botanist.DeployWorker).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources && workerDeploymentNeeded && !shootNamespaceInDeletion),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, initializeShootClients, computeShootOSConfig),
		})
		deployKubeControllerManager = g.Add(flow.Task{
			Name:         "Deploying Kubernetes controller manager",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DeployKubeControllerManager).DoIf(cleanupShootResources && kubeControllerManagerDeploymentFound && !shootNamespaceInDeletion).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, waitUntilControlPlaneReady, initializeShootClients),
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

		cleanupWebhooks = g.Add(flow.Task{
			Name:         "Cleaning up webhooks",
			Fn:           flow.TaskFn(botanist.CleanWebhooks).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(initializeShootClients, wakeUpControlPlane),
		})
		waitForControllersToBeActive = g.Add(flow.Task{
			Name:         "Waiting until kube-controller-manager is active",
			Fn:           flow.TaskFn(botanist.WaitForControllersToBeActive).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients, cleanupWebhooks, deployControlPlane, deployKubeControllerManager),
		})
		cleanExtendedAPIs = g.Add(flow.Task{
			Name:         "Cleaning extended API groups",
			Fn:           flow.TaskFn(botanist.CleanExtendedAPIs).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deleteClusterAutoscaler, waitForControllersToBeActive),
		})

		syncPointReadyForCleanup = flow.NewTaskIDs(
			initializeShootClients,
			cleanExtendedAPIs,
			deployControlPlane,
			deployWorker,
			deployKubeControllerManager,
			waitForControllersToBeActive,
		)

		cleanKubernetesResources = g.Add(flow.Task{
			Name:         "Cleaning kubernetes resources",
			Fn:           flow.TaskFn(botanist.CleanKubernetesResources).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(syncPointReadyForCleanup),
		})
		cleanShootNamespaces = g.Add(flow.Task{
			Name:         "Cleaning Shoot namespaces",
			Fn:           flow.TaskFn(botanist.CleanShootNamespaces).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		destroyNetwork = g.Add(flow.Task{
			Name:         "Destroying shoot network plugin",
			Fn:           flow.TaskFn(botanist.DestroyNetwork).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanShootNamespaces),
		})
		waitUntilNetworkIsDestroyed = g.Add(flow.Task{
			Name:         "Waiting until shoot network plugin has been destroyed",
			Fn:           flow.TaskFn(botanist.WaitUntilNetworkIsDeleted),
			Dependencies: flow.NewTaskIDs(destroyNetwork),
		})
		destroyWorker = g.Add(flow.Task{
			Name:         "Destroying Shoot workers",
			Fn:           flow.TaskFn(botanist.DestroyWorker).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanShootNamespaces),
		})
		waitUntilWorkerDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot worker nodes have been terminated",
			Fn:           botanist.WaitUntilWorkerDeleted,
			Dependencies: flow.NewTaskIDs(destroyWorker),
		})
		deleteManagedResources = g.Add(flow.Task{
			Name:         "Deleting managed resources",
			Fn:           flow.TaskFn(botanist.DeleteManagedResources).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanShootNamespaces, waitUntilWorkerDeleted),
		})
		waitUntilManagedResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until managed resources have been deleted",
			Fn:           flow.TaskFn(botanist.WaitUntilManagedResourcesDeleted).DoIf(cleanupShootResources).Timeout(10 * time.Minute),
			Dependencies: flow.NewTaskIDs(deleteManagedResources),
		})
		deleteExtensionResources = g.Add(flow.Task{
			Name:         "Deleting extension resources",
			Fn:           flow.TaskFn(botanist.DeleteExtensionResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, waitUntilManagedResourcesDeleted),
		})
		waitUntilExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been deleted",
			Fn:           botanist.WaitUntilExtensionResourcesDeleted,
			Dependencies: flow.NewTaskIDs(deleteExtensionResources),
		})

		// Services (and other objects that have a footprint in the infrastructure) still don't have finalizers yet. There is no way to
		// determine whether all the resources have been deleted successfully yet, whether there was an error, or whether they are still
		// pending. While most providers have implemented custom clean up already (basically, duplicated the code in the CCM) not everybody
		// has, especially not for all objects.
		// Until service finalizers are enabled by default with Kubernetes 1.16 and our minimum supported seed version is raised to 1.16 we
		// can not do much more than best-effort waiting until everything has been cleaned up. That's what the following task is doing.
		timeForInfrastructureResourceCleanup = g.Add(flow.Task{
			Name: "Waiting until time for infrastructure resource cleanup has elapsed",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()

				<-ctx.Done()
				return nil
			}).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(deleteManagedResources),
		})

		syncPointCleaned = flow.NewTaskIDs(
			cleanupWebhooks,
			cleanExtendedAPIs,
			cleanKubernetesResources,
			cleanShootNamespaces,
			waitUntilWorkerDeleted,
			waitUntilManagedResourcesDeleted,
			timeForInfrastructureResourceCleanup,
			destroyNetwork,
			waitUntilNetworkIsDestroyed,
			waitUntilExtensionResourcesDeleted,
		)

		destroyControlPlane = g.Add(flow.Task{
			Name:         "Destroying Shoot control plane",
			Fn:           flow.TaskFn(botanist.DestroyControlPlane).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointCleaned),
		})
		waitUntilControlPlaneDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot control plane has been destroyed",
			Fn:           botanist.WaitUntilControlPlaneDeleted,
			Dependencies: flow.NewTaskIDs(destroyControlPlane),
		})

		deleteKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting Kubernetes API server",
			Fn:           flow.TaskFn(botanist.DeleteKubeAPIServer).Retry(defaultInterval),
			Dependencies: flow.NewTaskIDs(syncPointCleaned, waitUntilControlPlaneDeleted),
		})

		destroyControlPlaneExposure = g.Add(flow.Task{
			Name:         "Destroying Shoot control plane exposure",
			Fn:           flow.TaskFn(botanist.DestroyControlPlaneExposure),
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		waitUntilControlPlaneExposureDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot control plane exposure has been destroyed",
			Fn:           flow.TaskFn(botanist.WaitUntilControlPlaneExposureDeleted),
			Dependencies: flow.NewTaskIDs(destroyControlPlaneExposure),
		})

		destroyNginxIngressDNSRecord = g.Add(flow.Task{
			Name:         "Destroying ingress DNS record",
			Fn:           botanist.DestroyIngressDNSRecord,
			Dependencies: flow.NewTaskIDs(syncPointCleaned),
		})
		// kube2iam is deprecated and is kept here only for backwards compatibility reasons because some end-users may depend
		// on it. It will be removed very soon in the future.
		destroyKube2IAMResources = g.Add(flow.Task{
			Name:         "Destroying Kube2IAM resources",
			Fn:           flow.SimpleTaskFn(func() error { return awsbotanist.DestroyKube2IAMResources(o) }).DoIf(o.Shoot.CloudProvider == gardenv1beta1.CloudProviderAWS),
			Dependencies: flow.NewTaskIDs(syncPointCleaned),
		})
		destroyInfrastructure = g.Add(flow.Task{
			Name:         "Destroying Shoot infrastructure",
			Fn:           flow.TaskFn(botanist.DestroyInfrastructure).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointCleaned, waitUntilControlPlaneDeleted),
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
			waitUntilControlPlaneExposureDeleted,
			destroyNginxIngressDNSRecord,
			destroyKube2IAMResources,
			destroyExternalDomainDNSRecord,
			waitUntilInfrastructureDeleted,
		)

		destroyInternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying internal domain DNS record",
			Fn:           botanist.DestroyInternalDomainDNSRecord,
			Dependencies: flow.NewTaskIDs(syncPoint),
		})
		deleteNamespace = g.Add(flow.Task{
			Name:         "Deleting Shoot namespace in Seed",
			Fn:           flow.TaskFn(botanist.DeleteNamespace).Retry(defaultInterval),
			Dependencies: flow.NewTaskIDs(syncPoint, destroyInternalDomainDNSRecord, deleteKubeAPIServer),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until Shoot namespace in Seed has been deleted",
			Fn:           botanist.WaitUntilSeedNamespaceDeleted,
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})

		f = g.Compile()
	)
	if err := f.Run(flow.Opts{
		Logger:           o.Logger,
		ProgressReporter: o.ReportShootProgress,
	}); err != nil {
		o.Logger.Errorf("Error deleting Shoot %q: %+v", o.Shoot.Info.Name, err)
		return gardencorev1alpha1helper.LastError(gardencorev1alpha1helper.FormatLastErrDescription(err), gardencorev1alpha1helper.ExtractErrorCodes(flow.Causes(err))...)
	}

	o.Logger.Infof("Successfully deleted Shoot %q", o.Shoot.Info.Name)
	return nil
}

func (c *Controller) updateShootStatusDeleteStart(o *operation.Operation) error {
	var (
		status = o.Shoot.Info.Status
		now    = metav1.NewTime(time.Now().UTC())
	)

	newShoot, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.Garden(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			if status.RetryCycleStartTime == nil ||
				(status.LastOperation != nil && status.LastOperation.Type != gardencorev1alpha1.LastOperationTypeDelete) ||
				o.Shoot.Info.Generation != o.Shoot.Info.Status.ObservedGeneration ||
				o.Shoot.Info.Status.Gardener.Version == version.Get().GitVersion ||
				(o.Shoot.Info.Status.LastOperation != nil && o.Shoot.Info.Status.LastOperation.State == gardencorev1alpha1.LastOperationStateFailed) {

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

func needsControlPlaneDeployment(o *operation.Operation, kubeAPIServerDeploymentFound bool) (bool, error) {
	var (
		client    = o.K8sSeedClient.Client()
		namespace = o.Shoot.SeedNamespace
		name      = o.Shoot.Info.Name
	)

	// If the `ControlPlane` resource and the kube-apiserver deployment do no longer exist then we don't want to re-deploy it.
	// The reason for the second condition is that some providers inject a cloud-provider-config into the kube-apiserver deployment
	// which is needed for it to run.
	exists, err := extensionResourceStillExists(client, &extensionsv1alpha1.ControlPlane{}, namespace, name)
	if err != nil {
		return false, err
	}
	if !exists && !kubeAPIServerDeploymentFound {
		return false, nil
	}

	// Get the infrastructure resource
	infrastructure := &extensionsv1alpha1.Infrastructure{}
	if err := client.Get(context.TODO(), kutil.Key(namespace, name), infrastructure); err != nil {
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

func needsWorkerDeployment(o *operation.Operation) (bool, error) {
	var (
		client    = o.K8sSeedClient.Client()
		namespace = o.Shoot.SeedNamespace
		name      = o.Shoot.Info.Name
	)

	// If the `Worker` resource does no longer exist then we don't want to re-deploy it.
	exists, err := extensionResourceStillExists(client, &extensionsv1alpha1.Worker{}, namespace, name)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func extensionResourceStillExists(c client.Client, obj runtime.Object, namespace, name string) (bool, error) {
	if err := c.Get(context.TODO(), kutil.Key(namespace, name), obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
