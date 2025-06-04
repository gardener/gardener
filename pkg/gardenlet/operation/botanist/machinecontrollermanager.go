// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultMachineControllerManager returns a deployer for the machine-controller-manager.
func (b *Botanist) DefaultMachineControllerManager() (machinecontrollermanager.Interface, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameMachineControllerManager, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return machinecontrollermanager.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		machinecontrollermanager.Values{
			Image:           image.String(),
			AutonomousShoot: b.Shoot.IsAutonomous(),
		},
	), nil
}

// DeployMachineControllerManager deploys the machine-controller-manager.
func (b *Botanist) DeployMachineControllerManager(ctx context.Context) error {
	machineDeploymentList := &machinev1alpha1.MachineDeploymentList{}
	if err := b.SeedClientSet.Client().List(ctx, machineDeploymentList, client.InNamespace(b.Shoot.ControlPlaneNamespace)); err != nil {
		return err
	}

	var replicas int32 = 1
	switch {
	// When the shoot shall be deleted then MCM is required to make sure the worker nodes can be removed
	case b.Shoot.GetInfo().DeletionTimestamp != nil:
		replicas = 1
	// if there are any existing machine deployments present with a positive replica count then MCM is needed.
	case machineDeploymentWithPositiveReplicaCountExist(machineDeploymentList):
		repl, err := b.determineControllerReplicas(ctx, v1beta1constants.DeploymentNameMachineControllerManager, 1)
		if err != nil {
			return err
		}
		replicas = repl
	// If the cluster is hibernated then there is no further need of MCM and therefore its desired replicas is 0
	case b.Shoot.HibernationEnabled && b.Shoot.GetInfo().Status.IsHibernated:
		replicas = 0
	// If the cluster is created with hibernation enabled, then desired replicas for MCM is 0
	case b.Shoot.HibernationEnabled && (b.Shoot.GetInfo().Status.LastOperation == nil || b.Shoot.GetInfo().Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate):
		replicas = 0
	// If shoot is either waking up or in the process of hibernation then, MCM is required and therefore its desired
	// replicas is 1
	case b.Shoot.HibernationEnabled != b.Shoot.GetInfo().Status.IsHibernated:
		replicas = 1
	// If the shoot cluster is currently being restored and MCM has not been deployed/scaled up yet, replicas is 0.
	// This is required so that the Machine and MachineSet objects can be restored from the ShootState before
	// MCM is started. The worker actuator is responsible for scaling MCM to 1 replica after the Machine and MachineSet
	// objects are restored and before the worker resource is reconciled.
	case b.IsRestorePhase():
		replicas = 0
	}
	b.Shoot.Components.ControlPlane.MachineControllerManager.SetReplicas(replicas)

	return b.Shoot.Components.ControlPlane.MachineControllerManager.Deploy(ctx)
}

// ScaleMachineControllerManagerToZero scales machine-controller-manager replicas to zero.
func (b *Botanist) ScaleMachineControllerManagerToZero(ctx context.Context) error {
	return kubernetesutils.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameMachineControllerManager}, 0)
}

func machineDeploymentWithPositiveReplicaCountExist(existingMachineDeployments *machinev1alpha1.MachineDeploymentList) bool {
	for _, machineDeployment := range existingMachineDeployments.Items {
		if machineDeployment.Status.Replicas > 0 {
			return true
		}
	}
	return false
}
