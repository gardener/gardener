// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
)

// runDeleteShootFlow deletes a Shoot cluster.
// It receives an Operation object <o> which stores the Shoot object and an ErrorContext which contains error from the previous operation.
func (r *Reconciler) runDeleteShootFlow(ctx context.Context, o *operation.Operation) *v1beta1helper.WrappedLastErrors {
	var (
		botanist                             *botanistpkg.Botanist
		kubeAPIServerDeploymentFound         = true
		kubeControllerManagerDeploymentFound = true
		kubeAPIServerDeploymentReplicas      int32
		infrastructure                       *extensionsv1alpha1.Infrastructure
		controlPlaneDeploymentNeeded         bool
		tasksWithErrors                      []string
		err                                  error
	)

	for _, lastError := range o.Shoot.GetInfo().Status.LastErrors {
		if lastError.TaskID != nil {
			tasksWithErrors = append(tasksWithErrors, *lastError.TaskID)
		}
	}

	errorContext := errors.NewErrorContext("Shoot cluster deletion", tasksWithErrors)

	err = errors.HandleErrors(errorContext,
		func(errorID string) error {
			o.CleanShootTaskError(ctx, errorID)
			return nil
		},
		nil,
		errors.ToExecute("Create botanist", func() error {
			return retryutils.UntilTimeout(ctx, 10*time.Second, 10*time.Minute, func(context.Context) (done bool, err error) {
				botanist, err = botanistpkg.New(ctx, o)
				if err != nil {
					return retryutils.MinorError(err)
				}
				return retryutils.Ok()
			})
		}),
		errors.ToExecute("Check required extensions exist", func() error {
			return botanist.WaitUntilRequiredExtensionsReady(ctx)
		}),
		// We first check whether the namespace in the Seed cluster does exist - if it does not, then we assume that
		// all resources have already been deleted. We can delete the Shoot resource as a consequence.
		errors.ToExecute("Retrieve the Shoot namespace in the Seed cluster", func() error {
			return checkIfSeedNamespaceExists(ctx, o, botanist)
		}),
		// We check whether the kube-apiserver deployment exists in the shoot namespace. If it does not, then we assume
		// that it has never been deployed successfully, or that we have deleted it in a previous run because we already
		// cleaned up. We follow that no (more) resources can have been deployed in the shoot cluster, thus there is nothing
		// to delete anymore.
		errors.ToExecute("Retrieve kube-apiserver deployment in the shoot namespace in the seed cluster", func() error {
			deploymentKubeAPIServer := &appsv1.Deployment{}
			if err := botanist.SeedClientSet.APIReader().Get(ctx, kubernetesutils.Key(o.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deploymentKubeAPIServer); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
				kubeAPIServerDeploymentFound = false
			} else if deploymentKubeAPIServer.DeletionTimestamp != nil {
				kubeAPIServerDeploymentFound = false
			} else if deploymentKubeAPIServer.Spec.Replicas != nil {
				kubeAPIServerDeploymentReplicas = *deploymentKubeAPIServer.Spec.Replicas
			}

			return nil
		}),
		// We check whether the kube-controller-manager deployment exists in the shoot namespace. If it does not, then we assume
		// that it has never been deployed successfully, or that we have deleted it in a previous run because we already
		// cleaned up.
		errors.ToExecute("Retrieve the kube-controller-manager deployment in the shoot namespace in the seed cluster", func() error {
			deploymentKubeControllerManager := &appsv1.Deployment{}
			if err := botanist.SeedClientSet.APIReader().Get(ctx, kubernetesutils.Key(o.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeControllerManager), deploymentKubeControllerManager); err != nil {
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
		errors.ToExecute("Retrieve the infrastructure resource", func() error {
			if o.Shoot.IsWorkerless {
				return nil
			}
			obj, err := botanist.Shoot.Components.Extensions.Infrastructure.Get(ctx)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			infrastructure = obj
			return nil
		}),
		errors.ToExecute("Check whether control plane deployment is needed", func() error {
			controlPlaneDeploymentNeeded, err = needsControlPlaneDeployment(ctx, o, kubeAPIServerDeploymentFound, infrastructure)
			return err
		}),
	)

	if err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	var (
		nonTerminatingNamespace = botanist.SeedNamespaceObject.UID != "" && botanist.SeedNamespaceObject.Status.Phase != corev1.NamespaceTerminating
		cleanupShootResources   = nonTerminatingNamespace && kubeAPIServerDeploymentFound && (infrastructure != nil || o.Shoot.IsWorkerless)
		defaultInterval         = 5 * time.Second
		defaultTimeout          = 30 * time.Second
		staticNodesCIDR         = o.Shoot.GetInfo().Spec.Networking != nil && o.Shoot.GetInfo().Spec.Networking.Nodes != nil
		useDNS                  = botanist.ShootUsesDNS()

		g = flow.NewGraph("Shoot cluster deletion")

		ensureShootStateExists = g.Add(flow.Task{
			Name: "Ensuring that ShootState exists",
			Fn:   flow.TaskFn(botanist.EnsureShootStateExists).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		deployNamespace = g.Add(flow.Task{
			Name: "Deploying Shoot namespace in Seed",
			Fn:   flow.TaskFn(botanist.DeploySeedNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(nonTerminatingNamespace),
		})
		ensureShootClusterIdentity = g.Add(flow.Task{
			Name:         "Ensuring Shoot cluster identity",
			Fn:           flow.TaskFn(botanist.EnsureShootClusterIdentity).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})

		// We need to ensure that the deployed cloud provider secret is up-to-date. In case it has changed then we
		// need to redeploy the cloud provider config (containing the secrets for some cloud providers) as well as
		// restart the components using the secrets (cloud controller, controller manager). We also need to update all
		// existing machine class secrets.
		deployCloudProviderSecret = g.Add(flow.Task{
			Name:         "Deploying cloud provider account secret",
			Fn:           flow.TaskFn(botanist.DeployCloudProviderSecret).DoIf(nonTerminatingNamespace).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		deployKubeAPIServerService = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server service in the Seed cluster",
			Fn:           flow.TaskFn(botanist.Shoot.Components.ControlPlane.KubeAPIServerService.Deploy).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace, ensureShootClusterIdentity),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server service SNI settings in the Seed cluster",
			Fn:           flow.TaskFn(botanist.DeployKubeAPIServerSNI).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerService),
		})
		waitUntilKubeAPIServerServiceIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API LoadBalancer in the Seed cluster has reported readiness",
			Fn:           flow.TaskFn(botanist.Shoot.Components.ControlPlane.KubeAPIServerService.Wait).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerService),
		})
		_ = g.Add(flow.Task{
			Name:         "Ensuring advertised addresses for the Shoot",
			Fn:           botanist.UpdateAdvertisedAddresses,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerServiceIsReady),
		})
		initializeSecretsManagement = g.Add(flow.Task{
			Name:         "Initializing secrets management",
			Fn:           flow.TaskFn(botanist.InitializeSecretsManagement).DoIf(nonTerminatingNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace, ensureShootStateExists),
		})
		deployReferencedResources = g.Add(flow.Task{
			Name:         "Deploying referenced resources",
			Fn:           flow.TaskFn(botanist.DeployReferencedResources).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(nonTerminatingNamespace),
			Dependencies: flow.NewTaskIDs(deployNamespace, ensureShootStateExists),
		})
		deployOwnerDomainDNSRecord = g.Add(flow.Task{
			Name:         "Deploying owner domain DNS record",
			Fn:           flow.TaskFn(botanist.DeployOwnerDNSResources).DoIf(nonTerminatingNamespace),
			Dependencies: flow.NewTaskIDs(ensureShootStateExists, deployReferencedResources),
		})
		deployInternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Deploying internal domain DNS record",
			Fn:           flow.TaskFn(botanist.DeployOrDestroyInternalDNSRecord).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilKubeAPIServerServiceIsReady, deployOwnerDomainDNSRecord),
		})
		deployETCD = g.Add(flow.Task{
			Name:         "Deploying main and events etcd",
			Fn:           flow.TaskFn(botanist.DeployEtcd).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, deployOwnerDomainDNSRecord),
		})
		scaleETCD = g.Add(flow.Task{
			Name:         "Scaling up etcd main and event",
			Fn:           flow.TaskFn(botanist.ScaleUpETCD).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(deployETCD),
		})
		waitUntilEtcdReady = g.Add(flow.Task{
			Name:         "Waiting until main and event etcd report readiness",
			Fn:           flow.TaskFn(botanist.WaitUntilEtcdsReady).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(scaleETCD),
		})
		// Redeploy the control plane to make sure all components that depend on the cloud provider secret are restarted
		// in case it has changed. Also, it's needed for other control plane components like the kube-apiserver or kube-
		// controller-manager to be updateable due to provider config injection.
		deployControlPlane = g.Add(flow.Task{
			Name:         "Deploying Shoot control plane",
			Fn:           flow.TaskFn(botanist.DeployControlPlane).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources && controlPlaneDeploymentNeeded).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, ensureShootClusterIdentity),
		})
		waitUntilControlPlaneReady = g.Add(flow.Task{
			Name: "Waiting until Shoot control plane has been reconciled",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlane.Wait(ctx)
			}).DoIf(cleanupShootResources && controlPlaneDeploymentNeeded).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(deployControlPlane),
		})
		deployKubeAPIServer = g.Add(flow.Task{
			Name: "Deploying Kubernetes API server",
			Fn:   flow.TaskFn(botanist.DeployKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(
				initializeSecretsManagement,
				deployETCD,
				waitUntilEtcdReady,
				waitUntilKubeAPIServerServiceIsReady,
				waitUntilControlPlaneReady,
			).InsertIf(!staticNodesCIDR),
		})
		scaleUpKubeAPIServer = g.Add(flow.Task{
			Name: "Scaling up Kubernetes API server",
			Fn: flow.TaskFn(botanist.ScaleKubeAPIServerToOne).
				RetryUntilTimeout(defaultInterval, defaultTimeout).
				DoIf(cleanupShootResources && kubeAPIServerDeploymentReplicas == 0),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServer),
		})
		waitUntilKubeAPIServerIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server reports readiness",
			Fn:           flow.TaskFn(botanist.Shoot.Components.ControlPlane.KubeAPIServer.Wait).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServer, scaleUpKubeAPIServer),
		})
		setGardenerResourceManagerReplicas = g.Add(flow.Task{
			Name: "Setting gardener-resource-manager replicas to 2",
			Fn: flow.TaskFn(func(_ context.Context) error {
				botanist.Shoot.Components.ControlPlane.ResourceManager.SetReplicas(pointer.Int32(2))
				return nil
			}).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady),
		})
		deployGardenerResourceManager = g.Add(flow.Task{
			Name:         "Deploying gardener-resource-manager",
			Fn:           flow.TaskFn(botanist.DeployGardenerResourceManager).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(setGardenerResourceManagerReplicas),
		})
		waitUntilGardenerResourceManagerReady = g.Add(flow.Task{
			Name:         "Waiting until gardener-resource-manager reports readiness",
			Fn:           flow.TaskFn(botanist.Shoot.Components.ControlPlane.ResourceManager.Wait).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		deployGardenerAccess = g.Add(flow.Task{
			Name:         "Deploying Gardener shoot access resources",
			Fn:           flow.TaskFn(botanist.Shoot.Components.GardenerAccess.Deploy).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, waitUntilGardenerResourceManagerReady),
		})
		deployControlPlaneExposure = g.Add(flow.Task{
			Name:         "Deploying shoot control plane exposure components",
			Fn:           flow.TaskFn(botanist.DeployControlPlaneExposure).RetryUntilTimeout(defaultInterval, defaultTimeout).SkipIf(o.Shoot.IsWorkerless || useDNS).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilKubeAPIServerIsReady),
		})
		waitUntilControlPlaneExposureReady = g.Add(flow.Task{
			Name: "Waiting until Shoot control plane exposure has been reconciled",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlaneExposure.Wait(ctx)
			}).SkipIf(o.Shoot.IsWorkerless || useDNS).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(deployControlPlaneExposure),
		})
		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.TaskFn(botanist.InitializeDesiredShootClients).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, waitUntilKubeAPIServerIsReady, deployInternalDomainDNSRecord, waitUntilControlPlaneExposureReady, deployGardenerAccess),
		})

		// Redeploy the worker extensions, and kube-controller-manager to make sure all components that depend on the
		// cloud provider secret are restarted in case it has changed.
		deployKubeControllerManager = g.Add(flow.Task{
			Name:         "Deploying Kubernetes controller manager",
			Fn:           flow.TaskFn(botanist.DeployKubeControllerManager).DoIf(cleanupShootResources && kubeControllerManagerDeploymentFound).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, waitUntilControlPlaneReady, initializeShootClients),
		})
		_ = g.Add(flow.Task{
			Name:         "Scaling up Kubernetes controller manager",
			Fn:           flow.TaskFn(botanist.ScaleKubeControllerManagerToOne).DoIf(cleanupShootResources && kubeControllerManagerDeploymentFound),
			Dependencies: flow.NewTaskIDs(deployKubeControllerManager),
		})
		deleteSeedMonitoring = g.Add(flow.Task{
			Name:         "Deleting shoot monitoring stack in Seed",
			Fn:           flow.TaskFn(botanist.DeleteSeedMonitoring).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		deleteClusterAutoscaler = g.Add(flow.Task{
			Name: "Deleting cluster autoscaler",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.ControlPlane.ClusterAutoscaler.Destroy(ctx)
			}).SkipIf(o.Shoot.IsWorkerless).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})

		cleanupWebhooks = g.Add(flow.Task{
			Name:         "Cleaning up webhooks",
			Fn:           flow.TaskFn(botanist.CleanWebhooks).Timeout(10 * time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deployGardenerResourceManager),
		})
		waitForControllersToBeActive = g.Add(flow.Task{
			Name:         "Waiting until kube-controller-manager is active",
			Fn:           flow.TaskFn(botanist.WaitForKubeControllerManagerToBeActive).DoIf(cleanupShootResources && kubeControllerManagerDeploymentFound).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients, cleanupWebhooks, deployControlPlane, deployKubeControllerManager),
		})
		cleanExtendedAPIs = g.Add(flow.Task{
			Name:         "Cleaning extended API groups",
			Fn:           flow.TaskFn(botanist.CleanExtendedAPIs).Timeout(10 * time.Minute).DoIf(cleanupShootResources && !metav1.HasAnnotation(o.Shoot.GetInfo().ObjectMeta, v1beta1constants.AnnotationShootSkipCleanup)),
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
		deleteMetricsServer = g.Add(flow.Task{
			Name: "Deleting metrics-server",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.SystemComponents.MetricsServer.Destroy(ctx)
			}).DoIf(cleanupShootResources).SkipIf(o.Shoot.IsWorkerless).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointReadyForCleanup),
		})
		syncPointCleanedKubernetesResources = flow.NewTaskIDs(
			cleanupWebhooks,
			cleanExtendedAPIs,
			cleanKubernetesResources,
			deleteMetricsServer,
		)

		destroyNetwork = g.Add(flow.Task{
			Name: "Destroying shoot network plugin",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Network.Destroy(ctx)
			}).SkipIf(o.Shoot.IsWorkerless).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointCleanedKubernetesResources),
		})
		waitUntilNetworkIsDestroyed = g.Add(flow.Task{
			Name: "Waiting until shoot network plugin has been destroyed",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Network.WaitCleanup(ctx)
			}).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(destroyNetwork),
		})
		deployMachineControllerManager = g.Add(flow.Task{
			Name:         "Deploying machine-controller-manager",
			Fn:           flow.TaskFn(botanist.DeployMachineControllerManager).SkipIf(o.Shoot.IsWorkerless).DoIf(features.DefaultFeatureGate.Enabled(features.MachineControllerManagerDeployment) && nonTerminatingNamespace),
			Dependencies: flow.NewTaskIDs(syncPointCleanedKubernetesResources),
		})
		destroyWorker = g.Add(flow.Task{
			Name: "Destroying shoot workers",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Worker.Destroy(ctx)
			}).SkipIf(o.Shoot.IsWorkerless).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployMachineControllerManager),
		})
		waitUntilWorkerDeleted = g.Add(flow.Task{
			Name: "Waiting until shoot worker nodes have been terminated",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Worker.WaitCleanup(ctx)
			}).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(destroyWorker),
		})
		_ = g.Add(flow.Task{
			Name: "Deleting machine-controller-manager",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.ControlPlane.MachineControllerManager.Destroy(ctx)
			}).SkipIf(o.Shoot.IsWorkerless).DoIf(features.DefaultFeatureGate.Enabled(features.MachineControllerManagerDeployment)),
			Dependencies: flow.NewTaskIDs(waitUntilWorkerDeleted),
		})
		deleteAllOperatingSystemConfigs = g.Add(flow.Task{
			Name: "Deleting operating system config resources",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.OperatingSystemConfig.Destroy(ctx)
			}).SkipIf(o.Shoot.IsWorkerless).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilWorkerDeleted),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until all operating system config resources are deleted",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.OperatingSystemConfig.WaitCleanup(ctx)
			}).SkipIf(o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deleteAllOperatingSystemConfigs),
		})
		deleteManagedResources = g.Add(flow.Task{
			Name:         "Deleting managed resources",
			Fn:           flow.TaskFn(botanist.DeleteManagedResources).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointCleanedKubernetesResources, waitUntilWorkerDeleted),
		})
		waitUntilManagedResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until managed resources have been deleted",
			Fn:           flow.TaskFn(botanist.WaitUntilManagedResourcesDeleted).DoIf(cleanupShootResources).Timeout(10 * time.Minute),
			Dependencies: flow.NewTaskIDs(deleteManagedResources),
		})
		deleteExtensionResourcesBeforeKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting extension resources before kube-apiserver",
			Fn:           flow.TaskFn(botanist.Shoot.Components.Extensions.Extension.DestroyBeforeKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, waitUntilManagedResourcesDeleted),
		})
		waitUntilExtensionResourcesBeforeKubeAPIServerDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources that should be handled before kube-apiserver have been deleted",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitCleanupBeforeKubeAPIServer,
			Dependencies: flow.NewTaskIDs(deleteExtensionResourcesBeforeKubeAPIServer),
		})
		deleteStaleExtensionResources = g.Add(flow.Task{
			Name:         "Deleting stale extension resources",
			Fn:           flow.TaskFn(botanist.Shoot.Components.Extensions.Extension.DeleteStaleResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, waitUntilManagedResourcesDeleted),
		})
		waitUntilStaleExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until all stale extension resources have been deleted",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitCleanupStaleResources,
			Dependencies: flow.NewTaskIDs(deleteStaleExtensionResources),
		})
		deleteContainerRuntimeResources = g.Add(flow.Task{
			Name: "Deleting container runtime resources",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ContainerRuntime.Destroy(ctx)
			}).SkipIf(o.Shoot.IsWorkerless).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients, syncPointCleanedKubernetesResources),
		})
		waitUntilContainerRuntimeResourcesDeleted = g.Add(flow.Task{
			Name: "Waiting until stale container runtime resources are deleted",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ContainerRuntime.WaitCleanup(ctx)
			}).SkipIf(o.Shoot.IsWorkerless),
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
				waitFor := 5 * time.Minute

				if v, ok := botanist.Shoot.GetInfo().Annotations[v1beta1constants.AnnotationShootInfrastructureCleanupWaitPeriodSeconds]; ok {
					seconds, err := strconv.Atoi(v)
					if err != nil {
						return err
					}

					if newWaitFor := time.Duration(seconds) * time.Second; newWaitFor < waitFor {
						waitFor = newWaitFor
					}
				}

				ctx, cancel := context.WithTimeout(ctx, waitFor)
				defer cancel()

				<-ctx.Done()
				return nil
			}).DoIf(cleanupShootResources).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(deleteManagedResources),
		})

		syncPointCleaned = flow.NewTaskIDs(
			syncPointCleanedKubernetesResources,
			deleteAllOperatingSystemConfigs,
			waitUntilWorkerDeleted,
			waitUntilManagedResourcesDeleted,
			timeForInfrastructureResourceCleanup,
			destroyNetwork,
			waitUntilNetworkIsDestroyed,
			waitUntilExtensionResourcesBeforeKubeAPIServerDeleted,
			waitUntilStaleExtensionResourcesDeleted,
			waitUntilContainerRuntimeResourcesDeleted,
		)
		destroyControlPlane = g.Add(flow.Task{
			Name: "Destroying shoot control plane",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlane.Destroy(ctx)
			}).SkipIf(o.Shoot.IsWorkerless).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointCleaned),
		})
		waitUntilControlPlaneDeleted = g.Add(flow.Task{
			Name: "Waiting until shoot control plane has been destroyed",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlane.WaitCleanup(ctx)
			}).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(destroyControlPlane),
		})

		waitUntilShootManagedResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot managed resources have been deleted",
			Fn:           flow.TaskFn(botanist.WaitUntilShootManagedResourcesDeleted).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneDeleted),
		})
		deleteKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting Kubernetes API server",
			Fn:           flow.TaskFn(botanist.DeleteKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointCleaned, waitUntilControlPlaneDeleted, waitUntilShootManagedResourcesDeleted),
		})
		waitUntilKubeAPIServerDeleted = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server has been deleted",
			Fn:           botanist.Shoot.Components.ControlPlane.KubeAPIServer.WaitCleanup,
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		deleteExtensionResourcesAfterKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting extension resources after kube-apiserver",
			Fn:           flow.TaskFn(botanist.Shoot.Components.Extensions.Extension.DestroyAfterKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		waitUntilExtensionResourcesAfterKubeAPIServerDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources that should be handled after kube-apiserver have been deleted",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitCleanupAfterKubeAPIServer,
			Dependencies: flow.NewTaskIDs(deleteExtensionResourcesAfterKubeAPIServer),
		})
		// Add this step in interest of completeness. All extension deletions should have already been triggered by previous steps.
		waitUntilExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until all extension resources have been deleted",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitCleanup,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		destroyKubeAPIServerSNI = g.Add(flow.Task{
			Name:         "Destroying Kubernetes API server service SNI",
			Fn:           flow.TaskFn(botanist.Shoot.Components.ControlPlane.KubeAPIServerSNI.Destroy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying Kubernetes API server ingress with trusted certificate",
			Fn:           flow.TaskFn(botanist.Shoot.Components.ControlPlane.KubeAPIServerIngress.Destroy),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying Kubernetes API server service",
			Fn:           flow.TaskFn(botanist.Shoot.Components.ControlPlane.KubeAPIServerService.Destroy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted, destroyKubeAPIServerSNI),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying gardener-resource-manager",
			Fn:           flow.TaskFn(botanist.Shoot.Components.ControlPlane.ResourceManager.Destroy),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})

		destroyControlPlaneExposure = g.Add(flow.Task{
			Name: "Destroying shoot control plane exposure",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlaneExposure.Destroy(ctx)
			}).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		waitUntilControlPlaneExposureDeleted = g.Add(flow.Task{
			Name: "Waiting until shoot control plane exposure has been destroyed",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlaneExposure.WaitCleanup(ctx)
			}).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(destroyControlPlaneExposure),
		})

		destroyIngressDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying nginx ingress DNS record",
			Fn:           flow.TaskFn(botanist.DestroyIngressDNSRecord).DoIf(nonTerminatingNamespace).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(syncPointCleaned),
		})
		deleteInfrastructure = g.Add(flow.Task{
			Name: "Destroying shoot infrastructure",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Infrastructure.Destroy(ctx)
			}).SkipIf(o.Shoot.IsWorkerless).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointCleaned, waitUntilControlPlaneDeleted),
		})
		waitUntilInfrastructureDeleted = g.Add(flow.Task{
			Name: "Waiting until shoot infrastructure has been deleted",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Infrastructure.WaitCleanup(ctx)
			}).SkipIf(o.Shoot.IsWorkerless),
			Dependencies: flow.NewTaskIDs(deleteInfrastructure),
		})
		destroyExternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying external domain DNS record",
			Fn:           flow.TaskFn(botanist.DestroyExternalDNSRecord).DoIf(nonTerminatingNamespace),
			Dependencies: flow.NewTaskIDs(syncPointCleaned, waitUntilKubeAPIServerDeleted),
		})
		deletePlutono = g.Add(flow.Task{
			Name:         "Deleting Plutono in Seed",
			Fn:           flow.TaskFn(botanist.DeletePlutono).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilInfrastructureDeleted),
		})

		syncPoint = flow.NewTaskIDs(
			deleteSeedMonitoring,
			deletePlutono,
			waitUntilKubeAPIServerDeleted,
			waitUntilControlPlaneDeleted,
			waitUntilControlPlaneExposureDeleted,
			waitUntilExtensionResourcesAfterKubeAPIServerDeleted,
			waitUntilExtensionResourcesDeleted,
			destroyIngressDomainDNSRecord,
			destroyExternalDomainDNSRecord,
			waitUntilInfrastructureDeleted,
		)

		destroyInternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying internal domain DNS record",
			Fn:           flow.TaskFn(botanist.DestroyInternalDNSRecord).DoIf(nonTerminatingNamespace),
			Dependencies: flow.NewTaskIDs(syncPoint),
		})
		destroyOwnerDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying owner domain DNS record",
			Fn:           flow.TaskFn(botanist.DestroyOwnerDNSResources).DoIf(nonTerminatingNamespace),
			Dependencies: flow.NewTaskIDs(syncPoint),
		})
		destroyReferencedResources = g.Add(flow.Task{
			Name:         "Deleting referenced resources",
			Fn:           flow.TaskFn(botanist.DestroyReferencedResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPoint),
		})
		destroyEtcd = g.Add(flow.Task{
			Name:         "Destroying main and events etcd",
			Fn:           flow.TaskFn(botanist.DestroyEtcd).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPoint),
		})
		waitUntilEtcdDeleted = g.Add(flow.Task{
			Name:         "Waiting until main and event etcd have been destroyed",
			Fn:           flow.TaskFn(botanist.WaitUntilEtcdsDeleted).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPoint, destroyEtcd),
		})
		deleteNamespace = g.Add(flow.Task{
			Name:         "Deleting shoot namespace in Seed",
			Fn:           flow.TaskFn(botanist.DeleteSeedNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPoint, destroyInternalDomainDNSRecord, destroyOwnerDomainDNSRecord, destroyReferencedResources, waitUntilEtcdDeleted),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until shoot namespace in Seed has been deleted",
			Fn:           botanist.WaitUntilSeedNamespaceDeleted,
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})
		_ = g.Add(flow.Task{
			Name: "Deleting Shoot State",
			Fn: func(ctx context.Context) error {
				return shootstate.Delete(ctx, botanist.GardenClient, botanist.Shoot.GetInfo())
			},
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})
		f = g.Compile()
	)

	if err := f.Run(ctx, flow.Opts{
		Log:              o.Logger,
		ProgressReporter: r.newProgressReporter(o.ReportShootProgress),
		ErrorCleaner:     o.CleanShootTaskError,
		ErrorContext:     errorContext,
	}); err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), flow.Errors(err))
	}

	// ensure that shoot client is invalidated after it has been deleted
	if err := o.ShootClientMap.InvalidateClient(keys.ForShoot(o.Shoot.GetInfo())); err != nil {
		err = fmt.Errorf("failed to invalidate shoot client: %w", err)
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	o.Logger.Info("Successfully deleted Shoot cluster")
	return nil
}
