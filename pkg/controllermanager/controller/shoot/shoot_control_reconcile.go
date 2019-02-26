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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	cloudbotanistpkg "github.com/gardener/gardener/pkg/operation/cloudbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	hybridbotanistpkg "github.com/gardener/gardener/pkg/operation/hybridbotanist"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// reconcileShoot reconciles the Shoot cluster's state.
// It receives a Garden object <garden> which stores the Shoot object and the operation type.
func (c *defaultControl) reconcileShoot(o *operation.Operation, operationType gardencorev1alpha1.LastOperationType) *gardencorev1alpha1.LastError {
	// We create the botanists (which will do the actual work).
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

	if err := botanist.RequiredExtensionsExist(o.Shoot.Info); err != nil {
		return formatError("Failed to check whether all required extensions exist", err)
	}

	var (
		defaultTimeout                  = 30 * time.Second
		defaultInterval                 = 5 * time.Second
		managedDNS                      = o.Shoot.Info.Spec.DNS.Provider != gardenv1beta1.DNSUnmanaged
		isCloud                         = o.Shoot.Info.Spec.Cloud.Local == nil
		creationPhase                   = operationType == gardencorev1alpha1.LastOperationTypeCreate
		requireInfrastructureDeployment = creationPhase || controllerutils.HasTask(o.Shoot.Info.Annotations, common.ShootTaskDeployInfrastructure)
		requireKube2IAMDeployment       = creationPhase || controllerutils.HasTask(o.Shoot.Info.Annotations, common.ShootTaskDeployKube2IAMResource)

		g               = flow.NewGraph("Shoot cluster reconciliation")
		deployNamespace = g.Add(flow.Task{
			Name: "Deploying Shoot namespace in Seed",
			Fn:   flow.SimpleTaskFn(botanist.DeployNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying cloud metadata service network policy",
			Fn:           flow.SimpleTaskFn(botanist.DeployCloudMetadataServiceNetworkPolicy).DoIf(isCloud).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		deployCloudProviderSecret = g.Add(flow.Task{
			Name:         "Deploying cloud provider account secret",
			Fn:           flow.SimpleTaskFn(botanist.DeployCloudProviderSecret).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		deployKubeAPIServerService = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server service",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DeployKubeAPIServerService).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		waitUntilKubeAPIServerServiceIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server service has reported readiness",
			Fn:           flow.SimpleTaskFn(botanist.WaitUntilKubeAPIServerServiceIsReady).DoIf(isCloud),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerService),
		})
		deploySecrets = g.Add(flow.Task{
			Name:         "Deploying Shoot certificates / keys",
			Fn:           flow.SimpleTaskFn(botanist.DeploySecrets),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerServiceIsReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying internal domain DNS record",
			Fn:           flow.SimpleTaskFn(botanist.DeployInternalDomainDNSRecord),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerServiceIsReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying external domain DNS record",
			Fn:           flow.SimpleTaskFn(botanist.DeployExternalDomainDNSRecord).DoIf(managedDNS),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		deployInfrastructure = g.Add(flow.Task{
			Name:         "Deploying Shoot infrastructure",
			Fn:           flow.SimpleTaskFn(shootCloudBotanist.DeployInfrastructure).DoIf(requireInfrastructureDeployment),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret),
		})
		deployBackupInfrastructure = g.Add(flow.Task{
			Name: "Deploying backup infrastructure",
			Fn:   flow.SimpleTaskFn(botanist.DeployBackupInfrastructure).DoIf(isCloud),
		})
		waitUntilBackupInfrastructureReconciled = g.Add(flow.Task{
			Name:         "Waiting until the backup infrastructure has been reconciled",
			Fn:           flow.SimpleTaskFn(botanist.WaitUntilBackupInfrastructureReconciled).DoIf(isCloud),
			Dependencies: flow.NewTaskIDs(deployBackupInfrastructure),
		})
		deployETCD = g.Add(flow.Task{
			Name:         "Deploying main and events etcd",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DeployETCD).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, waitUntilBackupInfrastructureReconciled),
		})
		waitUntilEtcdReady = g.Add(flow.Task{
			Name:         "Waiting until main and event etcd report readiness",
			Fn:           flow.SimpleTaskFn(botanist.WaitUntilEtcdReady).SkipIf(o.Shoot.IsHibernated),
			Dependencies: flow.NewTaskIDs(deployETCD),
		})
		deployKubeAPIServer = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DeployKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployETCD, waitUntilEtcdReady, waitUntilKubeAPIServerServiceIsReady),
		})
		deployCloudProviderConfig = g.Add(flow.Task{
			Name:         "Deploying cloud provider configuration",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DeployCloudProviderConfig).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployInfrastructure, deployCloudProviderSecret),
		})
		waitUntilKubeAPIServerIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server reports readiness",
			Fn:           flow.SimpleTaskFn(botanist.WaitUntilKubeAPIServerReady).SkipIf(o.Shoot.IsHibernated),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServer),
		})
		deployCloudSpecificControlPlane = g.Add(flow.Task{
			Name:         "Deploying cloud-specific control plane components",
			Fn:           flow.SimpleTaskFn(seedCloudBotanist.DeployCloudSpecificControlPlane).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady),
		})
		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.SimpleTaskFn(botanist.InitializeShootClients).RetryUntilTimeout(defaultInterval, 2*time.Minute).SkipIf(o.Shoot.IsHibernated),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady, deployCloudSpecificControlPlane),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Kubernetes scheduler",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DeployKubeScheduler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployKubeAPIServer),
		})
		deployCloudControllerManager = g.Add(flow.Task{
			Name:         "Deploying cloud controller manager",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DeployCloudControllerManager).DoIf(isCloud).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, deployKubeAPIServer, deployCloudProviderConfig),
		})
		deployKubeControllerManager = g.Add(flow.Task{
			Name:         "Deploying Kubernetes controller manager",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DeployKubeControllerManager).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, deployKubeAPIServer, deployCloudProviderConfig, initializeShootClients),
		})
		_ = g.Add(flow.Task{
			Name:         "Syncing shoot access credentials to project namespace in Garden",
			Fn:           flow.SimpleTaskFn(botanist.SyncShootCredentialsToGarden).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, initializeShootClients, deployCloudControllerManager, deployKubeControllerManager),
		})
		computeShootOSConfig = g.Add(flow.Task{
			Name:         "Computing operating system specific configuration for shoot workers",
			Fn:           flow.SimpleTaskFn(hybridBotanist.ComputeShootOperatingSystemConfig).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deployInfrastructure),
		})
		deployKubeAddonManager = g.Add(flow.Task{
			Name:         "Deploying Kubernetes addon manager",
			Fn:           flow.SimpleTaskFn(hybridBotanist.DeployKubeAddonManager).RetryUntilTimeout(defaultInterval, defaultTimeout).SkipIf(o.Shoot.IsHibernated),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deployInfrastructure, computeShootOSConfig),
		})
		deployMachineControllerManager = g.Add(flow.Task{
			Name:         "Deploying machine controller manager",
			Fn:           flow.SimpleTaskFn(botanist.DeployMachineControllerManager).DoIf(isCloud).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deployKubeAddonManager),
		})
		reconcileMachines = g.Add(flow.Task{
			Name:         "Reconciling Shoot workers",
			Fn:           flow.SimpleTaskFn(hybridBotanist.ReconcileMachines).DoIf(isCloud).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(computeShootOSConfig, deployMachineControllerManager, deployInfrastructure, initializeShootClients),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Kube2IAM resources",
			Fn:           flow.SimpleTaskFn(shootCloudBotanist.DeployKube2IAMResources).DoIf(requireKube2IAMDeployment).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployInfrastructure),
		})
		_ = g.Add(flow.Task{
			Name:         "Ensuring ingress DNS record",
			Fn:           flow.SimpleTaskFn(botanist.EnsureIngressDNSRecord).DoIf(managedDNS).RetryUntilTimeout(defaultInterval, 10*time.Minute),
			Dependencies: flow.NewTaskIDs(deployKubeAddonManager),
		})
		waitUntilVPNConnectionExists = g.Add(flow.Task{
			Name:         "Waiting until the Kubernetes API server can connect to the Shoot workers",
			Fn:           flow.SimpleTaskFn(botanist.WaitUntilVPNConnectionExists).SkipIf(o.Shoot.IsHibernated),
			Dependencies: flow.NewTaskIDs(deployKubeAddonManager, reconcileMachines),
		})
		deploySeedMonitoring = g.Add(flow.Task{
			Name:         "Deploying Shoot monitoring stack in Seed",
			Fn:           flow.SimpleTaskFn(botanist.DeploySeedMonitoring).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady, initializeShootClients, waitUntilVPNConnectionExists, reconcileMachines),
		})
		deploySeedLogging = g.Add(flow.Task{
			Name:         "Deploying shoot logging stack in Seed",
			Fn:           flow.SimpleTaskFn(botanist.DeploySeedLogging).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady, initializeShootClients, waitUntilVPNConnectionExists, reconcileMachines),
		})
		deployClusterAutoscaler = g.Add(flow.Task{
			Name:         "Deploying cluster autoscaler",
			Fn:           flow.SimpleTaskFn(botanist.DeployClusterAutoscaler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(reconcileMachines, deployKubeAddonManager, deploySeedMonitoring),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Cert-Broker",
			Fn:           flow.SimpleTaskFn(botanist.DeployCertBroker).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deployKubeAddonManager),
		})
		_ = g.Add(flow.Task{
			Name:         "Hibernating control plane",
			Fn:           flow.TaskFn(botanist.HibernateControlPlane).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(o.Shoot.IsHibernated),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deploySeedMonitoring, deploySeedLogging, deployClusterAutoscaler),
		})
		f = g.Compile()
	)

	err = f.Run(flow.Opts{Logger: o.Logger, ProgressReporter: o.ReportShootProgress})
	if err != nil {
		o.Logger.Errorf("Failed to reconcile Shoot %q: %+v", o.Shoot.Info.Name, err)

		return &gardencorev1alpha1.LastError{
			Codes:       gardencorev1alpha1helper.ExtractErrorCodes(flow.Causes(err)),
			Description: gardencorev1alpha1helper.FormatLastErrDescription(err),
		}
	}

	// Register the Shoot as Seed cluster if it was annotated properly and in the garden namespace
	if o.Shoot.Info.Namespace == common.GardenNamespace {
		if o.ShootedSeed != nil {
			if err := botanist.RegisterAsSeed(o.ShootedSeed.Protected, o.ShootedSeed.Visible, o.ShootedSeed.MinimumVolumeSize); err != nil {
				o.Logger.Errorf("Could not register Shoot %q as Seed: %+v", o.Shoot.Info.Name, err)
			}
		} else {
			if err := botanist.UnregisterAsSeed(); err != nil {
				o.Logger.Errorf("Could not unregister Shoot %q as Seed: %+v", o.Shoot.Info.Name, err)
			}
		}
	}

	o.Logger.Infof("Successfully reconciled Shoot %q", o.Shoot.Info.Name)
	return nil
}

func (c *defaultControl) updateShootStatusReconcile(o *operation.Operation, operationType gardencorev1alpha1.LastOperationType, state gardencorev1alpha1.LastOperationState, retryCycleStartTime *metav1.Time) error {
	var (
		status             = o.Shoot.Info.Status
		now                = metav1.Now()
		observedGeneration = o.Shoot.Info.Generation
	)

	newShoot, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.Garden(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			if len(status.UID) == 0 {
				shoot.Status.UID = shoot.UID
			}
			if len(status.TechnicalID) == 0 {
				shoot.Status.TechnicalID = o.Shoot.SeedNamespace
			}
			if retryCycleStartTime != nil {
				shoot.Status.RetryCycleStartTime = retryCycleStartTime
			}

			shoot.Status.Gardener = *(o.GardenerInfo)
			shoot.Status.ObservedGeneration = observedGeneration
			shoot.Status.LastOperation = &gardencorev1alpha1.LastOperation{
				Type:           operationType,
				State:          state,
				Progress:       1,
				Description:    "Reconciliation of Shoot cluster state in progress.",
				LastUpdateTime: now,
			}
			return shoot, nil
		})
	if err == nil {
		o.Shoot.Info = newShoot
	}
	return err
}

func (c *defaultControl) updateShootStatusResetRetry(o *operation.Operation, operationType gardencorev1alpha1.LastOperationType) error {
	now := metav1.Now()
	return c.updateShootStatusReconcile(o, operationType, gardencorev1alpha1.LastOperationStateError, &now)
}

func (c *defaultControl) updateShootStatusReconcileStart(o *operation.Operation, operationType gardencorev1alpha1.LastOperationType) error {
	var retryCycleStartTime *metav1.Time

	if o.Shoot.Info.Status.RetryCycleStartTime == nil || o.Shoot.Info.Generation != o.Shoot.Info.Status.ObservedGeneration {
		now := metav1.Now()
		retryCycleStartTime = &now
	}

	return c.updateShootStatusReconcile(o, operationType, gardencorev1alpha1.LastOperationStateProcessing, retryCycleStartTime)
}

func (c *defaultControl) updateShootStatusReconcileSuccess(o *operation.Operation, operationType gardencorev1alpha1.LastOperationType) error {
	// Remove task list from Shoot annotations since reconciliation was successful.
	newShoot, err := kutil.TryUpdateShootAnnotations(c.k8sGardenClient.Garden(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			controllerutils.RemoveAllTasks(shoot.Annotations)
			return shoot, nil
		})

	if err != nil {
		return err
	}

	newShoot, err = kutil.TryUpdateShootStatus(c.k8sGardenClient.Garden(), retry.DefaultRetry, newShoot.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			shoot.Status.RetryCycleStartTime = nil
			shoot.Status.Seed = o.Seed.Info.Name
			shoot.Status.LastError = nil
			shoot.Status.LastOperation = &gardencorev1alpha1.LastOperation{
				Type:           operationType,
				State:          gardencorev1alpha1.LastOperationStateSucceeded,
				Progress:       100,
				Description:    "Shoot cluster state has been successfully reconciled.",
				LastUpdateTime: metav1.Now(),
			}
			return shoot, nil
		})

	if err == nil {
		o.Shoot.Info = newShoot
	}
	return err
}

func (c *defaultControl) updateShootStatusReconcileError(o *operation.Operation, operationType gardencorev1alpha1.LastOperationType, lastError *gardencorev1alpha1.LastError) (gardencorev1alpha1.LastOperationState, error) {
	var (
		state         = gardencorev1alpha1.LastOperationStateFailed
		description   = lastError.Description
		lastOperation = o.Shoot.Info.Status.LastOperation
		progress      = 1
		willRetry     = !utils.TimeElapsed(o.Shoot.Info.Status.RetryCycleStartTime, c.config.Controllers.Shoot.RetryDuration.Duration)
	)

	newShoot, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.Garden(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			if willRetry {
				description += " Operation will be retried."
				state = gardencorev1alpha1.LastOperationStateError
			} else {
				shoot.Status.RetryCycleStartTime = nil
			}

			if lastOperation != nil {
				progress = lastOperation.Progress
			}

			shoot.Status.LastError = lastError
			shoot.Status.LastOperation = &gardencorev1alpha1.LastOperation{
				Type:           operationType,
				State:          state,
				Progress:       progress,
				Description:    description,
				LastUpdateTime: metav1.Now(),
			}
			shoot.Status.Gardener = *(o.GardenerInfo)
			return shoot, nil
		})
	if err == nil {
		o.Shoot.Info = newShoot
	}

	newShootAfterLabel, err := kutil.TryUpdateShootLabels(c.k8sGardenClient.Garden(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta, StatusLabelTransform(StatusUnhealthy))
	if err == nil {
		o.Shoot.Info = newShootAfterLabel
	}

	return state, err
}
