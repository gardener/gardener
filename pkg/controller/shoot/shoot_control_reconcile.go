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
	"fmt"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	cloudbotanistpkg "github.com/gardener/gardener/pkg/operation/cloudbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	hybridbotanistpkg "github.com/gardener/gardener/pkg/operation/hybridbotanist"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// reconcileShoot reconciles the Shoot cluster's state.
// It receives a Garden object <garden> which stores the Shoot object and the operation type.
func (c *defaultControl) reconcileShoot(o *operation.Operation, operationType gardenv1beta1.ShootLastOperationType) *gardenv1beta1.LastError {
	// We create the botanists (which will do the actual work).
	botanist, err := botanistpkg.New(o)
	if err != nil {
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

	var (
		defaultRetry = 30 * time.Second
		managedDNS   = o.Shoot.Info.Spec.DNS.Provider != gardenv1beta1.DNSUnmanaged
		isCloud      = o.Shoot.Info.Spec.Cloud.Local == nil

		f                                       = flow.New("Shoot cluster reconciliation").SetProgressReporter(o.ReportShootProgress).SetLogger(o.Logger)
		deployNamespace                         = f.AddTask(botanist.DeployNamespace, defaultRetry)
		deployKubeAPIServerService              = f.AddTask(botanist.DeployKubeAPIServerService, defaultRetry, deployNamespace)
		waitUntilKubeAPIServerServiceIsReady    = f.AddTaskConditional(botanist.WaitUntilKubeAPIServerServiceIsReady, 0, isCloud, deployKubeAPIServerService)
		deploySecrets                           = f.AddTask(botanist.DeploySecrets, 0, waitUntilKubeAPIServerServiceIsReady)
		_                                       = f.AddTask(botanist.DeployInternalDomainDNSRecord, 0, waitUntilKubeAPIServerServiceIsReady)
		_                                       = f.AddTaskConditional(botanist.DeployExternalDomainDNSRecord, 0, managedDNS)
		deployInfrastructure                    = f.AddTask(shootCloudBotanist.DeployInfrastructure, 0, deploySecrets)
		deployBackupInfrastructure              = f.AddTaskConditional(botanist.DeployBackupInfrastructure, 0, isCloud)
		waitUntilBackupInfrastructureReconciled = f.AddTaskConditional(botanist.WaitUntilBackupInfrastructureReconciled, 0, isCloud, deployBackupInfrastructure)
		deployETCD                              = f.AddTask(hybridBotanist.DeployETCD, defaultRetry, deploySecrets, waitUntilBackupInfrastructureReconciled)
		deployCloudProviderConfig               = f.AddTask(hybridBotanist.DeployCloudProviderConfig, defaultRetry, deployInfrastructure)
		deployKubeAPIServer                     = f.AddTask(hybridBotanist.DeployKubeAPIServer, defaultRetry, deploySecrets, deployETCD, waitUntilKubeAPIServerServiceIsReady, deployCloudProviderConfig)
		_                                       = f.AddTask(hybridBotanist.DeployKubeControllerManager, defaultRetry, deploySecrets, deployCloudProviderConfig, deployKubeAPIServer)
		_                                       = f.AddTask(hybridBotanist.DeployKubeScheduler, defaultRetry, deploySecrets, deployKubeAPIServer)
		waitUntilKubeAPIServerIsReady           = f.AddTask(botanist.WaitUntilKubeAPIServerReady, 0, deployKubeAPIServer)
		initializeShootClients                  = f.AddTask(botanist.InitializeShootClients, 2*time.Minute, waitUntilKubeAPIServerIsReady)
		deployMachineControllerManager          = f.AddTaskConditional(botanist.DeployMachineControllerManager, defaultRetry, isCloud, initializeShootClients)
		reconcileMachines                       = f.AddTaskConditional(hybridBotanist.ReconcileMachines, defaultRetry, isCloud, deployMachineControllerManager, deployInfrastructure, initializeShootClients)
		deployKubeAddonManager                  = f.AddTask(hybridBotanist.DeployKubeAddonManager, defaultRetry, initializeShootClients, deployInfrastructure)
		_                                       = f.AddTask(shootCloudBotanist.DeployKube2IAMResources, defaultRetry, deployInfrastructure)
		_                                       = f.AddTaskConditional(botanist.EnsureIngressDNSRecord, 10*time.Minute, managedDNS, deployKubeAddonManager)
		waitUntilVPNConnectionExists            = f.AddTaskConditional(botanist.WaitUntilVPNConnectionExists, 0, !o.Shoot.Hibernated, deployKubeAddonManager, reconcileMachines)
		applyCreateHook                         = f.AddTask(seedCloudBotanist.ApplyCreateHook, defaultRetry, waitUntilVPNConnectionExists)
		deploySeedMonitoring                    = f.AddTask(botanist.DeploySeedMonitoring, defaultRetry, waitUntilKubeAPIServerIsReady, initializeShootClients, waitUntilVPNConnectionExists, reconcileMachines, applyCreateHook)
		_                                       = f.AddTask(botanist.DeployClusterAutoscaler, defaultRetry, reconcileMachines, deployKubeAddonManager, deploySeedMonitoring)
	)

	if e := f.Execute(); e != nil {
		e.Description = fmt.Sprintf("Failed to reconcile Shoot cluster state: %s", e.Description)
		return e
	}

	// Register the Shoot as Seed cluster if it was annotated properly and in the garden namespace
	if o.Shoot.Info.Namespace == common.GardenNamespace {
		if shootUsedAsSeed, protected, visible := helper.IsUsedAsSeed(o.Shoot.Info); shootUsedAsSeed {
			if err := botanist.RegisterAsSeed(protected, visible); err != nil {
				o.Logger.Errorf("Could not register '%s' as Seed: '%s'", o.Shoot.Info.Name, err.Error())
			}
		} else {
			if err := botanist.UnregisterAsSeed(); err != nil {
				o.Logger.Errorf("Could not unregister '%s' as Seed: '%s'", o.Shoot.Info.Name, err.Error())
			}
		}
	}

	o.Logger.Infof("Successfully reconciled Shoot cluster state '%s'", o.Shoot.Info.Name)
	return nil
}

func (c *defaultControl) updateShootStatusReconcileStart(o *operation.Operation, operationType gardenv1beta1.ShootLastOperationType) error {
	var (
		status = o.Shoot.Info.Status
		now    = metav1.Now()
	)

	if len(status.UID) == 0 {
		o.Shoot.Info.Status.UID = o.Shoot.Info.UID
	}
	if status.RetryCycleStartTime == nil || o.Shoot.Info.Generation != o.Shoot.Info.Status.ObservedGeneration {
		o.Shoot.Info.Status.RetryCycleStartTime = &now
	}
	if len(status.TechnicalID) == 0 {
		o.Shoot.Info.Status.TechnicalID = o.Shoot.SeedNamespace
	}

	o.Shoot.Info.Status.Conditions = nil
	o.Shoot.Info.Status.Gardener = *(o.GardenerInfo)
	o.Shoot.Info.Status.ObservedGeneration = o.Shoot.Info.Generation
	o.Shoot.Info.Status.LastOperation = &gardenv1beta1.LastOperation{
		Type:           operationType,
		State:          gardenv1beta1.ShootLastOperationStateProcessing,
		Progress:       1,
		Description:    "Reconciliation of Shoot cluster state in progress.",
		LastUpdateTime: now,
	}

	newShoot, err := c.updater.UpdateShootStatus(o.Shoot.Info)
	if err == nil {
		o.Shoot.Info = newShoot
	}
	return err
}

func (c *defaultControl) updateShootStatusReconcileSuccess(o *operation.Operation, operationType gardenv1beta1.ShootLastOperationType) error {
	o.Shoot.Info.Status.RetryCycleStartTime = nil
	o.Shoot.Info.Status.Seed = o.Seed.Info.Name
	o.Shoot.Info.Status.LastError = nil
	o.Shoot.Info.Status.LastOperation = &gardenv1beta1.LastOperation{
		Type:           operationType,
		State:          gardenv1beta1.ShootLastOperationStateSucceeded,
		Progress:       100,
		Description:    "Shoot cluster state has been successfully reconciled.",
		LastUpdateTime: metav1.Now(),
	}

	newShoot, err := c.updater.UpdateShootStatus(o.Shoot.Info)
	if err == nil {
		o.Shoot.Info = newShoot
	}
	return err
}

func (c *defaultControl) updateShootStatusReconcileError(o *operation.Operation, operationType gardenv1beta1.ShootLastOperationType, lastError *gardenv1beta1.LastError) (gardenv1beta1.ShootLastOperationState, error) {
	var (
		state         = gardenv1beta1.ShootLastOperationStateFailed
		description   = lastError.Description
		lastOperation = o.Shoot.Info.Status.LastOperation
		progress      = 1
	)

	if !utils.TimeElapsed(o.Shoot.Info.Status.RetryCycleStartTime, c.config.Controllers.Shoot.RetryDuration.Duration) {
		description += " Operation will be retried."
		state = gardenv1beta1.ShootLastOperationStateError
	} else {
		o.Shoot.Info.Status.RetryCycleStartTime = nil
	}

	if lastOperation != nil {
		progress = lastOperation.Progress
	}

	o.Shoot.Info.Status.LastError = lastError
	o.Shoot.Info.Status.LastOperation = &gardenv1beta1.LastOperation{
		Type:           operationType,
		State:          state,
		Progress:       progress,
		Description:    description,
		LastUpdateTime: metav1.Now(),
	}
	o.Shoot.Info.Status.Gardener = *(o.GardenerInfo)

	o.Logger.Error(description)

	if newShoot, err := c.updater.UpdateShootStatus(o.Shoot.Info); err == nil {
		o.Shoot.Info = newShoot
	}

	newShootAfterLabel, err := c.updater.UpdateShootLabels(o.Shoot.Info, computeLabelsWithShootHealthiness(false))
	if err == nil {
		o.Shoot.Info = newShootAfterLabel
	}
	return state, err
}
