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
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	cloudbotanistpkg "github.com/gardener/gardener/pkg/operation/cloudbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
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

	// We check whether the Shoot namespace in the Seed cluster is already in a terminating state, i.e. whether
	// we have tried to delete it in a previous run. In that case, we do not need to cleanup Shoot resource because
	// that would have already been done.
	// We also check whether the kube-apiserver pod exists in the Shoot namespace within the Seed cluster. If it does not,
	// then we assume that it has never been deployed successfully. We follow that no resources can have been deployed
	// at all in the Shoot cluster, thus there is nothing to delete at all.
	podList, err := botanist.K8sSeedClient.ListPods(o.Shoot.SeedNamespace, metav1.ListOptions{
		LabelSelector: "app=kubernetes,role=apiserver",
	})
	if err != nil {
		return formatError("Failed to retrieve the list of pods running in the Shoot namespace in the Seed cluster", err)
	}

	// Unregister the Shoot as Seed if it was annotated properly.
	if err := botanist.UnregisterAsSeed(); err != nil {
		o.Logger.Errorf("Could not unregister '%s' as Seed: '%s'", o.Shoot.Info.Name, err.Error())
	}

	var (
		cleanupShootResources = namespace.Status.Phase != corev1.NamespaceTerminating && len(podList.Items) != 0
		defaultRetry          = 5 * time.Minute
		cleanupRetry          = 10 * time.Minute

		f                                = flow.New("Shoot cluster deletion").SetProgressReporter(o.ReportShootProgress).SetLogger(o.Logger)
		ensureImagePullSecretsGarden     = f.AddTask(botanist.EnsureImagePullSecretsGarden, defaultRetry)
		initializeShootClients           = f.AddTaskConditional(botanist.InitializeShootClients, defaultRetry, cleanupShootResources)
		applyDeleteHook                  = f.AddTask(shootCloudBotanist.ApplyDeleteHook, defaultRetry, initializeShootClients)
		deleteSeedMonitoring             = f.AddTask(botanist.DeleteSeedMonitoring, defaultRetry, applyDeleteHook)
		deleteKubeAddonManager           = f.AddTask(botanist.DeleteKubeAddonManager, defaultRetry, applyDeleteHook)
		waitUntilKubeAddonManagerDeleted = f.AddTask(botanist.WaitUntilKubeAddonManagerDeleted, 0, deleteKubeAddonManager)
		// We need to clean up the cluster resources which may have footprints in the infrastructure (such as
		// LoadBalancers, volumes, ...). We do that by deleting all namespaces other than the three standard
		// namespaces which cannot be deleted (kube-system, default, kube-public). In those three namespaces
		// we delete all TPR/CRD data, workload, services and PVCs. Only if none if those resources remain, we
		// go ahead and trigger the infrastructure deletion.
		cleanupCRDs                       = f.AddTaskConditional(botanist.CleanupCRDs, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkCRDCleanup                   = f.AddTaskConditional(botanist.CheckCRDCleanup, cleanupRetry, cleanupShootResources, cleanupCRDs)
		cleanupNamespaces                 = f.AddTaskConditional(botanist.CleanupNamespaces, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkNamespaceCleanup             = f.AddTaskConditional(botanist.CheckNamespaceCleanup, cleanupRetry, cleanupShootResources, cleanupNamespaces)
		cleanupStatefulSets               = f.AddTaskConditional(botanist.CleanupStatefulSets, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkStatefulSetCleanup           = f.AddTaskConditional(botanist.CheckStatefulSetCleanup, cleanupRetry, cleanupShootResources, cleanupStatefulSets)
		cleanupDeployments                = f.AddTaskConditional(botanist.CleanupDeployments, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkDeploymentCleanup            = f.AddTaskConditional(botanist.CheckDeploymentCleanup, cleanupRetry, cleanupShootResources, cleanupDeployments)
		cleanupReplicationControllers     = f.AddTaskConditional(botanist.CleanupReplicationControllers, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkReplicationControllerCleanup = f.AddTaskConditional(botanist.CheckReplicationControllerCleanup, cleanupRetry, cleanupShootResources, cleanupReplicationControllers)
		cleanupReplicaSets                = f.AddTaskConditional(botanist.CleanupReplicaSets, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkReplicaSetCleanup            = f.AddTaskConditional(botanist.CheckReplicaSetCleanup, cleanupRetry, cleanupShootResources, cleanupReplicaSets)
		cleanupDaemonSets                 = f.AddTaskConditional(botanist.CleanupDaemonSets, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkDaemonSetCleanup             = f.AddTaskConditional(botanist.CheckDaemonSetCleanup, cleanupRetry, cleanupShootResources, cleanupDaemonSets)
		cleanupJobs                       = f.AddTaskConditional(botanist.CleanupJobs, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkJobCleanup                   = f.AddTaskConditional(botanist.CheckJobCleanup, cleanupRetry, cleanupShootResources, cleanupJobs)
		cleanupPods                       = f.AddTaskConditional(botanist.CleanupPods, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkPodCleanup                   = f.AddTaskConditional(botanist.CheckPodCleanup, cleanupRetry, cleanupShootResources, cleanupPods)
		cleanupServices                   = f.AddTaskConditional(botanist.CleanupServices, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkServiceCleanup               = f.AddTaskConditional(botanist.CheckServiceCleanup, cleanupRetry, cleanupShootResources, cleanupServices)
		cleanupPersistentVolumeClaims     = f.AddTaskConditional(botanist.CleanupPersistentVolumeClaims, defaultRetry, cleanupShootResources, waitUntilKubeAddonManagerDeleted)
		checkPersistentVolumeClaimCleanup = f.AddTaskConditional(botanist.CheckPersistentVolumeClaimCleanup, cleanupRetry, cleanupShootResources, cleanupPersistentVolumeClaims)
		syncPointCleanup                  = f.AddSyncPoint(ensureImagePullSecretsGarden, checkCRDCleanup, checkNamespaceCleanup, checkStatefulSetCleanup, checkDeploymentCleanup, checkReplicationControllerCleanup, checkReplicaSetCleanup, checkDaemonSetCleanup, checkJobCleanup, checkPodCleanup, checkServiceCleanup, checkPersistentVolumeClaimCleanup)
		destroyNginxIngressResources      = f.AddTask(botanist.DestroyNginxIngressResources, 0, syncPointCleanup)
		destroyKube2IAMResources          = f.AddTask(shootCloudBotanist.DestroyKube2IAMResources, 0, syncPointCleanup)
		destroyInfrastructure             = f.AddTask(shootCloudBotanist.DestroyInfrastructure, 0, syncPointCleanup)
		destroyExternalDomainDNSRecord    = f.AddTask(botanist.DestroyExternalDomainDNSRecord, 0, syncPointCleanup)
		destroyBackupInfrastructure       = f.AddTask(seedCloudBotanist.DestroyBackupInfrastructure, 0, syncPointCleanup)
		destroyInternalDomainDNSRecord    = f.AddTask(botanist.DestroyInternalDomainDNSRecord, 0, syncPointCleanup)
		syncPointTerraformers             = f.AddSyncPoint(deleteSeedMonitoring, destroyNginxIngressResources, destroyKube2IAMResources, destroyInfrastructure, destroyExternalDomainDNSRecord, destroyBackupInfrastructure, destroyInternalDomainDNSRecord)
		deleteNamespace                   = f.AddTask(botanist.DeleteNamespace, defaultRetry, syncPointTerraformers)
		_                                 = f.AddTask(botanist.WaitUntilNamespaceDeleted, 0, deleteNamespace)
		_                                 = f.AddTask(botanist.DeleteGardenSecrets, defaultRetry, deleteNamespace)
	)
	e := f.Execute()
	if e != nil {
		e.Description = fmt.Sprintf("Failed to delete Shoot cluster: %s", e.Description)
		return e
	}

	o.Logger.Infof("Successfully deleted Shoot cluster '%s'", o.Shoot.Info.Name)
	return nil
}

func (c *defaultControl) updateShootStatusDeleteStart(o *operation.Operation) error {
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

	newShoot, err := c.
		updater.
		UpdateShootStatus(o.Shoot.Info)
	if err == nil {
		o.Shoot.Info = newShoot
	}
	return err
}

func (c *defaultControl) updateShootStatusDeleteSuccess(o *operation.Operation) error {
	finalizers := sets.NewString(o.Shoot.Info.Finalizers...)
	finalizers.Delete(gardenv1beta1.GardenerName)

	o.Shoot.Info.Finalizers = finalizers.List()
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
	return err
}

func (c *defaultControl) updateShootStatusDeleteError(o *operation.Operation, lastError *gardenv1beta1.LastError) error {
	var (
		state       = gardenv1beta1.ShootLastOperationStateFailed
		description = lastError.Description
	)

	if !utils.TimeElapsed(o.Shoot.Info.DeletionTimestamp, c.config.Controller.Reconciliation.RetryDuration.Duration) {
		description += " Operation will be retried."
		state = gardenv1beta1.ShootLastOperationStateError
		delete(o.Shoot.Info.Annotations, common.ConfirmationDeletionTimestamp)
	}

	o.Shoot.Info.Status.LastError = lastError
	o.Shoot.Info.Status.Gardener = *o.GardenerInfo
	o.Shoot.Info.Status.LastOperation.Type = gardenv1beta1.ShootLastOperationTypeDelete
	o.Shoot.Info.Status.LastOperation.State = state
	o.Shoot.Info.Status.LastOperation.Description = description
	o.Shoot.Info.Status.LastOperation.LastUpdateTime = metav1.Now()

	o.Logger.Error(description)

	newShoot, err := c.updater.UpdateShootStatus(o.Shoot.Info)
	if err == nil {
		o.Shoot.Info = newShoot
	}
	return err
}
