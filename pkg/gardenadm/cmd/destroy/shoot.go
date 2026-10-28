// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package destroy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// ensureNotSelfHostedShoot returns an error if the targeted cluster is a self-hosted shoot cluster, e.g., when the
// kubeconfig of the self-hosted shoot is passed as the bootstrap cluster's kubeconfig by mistake. Self-hosted shoot
// clusters run their control plane in the kube-system namespace, hence they have a Cluster resource with this name.
func ensureNotSelfHostedShoot(ctx context.Context, c client.Client) error {
	if err := c.Get(ctx, client.ObjectKey{Name: metav1.NamespaceSystem}, &extensionsv1alpha1.Cluster{}); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("failed checking whether the targeted cluster is a self-hosted shoot cluster: %w", err)
	}

	return fmt.Errorf("the targeted cluster is a self-hosted shoot cluster (Cluster %q exists): "+
		"`gardenadm destroy` must target a bootstrap cluster, e.g., a new kind cluster", metav1.NamespaceSystem)
}

// stopMachineControllers scales down cluster-autoscaler and machine-controller-manager in the self-hosted shoot
// cluster, so that they don't change or recreate the machines while the ShootState is computed and the machines are
// deleted. cluster-autoscaler is stopped first, as it changes the replicas of the MachineDeployments.
func stopMachineControllers(ctx context.Context, log logr.Logger, c client.Client) error {
	for _, name := range []string{
		v1beta1constants.DeploymentNameClusterAutoscaler,
		v1beta1constants.DeploymentNameMachineControllerManager,
	} {
		key := client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: name}

		if err := kubernetesutils.ScaleDeployment(ctx, c, key, 0); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed scaling down deployment %s in the shoot cluster: %w", key, err)
		}

		log.Info("Waiting until deployment is scaled down in the shoot cluster", "deployment", key)
		if err := kubernetesutils.WaitUntilDeploymentScaledToDesiredReplicas(ctx, c, key, 0); err != nil {
			return fmt.Errorf("failed waiting for deployment %s in the shoot cluster to be scaled down: %w", key, err)
		}
	}

	return nil
}

// newShootCleanupBotanist returns a botanist for cleaning up the resources in the self-hosted shoot cluster with the
// same methods as in the deletion of hosted shoots (CleanWebhooks, CleanExtendedAPIs, CleanKubernetesResources). It only
// initializes the fields required by these methods. The cleanup skips the control plane components of the self-hosted
// shoot cluster in kube-system, garden, and the extension namespaces.
func newShootCleanupBotanist(log logr.Logger, shootClientSet kubernetes.Interface, shoot *gardencorev1beta1.Shoot) *botanist.Botanist {
	o := &operation.Operation{
		Logger: log,
		Clock:  clock.RealClock{},
		// The cleanup records its start time on the control plane namespace, which is kube-system in self-hosted shoots.
		SeedClientSet:       shootClientSet,
		SeedNamespaceObject: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem}},
		ShootClientSet:      shootClientSet,
		Shoot:               &shootpkg.Shoot{ControlPlaneNamespace: metav1.NamespaceSystem},
	}
	o.Shoot.SetInfo(shoot.DeepCopy())

	return &botanist.Botanist{Operation: o}
}

// removeNodeLabelsFromMachineState removes the node label from the Machines in the machine state of the ShootState.
// machine-controller-manager runs without a target cluster in the bootstrap cluster, but still looks up the Node of
// Machines with a node label, which fails (it panics because it has no Node lister). The node label is not needed for
// deleting the machines.
func removeNodeLabelsFromMachineState(shootState *gardencorev1beta1.ShootState) error {
	for i, data := range shootState.Spec.Gardener {
		if data.Type != v1beta1constants.DataTypeMachineState {
			continue
		}

		machineState, err := shootstate.UnmarshalMachineState(data.Data.Raw)
		if err != nil {
			return fmt.Errorf("failed unmarshalling machine state: %w", err)
		}

		for _, machineDeploymentState := range machineState.MachineDeployments {
			for j := range machineDeploymentState.Machines {
				delete(machineDeploymentState.Machines[j].Labels, machinev1alpha1.NodeLabelKey)
			}
		}

		raw, err := shootstate.MarshalMachineState(machineState)
		if err != nil {
			return fmt.Errorf("failed marshalling machine state: %w", err)
		}
		shootState.Spec.Gardener[i].Data.Raw = raw
	}

	return nil
}
