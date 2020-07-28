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
	"fmt"
	"strings"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	workerhealthcheck "github.com/gardener/gardener/extensions/pkg/controller/healthcheck/worker"
	extensionsworker "github.com/gardener/gardener/extensions/pkg/controller/worker"
	workerhelper "github.com/gardener/gardener/extensions/pkg/controller/worker/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (a *genericActuator) Reconcile(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *controller.Cluster) error {
	workerDelegate, err := a.delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		return errors.Wrapf(err, "could not instantiate actuator context")
	}

	// If the shoot is hibernated then we want to scale down the machine-controller-manager. However, we want to first allow it to delete
	// all remaining worker nodes. Hence, we cannot set the replicas=0 here (otherwise it would be offline and not able to delete the nodes).
	var replicaFunc = func() (int32, error) {
		if extensionscontroller.IsHibernated(cluster) {
			deployment := &appsv1.Deployment{}
			if err := a.client.Get(ctx, kutil.Key(worker.Namespace, a.mcmName), deployment); err != nil && !apierrors.IsNotFound(err) {
				return 0, err
			}
			if replicas := deployment.Spec.Replicas; replicas != nil {
				return *replicas, nil
			}
		}
		return 1, nil
	}

	// Deploy the machine-controller-manager into the cluster.
	a.logger.Info("Deploying the machine-controller-manager", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	if err := a.deployMachineControllerManager(ctx, worker, cluster, workerDelegate, replicaFunc); err != nil {
		return err
	}

	// Generate the desired machine deployments.
	wantedMachineDeployments, err := workerDelegate.GenerateMachineDeployments(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to generate the machine deployments")
	}

	var clusterAutoscalerUsed = extensionsv1alpha1helper.ClusterAutoscalerRequired(worker.Spec.Pools)

	// When the Shoot is hibernated we want to remove the cluster autoscaler so that it does not interfer
	// with Gardeners modifications on the machine deployment's replicas fields.
	isHibernated := controller.IsHibernated(cluster)
	if clusterAutoscalerUsed && isHibernated {
		if err = a.scaleClusterAutoscaler(ctx, worker, 0); err != nil {
			return err
		}
	}

	// Get list of existing machine class names
	existingMachineClassNames, err := a.listMachineClassNames(ctx, worker.Namespace, workerDelegate.MachineClassList())
	if err != nil {
		return err
	}

	// Deploy generated machine classes.
	a.logger.Info("Deploying the machine classes", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	if err := workerDelegate.DeployMachineClasses(ctx); err != nil {
		return errors.Wrapf(err, "failed to deploy the machine classes")
	}

	// Store machine image information in worker provider status.
	machineImages, err := workerDelegate.GetMachineImages(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get the machine images")
	}
	if err := a.updateWorkerStatusMachineImages(ctx, worker, machineImages); err != nil {
		return errors.Wrapf(err, "failed to update the machine images in worker status")
	}

	// Get the list of all existing machine deployments.
	existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
	if err := a.client.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
		return err
	}

	existingMachineDeploymentNames := sets.String{}
	for _, deployment := range existingMachineDeployments.Items {
		existingMachineDeploymentNames.Insert(deployment.Name)
	}

	// Generate machine deployment configuration based on previously computed list of deployments and deploy them.
	a.logger.Info("Deploying the machine deployments", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	if err := a.deployMachineDeployments(ctx, cluster, worker, existingMachineDeployments, wantedMachineDeployments, workerDelegate.MachineClassKind(), clusterAutoscalerUsed); err != nil {
		return errors.Wrapf(err, "failed to generate the machine deployment config")
	}

	// Wait until all generated machine deployments are healthy/available.
	if err := a.waitUntilWantedMachineDeploymentsAvailable(ctx, cluster, worker, existingMachineDeploymentNames, existingMachineClassNames, wantedMachineDeployments, clusterAutoscalerUsed); err != nil {
		// check if the machine controller manager is stuck
		isStuck, msg, err2 := a.IsMachineControllerStuck(ctx, worker)
		if err2 != nil {
			a.logger.Error(err2, "failed to check if the machine controller manager pod is stuck after unsuccessfully waiting for all machine deployments to be ready.", "namespace", worker.Namespace)
			// continue in order to return `err` and determine error codes
		}

		if isStuck {
			podList := corev1.PodList{}
			if err2 := a.client.List(ctx, &podList, client.InNamespace(worker.Namespace), client.MatchingLabels{"role": "machine-controller-manager"}); err2 != nil {
				return errors.Wrapf(err2, "failed to list machine controller manager pods for worker (%s/%s)", worker.Namespace, worker.Name)
			}

			for _, pod := range podList.Items {
				if err2 := a.client.Delete(ctx, &pod); err2 != nil {
					return errors.Wrapf(err2, "failed to delete stuck machine controller manager pod for worker (%s/%s)", worker.Namespace, worker.Name)
				}
			}
			a.logger.Info("Successfully deleted stuck machine controller manager pod", "reason", msg, "worker namespace", worker.Namespace, "worker name", worker.Name)
		}

		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("Failed while waiting for all machine deployments to be ready: '%s'", err.Error()))
	}

	// Delete all old machine deployments (i.e. those which were not previously computed but exist in the cluster).
	if err := a.cleanupMachineDeployments(ctx, existingMachineDeployments, wantedMachineDeployments); err != nil {
		return errors.Wrapf(err, "failed to cleanup the machine deployments")
	}

	// Delete all old machine classes (i.e. those which were not previously computed but exist in the cluster).
	if err := a.cleanupMachineClasses(ctx, worker.Namespace, workerDelegate.MachineClassList(), wantedMachineDeployments); err != nil {
		return errors.Wrapf(err, "failed to cleanup the machine classes")
	}

	// Delete all old machine class secrets (i.e. those which were not previously computed but exist in the cluster).
	if err := a.cleanupMachineClassSecrets(ctx, worker.Namespace, wantedMachineDeployments); err != nil {
		return errors.Wrapf(err, "failed to cleanup the orphaned machine class secrets")
	}

	// Wait until all unwanted machine deployments are deleted from the system.
	if err := a.waitUntilUnwantedMachineDeploymentsDeleted(ctx, worker, wantedMachineDeployments); err != nil {
		return errors.Wrapf(err, "error while waiting for all undesired machine deployments to be deleted")
	}

	// Delete MachineSets having number of desired and actual replicas equaling 0
	if err := a.cleanupMachineSets(ctx, worker.Namespace); err != nil {
		return errors.Wrapf(err, "failed to cleanup the machine sets")
	}

	// Scale down machine-controller-manager if shoot is hibernated.
	if isHibernated {
		if err := kubernetes.ScaleDeployment(ctx, a.client, kutil.Key(worker.Namespace, a.mcmName), 0); err != nil {
			return err
		}
	}

	if clusterAutoscalerUsed && !isHibernated {
		if err = a.scaleClusterAutoscaler(ctx, worker, 1); err != nil {
			return err
		}
	}

	if err := a.updateWorkerStatusMachineDeployments(ctx, worker, wantedMachineDeployments, false); err != nil {
		return errors.Wrapf(err, "failed to update the machine deployments in worker status")
	}

	return nil
}

func (a *genericActuator) scaleClusterAutoscaler(ctx context.Context, worker *extensionsv1alpha1.Worker, replicas int32) error {
	return client.IgnoreNotFound(kubernetes.ScaleDeployment(ctx, a.client, kutil.Key(worker.Namespace, v1beta1constants.DeploymentNameClusterAutoscaler), replicas))
}

func (a *genericActuator) deployMachineDeployments(ctx context.Context, cluster *controller.Cluster, worker *extensionsv1alpha1.Worker, existingMachineDeployments *machinev1alpha1.MachineDeploymentList, wantedMachineDeployments extensionsworker.MachineDeployments, classKind string, clusterAutoscalerUsed bool) error {
	for _, deployment := range wantedMachineDeployments {
		var (
			labels                    = map[string]string{"name": deployment.Name}
			existingMachineDeployment = getExistingMachineDeployment(existingMachineDeployments, deployment.Name)
			replicas                  int32
		)

		switch {
		// If the Shoot is hibernated then the machine deployment's replicas should be zero.
		// Also mark all machines for forceful deletion to avoid respecting of PDBs/SLAs in case of cluster hibernation.
		case controller.IsHibernated(cluster):
			replicas = 0
			a.logger.Info("Adding force deletion label on machine objects", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
			if err := a.markAllMachinesForcefulDeletion(ctx, worker.Namespace); err != nil {
				return errors.Wrapf(err, "marking all machines for forceful deletion failed")
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
		case shootIsAwake(controller.IsHibernated(cluster), existingMachineDeployments):
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

		if _, err := controllerutil.CreateOrUpdate(ctx, a.client, machineDeployment, func() error {
			machineDeployment.Spec = machinev1alpha1.MachineDeploymentSpec{
				Replicas:        int32(replicas),
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
func (a *genericActuator) waitUntilWantedMachineDeploymentsAvailable(ctx context.Context, cluster *controller.Cluster, worker *extensionsv1alpha1.Worker, alreadyExistingMachineDeploymentNames sets.String, alreadyExistingMachineClassNames sets.String, wantedMachineDeployments extensionsworker.MachineDeployments, clusterAutoscalerUsed bool) error {
	autoscalerIsScaledDown := false
	workerStatusUpdatedForRollingUpdate := false

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
		ownerReferenceToMachineSet := workerhelper.BuildOwnerToMachineSetsMap(machineSets.Items)

		// Collect the numbers of available and desired replicas.
		for _, deployment := range machineDeployments.Items {
			wantedDeployment := wantedMachineDeployments.FindByName(deployment.Name)

			// Filter out all machine deployments that are not desired (any more).
			if wantedDeployment == nil {
				continue
			}

			machineSets := ownerReferenceToMachineSet[deployment.Name]

			// use `wanted deployment` for these checks, as the existing deployments can be based on an outdated cache
			alreadyExistingMachineDeployment := alreadyExistingMachineDeploymentNames.Has(wantedDeployment.Name)
			newMachineClass := !alreadyExistingMachineClassNames.Has(wantedDeployment.ClassName)

			if alreadyExistingMachineDeployment && newMachineClass {
				a.logger.V(6).Info(fmt.Sprintf("Machine deployment %s is performing a rolling update", deployment.Name))
				// Already existing machine deployments with a rolling update should have > 1 machine sets
				if len(machineSets) <= 1 {
					return retryutils.MinorError(fmt.Errorf("waiting for the MachineControllerManager to create the machine sets for the machine deployment (%s/%s)...", deployment.Namespace, deployment.Name))
				}
			}

			// make sure that the machine set with the correct machine class for the machine deployment is deployed already
			if machineSet := workerhelper.GetMachineSetWithMachineClass(wantedDeployment.Name, wantedDeployment.ClassName, ownerReferenceToMachineSet); machineSet == nil {
				return retryutils.MinorError(fmt.Errorf("waiting for the machine controller manager to create the updated machine set for the machine deployment (%s/%s)...", deployment.Namespace, deployment.Name))
			}

			// If the shoot get hibernated we want to wait until all wanted machine deployments have been deleted
			// entirely.
			numberOfAwakeMachines += deployment.Status.Replicas
			if controller.IsHibernated(cluster) {
				continue
			}

			// If the Shoot is not hibernated we want to wait until all wanted machine deployments have as many
			// available replicas as desired (specified in the .spec.replicas). However, if we see any error in the
			// status of the deployment then we return it.
			for _, failedMachine := range deployment.Status.FailedMachines {
				return retryutils.SevereError(fmt.Errorf("machine %s failed: %s", failedMachine.Name, failedMachine.LastOperation.Description))
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
		case !controller.IsHibernated(cluster):
			// numUpdated == numberOfAwakeMachines waits until the old machine is deleted in the case of a rolling update with maxUnavailability = 0
			// numUnavailable == 0 makes sure that every machine joined the cluster (during creation & in the case of a rolling update with maxUnavailability > 0)
			if numUnavailable == 0 && numUpdated == numberOfAwakeMachines && int(numHealthyDeployments) == len(wantedMachineDeployments) {
				return retryutils.Ok()
			}

			// scale down cluster autoscaler during creation or rolling update
			if clusterAutoscalerUsed && !autoscalerIsScaledDown {
				if err := a.scaleClusterAutoscaler(ctx, worker, 0); err != nil {
					return retryutils.SevereError(err)
				}
				a.logger.V(6).Info("Scaled down the cluster autoscaler", "namespace", worker.Namespace)
				autoscalerIsScaledDown = true
			}

			// update worker status with condition that indicates an ongoing rolling update operation
			if !workerStatusUpdatedForRollingUpdate {
				if err := a.updateWorkerStatusMachineDeployments(ctx, worker, extensionsworker.MachineDeployments{}, true); err != nil {
					return retryutils.SevereError(errors.Wrapf(err, "failed to update the machine status rolling update condition"))
				}
				workerStatusUpdatedForRollingUpdate = true
			}

			if numUnavailable == 0 && numAvailable == numDesired && numUpdated < numberOfAwakeMachines {
				msg = fmt.Sprintf("Waiting until all old machines are drained and terminated. Waiting for %d machine(s)  ...", numberOfAwakeMachines-numUpdated)
				break
			}

			msg = fmt.Sprintf("Waiting until machines are available (%d/%d desired machine(s) available, %d machine(s) pending, %d/%d machinedeployments available)...", numAvailable, numDesired, numUnavailable, numHealthyDeployments, len(wantedMachineDeployments))
		default:
			if numberOfAwakeMachines == 0 {
				return retryutils.Ok()
			}
			msg = fmt.Sprintf("Waiting until all machines have been hibernated (%d still awake)...", numberOfAwakeMachines)
		}

		a.logger.Info(msg, "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		return retryutils.MinorError(errors.New(msg))
	})
}

// waitUntilUnwantedMachineDeploymentsDeleted waits until all the undesired <machineDeployments> are deleted from the
// system. It polls the status every 5 seconds.
func (a *genericActuator) waitUntilUnwantedMachineDeploymentsDeleted(ctx context.Context, worker *extensionsv1alpha1.Worker, wantedMachineDeployments extensionsworker.MachineDeployments) error {
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

				a.logger.Info(fmt.Sprintf("Waiting until unwanted machine deployment is deleted: %s ...", existingMachineDeployment.Name), "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
				return retryutils.MinorError(fmt.Errorf("at least one unwanted machine deployment (%s) still exists", existingMachineDeployment.Name))
			}
		}

		return retryutils.Ok()
	})
}

func (a *genericActuator) updateWorkerStatusMachineDeployments(ctx context.Context, worker *extensionsv1alpha1.Worker, machineDeployments extensionsworker.MachineDeployments, isRollingUpdate bool) error {
	var statusMachineDeployments []extensionsv1alpha1.MachineDeployment

	for _, machineDeployment := range machineDeployments {
		statusMachineDeployments = append(statusMachineDeployments, extensionsv1alpha1.MachineDeployment{
			Name:    machineDeployment.Name,
			Minimum: machineDeployment.Minimum,
			Maximum: machineDeployment.Maximum,
		})
	}

	rollingUpdateCondition, err := buildRollingUpdateCondition(worker.Status.Conditions, isRollingUpdate)
	if err != nil {
		return err
	}

	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, a.client, worker, func() error {
		if len(statusMachineDeployments) > 0 {
			worker.Status.MachineDeployments = statusMachineDeployments
		}

		worker.Status.Conditions = gardencorev1beta1helper.MergeConditions(worker.Status.Conditions, rollingUpdateCondition)
		return nil
	})
}

const (
	// ReasonRollingUpdateProgressing indicates that a rolling update is in progress
	ReasonRollingUpdateProgressing = "RollingUpdateProgressing"
	// ReasonNoRollingUpdate indicates that no rolling update is currently in progress
	ReasonNoRollingUpdate = "NoRollingUpdate"
)

func buildRollingUpdateCondition(conditions []gardencorev1beta1.Condition, isRollingUpdate bool) (gardencorev1beta1.Condition, error) {
	bldr, err := gardencorev1beta1helper.NewConditionBuilder(extensionsv1alpha1.WorkerRollingUpdate)
	if err != nil {
		return gardencorev1beta1.Condition{}, err
	}

	if c := gardencorev1beta1helper.GetCondition(conditions, extensionsv1alpha1.WorkerRollingUpdate); c != nil {
		bldr.WithOldCondition(*c)
	}
	if isRollingUpdate {
		bldr.WithStatus(gardencorev1beta1.ConditionTrue)
		bldr.WithReason(ReasonRollingUpdateProgressing)
		bldr.WithMessage("Rolling update in progress")
	} else {
		bldr.WithStatus(gardencorev1beta1.ConditionFalse)
		bldr.WithReason(ReasonNoRollingUpdate)
		bldr.WithMessage("No rolling update in progress")
	}

	condition, _ := bldr.WithNowFunc(metav1.Now).Build()
	return condition, nil
}

func (a *genericActuator) updateWorkerStatusMachineImages(ctx context.Context, worker *extensionsv1alpha1.Worker, machineImages runtime.Object) error {
	if machineImages == nil {
		return nil
	}

	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, a.client, worker, func() error {
		worker.Status.ProviderStatus = &runtime.RawExtension{Object: machineImages}
		return nil
	})
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
