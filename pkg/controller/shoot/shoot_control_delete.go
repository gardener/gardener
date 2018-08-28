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
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	cloudbotanistpkg "github.com/gardener/gardener/pkg/operation/cloudbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	hybridbotanistpkg "github.com/gardener/gardener/pkg/operation/hybridbotanist"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// deleteShoot deletes a Shoot cluster entirely.
// It receives a Garden object <garden> which stores the Shoot object.
func (c *defaultControl) deleteShoot(o *operation.Operation) *gardenv1beta1.LastError {
	// If the .status.uid field is empty, then we assume that there has never been any operation running for this Shoot
	// cluster. This implies that there can not be any resource which we have to delete. We accept the deletion.
	if len(o.Shoot.Info.Status.UID) == 0 {
		o.Logger.Info("`.status.uid` is empty, assuming Shoot cluster did never exist. Deletion accepted.")
		return nil
	}

	// We create botanists (which will do the actual work).
	botanist, err := botanistpkg.New(o)
	if err != nil {
		return formatError("Failed to create a Botanist", err)
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

	// We check whether the Shoot namespace in the Seed cluster is already in a terminating state, i.e. whether
	// we have tried to delete it in a previous run. In that case, we do not need to cleanup Shoot resource because
	// that would have already been done.
	// We also check whether the kube-apiserver pod exists in the Shoot namespace within the Seed cluster. If it does not,
	// then we assume that it has never been deployed successfully. We follow that no resources can have been deployed
	// at all in the Shoot cluster, thus there is nothing to delete at all.
	kubeAPIServerFound := false
	podList, err := botanist.K8sSeedClient.ListPods(o.Shoot.SeedNamespace, metav1.ListOptions{
		LabelSelector: "app=kubernetes,role=apiserver",
	})
	if err != nil {
		return formatError("Failed to retrieve the list of pods running in the Shoot namespace in the Seed cluster", err)
	}
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp == nil {
			kubeAPIServerFound = true
			break
		}
	}

	var (
		nonTerminatingNamespace = namespace.Status.Phase != corev1.NamespaceTerminating
		cleanupShootResources   = nonTerminatingNamespace && kubeAPIServerFound
		defaultInterval         = 5 * time.Second
		defaultTimeout          = 30 * time.Second
		isCloud                 = o.Shoot.Info.Spec.Cloud.Local == nil

		g = flow.NewGraph("Shoot cluster deletion")

		// We need to ensure that the deployed cloud provider secret is up-to-date. In case it has changed then we
		// need to redeploy the cloud provider config (containing the secrets for some cloud providers) as well as
		// restart the components using the secrets (API server, controller manager). We also need to update all
		// existing machine class secrets.
		deploySecrets = g.Add(flow.Task{
			Name: "Deploying Shoot certificates / keys",
			Fn:   flow.TaskFn(botanist.DeploySecrets).DoIf(cleanupShootResources),
		})
		refreshMachineClassSecrets = g.Add(flow.Task{
			Name:         "Refreshing machine class secrets",
			Fn:           flow.TaskFn(hybridBotanist.RefreshMachineClassSecrets).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets),
		})
		refreshCloudProviderConfig = g.Add(flow.Task{
			Name:         "Refreshing cloud provider configuration",
			Fn:           flow.TaskFn(hybridBotanist.RefreshCloudProviderConfig).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets),
		})
		refreshCloudControllerManager = g.Add(flow.Task{
			Name:         "Refreshing cloud controller manager checksums",
			Fn:           flow.TaskFn(botanist.RefreshCloudControllerManagerChecksums).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, refreshCloudProviderConfig),
		})
		refreshKubeControllerManager = g.Add(flow.Task{
			Name:         "Refreshing Kube controller manager checksums",
			Fn:           flow.TaskFn(botanist.RefreshKubeControllerManagerChecksums).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, refreshCloudProviderConfig),
		})

		// Deletion of monitoring stack (to avoid false positive alerts) and kube-addon-manager (to avoid redeploying
		// resources).
		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.TaskFn(botanist.InitializeShootClients).DoIf(cleanupShootResources).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(deploySecrets, refreshMachineClassSecrets, refreshCloudProviderConfig, refreshCloudControllerManager, refreshKubeControllerManager),
		})
		deleteSeedMonitoring = g.Add(flow.Task{
			Name:         "Deleting Shoot monitoring stack in Seed",
			Fn:           flow.TaskFn(botanist.DeleteSeedMonitoring).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		deleteKubeAddonManager = g.Add(flow.Task{
			Name:         "Deleting Kube addon manager",
			Fn:           flow.TaskFn(botanist.DeleteKubeAddonManager).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		deleteClusterAutoscaler = g.Add(flow.Task{
			Name:         "Deleting cluster autoscaler",
			Fn:           flow.TaskFn(botanist.DeleteClusterAutoscaler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		waitUntilKubeAddonManagerDeleted = g.Add(flow.Task{
			Name:         "Waiting until Kube addon manager has been deleted",
			Fn:           botanist.WaitUntilKubeAddonManagerDeleted,
			Dependencies: flow.NewTaskIDs(deleteKubeAddonManager),
		})

		// We need to clean up the cluster resources which may have footprints in the infrastructure (such as
		// LoadBalancers, volumes, ...). We do that by deleting all namespaces other than the three standard
		// namespaces which cannot be deleted (kube-system, default, kube-public). In those three namespaces
		// we delete all CRDs, workload, services and PVCs. Only if none of those resources remain, we go
		// ahead and trigger the infrastructure deletion.
		cleanCustomResourceDefinitions = g.Add(flow.Task{
			Name:         "Cleaning custom resource definitions",
			Fn:           flow.TaskFn(botanist.CleanCustomResourceDefinitions).RetryUntilTimeout(defaultInterval, 5*time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAddonManagerDeleted, deleteClusterAutoscaler),
		})
		cleanKubernetesResources = g.Add(flow.Task{
			Name:         "Cleaning kubernetes resources",
			Fn:           flow.TaskFn(botanist.CleanKubernetesResources).RetryUntilTimeout(defaultInterval, 5*time.Minute).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(cleanCustomResourceDefinitions),
		})
		destroyMachines = g.Add(flow.Task{
			Name:         "Destroying Shoot workers",
			Fn:           flow.TaskFn(hybridBotanist.DestroyMachines).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(isCloud),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		destroyNginxIngressResources = g.Add(flow.Task{
			Name:         "Destroying ingress DNS record",
			Fn:           flow.TaskFn(botanist.DestroyIngressDNSRecord),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		destroyKube2IAMResources = g.Add(flow.Task{
			Name:         "Destroying Kube2IAM resources",
			Fn:           flow.TaskFn(shootCloudBotanist.DestroyKube2IAMResources),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})
		destroyInfrastructure = g.Add(flow.Task{
			Name:         "Destroying Shoot infrastructure",
			Fn:           flow.TaskFn(shootCloudBotanist.DestroyInfrastructure),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources, destroyMachines),
		})
		destroyExternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying external domain DNS record",
			Fn:           flow.TaskFn(botanist.DestroyExternalDomainDNSRecord),
			Dependencies: flow.NewTaskIDs(cleanKubernetesResources),
		})

		syncPointTerraformers = flow.NewTaskIDs(
			deleteSeedMonitoring,
			destroyNginxIngressResources,
			destroyKube2IAMResources,
			destroyInfrastructure,
			destroyExternalDomainDNSRecord,
		)

		deleteKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting Kube API server",
			Fn:           flow.TaskFn(botanist.DeleteKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointTerraformers),
		})
		deleteBackupInfrastructure = g.Add(flow.Task{
			Name:         "Deleting backup infrastructure",
			Fn:           flow.TaskFn(botanist.DeleteBackupInfrastructure),
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		destroyInternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Destroying internal domain DNS record",
			Fn:           flow.TaskFn(botanist.DestroyInternalDomainDNSRecord),
			Dependencies: flow.NewTaskIDs(syncPointTerraformers),
		})
		deleteNamespace = g.Add(flow.Task{
			Name:         "Deleteing Shoot namespace in Seed",
			Fn:           flow.TaskFn(botanist.DeleteNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPointTerraformers, destroyInternalDomainDNSRecord, deleteBackupInfrastructure, deleteKubeAPIServer),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until Shoot namespace in Seed has been deleted",
			Fn:           flow.TaskFn(botanist.WaitUntilSeedNamespaceDeleted),
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})
		_ = g.Add(flow.Task{
			Name:         "Deleting Garden secrets",
			Fn:           flow.TaskFn(botanist.DeleteGardenSecrets).RetryUntilTimeout(defaultInterval, defaultTimeout),
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

		return &gardenv1beta1.LastError{
			Codes:       helper.ExtractErrorCodes(flow.Causes(err)),
			Description: helper.FormatLastErrDescription(err),
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

	if status.RetryCycleStartTime == nil || (status.LastOperation != nil && status.LastOperation.Type != gardenv1beta1.ShootLastOperationTypeDelete) {
		o.Shoot.Info.Status.RetryCycleStartTime = &now
	}
	if len(status.TechnicalID) == 0 {
		o.Shoot.Info.Status.TechnicalID = o.Shoot.SeedNamespace
	}

	o.Shoot.Info.Status.Gardener = *o.GardenerInfo
	o.Shoot.Info.Status.Conditions = nil
	o.Shoot.Info.Status.ObservedGeneration = o.Shoot.Info.Generation
	o.Shoot.Info.Status.LastOperation = &gardenv1beta1.LastOperation{
		Type:           gardenv1beta1.ShootLastOperationTypeDelete,
		State:          gardenv1beta1.ShootLastOperationStateProcessing,
		Progress:       1,
		Description:    "Deletion of Shoot cluster in progress.",
		LastUpdateTime: metav1.Now(),
	}

	newShoot, err := c.updater.UpdateShootStatus(o.Shoot.Info)
	if err == nil {
		o.Shoot.Info = newShoot
	}
	return err
}

func (c *defaultControl) updateShootStatusDeleteSuccess(o *operation.Operation) error {
	o.Shoot.Info.Status.RetryCycleStartTime = nil
	o.Shoot.Info.Status.LastError = nil
	o.Shoot.Info.Status.LastOperation = &gardenv1beta1.LastOperation{
		Type:           gardenv1beta1.ShootLastOperationTypeDelete,
		State:          gardenv1beta1.ShootLastOperationStateSucceeded,
		Progress:       100,
		Description:    "Shoot cluster has been successfully deleted.",
		LastUpdateTime: metav1.Now(),
	}

	newShoot, err := c.updater.UpdateShootStatus(o.Shoot.Info)
	if err != nil {
		return err
	}
	o.Shoot.Info = newShoot

	// Remove finalizer
	finalizers := sets.NewString(o.Shoot.Info.Finalizers...)
	finalizers.Delete(gardenv1beta1.GardenerName)
	o.Shoot.Info.Finalizers = finalizers.List()
	newShoot, err = c.k8sGardenClient.GardenClientset().GardenV1beta1().Shoots(o.Shoot.Info.Namespace).Update(o.Shoot.Info)
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
		if !sets.NewString(shoot.Finalizers...).Has(gardenv1beta1.GardenerName) && lastOperation != nil && lastOperation.Type == gardenv1beta1.ShootLastOperationTypeDelete && lastOperation.State == gardenv1beta1.ShootLastOperationStateSucceeded {
			return true, nil
		}
		return false, nil
	})
}

func (c *defaultControl) updateShootStatusDeleteError(o *operation.Operation, lastError *gardenv1beta1.LastError) (gardenv1beta1.ShootLastOperationState, error) {
	var (
		state       = gardenv1beta1.ShootLastOperationStateFailed
		description = lastError.Description
	)

	if !utils.TimeElapsed(o.Shoot.Info.Status.RetryCycleStartTime, c.config.Controllers.Shoot.RetryDuration.Duration) {
		description += " Operation will be retried."
		state = gardenv1beta1.ShootLastOperationStateError
	} else {
		o.Shoot.Info.Status.RetryCycleStartTime = nil
	}

	o.Shoot.Info.Status.Gardener = *o.GardenerInfo
	o.Shoot.Info.Status.LastError = lastError
	o.Shoot.Info.Status.LastOperation.Type = gardenv1beta1.ShootLastOperationTypeDelete
	o.Shoot.Info.Status.LastOperation.State = state
	o.Shoot.Info.Status.LastOperation.Description = description
	o.Shoot.Info.Status.LastOperation.LastUpdateTime = metav1.Now()

	o.Logger.Error(description)

	newShoot, err := c.updater.UpdateShootStatus(o.Shoot.Info)
	if err == nil {
		o.Shoot.Info = newShoot
	}

	newShootAfterLabel, err := c.updater.UpdateShootLabels(o.Shoot.Info, computeLabelsWithShootHealthiness(false))
	if err == nil {
		o.Shoot.Info = newShootAfterLabel
	}
	return state, err
}
