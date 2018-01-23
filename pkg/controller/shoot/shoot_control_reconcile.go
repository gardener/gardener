// Copyright 2018 The Gardener Authors.
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
	"strconv"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	cloudbotanistpkg "github.com/gardener/gardener/pkg/operation/cloudbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	helperbotanistpkg "github.com/gardener/gardener/pkg/operation/helperbotanist"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// reconcileShoot reconciles the Shoot cluster's state.
// It receives a Garden object <garden> which stores the Shoot object and the operation type.
func (c *defaultControl) reconcileShoot(o *operation.Operation, operationType gardenv1beta1.ShootLastOperationType, updater UpdaterInterface) *gardenv1beta1.LastError {
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

	helperBotanist := &helperbotanistpkg.HelperBotanist{
		Operation:          o,
		Botanist:           botanist,
		SeedCloudBotanist:  seedCloudBotanist,
		ShootCloudBotanist: shootCloudBotanist,
	}

	var (
		defaultRetry = 5 * time.Minute
		addons       = o.Shoot.Info.Spec.Addons
		managedDNS   = o.Shoot.Info.Spec.DNS.Provider != gardenv1beta1.DNSUnmanaged

		f                                    = flow.New("Shoot cluster creation").SetProgressReporter(o.ReportShootProgress).SetLogger(o.Logger)
		deployNamespace                      = f.AddTask(botanist.DeployNamespace, defaultRetry)
		ensureImagePullSecretsGarden         = f.AddTask(botanist.EnsureImagePullSecretsGarden, defaultRetry)
		ensureImagePullSecretsSeed           = f.AddTask(botanist.EnsureImagePullSecretsSeed, defaultRetry, deployNamespace)
		deployKubeAPIServerService           = f.AddTask(botanist.DeployKubeAPIServerService, defaultRetry, deployNamespace)
		waitUntilKubeAPIServerServiceIsReady = f.AddTask(botanist.WaitUntilKubeAPIServerServiceIsReady, 0, deployKubeAPIServerService)
		deploySecrets                        = f.AddTask(botanist.DeploySecrets, 0, waitUntilKubeAPIServerServiceIsReady)
		deployETCDOperator                   = f.AddTask(botanist.DeployETCDOperator, defaultRetry, deployNamespace, ensureImagePullSecretsSeed)
		_                                    = f.AddTask(botanist.DeployInternalDomainDNSRecord, 0, waitUntilKubeAPIServerServiceIsReady, ensureImagePullSecretsGarden)
		_                                    = f.AddTaskConditional(botanist.DeployExternalDomainDNSRecord, 0, managedDNS, ensureImagePullSecretsGarden)
		deployInfrastructure                 = f.AddTask(shootCloudBotanist.DeployInfrastructure, 0, deploySecrets, ensureImagePullSecretsGarden)
		deployBackupInfrastructure           = f.AddTask(seedCloudBotanist.DeployBackupInfrastructure, 0, ensureImagePullSecretsGarden)
		deployETCD                           = f.AddTask(helperBotanist.DeployETCD, defaultRetry, deployETCDOperator, deployBackupInfrastructure)
		deployCloudProviderConfig            = f.AddTask(helperBotanist.DeployCloudProviderConfig, defaultRetry, deployInfrastructure)
		deployKubeAPIServer                  = f.AddTask(helperBotanist.DeployKubeAPIServer, defaultRetry, deploySecrets, deployETCD, waitUntilKubeAPIServerServiceIsReady, deployCloudProviderConfig)
		_                                    = f.AddTask(helperBotanist.DeployKubeControllerManager, defaultRetry, deployCloudProviderConfig, deployKubeAPIServer)
		_                                    = f.AddTask(helperBotanist.DeployKubeScheduler, defaultRetry, deployKubeAPIServer)
		waitUntilKubeAPIServerIsReady        = f.AddTask(botanist.WaitUntilKubeAPIServerIsReady, 0, deployKubeAPIServer)
		initializeShootClients               = f.AddTask(botanist.InitializeShootClients, defaultRetry, waitUntilKubeAPIServerIsReady)
		ensureImagePullSecretsShoot          = f.AddTask(botanist.EnsureImagePullSecretsShoot, defaultRetry, initializeShootClients)
		deployKubeAddonManager               = f.AddTask(helperBotanist.DeployKubeAddonManager, defaultRetry, ensureImagePullSecretsShoot)
		_                                    = f.AddTask(shootCloudBotanist.DeployAutoNodeRepair, defaultRetry, waitUntilKubeAPIServerIsReady, deployInfrastructure)
		_                                    = f.AddTaskConditional(shootCloudBotanist.DeployKube2IAMResources, defaultRetry, addons.Kube2IAM.Enabled, deployInfrastructure)
		_                                    = f.AddTaskConditional(botanist.DeployNginxIngressResources, 10*time.Minute, addons.NginxIngress.Enabled && managedDNS, deployKubeAddonManager, ensureImagePullSecretsShoot)
		waitUntilVPNConnectionExists         = f.AddTask(botanist.WaitUntilVPNConnectionExists, 0, deployKubeAddonManager)
		applyCreateHook                      = f.AddTask(seedCloudBotanist.ApplyCreateHook, defaultRetry, waitUntilVPNConnectionExists)
		_                                    = f.AddTask(botanist.DeploySeedMonitoring, defaultRetry, waitUntilKubeAPIServerIsReady, initializeShootClients, waitUntilVPNConnectionExists, applyCreateHook)
	)

	e := f.Execute()
	if e != nil {
		e.Description = fmt.Sprintf("Failed to reconcile Shoot cluster state: %s", e.Description)
		return e
	}

	// Register the Shoot as Seed cluster if it was annotated properly.
	registerAsSeed := false
	if val, ok := o.Shoot.Info.Annotations[common.ShootUseAsSeed]; ok {
		useAsSeed, err := strconv.ParseBool(val)
		if err == nil && useAsSeed {
			registerAsSeed = true
		}
	}
	if registerAsSeed {
		if err := botanist.RegisterAsSeed(); err != nil {
			o.Logger.Errorf("Could not register '%s' as Seed: '%s'", o.Shoot.Info.Name, err.Error())
		}
	} else {
		if err := botanist.UnregisterAsSeed(); err != nil {
			o.Logger.Errorf("Could not unregister '%s' as Seed: '%s'", o.Shoot.Info.Name, err.Error())
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
	if status.OperationStartTime == nil {
		o.Shoot.Info.Status.OperationStartTime = &now
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

func (c *defaultControl) updateShootStatusReconcileError(o *operation.Operation, operationType gardenv1beta1.ShootLastOperationType, lastError *gardenv1beta1.LastError) error {
	var (
		state         = gardenv1beta1.ShootLastOperationStateFailed
		description   = lastError.Description
		lastOperation = o.Shoot.Info.Status.LastOperation
		progress      int
	)

	if !utils.TimeElapsed(o.Shoot.Info.Status.OperationStartTime, c.config.Controller.Reconciliation.RetryDuration.Duration) {
		description += " Operation will be retried."
		state = gardenv1beta1.ShootLastOperationStateError
	}

	if lastOperation != nil {
		progress = lastOperation.Progress
	} else {
		progress = 1
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

	newShoot, err := c.updater.UpdateShootStatus(o.Shoot.Info)
	if err == nil {
		o.Shoot.Info = newShoot
	}
	return err
}
