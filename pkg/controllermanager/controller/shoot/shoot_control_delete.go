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
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

// deleteShoot deletes a Shoot cluster entirely.
// It receives a Garden object <garden> which stores the Shoot object.
func (c *defaultControl) deleteShoot(o *operation.Operation) *gardencorev1alpha1.LastError {
	// If the .status.uid field is empty, then we assume that there has never been any operation running for this Shoot
	// cluster. This implies that there can not be any resource which we have to delete. We accept the deletion.
	if len(o.Shoot.Info.Status.UID) == 0 {
		o.Logger.Info("`.status.uid` is empty, assuming Shoot cluster did never exist. Deletion accepted.")
		return nil
	}

	// We create botanists (which will do the actual work).
	var botanist *botanistpkg.Botanist
	if err := utils.Retry(10*time.Second, 10*time.Minute, func() (ok, severe bool, err error) {
		botanist, err = botanistpkg.New(o)
		if err != nil {
			return false, false, err
		}
		return true, false, nil
	}); err != nil {
		return formatError("Failed to create a Botanist", err)
	}

	if err := botanist.RequiredExtensionsExist(o.Shoot.Info); err != nil {
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

	var (
		nonTerminatingNamespace = namespace.Status.Phase != corev1.NamespaceTerminating
		cleanupShootResources   = nonTerminatingNamespace && kubeAPIServerDeploymentFound
		defaultInterval         = 5 * time.Second
		defaultTimeout          = 30 * time.Second
		isCloud                 = o.Shoot.Info.Spec.Cloud.Local == nil

		g = flow.NewGraph("Shoot cluster deletion")

		// We need to ensure that the deployed cloud provider secret is up-to-date. In case it has changed then we
		// need to redeploy the cloud provider config (containing the secrets for some cloud providers) as well as
		// restart the components using the secrets (cloud controller, controller manager). We also need to update all
		// existing machine class secrets.
		deployCloudProviderSecret = g.Add(flow.Task{
			Name: "Deploying cloud provider account secret",
			Fn:   flow.SimpleTaskFn(botanist.DeployCloudProviderSecret).DoIf(cleanupShootResources),
		})
		refreshMachineClassSecrets = g.Add(flow.Task{
			Name:         "Refreshing machine class secrets",
			Fn:           flow.SimpleTaskFn(hybridBotanist.RefreshMachineClassSecrets).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret),
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
		wakeUpControlPlane = g.Add(flow.Task{
			Name: "Waking up control plane to ensure proper cleanup of resources",
			Fn:   flow.TaskFn(botanist.WakeUpControlPlane).DoIf(o.Shoot.IsHibernated && cleanupShootResources),
		})

		// Deletion of monitoring stack (to avoid false positive alerts) and kube-addon-manager (to avoid redeploying
		// resources).
		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.SimpleTaskFn(botanist.InitializeShootClients).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, refreshMachineClassSecrets, refreshCloudProviderConfig, refreshCloudControllerManager, refreshKubeControllerManager, wakeUpControlPlane),
		})
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
		deleteClusterAutoscaler = g.Add(flow.Task{
			Name:         "Deleting cluster autoscaler",
			Fn:           flow.SimpleTaskFn(botanist.DeleteClusterAutoscaler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		waitUntilKubeAddonManagerDeleted = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes addon manager has been deleted",
			Fn:           flow.SimpleTaskFn(botanist.WaitUntilKubeAddonManagerDeleted),
			Dependencies: flow.NewTaskIDs(deleteKubeAddonManager),
		})

		cleanupWebhooks = g.Add(flow.Task{
			Name:         "Cleaning up non-gardener webhooks",
			Fn:           flow.TaskFn(botanist.CleanWebhooks).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(refreshKubeControllerManager, refreshCloudControllerManager, wakeUpControlPlane, waitUntilKubeAddonManagerDeleted),
		})
		waitForControllersToBeActive = g.Add(flow.Task{
			Name:         "Waiting until both cloud-controller-manager and kube-controller-manager are active",
			Fn:           flow.SimpleTaskFn(botanist.WaitForControllersToBeActive).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(cleanupWebhooks),
		})

		// We need to clean up the cluster resources which may have footprints in the infrastructure (such as
		// LoadBalancers, volumes, ...). We do that by deleting all namespaces other than the three standard
		// namespaces which cannot be deleted (kube-system, default, kube-public). In those three namespaces
		// we delete all CRDs, workload, services and PVCs.
		// If the deletion of a CRD or an API service fails, then it is force deleted.
		cleanCustomResourceDefinitions = g.Add(flow.Task{
			Name: "Cleaning custom resource definitions",
			Fn: flow.SimpleTaskFn(botanist.CleanCustomResourceDefinitions).RetryUntilTimeout(defaultInterval, 5*time.Minute).DoIf(cleanupShootResources).
				RecoverTimeout(flow.SimpleTaskFn(botanist.ForceDeleteCustomResourceDefinitions).RetryUntilTimeout(defaultInterval, 2*time.Minute).ToRecoverFn()),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAddonManagerDeleted, deleteClusterAutoscaler, waitForControllersToBeActive),
		})
		cleanCustomAPIServices = g.Add(flow.Task{
			Name: "Cleaning custom API service definitions",
			Fn: flow.SimpleTaskFn(botanist.CleanupCustomAPIServices).RetryUntilTimeout(defaultInterval, 5*time.Minute).DoIf(cleanupShootResources).
				RecoverTimeout(flow.SimpleTaskFn(botanist.ForceDeleteCustomAPIServices).RetryUntilTimeout(defaultInterval, 2*time.Minute).ToRecoverFn()),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAddonManagerDeleted, deleteClusterAutoscaler, waitForControllersToBeActive),
		})
		cleanKubernetesResources = g.Add(flow.Task{
			Name:         "Cleaning kubernetes resources",
			Fn:           flow.SimpleTaskFn(botanist.CleanKubernetesResources).RetryUntilTimeout(defaultInterval, 5*time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(cleanCustomResourceDefinitions, cleanCustomAPIServices),
		})
		destroyMachines = g.Add(flow.Task{
			Name:         "Destroying Shoot workers",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DestroyMachines).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(isCloud),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		deleteKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting Kubernetes API server",
			Fn:           flow.SimpleTaskFn(botanist.DeleteKubeAPIServer).Retry(defaultInterval),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, destroyMachines),
		})

		destroyNginxIngressResources = g.Add(flow.Task{
			Name:         "Destroying ingress DNS record",
			Fn:           flow.SimpleTaskFn(botanist.DestroyIngressDNSRecord),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		destroyKube2IAMResources = g.Add(flow.Task{
			Name:         "Destroying Kube2IAM resources",
			Fn:           flow.SimpleTaskFn(shootCloudBotanist.DestroyKube2IAMResources),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		destroyInfrastructure = g.Add(flow.Task{
			Name:         "Destroying Shoot infrastructure",
			Fn:           flow.SimpleTaskFn(shootCloudBotanist.DestroyInfrastructure),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, destroyMachines),
		})
		destroyExternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying external domain DNS record",
			Fn:           flow.SimpleTaskFn(botanist.DestroyExternalDomainDNSRecord),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		syncPoint = flow.NewTaskIDs(
			deleteSeedMonitoring,
			deleteKubeAPIServer,
			destroyNginxIngressResources,
			destroyKube2IAMResources,
			destroyInfrastructure,
			destroyExternalDomainDNSRecord,
		)

		deleteBackupInfrastructure = g.Add(flow.Task{
			Name:         "Deleting backup infrastructure",
			Fn:           flow.SimpleTaskFn(botanist.DeleteBackupInfrastructure),
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		destroyInternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying internal domain DNS record",
			Fn:           flow.SimpleTaskFn(botanist.DestroyInternalDomainDNSRecord),
			Dependencies: flow.NewTaskIDs(syncPoint),
		})
		deleteNamespace = g.Add(flow.Task{
			Name:         "Deleting Shoot namespace in Seed",
			Fn:           flow.SimpleTaskFn(botanist.DeleteNamespace).Retry(defaultInterval),
			Dependencies: flow.NewTaskIDs(syncPoint, destroyInternalDomainDNSRecord, deleteBackupInfrastructure, deleteKubeAPIServer),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until Shoot namespace in Seed has been deleted",
			Fn:           flow.SimpleTaskFn(botanist.WaitUntilSeedNamespaceDeleted),
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
	return wait.PollImmediate(time.Second, 30*time.Second, func() (bool, error) {
		shoot, err := c.k8sGardenInformers.Shoots().Lister().Shoots(o.Shoot.Info.Namespace).Get(o.Shoot.Info.Name)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		lastOperation := shoot.Status.LastOperation
		if !sets.NewString(shoot.Finalizers...).Has(gardenv1beta1.GardenerName) && lastOperation != nil && lastOperation.Type == gardencorev1alpha1.LastOperationTypeDelete && lastOperation.State == gardencorev1alpha1.LastOperationStateSucceeded {
			return true, nil
		}
		return false, nil
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
