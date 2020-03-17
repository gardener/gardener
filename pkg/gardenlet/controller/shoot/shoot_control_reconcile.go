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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/gardener/gardener/pkg/version"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// runReconcileShootFlow reconciles the Shoot cluster's state.
// It receives an Operation object <o> which stores the Shoot object.
func (c *Controller) runReconcileShootFlow(o *operation.Operation, operationType gardencorev1beta1.LastOperationType) *gardencorev1beta1helper.WrappedLastErrors {
	// We create the botanists (which will do the actual work).
	var (
		botanist             *botanistpkg.Botanist
		enableEtcdEncryption bool
		tasksWithErrors      []string
		err                  error
	)

	for _, lastError := range o.Shoot.Info.Status.LastErrors {
		if lastError.TaskID != nil {
			tasksWithErrors = append(tasksWithErrors, *lastError.TaskID)
		}
	}

	errorContext := errors.NewErrorContext("Shoot cluster reconciliation", tasksWithErrors)

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
		errors.ToExecute("Check required extensions", func() error {
			return botanist.RequiredExtensionsExist(context.TODO())
		}),
		errors.ToExecute("Check version constraint", func() error {
			enableEtcdEncryption, err = versionutils.CheckVersionMeetsConstraint(botanist.Shoot.Info.Spec.Kubernetes.Version, ">= 1.13")
			return err
		}),
	)

	if err != nil {
		return gardencorev1beta1helper.NewWrappedLastErrors(gardencorev1beta1helper.FormatLastErrDescription(err), err)
	}

	var (
		defaultTimeout     = 30 * time.Second
		defaultInterval    = 5 * time.Second
		dnsEnabled         = !o.Shoot.DisableDNS
		managedExternalDNS = o.Shoot.ExternalDomain != nil && o.Shoot.ExternalDomain.Provider != "unmanaged"
		managedInternalDNS = o.Garden.InternalDomain != nil && o.Garden.InternalDomain.Provider != "unmanaged"
		allowBackup        = o.Seed.Info.Spec.Backup != nil
		staticNodesCIDR    = o.Shoot.Info.Spec.Networking.Nodes != nil

		g                         = flow.NewGraph("Shoot cluster reconciliation")
		syncClusterResourceToSeed = g.Add(flow.Task{
			Name: "Syncing shoot cluster information to seed",
			Fn:   flow.TaskFn(botanist.SyncClusterResourceToSeed).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name: "Ensuring that ShootState exists",
			Fn:   flow.TaskFn(botanist.EnsureShootStateExists).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		deployNamespace = g.Add(flow.Task{
			Name:         "Deploying Shoot namespace in Seed",
			Fn:           flow.TaskFn(botanist.DeployNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncClusterResourceToSeed),
		})
		deployCloudProviderSecret = g.Add(flow.Task{
			Name:         "Deploying cloud provider account secret",
			Fn:           flow.TaskFn(botanist.DeployCloudProviderSecret).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		deployKubeAPIServerService = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server service in the Seed cluster",
			Fn:           flow.TaskFn(botanist.DeployKubeAPIServerService).RetryUntilTimeout(defaultInterval, defaultTimeout).SkipIf(o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		waitUntilKubeAPIServerServiceIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server service in the Seed cluster has reported readiness",
			Fn:           flow.TaskFn(botanist.WaitUntilKubeAPIServerServiceIsReady).SkipIf(o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerService),
		})
		deploySecrets = g.Add(flow.Task{
			Name: "Deploying Shoot certificates / keys",
			Fn:   flow.TaskFn(botanist.DeploySecrets),
			Dependencies: func() flow.TaskIDs {
				taskIDs := flow.NewTaskIDs(deployNamespace)
				if !dnsEnabled && !o.Shoot.HibernationEnabled {
					taskIDs.Insert(waitUntilKubeAPIServerServiceIsReady)
				}
				return taskIDs
			}(),
		})
		deployInternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Deploying internal domain DNS record",
			Fn:           flow.TaskFn(botanist.DeployInternalDomainDNSRecord).DoIf(dnsEnabled && managedInternalDNS && !o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerServiceIsReady),
		})
		deployExternalDomainDNSRecord = g.Add(flow.Task{
			Name:         "Deploying external domain DNS record",
			Fn:           flow.TaskFn(botanist.DeployExternalDomainDNSRecord).DoIf(dnsEnabled && managedExternalDNS && !o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deployNamespace, waitUntilKubeAPIServerServiceIsReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying additional DNS providers",
			Fn:           flow.TaskFn(botanist.DeployAdditionalDNSProviders).DoIf(dnsEnabled && !o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deployInternalDomainDNSRecord, deployExternalDomainDNSRecord),
		})
		deployInfrastructure = g.Add(flow.Task{
			Name:         "Deploying Shoot infrastructure",
			Fn:           flow.TaskFn(botanist.DeployInfrastructure).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret),
		})
		waitUntilInfrastructureReady = g.Add(flow.Task{
			Name:         "Waiting until shoot infrastructure has been reconciled",
			Fn:           flow.TaskFn(botanist.WaitUntilInfrastructureReady),
			Dependencies: flow.NewTaskIDs(deployInfrastructure),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying network policies",
			Fn:           flow.TaskFn(botanist.DeployNetworkPolicies).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace).InsertIf(!staticNodesCIDR, waitUntilInfrastructureReady),
		})
		deployBackupEntryInGarden = g.Add(flow.Task{
			Name: "Deploying backup entry",
			Fn:   flow.TaskFn(botanist.DeployBackupEntryInGarden).DoIf(allowBackup),
		})
		wailtUntilBackupEntryInGardenReconciled = g.Add(flow.Task{
			Name:         "Waiting until the backup entry has been reconciled",
			Fn:           flow.TaskFn(botanist.WaitUntilBackupEntryInGardenReconciled).DoIf(allowBackup),
			Dependencies: flow.NewTaskIDs(deployBackupEntryInGarden),
		})
		deployETCD = g.Add(flow.Task{
			Name:         "Deploying main and events etcd",
			Fn:           flow.TaskFn(botanist.DeployETCD).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, wailtUntilBackupEntryInGardenReconciled),
		})
		waitUntilEtcdReady = g.Add(flow.Task{
			Name:         "Waiting until main and event etcd report readiness",
			Fn:           flow.TaskFn(botanist.WaitUntilEtcdReady).SkipIf(o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deployETCD),
		})
		deployControlPlane = g.Add(flow.Task{
			Name:         "Deploying shoot control plane components",
			Fn:           flow.TaskFn(botanist.DeployControlPlane).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, waitUntilInfrastructureReady),
		})
		waitUntilControlPlaneReady = g.Add(flow.Task{
			Name:         "Waiting until shoot control plane has been reconciled",
			Fn:           flow.TaskFn(botanist.WaitUntilControlPlaneReady),
			Dependencies: flow.NewTaskIDs(deployControlPlane),
		})
		createOrUpdateEtcdEncryptionConfiguration = g.Add(flow.Task{
			Name:         "Applying etcd encryption configuration",
			Fn:           flow.TaskFn(botanist.ApplyEncryptionConfiguration).DoIf(enableEtcdEncryption),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		deployKubeAPIServer = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server",
			Fn:           flow.TaskFn(botanist.DeployKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployETCD, waitUntilEtcdReady, waitUntilKubeAPIServerServiceIsReady, waitUntilControlPlaneReady, createOrUpdateEtcdEncryptionConfiguration).InsertIf(!staticNodesCIDR, waitUntilInfrastructureReady),
		})
		waitUntilKubeAPIServerIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server reports readiness",
			Fn:           flow.TaskFn(botanist.WaitUntilKubeAPIServerReady).SkipIf(o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServer),
		})
		deployControlPlaneExposure = g.Add(flow.Task{
			Name:         "Deploying shoot control plane exposure components",
			Fn:           flow.TaskFn(botanist.DeployControlPlaneExposure).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady),
		})
		waitUntilControlPlaneExposureReady = g.Add(flow.Task{
			Name:         "Waiting until Shoot control plane exposure has been reconciled",
			Fn:           flow.TaskFn(botanist.WaitUntilControlPlaneExposureReady),
			Dependencies: flow.NewTaskIDs(deployControlPlaneExposure),
		})
		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.SimpleTaskFn(botanist.InitializeShootClients).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady, waitUntilControlPlaneExposureReady, deployInternalDomainDNSRecord),
		})
		_ = g.Add(flow.Task{
			Name:         "Rewriting Shoot secrets if EncryptionConfiguration has changed",
			Fn:           flow.TaskFn(botanist.RewriteShootSecretsIfEncryptionConfigurationChanged).DoIf(enableEtcdEncryption && !o.Shoot.HibernationEnabled).RetryUntilTimeout(defaultInterval, 15*time.Minute),
			Dependencies: flow.NewTaskIDs(initializeShootClients, createOrUpdateEtcdEncryptionConfiguration),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Kubernetes scheduler",
			Fn:           flow.TaskFn(botanist.DeployKubeScheduler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, waitUntilKubeAPIServerIsReady),
		})
		deployKubeControllerManager = g.Add(flow.Task{
			Name:         "Deploying Kubernetes controller manager",
			Fn:           flow.TaskFn(botanist.DeployKubeControllerManager).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, deployCloudProviderSecret, waitUntilKubeAPIServerIsReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Syncing shoot access credentials to project namespace in Garden",
			Fn:           flow.TaskFn(botanist.SyncShootCredentialsToGarden).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deploySecrets, initializeShootClients, deployKubeControllerManager),
		})
		computeShootOSConfig = g.Add(flow.Task{
			Name:         "Computing operating system specific configuration for shoot workers",
			Fn:           flow.TaskFn(botanist.ComputeShootOperatingSystemConfig).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilInfrastructureReady),
		})
		deployGardenerResourceManager = g.Add(flow.Task{
			Name:         "Deploying gardener-resource-manager",
			Fn:           flow.TaskFn(botanist.DeployGardenerResourceManager).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		deployNetworking = g.Add(flow.Task{
			Name:         "Deploying shoot network plugin",
			Fn:           flow.TaskFn(botanist.DeployNetwork).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, computeShootOSConfig),
		})
		waitUntilNetworkIsReady = g.Add(flow.Task{
			Name:         "Waiting until shoot network plugin has been reconciled",
			Fn:           flow.TaskFn(botanist.WaitUntilNetworkIsReady),
			Dependencies: flow.NewTaskIDs(deployNetworking),
		})
		deployManagedResources = g.Add(flow.Task{
			Name:         "Deploying managed resources",
			Fn:           flow.TaskFn(botanist.DeployManagedResources).RetryUntilTimeout(defaultInterval, defaultTimeout).SkipIf(o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, computeShootOSConfig),
		})
		deployWorker = g.Add(flow.Task{
			Name:         "Configuring shoot worker pools",
			Fn:           flow.TaskFn(botanist.DeployWorker).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, waitUntilInfrastructureReady, initializeShootClients, computeShootOSConfig, waitUntilNetworkIsReady),
		})
		waitUntilWorkerReady = g.Add(flow.Task{
			Name:         "Waiting until shoot worker nodes have been reconciled",
			Fn:           flow.TaskFn(botanist.WaitUntilWorkerReady),
			Dependencies: flow.NewTaskIDs(deployWorker),
		})
		_ = g.Add(flow.Task{
			Name:         "Ensuring ingress DNS record",
			Fn:           flow.TaskFn(botanist.EnsureIngressDNSRecord).DoIf(dnsEnabled && managedExternalDNS).RetryUntilTimeout(defaultInterval, 10*time.Minute),
			Dependencies: flow.NewTaskIDs(deployManagedResources),
		})
		waitUntilVPNConnectionExists = g.Add(flow.Task{
			Name:         "Waiting until the Kubernetes API server can connect to the Shoot workers",
			Fn:           flow.TaskFn(botanist.WaitUntilVPNConnectionExists).SkipIf(o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deployManagedResources, waitUntilNetworkIsReady, waitUntilWorkerReady),
		})
		deploySeedMonitoring = g.Add(flow.Task{
			Name:         "Deploying Shoot monitoring stack in Seed",
			Fn:           flow.TaskFn(botanist.DeploySeedMonitoring).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady, initializeShootClients, waitUntilVPNConnectionExists, waitUntilWorkerReady).InsertIf(!staticNodesCIDR, waitUntilInfrastructureReady),
		})
		deploySeedLogging = g.Add(flow.Task{
			Name:         "Deploying shoot logging stack in Seed",
			Fn:           flow.TaskFn(botanist.DeploySeedLogging).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady, initializeShootClients, waitUntilVPNConnectionExists, waitUntilWorkerReady),
		})
		deployClusterAutoscaler = g.Add(flow.Task{
			Name:         "Deploying cluster autoscaler",
			Fn:           flow.TaskFn(botanist.DeployClusterAutoscaler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilWorkerReady, deployManagedResources, deploySeedMonitoring),
		})
		_ = g.Add(flow.Task{
			Name:         "Hibernating control plane",
			Fn:           flow.TaskFn(botanist.HibernateControlPlane).RetryUntilTimeout(defaultInterval, 2*time.Minute).DoIf(o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deploySeedMonitoring, deploySeedLogging, deployClusterAutoscaler),
		})
		deployExtensionResources = g.Add(flow.Task{
			Name:         "Deploying extension resources",
			Fn:           flow.TaskFn(botanist.DeployExtensionResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension resources are ready",
			Fn:           flow.TaskFn(botanist.WaitUntilExtensionResourcesReady),
			Dependencies: flow.NewTaskIDs(deployExtensionResources),
		})
		deleteStaleExtensionResources = g.Add(flow.Task{
			Name:         "Delete stale extension resources",
			Fn:           flow.TaskFn(botanist.DeleteStaleExtensionResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until stale extension resources are deleted",
			Fn:           flow.TaskFn(botanist.WaitUntilExtensionResourcesDeleted).SkipIf(o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deleteStaleExtensionResources),
		})
		_ = g.Add(flow.Task{
			Name:         "Maintain shoot annotations",
			Fn:           flow.TaskFn(botanist.MaintainShootAnnotations).SkipIf(o.Shoot.HibernationEnabled),
			Dependencies: flow.NewTaskIDs(deleteStaleExtensionResources),
		})
		f = g.Compile()
	)

	if err := f.Run(flow.Opts{Logger: o.Logger, ProgressReporter: o.ReportShootProgress, ErrorContext: errorContext, ErrorCleaner: o.CleanShootTaskError}); err != nil {
		o.Logger.Errorf("Failed to reconcile Shoot %q: %+v", o.Shoot.Info.Name, err)
		return gardencorev1beta1helper.NewWrappedLastErrors(gardencorev1beta1helper.FormatLastErrDescription(err), flow.Errors(err))
	}

	o.Logger.Infof("Successfully reconciled Shoot %q", o.Shoot.Info.Name)
	return nil
}

func (c *Controller) updateShootStatusReconcile(o *operation.Operation, operationType gardencorev1beta1.LastOperationType, state gardencorev1beta1.LastOperationState, retryCycleStartTime *metav1.Time) error {
	var (
		status             = o.Shoot.Info.Status
		now                = metav1.Now()
		observedGeneration = o.Shoot.Info.Generation
	)

	newShoot, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.GardenCore(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
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
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
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

func (c *Controller) updateShootStatusReconcileStart(o *operation.Operation, operationType gardencorev1beta1.LastOperationType) error {
	var retryCycleStartTime *metav1.Time

	if o.Shoot.Info.Status.RetryCycleStartTime == nil ||
		o.Shoot.Info.Generation != o.Shoot.Info.Status.ObservedGeneration ||
		o.Shoot.Info.Status.Gardener.Version == version.Get().GitVersion ||
		(o.Shoot.Info.Status.LastOperation != nil && o.Shoot.Info.Status.LastOperation.State == gardencorev1beta1.LastOperationStateFailed) {

		now := metav1.NewTime(time.Now().UTC())
		retryCycleStartTime = &now
	}

	return c.updateShootStatusReconcile(o, operationType, gardencorev1beta1.LastOperationStateProcessing, retryCycleStartTime)
}

func (c *Controller) updateShootStatusReconcileSuccess(o *operation.Operation, operationType gardencorev1beta1.LastOperationType) error {
	// Remove task list from Shoot annotations since reconciliation was successful.
	newShoot, err := kutil.TryUpdateShootAnnotations(c.k8sGardenClient.GardenCore(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			controllerutils.RemoveAllTasks(shoot.Annotations)
			return shoot, nil
		},
	)
	if err != nil {
		return err
	}

	newShoot, err = kutil.TryUpdateShootStatus(c.k8sGardenClient.GardenCore(), retry.DefaultRetry, newShoot.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			shoot.Status.RetryCycleStartTime = nil
			shoot.Status.SeedName = &o.Seed.Info.Name
			shoot.Status.IsHibernated = o.Shoot.HibernationEnabled
			shoot.Status.LastErrors = nil
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:           operationType,
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				Progress:       100,
				Description:    "Shoot cluster state has been successfully reconciled.",
				LastUpdateTime: metav1.Now(),
			}
			return shoot, nil
		},
	)
	if err == nil {
		o.Shoot.Info = newShoot
	}
	return err
}

func (c *Controller) updateShootStatusReconcileError(o *operation.Operation, operationType gardencorev1beta1.LastOperationType, description string, lastErrors ...gardencorev1beta1.LastError) error {
	var (
		state         = gardencorev1beta1.LastOperationStateFailed
		lastOperation = o.Shoot.Info.Status.LastOperation
		progress      = 1
		willRetry     = !utils.TimeElapsed(o.Shoot.Info.Status.RetryCycleStartTime, c.config.Controllers.Shoot.RetryDuration.Duration)
	)

	newShoot, err := kutil.TryUpdateShootStatus(c.k8sGardenClient.GardenCore(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			if willRetry {
				description += " Operation will be retried."
				state = gardencorev1beta1.LastOperationStateError
			} else {
				shoot.Status.RetryCycleStartTime = nil
			}

			if lastOperation != nil {
				progress = lastOperation.Progress
			}

			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:           operationType,
				State:          state,
				Progress:       progress,
				Description:    description,
				LastUpdateTime: metav1.Now(),
			}
			shoot.Status.LastErrors = lastErrors
			shoot.Status.Gardener = *(o.GardenerInfo)
			return shoot, nil
		})
	if err == nil {
		o.Shoot.Info = newShoot
	}

	newShootAfterLabel, err := kutil.TryUpdateShootLabels(c.k8sGardenClient.GardenCore(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta, StatusLabelTransform(StatusUnhealthy))
	if err == nil {
		o.Shoot.Info = newShootAfterLabel
	}

	return err
}
