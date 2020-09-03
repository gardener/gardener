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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// runDeleteShootFlow deletes a Shoot cluster entirely.
// It receives an Operation object <o> which stores the Shoot object and an ErrorContext which contains error from the previous operation.
func (c *Controller) runDeleteShootFlow(ctx context.Context, o *operation.Operation) *gardencorev1beta1helper.WrappedLastErrors {
	var (
		botanist                             *botanistpkg.Botanist
		shootNamespaceInDeletion             bool
		kubeAPIServerDeploymentFound         = true
		kubeControllerManagerDeploymentFound = true
		controlPlaneDeploymentNeeded         bool
		tasksWithErrors                      []string
		err                                  error
	)

	for _, lastError := range o.Shoot.Info.Status.LastErrors {
		if lastError.TaskID != nil {
			tasksWithErrors = append(tasksWithErrors, *lastError.TaskID)
		}
	}

	errorContext := errors.NewErrorContext("Shoot cluster deletion", tasksWithErrors)

	err = errors.HandleErrors(errorContext,
		func(errorID string) error {
			o.CleanShootTaskError(context.TODO(), errorID)
			return nil
		},
		nil,
		errors.ToExecute("Create botanist", func() error {
			return retryutils.UntilTimeout(context.TODO(), 10*time.Second, 10*time.Minute, func(context.Context) (done bool, err error) {
				botanist, err = botanistpkg.New(o)
				if err != nil {
					return retryutils.MinorError(err)
				}
				return retryutils.Ok()
			})
		}),
		errors.ToExecute("Check required extensions exist", func() error {
			return botanist.WaitUntilRequiredExtensionsReady(context.TODO())
		}),
		// We first check whether the namespace in the Seed cluster does exist - if it does not, then we assume that
		// all resources have already been deleted. We can delete the Shoot resource as a consequence.
		errors.ToExecute("Retrieve the Shoot namespace in the Seed cluster", func() error {
			botanist.SeedNamespaceObject = &corev1.Namespace{}
			err := botanist.K8sSeedClient.DirectClient().Get(context.TODO(), client.ObjectKey{Name: o.Shoot.SeedNamespace}, botanist.SeedNamespaceObject)
			if err != nil {
				if apierrors.IsNotFound(err) {
					o.Logger.Infof("Did not find '%s' namespace in the Seed cluster - nothing to be done", o.Shoot.SeedNamespace)
					return errors.Cancel()
				}
			}
			return err
		}),
		// Check if Seed object for shooted seed has been deleted
		errors.ToExecute("Check if Seed object for shooted seed has been deleted", func() error {
			if o.ShootedSeed != nil {
				if err := o.K8sGardenClient.DirectClient().Get(context.TODO(), kutil.Key(o.Shoot.Info.Name), &gardencorev1beta1.Seed{}); err != nil {
					if !apierrors.IsNotFound(err) {
						return err
					}
					return nil
				}
				return fmt.Errorf("seed object for shooted seed is not yet deleted - can't delete shoot")
			}
			return nil
		}),
		errors.ToExecute("Wait for seed deletion", func() error {
			if o.Shoot.Info.Namespace == v1beta1constants.GardenNamespace && o.ShootedSeed != nil {
				// wait for seed object to be deleted before going on with shoot deletion
				if err := retryutils.UntilTimeout(context.TODO(), time.Second, 300*time.Second, func(context.Context) (done bool, err error) {
					_, err = o.K8sGardenClient.GardenCore().CoreV1beta1().Seeds().Get(ctx, o.Shoot.Info.Name, kubernetes.DefaultGetOptions())
					if apierrors.IsNotFound(err) {
						return retryutils.Ok()
					}
					if err != nil {
						return retryutils.SevereError(err)
					}
					return retryutils.NotOk()
				}); err != nil {
					return fmt.Errorf("failed while waiting for seed %s to be deleted, err=%s", o.Shoot.Info.Name, err.Error())
				}
			}
			return nil
		}),
		errors.ToExecute("Check deletion timestamp for the Shoot namespace", func() error {
			var deletionError error
			shootNamespaceInDeletion, deletionError = kutil.HasDeletionTimestamp(botanist.SeedNamespaceObject)
			return deletionError
		}),
		// We check whether the kube-apiserver deployment exists in the shoot namespace. If it does not, then we assume
		// that it has never been deployed successfully, or that we have deleted it in a previous run because we already
		// cleaned up. We follow that no (more) resources can have been deployed in the shoot cluster, thus there is nothing
		// to delete anymore.
		errors.ToExecute("Retrieve kube-apiserver deployment in the shoot namespace in the seed cluster", func() error {
			deploymentKubeAPIServer := &appsv1.Deployment{}
			if err := botanist.K8sSeedClient.DirectClient().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deploymentKubeAPIServer); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
				kubeAPIServerDeploymentFound = false
			}
			if deploymentKubeAPIServer.DeletionTimestamp != nil {
				kubeAPIServerDeploymentFound = false
			}
			return nil
		}),
		// We check whether the kube-controller-manager deployment exists in the shoot namespace. If it does not, then we assume
		// that it has never been deployed successfully, or that we have deleted it in a previous run because we already
		// cleaned up.
		errors.ToExecute("Retrieve the kube-controller-manager deployment in the shoot namespace in the seed cluster", func() error {
			deploymentKubeControllerManager := &appsv1.Deployment{}
			if err := botanist.K8sSeedClient.DirectClient().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeControllerManager), deploymentKubeControllerManager); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
				kubeControllerManagerDeploymentFound = false
			}
			if deploymentKubeControllerManager.DeletionTimestamp != nil {
				kubeControllerManagerDeploymentFound = false
			}
			return nil
		}),
		errors.ToExecute("Check whether control plane deployment is needed", func() error {
			controlPlaneDeploymentNeeded, err = needsControlPlaneDeployment(o, kubeAPIServerDeploymentFound)
			return err
		}),
	)

	if err != nil {
		if errors.WasCanceled(err) {
			return nil
		}
		return gardencorev1beta1helper.NewWrappedLastErrors(gardencorev1beta1helper.FormatLastErrDescription(err), err)
	}

	var (
		nonTerminatingNamespace = botanist.SeedNamespaceObject.Status.Phase != corev1.NamespaceTerminating
		cleanupShootResources   = nonTerminatingNamespace && kubeAPIServerDeploymentFound
		wakeupRequired          = (o.Shoot.Info.Status.IsHibernated || (!o.Shoot.Info.Status.IsHibernated && o.Shoot.HibernationEnabled)) && cleanupShootResources
		defaultInterval         = 5 * time.Second
		defaultTimeout          = 30 * time.Second

		g = flow.NewGraph("Shoot cluster deletion")

		ensureShootStateExists = g.Add(flow.Task{
			Name: "Ensuring that ShootState exists",
			Fn:   flow.TaskFn(botanist.EnsureShootStateExists).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		ensureShootClusterIdentity = g.Add(flow.Task{
			Name: "Ensuring Shoot cluster identity",
			Fn:   flow.TaskFn(botanist.EnsureClusterIdentity).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})

		// We need to ensure that the deployed cloud provider secret is up-to-date. In case it has changed then we
		// need to redeploy the cloud provider config (containing the secrets for some cloud providers) as well as
		// restart the components using the secrets (cloud controller, controller manager). We also need to update all
		// existing machine class secrets.
		deployCloudProviderSecret = g.Add(flow.Task{
			Name: "Deploying cloud provider account secret",
			Fn:   flow.TaskFn(botanist.DeployCloudProviderSecret).SkipIf(shootNamespaceInDeletion),
		})
		generateSecrets = g.Add(flow.Task{
			Name:         "Generating secrets and saving them into ShootState",
			Fn:           flow.TaskFn(botanist.GenerateAndSaveSecrets).SkipIf(shootNamespaceInDeletion),
			Dependencies: flow.NewTaskIDs(ensureShootStateExists),
		})
		deploySecrets = g.Add(flow.Task{
			Name:         "Deploying Shoot certificates / keys",
			Fn:           flow.TaskFn(botanist.DeploySecrets).SkipIf(shootNamespaceInDeletion),
			Dependencies: flow.NewTaskIDs(ensureShootStateExists, generateSecrets),
		})
		// Redeploy the control plane to make sure all components that depend on the cloud provider secret are restarted
		// in case it has changed. Also, it's needed for other control plane components like the kube-apiserver or kube-
		// controller-manager to be updateable due to provider config injection.
		deployControlPlane = g.Add(flow.Task{
			Name:         "Deploying Shoot control plane",
			Fn:           flow.TaskFn(botanist.DeployControlPlane).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources && controlPlaneDeploymentNeeded && !shootNamespaceInDeletion),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, ensureShootClusterIdentity),
		})
		waitUntilControlPlaneReady = g.Add(flow.Task{
			Name:         "Waiting until Shoot control plane has been reconciled",
			Fn:           flow.TaskFn(botanist.WaitUntilControlPlaneReady).DoIf(cleanupShootResources && controlPlaneDeploymentNeeded && !shootNamespaceInDeletion),
			Dependencies: flow.NewTaskIDs(deployControlPlane),
		})
		wakeUpControlPlane = g.Add(flow.Task{
			Name:         "Waking up control plane to ensure proper cleanup of resources",
			Fn:           flow.TaskFn(botanist.WakeUpControlPlane).DoIf(wakeupRequired),
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneReady, ensureShootClusterIdentity),
		})
		waitUntilKubeAPIServerIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server reports readiness",
			Fn:           flow.TaskFn(botanist.WaitUntilKubeAPIServerReady).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(wakeUpControlPlane),
		})
		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.TaskFn(botanist.InitializeShootClients).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, waitUntilKubeAPIServerIsReady),
		})

		// Redeploy the worker extensions, and kube-controller-manager to make sure all components that depend on the
		// cloud provider secret are restarted in case it has changed.
		deployKubeControllerManager = g.Add(flow.Task{
			Name:         "Deploying Kubernetes controller manager",
			Fn:           flow.TaskFn(botanist.DeployKubeControllerManager).DoIf(cleanupShootResources && kubeControllerManagerDeploymentFound && !shootNamespaceInDeletion).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, waitUntilControlPlaneReady, initializeShootClients),
		})

		deleteSeedMonitoring = g.Add(flow.Task{
			Name:         "Deleting shoot monitoring stack in Seed",
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
			Fn:           flow.TaskFn(botanist.CleanExtendedAPIs).Timeout(10 * time.Minute).DoIf(cleanupShootResources && !metav1.HasAnnotation(o.Shoot.Info.ObjectMeta, v1beta1constants.AnnotationShootSkipCleanup)),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deleteClusterAutoscaler, waitForControllersToBeActive),
		})

		syncPointReadyForCleanup = flow.NewTaskIDs(
			initializeShootClients,
			cleanExtendedAPIs,
			deployControlPlane,
			deployKubeControllerManager,
			waitForControllersToBeActive,
		)

		cleanKubernetesResources = g.Add(flow.Task{
			Name:         "Cleaning Kubernetes resources",
			Fn:           flow.TaskFn(botanist.CleanKubernetesResources).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(syncPointReadyForCleanup),
		})
		cleanShootNamespaces = g.Add(flow.Task{
			Name:         "Cleaning shoot namespaces",
			Fn:           flow.TaskFn(botanist.CleanShootNamespaces).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		destroyNetwork = g.Add(flow.Task{
			Name:         "Destroying shoot network plugin",
			Fn:           flow.TaskFn(botanist.Shoot.Components.Extensions.Network.Destroy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanShootNamespaces),
		})
		waitUntilNetworkIsDestroyed = g.Add(flow.Task{
			Name:         "Waiting until shoot network plugin has been destroyed",
			Fn:           botanist.Shoot.Components.Extensions.Network.WaitCleanup,
			Dependencies: flow.NewTaskIDs(destroyNetwork),
		})
		destroyWorker = g.Add(flow.Task{
			Name:         "Destroying shoot workers",
			Fn:           flow.TaskFn(botanist.DestroyWorker).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanShootNamespaces),
		})
		waitUntilWorkerDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot worker nodes have been terminated",
			Fn:           botanist.WaitUntilWorkerDeleted,
			Dependencies: flow.NewTaskIDs(destroyWorker),
		})
		_ = g.Add(flow.Task{
			Name:         "Deleting operating system config resources",
			Fn:           flow.TaskFn(botanist.DeleteAllOperatingSystemConfigs).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilWorkerDeleted),
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
			Fn:           flow.TaskFn(botanist.DeleteAllExtensionResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, waitUntilManagedResourcesDeleted),
		})
		waitUntilExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been deleted",
			Fn:           botanist.WaitUntilExtensionResourcesDeleted,
			Dependencies: flow.NewTaskIDs(deleteExtensionResources),
		})
		deleteContainerRuntimeResources = g.Add(flow.Task{
			Name:         "Deleting container runtime resources",
			Fn:           flow.TaskFn(botanist.Shoot.Components.Extensions.ContainerRuntime.Destroy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients, cleanKubernetesResources, cleanShootNamespaces),
		})
		waitUntilContainerRuntimeResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until stale container runtime resources are deleted",
			Fn:           botanist.Shoot.Components.Extensions.ContainerRuntime.WaitCleanup,
			Dependencies: flow.NewTaskIDs(deleteContainerRuntimeResources),
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
			waitUntilContainerRuntimeResourcesDeleted,
		)
		destroyControlPlane = g.Add(flow.Task{
			Name:         "Destroying shoot control plane",
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
			Name:         "Destroying shoot control plane exposure",
			Fn:           flow.TaskFn(botanist.DestroyControlPlaneExposure),
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		waitUntilControlPlaneExposureDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot control plane exposure has been destroyed",
			Fn:           flow.TaskFn(botanist.WaitUntilControlPlaneExposureDeleted),
			Dependencies: flow.NewTaskIDs(destroyControlPlaneExposure),
		})

		destroyNginxIngressDNSRecord = g.Add(flow.Task{
			Name:         "Destroying nginx ingress DNS record",
			Fn:           flow.TaskFn(botanist.DestroyIngressDNSRecord),
			Dependencies: flow.NewTaskIDs(syncPointCleaned),
		})
		destroyInfrastructure = g.Add(flow.Task{
			Name:         "Destroying shoot infrastructure",
			Fn:           flow.TaskFn(botanist.Shoot.Components.Extensions.Infrastructure.Destroy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointCleaned, waitUntilControlPlaneDeleted),
		})
		waitUntilInfrastructureDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot infrastructure has been destroyed",
			Fn:           botanist.Shoot.Components.Extensions.Infrastructure.WaitCleanup,
			Dependencies: flow.NewTaskIDs(destroyInfrastructure),
		})
		destroyExternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying external domain DNS record",
			Fn:           flow.TaskFn(botanist.DestroyExternalDNS),
			Dependencies: flow.NewTaskIDs(syncPointCleaned),
		})

		syncPoint = flow.NewTaskIDs(
			deleteSeedMonitoring,
			deleteKubeAPIServer,
			waitUntilControlPlaneDeleted,
			waitUntilControlPlaneExposureDeleted,
			destroyNginxIngressDNSRecord,
			destroyExternalDomainDNSRecord,
			waitUntilInfrastructureDeleted,
		)

		destroyInternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying internal domain DNS record",
			Fn:           flow.TaskFn(botanist.DestroyInternalDNS),
			Dependencies: flow.NewTaskIDs(syncPoint),
		})
		deleteDNSProviders = g.Add(flow.Task{
			Name:         "Deleting additional DNS providers",
			Fn:           flow.TaskFn(botanist.DeleteDNSProviders),
			Dependencies: flow.NewTaskIDs(destroyInternalDomainDNSRecord),
		})
		destroyReferencedResources = g.Add(flow.Task{
			Name:         "Deleting referenced resources",
			Fn:           flow.TaskFn(botanist.DestroyReferencedResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPoint, deleteDNSProviders),
		})
		deleteNamespace = g.Add(flow.Task{
			Name:         "Deleting shoot namespace in Seed",
			Fn:           flow.TaskFn(botanist.DeleteNamespace).Retry(defaultInterval),
			Dependencies: flow.NewTaskIDs(syncPoint, deleteDNSProviders, destroyReferencedResources),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until shoot namespace in Seed has been deleted",
			Fn:           botanist.WaitUntilSeedNamespaceDeleted,
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})
		f = g.Compile()
	)

	if err := f.Run(flow.Opts{
		Logger:           o.Logger,
		ProgressReporter: o.ReportShootProgress,
		ErrorCleaner:     o.CleanShootTaskError,
		ErrorContext:     errorContext,
	}); err != nil {
		o.Logger.Errorf("Error deleting Shoot %q: %+v", o.Shoot.Info.Name, err)
		return gardencorev1beta1helper.NewWrappedLastErrors(gardencorev1beta1helper.FormatLastErrDescription(err), flow.Errors(err))
	}

	// ensure that shoot client is invalidated after it has been deleted
	if err := o.ClientMap.InvalidateClient(keys.ForShoot(o.Shoot.Info)); err != nil {
		err = fmt.Errorf("failed to invalidate shoot client: %w", err)
		return gardencorev1beta1helper.NewWrappedLastErrors(gardencorev1beta1helper.FormatLastErrDescription(err), err)
	}

	o.Logger.Infof("Successfully deleted Shoot %q", o.Shoot.Info.Name)
	return nil
}

func (c *Controller) removeFinalizerFrom(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) error {
	newShoot, err := c.updateShootStatusOperationSuccess(ctx, gardenClient.GardenCore(), shoot, "", gardencorev1beta1.LastOperationTypeDelete)
	if err != nil {
		return err
	}

	// Remove finalizer with retry on conflict
	if err := controllerutils.RemoveGardenerFinalizer(ctx, gardenClient.DirectClient(), newShoot); err != nil {
		return fmt.Errorf("could not remove finalizer from Shoot: %s", err.Error())
	}

	// Wait until the above modifications are reflected in the cache to prevent unwanted reconcile
	// operations (sometimes the cache is not synced fast enough).
	return retryutils.UntilTimeout(ctx, time.Second, 30*time.Second, func(context.Context) (done bool, err error) {
		shoot, err := c.shootLister.Shoots(shoot.Namespace).Get(shoot.Name)
		if apierrors.IsNotFound(err) {
			return retryutils.Ok()
		}
		if err != nil {
			return retryutils.SevereError(err)
		}
		lastOperation := shoot.Status.LastOperation
		if !sets.NewString(shoot.Finalizers...).Has(gardencorev1beta1.GardenerName) && lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeDelete && lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
			return retryutils.Ok()
		}
		return retryutils.MinorError(fmt.Errorf("shoot still has finalizer %s", gardencorev1beta1.GardenerName))
	})
}

func needsControlPlaneDeployment(o *operation.Operation, kubeAPIServerDeploymentFound bool) (bool, error) {
	var (
		client    = o.K8sSeedClient.DirectClient()
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

func extensionResourceStillExists(c client.Client, obj runtime.Object, namespace, name string) (bool, error) {
	if err := c.Get(context.TODO(), kutil.Key(namespace, name), obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
