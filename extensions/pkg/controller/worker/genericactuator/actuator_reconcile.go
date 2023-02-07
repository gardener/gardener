// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package genericactuator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	workerhealthcheck "github.com/gardener/gardener/extensions/pkg/controller/healthcheck/worker"
	extensionsworkercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsworkerhelper "github.com/gardener/gardener/extensions/pkg/controller/worker/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
)

func (a *genericActuator) Reconcile(ctx context.Context, log logr.Logger, worker *extensionsv1alpha1.Worker, cluster *controller.Cluster) error {
	log = log.WithValues("operation", "reconcile")

	workerDelegate, err := a.delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		return fmt.Errorf("could not instantiate actuator context: %w", err)
	}

	// Call pre reconcilation hook to prepare Worker reconciliation.
	if err := workerDelegate.PreReconcileHook(ctx); err != nil {
		return fmt.Errorf("pre worker reconciliation hook failed: %w", err)
	}

	// Get the list of all existing machine deployments.
	existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
	if err := a.client.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
		return err
	}

	// mcmReplicaFunc returns the desired replicas for machine controller manager
	var mcmReplicaFunc = func() int32 {
		switch {
		// if there are any existing machine deployments present with a positive replica count then MCM is needed.
		case isExistingMachineDeploymentWithPositiveReplicaCountPresent(existingMachineDeployments):
			return 1
		// If the cluster is hibernated then there is no further need of MCM and therefore its desired replicas is 0
		case extensionscontroller.IsHibernated(cluster):
			return 0
		// If the cluster is created with hibernation enabled, then desired replicas for MCM is 0
		case extensionscontroller.IsHibernationEnabled(cluster) && extensionscontroller.IsCreationInProcess(cluster):
			return 0
		// If shoot is either waking up or in the process of hibernation then, MCM is required and therefore its desired replicas is 1
		case extensionscontroller.IsHibernatingOrWakingUp(cluster):
			return 1
		// If the shoot is awake then MCM should be available and therefore its desired replicas is 1
		default:
			return 1
		}
	}

	// Deploy machine dependencies.
	// TODO(dkistner): Remove in a future release.
	if err := workerDelegate.DeployMachineDependencies(ctx); err != nil {
		return fmt.Errorf("failed to deploy machine dependencies: %w", err)
	}

	// Deploy the machine-controller-manager into the cluster.
	if err := a.deployMachineControllerManager(ctx, log, worker, cluster, workerDelegate, mcmReplicaFunc); err != nil {
		return err
	}

	// Generate the desired machine deployments.
	log.Info("Generating machine deployments")
	wantedMachineDeployments, err := workerDelegate.GenerateMachineDeployments(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate the machine deployments: %w", err)
	}

	var clusterAutoscalerUsed = extensionsv1alpha1helper.ClusterAutoscalerRequired(worker.Spec.Pools)

	// When the Shoot is hibernated we want to remove the cluster autoscaler so that it does not interfer
	// with Gardeners modifications on the machine deployment's replicas fields.
	isHibernationEnabled := controller.IsHibernationEnabled(cluster)
	if clusterAutoscalerUsed && isHibernationEnabled {
		if err = a.scaleClusterAutoscaler(ctx, log, worker, 0); err != nil {
			return err
		}
	}

	// Get list of existing machine class names
	existingMachineClassNames, err := a.listMachineClassNames(ctx, worker.Namespace, workerDelegate.MachineClassList())
	if err != nil {
		return err
	}

	// Deploy generated machine classes.
	log.Info("Deploying machine classes")
	if err := workerDelegate.DeployMachineClasses(ctx); err != nil {
		return fmt.Errorf("failed to deploy the machine classes: %w", err)
	}

	if workerCredentialsDelegate, ok := workerDelegate.(WorkerCredentialsDelegate); ok {
		// Update cloud credentials for all existing machine class secrets
		cloudCredentials, err := workerCredentialsDelegate.GetMachineControllerManagerCloudCredentials(ctx)
		if err != nil {
			return fmt.Errorf("failed to get the cloud credentials in namespace %s: %w", worker.Namespace, err)
		}
		if err = a.updateCloudCredentialsInAllMachineClassSecrets(ctx, log, cloudCredentials, worker.Namespace); err != nil {
			return fmt.Errorf("failed to update cloud credentials in machine class secrets for namespace %s: %w", worker.Namespace, err)
		}
	}

	// Update the machine images in the worker provider status.
	if err := workerDelegate.UpdateMachineImagesStatus(ctx); err != nil {
		return fmt.Errorf("failed to update the machine image status: %w", err)
	}

	existingMachineDeploymentNames := sets.Set[string]{}
	for _, deployment := range existingMachineDeployments.Items {
		existingMachineDeploymentNames.Insert(deployment.Name)
	}

	// Generate machine deployment configuration based on previously computed list of deployments and deploy them.
	if err := a.deployMachineDeployments(ctx, log, cluster, worker, existingMachineDeployments, wantedMachineDeployments, workerDelegate.MachineClassKind(), clusterAutoscalerUsed); err != nil {
		return fmt.Errorf("failed to generate the machine deployment config: %w", err)
	}

	// Wait until all generated machine deployments are healthy/available.
	if err := a.waitUntilWantedMachineDeploymentsAvailable(ctx, log, cluster, worker, existingMachineDeploymentNames, existingMachineClassNames, wantedMachineDeployments); err != nil {
		// check if the machine-controller-manager is stuck
		isStuck, msg, err2 := a.IsMachineControllerStuck(ctx, worker)
		if err2 != nil {
			log.Error(err2, "Failed to check if the machine-controller-manager pod is stuck after unsuccessfully waiting for all machine deployments to be ready")
			// continue in order to return `err` and determine error codes
		}

		if isStuck {
			podList := corev1.PodList{}
			if err2 := a.client.List(ctx, &podList, client.InNamespace(worker.Namespace), client.MatchingLabels{"role": "machine-controller-manager"}); err2 != nil {
				return fmt.Errorf("failed to list machine-controller-manager pods for worker (%s/%s): %w", worker.Namespace, worker.Name, err2)
			}

			for _, pod := range podList.Items {
				if err2 := a.client.Delete(ctx, &pod); err2 != nil {
					return fmt.Errorf("failed to delete stuck machine-controller-manager pod for worker (%s/%s): %w", worker.Namespace, worker.Name, err2)
				}
			}
			log.Info("Successfully deleted stuck machine-controller-manager pod", "reason", msg)
		}

		return fmt.Errorf("Failed while waiting for all machine deployments to be ready: %w", err)
	}

	// Delete all old machine deployments (i.e. those which were not previously computed but exist in the cluster).
	if err := a.cleanupMachineDeployments(ctx, log, existingMachineDeployments, wantedMachineDeployments); err != nil {
		return fmt.Errorf("failed to cleanup the machine deployments: %w", err)
	}

	// Delete all old machine classes (i.e. those which were not previously computed but exist in the cluster).
	if err := a.cleanupMachineClasses(ctx, log, worker.Namespace, workerDelegate.MachineClassList(), wantedMachineDeployments); err != nil {
		return fmt.Errorf("failed to cleanup the machine classes: %w", err)
	}

	// Delete all old machine class secrets (i.e. those which were not previously computed but exist in the cluster).
	if err := a.cleanupMachineClassSecrets(ctx, log, worker.Namespace, wantedMachineDeployments); err != nil {
		return fmt.Errorf("failed to cleanup the orphaned machine class secrets: %w", err)
	}

	replicas := mcmReplicaFunc()
	if replicas > 0 {
		// Wait until all unwanted machine deployments are deleted from the system.
		if err := a.waitUntilUnwantedMachineDeploymentsDeleted(ctx, log, worker, wantedMachineDeployments); err != nil {
			return fmt.Errorf("error while waiting for all undesired machine deployments to be deleted: %w", err)
		}
	}

	// Delete MachineSets having number of desired and actual replicas equaling 0
	if err := a.cleanupMachineSets(ctx, log, worker.Namespace); err != nil {
		return fmt.Errorf("failed to cleanup the machine sets: %w", err)
	}

	// Scale down machine-controller-manager if shoot is hibernated.
	if isHibernationEnabled {
		if err := a.scaleMachineControllerManager(ctx, log, worker, 0); err != nil {
			return err
		}
	}

	if clusterAutoscalerUsed && !isHibernationEnabled {
		if err = a.scaleClusterAutoscaler(ctx, log, worker, 1); err != nil {
			return err
		}
	}

	if err := a.updateWorkerStatusMachineDeployments(ctx, worker, wantedMachineDeployments); err != nil {
		return fmt.Errorf("failed to update the machine deployments in worker status: %w", err)
	}

	// Cleanup machine dependencies.
	// TODO(dkistner): Remove in a future release.
	if err := workerDelegate.CleanupMachineDependencies(ctx); err != nil {
		return fmt.Errorf("failed to cleanup machine dependencies: %w", err)
	}

	// Call post reconcilation hook after Worker reconciliation has happened.
	if err := workerDelegate.PostReconcileHook(ctx); err != nil {
		return fmt.Errorf("post worker reconciliation hook failed: %w", err)
	}

	return nil
}

func (a *genericActuator) scaleClusterAutoscaler(ctx context.Context, log logr.Logger, worker *extensionsv1alpha1.Worker, replicas int32) error {
	log.Info("Scaling cluster-autoscaler", "replicas", replicas)
	return client.IgnoreNotFound(kubernetes.ScaleDeployment(ctx, a.client, kubernetesutils.Key(worker.Namespace, v1beta1constants.DeploymentNameClusterAutoscaler), replicas))
}

func (a *genericActuator) deployMachineDeployments(ctx context.Context, log logr.Logger, cluster *extensionscontroller.Cluster, worker *extensionsv1alpha1.Worker, existingMachineDeployments *machinev1alpha1.MachineDeploymentList, wantedMachineDeployments extensionsworkercontroller.MachineDeployments, classKind string, clusterAutoscalerUsed bool) error {
	log.Info("Deploying machine deployments")
	for _, deployment := range wantedMachineDeployments {
		var (
			labels                    = map[string]string{"name": deployment.Name}
			existingMachineDeployment = getExistingMachineDeployment(existingMachineDeployments, deployment.Name)
			replicas                  int32
		)

		switch {
		// If the Shoot is hibernated then the machine deployment's replicas should be zero.
		// Also mark all machines for forceful deletion to avoid respecting of PDBs/SLAs in case of cluster hibernation.
		case controller.IsHibernationEnabled(cluster):
			replicas = 0
			if err := a.markAllMachinesForcefulDeletion(ctx, log, worker.Namespace); err != nil {
				return fmt.Errorf("marking all machines for forceful deletion failed: %w", err)
			}
		// If the cluster autoscaler is not enabled then min=max (as per API validation), hence
		// we can use either min or max.
		case !clusterAutoscalerUsed:
			replicas = deployment.Minimum
		// If the machine deployment does not yet exist we set replicas to min so that the cluster
		// autoscaler can scale them as required.
		case existingMachineDeployment == nil:
			if deployment.State != nil {
				// During restoration the actual replica count is in the State.Replicas
				// If wanted deployment has no corresponding existing deployment, but has State, then we are in restoration process
				replicas = deployment.State.Replicas
			} else {
				replicas = deployment.Minimum
			}
		// If the Shoot was hibernated and is now woken up we set replicas to min so that the cluster
		// autoscaler can scale them as required.
		case shootIsAwake(controller.IsHibernationEnabled(cluster), existingMachineDeployments):
			replicas = deployment.Minimum
		// If the shoot worker pool minimum was updated and if the current machine deployment replica
		// count is less than minimum, we update the machine deployment replica count to updated minimum.
		case existingMachineDeployment.Spec.Replicas < deployment.Minimum:
			replicas = deployment.Minimum
		// If the shoot worker pool maximum was updated and if the current machine deployment replica
		// count is greater than maximum, we update the machine deployment replica count to updated maximum.
		case existingMachineDeployment.Spec.Replicas > deployment.Maximum:
			replicas = deployment.Maximum
		// In this case the machine deployment must exist (otherwise the above case was already true),
		// and the cluster autoscaler must be enabled. We do not want to override the machine deployment's
		// replicas as the cluster autoscaler is responsible for setting appropriate values.
		default:
			replicas = getDeploymentSpecReplicas(existingMachineDeployments, deployment.Name)
			if replicas == -1 {
				replicas = deployment.Minimum
			}
		}

		machineDeployment := &machinev1alpha1.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deployment.Name,
				Namespace: worker.Namespace,
			},
		}

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, machineDeployment, func() error {
			machineDeployment.Spec = machinev1alpha1.MachineDeploymentSpec{
				Replicas:        replicas,
				MinReadySeconds: 500,
				Strategy: machinev1alpha1.MachineDeploymentStrategy{
					Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
					RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
						MaxSurge:       &deployment.MaxSurge,
						MaxUnavailable: &deployment.MaxUnavailable,
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
				Template: machinev1alpha1.MachineTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					Spec: machinev1alpha1.MachineSpec{
						Class: machinev1alpha1.ClassSpec{
							Kind: classKind,
							Name: deployment.ClassName,
						},
						NodeTemplateSpec: machinev1alpha1.NodeTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: deployment.Annotations,
								Labels:      deployment.Labels,
							},
							Spec: corev1.NodeSpec{
								Taints: deployment.Taints,
							},
						},
						MachineConfiguration: deployment.MachineConfiguration,
					},
				},
			}

			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

// waitUntilWantedMachineDeploymentsAvailable waits until all the desired <machineDeployments> were marked as healthy /
// available by the machine-controller-manager. It polls the status every 5 seconds.
func (a *genericActuator) waitUntilWantedMachineDeploymentsAvailable(ctx context.Context, log logr.Logger, cluster *extensionscontroller.Cluster, worker *extensionsv1alpha1.Worker, alreadyExistingMachineDeploymentNames sets.Set[string], alreadyExistingMachineClassNames sets.Set[string], wantedMachineDeployments extensionsworkercontroller.MachineDeployments) error {
	log.Info("Waiting until wanted machine deployments are available")

	return retryutils.UntilTimeout(ctx, 5*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		var numHealthyDeployments, numUpdated, numAvailable, numUnavailable, numDesired, numberOfAwakeMachines int32

		// Get the list of all machine deployments
		machineDeployments := &machinev1alpha1.MachineDeploymentList{}
		if err := a.client.List(ctx, machineDeployments, client.InNamespace(worker.Namespace)); err != nil {
			return retryutils.SevereError(err)
		}

		// Get the list of all machine sets
		machineSets := &machinev1alpha1.MachineSetList{}
		if err := a.client.List(ctx, machineSets, client.InNamespace(worker.Namespace)); err != nil {
			return retryutils.SevereError(err)
		}

		// map the owner reference to the machine sets
		ownerReferenceToMachineSet := extensionsworkerhelper.BuildOwnerToMachineSetsMap(machineSets.Items)

		// Collect the numbers of available and desired replicas.
		for _, deployment := range machineDeployments.Items {
			wantedDeployment := wantedMachineDeployments.FindByName(deployment.Name)

			// Filter out all machine deployments that are not desired (any more).
			if wantedDeployment == nil {
				continue
			}

			// We want to wait until all wanted machine deployments have as many
			// available replicas as desired (specified in the .spec.replicas).
			// However, if we see any error in the status of the deployment then we return it.
			if machineErrs := extensionsworkerhelper.ReportFailedMachines(deployment.Status); machineErrs != nil {
				return retryutils.SevereError(machineErrs)
			}

			numberOfAwakeMachines += deployment.Status.Replicas

			// Skip further checks if cluster is hibernated because machine-controller-manager is usually scaled down during hibernation.
			if controller.IsHibernationEnabled(cluster) {
				continue
			}

			// we only care about rolling updates when the cluster is not hibernated
			// when hibernated, just wait until the sum of `.Status.Replicas` over all machine deployments equals 0.
			machineSets := ownerReferenceToMachineSet[deployment.Name]
			// use `wanted deployment` for these checks, as the existing deployments can be based on an outdated cache
			alreadyExistingMachineDeployment := alreadyExistingMachineDeploymentNames.Has(wantedDeployment.Name)
			newMachineClass := !alreadyExistingMachineClassNames.Has(wantedDeployment.ClassName)

			if alreadyExistingMachineDeployment && newMachineClass {
				log.Info("Machine deployment is performing a rolling update", "machineDeployment", &deployment)
				// Already existing machine deployments with a rolling update should have > 1 machine sets
				if len(machineSets) <= 1 {
					err := fmt.Errorf("waiting for the machine-controller-manager to create the machine sets for the machine deployment (%s/%s)", deployment.Namespace, deployment.Name)
					log.Error(err, "Minor error while waiting for wanted MachineDeployments to become available")
					return retryutils.MinorError(err)
				}
			}

			// If the Shoot is not hibernated we want to make sure that the machine set with the right
			// machine class for the machine deployment is deployed by the machine-controller-manager
			if machineSet := extensionsworkerhelper.GetMachineSetWithMachineClass(wantedDeployment.Name, wantedDeployment.ClassName, ownerReferenceToMachineSet); machineSet == nil {
				return retryutils.MinorError(fmt.Errorf("waiting for the machine-controller-manager to create the updated machine set for the machine deployment (%s/%s)", deployment.Namespace, deployment.Name))
			}

			// If the Shoot is not hibernated we want to wait until all wanted machine deployments have as many
			// available replicas as desired (specified in the .spec.replicas).
			if workerhealthcheck.CheckMachineDeployment(&deployment) == nil {
				numHealthyDeployments++
			}
			numDesired += deployment.Spec.Replicas
			numUpdated += deployment.Status.UpdatedReplicas
			numAvailable += deployment.Status.AvailableReplicas
			numUnavailable += deployment.Status.UnavailableReplicas
		}

		var msg string
		switch {
		case !controller.IsHibernationEnabled(cluster):
			// numUpdated == numberOfAwakeMachines waits until the old machine is deleted in the case of a rolling update with maxUnavailability = 0
			// numUnavailable == 0 makes sure that every machine joined the cluster (during creation & in the case of a rolling update with maxUnavailability > 0)
			if numUnavailable == 0 && numUpdated == numberOfAwakeMachines && int(numHealthyDeployments) == len(wantedMachineDeployments) {
				return retryutils.Ok()
			}

			if numUnavailable == 0 && numAvailable == numDesired && numUpdated < numberOfAwakeMachines {
				msg = fmt.Sprintf("Waiting until all old machines are drained and terminated. Waiting for %d machine(s)...", numberOfAwakeMachines-numUpdated)
				break
			}

			msg = fmt.Sprintf("Waiting until machines are available (%d/%d desired machine(s) available, %d/%d machine(s) updated, %d machine(s) pending, %d/%d machinedeployments available)...", numAvailable, numDesired, numUpdated, numDesired, numUnavailable, numHealthyDeployments, len(wantedMachineDeployments))
		default:
			if numberOfAwakeMachines == 0 {
				return retryutils.Ok()
			}
			msg = fmt.Sprintf("Waiting until all machines have been hibernated (%d still awake)...", numberOfAwakeMachines)
		}

		// TODO: Rework logging in this method. There should be one proper log message per MachineDeployment and on aggregated error to return.
		// Currently, later MachineDeployments override earlier messages, logs are not structured, etc.
		log.Info(msg) //nolint:logcheck
		return retryutils.MinorError(errors.New(msg))
	})
}

// waitUntilUnwantedMachineDeploymentsDeleted waits until all the undesired <machineDeployments> are deleted from the
// system. It polls the status every 5 seconds.
func (a *genericActuator) waitUntilUnwantedMachineDeploymentsDeleted(ctx context.Context, log logr.Logger, worker *extensionsv1alpha1.Worker, wantedMachineDeployments extensionsworkercontroller.MachineDeployments) error {
	log.Info("Waiting until unwanted machine deployments are deleted")
	return retryutils.UntilTimeout(ctx, 5*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
		if err := a.client.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
			return retryutils.SevereError(err)
		}

		for _, existingMachineDeployment := range existingMachineDeployments.Items {
			if !wantedMachineDeployments.HasDeployment(existingMachineDeployment.Name) {
				for _, failedMachine := range existingMachineDeployment.Status.FailedMachines {
					return retryutils.SevereError(fmt.Errorf("machine %s failed: %s", failedMachine.Name, failedMachine.LastOperation.Description))
				}

				log.Info("Waiting until unwanted machine deployment is deleted", "machineDeployment", &existingMachineDeployment)
				return retryutils.MinorError(fmt.Errorf("at least one unwanted machine deployment (%s) still exists", existingMachineDeployment.Name))
			}
		}

		return retryutils.Ok()
	})
}

func (a *genericActuator) updateWorkerStatusMachineDeployments(ctx context.Context, worker *extensionsv1alpha1.Worker, machineDeployments extensionsworkercontroller.MachineDeployments) error {
	if len(machineDeployments) == 0 {
		return nil
	}

	var statusMachineDeployments []extensionsv1alpha1.MachineDeployment
	for _, machineDeployment := range machineDeployments {
		statusMachineDeployments = append(statusMachineDeployments, extensionsv1alpha1.MachineDeployment{
			Name:    machineDeployment.Name,
			Minimum: machineDeployment.Minimum,
			Maximum: machineDeployment.Maximum,
		})
	}

	patch := client.MergeFrom(worker.DeepCopy())
	worker.Status.MachineDeployments = statusMachineDeployments
	return a.client.Status().Patch(ctx, worker, patch)
}

// Helper functions

func shootIsAwake(isHibernated bool, existingMachineDeployments *machinev1alpha1.MachineDeploymentList) bool {
	if isHibernated {
		return false
	}

	for _, existingMachineDeployment := range existingMachineDeployments.Items {
		if existingMachineDeployment.Spec.Replicas != 0 {
			return false
		}
	}
	return true
}

func getDeploymentSpecReplicas(existingMachineDeployments *machinev1alpha1.MachineDeploymentList, name string) int32 {
	for _, existingMachineDeployment := range existingMachineDeployments.Items {
		if existingMachineDeployment.Name == name {
			return existingMachineDeployment.Spec.Replicas
		}
	}
	return -1
}

func getExistingMachineDeployment(existingMachineDeployments *machinev1alpha1.MachineDeploymentList, name string) *machinev1alpha1.MachineDeployment {
	for _, machineDeployment := range existingMachineDeployments.Items {
		if machineDeployment.Name == name {
			return &machineDeployment
		}
	}
	return nil
}

func isExistingMachineDeploymentWithPositiveReplicaCountPresent(existingMachineDeployments *machinev1alpha1.MachineDeploymentList) bool {
	for _, machineDeployment := range existingMachineDeployments.Items {
		if machineDeployment.Status.Replicas > 0 {
			return true
		}
	}
	return false
}

// ReadMachineConfiguration reads the configuration from worker-pool and returns the corresponding configuration of machine-deployment.
func ReadMachineConfiguration(pool extensionsv1alpha1.WorkerPool) *machinev1alpha1.MachineConfiguration {
	machineConfiguration := &machinev1alpha1.MachineConfiguration{}
	poolSettings := pool.MachineControllerManagerSettings
	if poolSettings != nil {
		if poolSettings.MachineDrainTimeout != nil {
			machineConfiguration.MachineDrainTimeout = poolSettings.MachineDrainTimeout
		}
		if poolSettings.MachineHealthTimeout != nil {
			machineConfiguration.MachineHealthTimeout = poolSettings.MachineHealthTimeout
		}
		if poolSettings.MachineCreationTimeout != nil {
			machineConfiguration.MachineCreationTimeout = poolSettings.MachineCreationTimeout
		}
		if poolSettings.MaxEvictRetries != nil {
			machineConfiguration.MaxEvictRetries = poolSettings.MaxEvictRetries
		}
		if len(poolSettings.NodeConditions) > 0 {
			nodeConditions := strings.Join(poolSettings.NodeConditions, ",")
			machineConfiguration.NodeConditions = &nodeConditions
		}
	}
	return machineConfiguration
}
