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
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	workerhealthcheck "github.com/gardener/gardener/extensions/pkg/controller/healthcheck/worker"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/extensions/pkg/util"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
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

	// Get list of existing machine class names and list of used machine class secrets.
	existingMachineClassNames, err := a.listMachineClassNames(ctx, worker.Namespace, workerDelegate.MachineClassList())
	if err != nil {
		return err
	}

	// During the time a rolling update happens we do not want the cluster autoscaler to interfer, hence it
	// is removed for now.
	var (
		clusterAutoscalerUsed = extensionsv1alpha1helper.ClusterAutoscalerRequired(worker.Spec.Pools)
		rollingUpdate         = false
	)

	if clusterAutoscalerUsed {
		// Check whether new machine classes have been computed (resulting in a rolling update of the nodes).
		for _, machineDeployment := range wantedMachineDeployments {
			if !existingMachineClassNames.Has(machineDeployment.ClassName) {
				rollingUpdate = true
				break
			}
		}

		// When the Shoot gets hibernated we want to remove the cluster auto scaler so that it does not interfer
		// with Gardeners modifications on the machine deployment's replicas fields.
		if controller.IsHibernated(cluster) || rollingUpdate {
			deployment := &appsv1.Deployment{}
			if err := a.client.Get(ctx, kutil.Key(worker.Namespace, v1beta1constants.DeploymentNameClusterAutoscaler), deployment); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
			} else {
				if err := util.ScaleDeployment(ctx, a.client, deployment, 0); err != nil {
					return err
				}
			}
		}
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

	// Generate machine deployment configuration based on previously computed list of deployments and deploy them.
	a.logger.Info("Deploying the machine deployments", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	if err := a.deployMachineDeployments(ctx, cluster, worker, existingMachineDeployments, wantedMachineDeployments, workerDelegate.MachineClassKind(), clusterAutoscalerUsed); err != nil {
		return errors.Wrapf(err, "failed to generate the machine deployment config")
	}

	// Wait until all generated machine deployments are healthy/available.
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := a.waitUntilWantedMachineDeploymentsAvailable(timeoutCtx, cluster, worker, wantedMachineDeployments); err != nil {
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
	timeoutCtx2, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := a.waitUntilUnwantedMachineDeploymentsDeleted(timeoutCtx2, worker, wantedMachineDeployments); err != nil {
		return errors.Wrapf(err, "error while waiting for all undesired machine deployments to be deleted")
	}

	// Delete MachineSets having number of desired and actual replicas equaling 0
	if err := a.cleanupMachineSets(ctx, worker.Namespace); err != nil {
		return errors.Wrapf(err, "failed to cleanup the machine sets")
	}

	// Scale down machine-controller-manager if shoot is hibernated.
	if controller.IsHibernated(cluster) {
		deployment := &appsv1.Deployment{}
		if err := a.client.Get(ctx, kutil.Key(worker.Namespace, a.mcmName), deployment); err != nil {
			return err
		}
		if err := util.ScaleDeployment(ctx, a.client, deployment, 0); err != nil {
			return err
		}
	}

	if rollingUpdate {
		deployment := &appsv1.Deployment{}
		if err := a.client.Get(ctx, kutil.Key(worker.Namespace, v1beta1constants.DeploymentNameClusterAutoscaler), deployment); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		} else {
			if err := util.ScaleDeployment(ctx, a.client, deployment, 1); err != nil {
				return err
			}
		}
	}

	if err := a.updateWorkerStatusMachineDeployments(ctx, worker, wantedMachineDeployments); err != nil {
		return errors.Wrapf(err, "failed to update the machine deployments in worker status")
	}

	return nil
}

func (a *genericActuator) deployMachineDeployments(ctx context.Context, cluster *controller.Cluster, worker *extensionsv1alpha1.Worker, existingMachineDeployments *machinev1alpha1.MachineDeploymentList, wantedMachineDeployments worker.MachineDeployments, classKind string, clusterAutoscalerUsed bool) error {
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
func (a *genericActuator) waitUntilWantedMachineDeploymentsAvailable(ctx context.Context, cluster *controller.Cluster, worker *extensionsv1alpha1.Worker, wantedMachineDeployments worker.MachineDeployments) error {
	return wait.PollUntil(5*time.Second, func() (bool, error) {
		var numHealthyDeployments, numUpdated, numDesired, numberOfAwakeMachines int32

		// Get the list of all existing machine deployments
		existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
		if err := a.client.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
			return false, err
		}

		// Collect the numbers of ready and desired replicas.
		for _, existingMachineDeployment := range existingMachineDeployments.Items {
			// Filter out all machine deployments that are not desired (any more).
			if !wantedMachineDeployments.HasDeployment(existingMachineDeployment.Name) {
				continue
			}

			// If the shoot get hibernated we want to wait until all wanted machine deployments have been deleted
			// entirely.
			if controller.IsHibernated(cluster) {
				numberOfAwakeMachines += existingMachineDeployment.Status.Replicas
				continue
			}

			// If the Shoot is not hibernated we want to wait until all wanted machine deployments have as many
			// ready replicas as desired (specified in the .spec.replicas). However, if we see any error in the
			// status of the deployment then we return it.
			for _, failedMachine := range existingMachineDeployment.Status.FailedMachines {
				return false, fmt.Errorf("Machine %s failed: %s", failedMachine.Name, failedMachine.LastOperation.Description)
			}

			// If the Shoot is not hibernated we want to wait until all wanted machine deployments have as many
			// ready replicas as desired (specified in the .spec.replicas).
			if workerhealthcheck.CheckMachineDeployment(&existingMachineDeployment) == nil {
				numHealthyDeployments++
			}
			numDesired += existingMachineDeployment.Spec.Replicas
			numUpdated += existingMachineDeployment.Status.UpdatedReplicas
		}

		switch {
		case !controller.IsHibernated(cluster):
			a.logger.Info(fmt.Sprintf("Waiting until all desired machines are ready (%d/%d machine objects up-to-date, %d/%d machinedeployments available)...", numUpdated, numDesired, numHealthyDeployments, len(wantedMachineDeployments)), "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
			if numUpdated >= numDesired && int(numHealthyDeployments) == len(wantedMachineDeployments) {
				return true, nil
			}
		default:
			if numberOfAwakeMachines == 0 {
				return true, nil
			}
			a.logger.Info(fmt.Sprintf("Waiting until all machines have been hibernated (%d still awake)...", numberOfAwakeMachines), "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		}

		return false, nil
	}, ctx.Done())
}

// waitUntilUnwantedMachineDeploymentsDeleted waits until all the undesired <machineDeployments> are deleted from the
// system. It polls the status every 5 seconds.
func (a *genericActuator) waitUntilUnwantedMachineDeploymentsDeleted(ctx context.Context, worker *extensionsv1alpha1.Worker, wantedMachineDeployments worker.MachineDeployments) error {
	return wait.PollUntil(5*time.Second, func() (bool, error) {
		existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
		if err := a.client.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
			return false, err
		}

		atLeastOneUnwantedMachineDeploymentExists := false
		for _, existingMachineDeployment := range existingMachineDeployments.Items {
			if !wantedMachineDeployments.HasDeployment(existingMachineDeployment.Name) {
				atLeastOneUnwantedMachineDeploymentExists = true
				break
			}
		}

		return !atLeastOneUnwantedMachineDeploymentExists, nil
	}, ctx.Done())
}

func (a *genericActuator) updateWorkerStatusMachineDeployments(ctx context.Context, worker *extensionsv1alpha1.Worker, machineDeployments worker.MachineDeployments) error {
	var statusMachineDeployments []extensionsv1alpha1.MachineDeployment

	for _, machineDeployment := range machineDeployments {
		statusMachineDeployments = append(statusMachineDeployments, extensionsv1alpha1.MachineDeployment{
			Name:    machineDeployment.Name,
			Minimum: machineDeployment.Minimum,
			Maximum: machineDeployment.Maximum,
		})
	}

	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, a.client, worker, func() error {
		worker.Status.MachineDeployments = statusMachineDeployments
		return nil
	})
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
