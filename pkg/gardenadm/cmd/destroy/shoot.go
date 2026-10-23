// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package destroy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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
