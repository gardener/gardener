// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllermanager

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	// IntervalWaitForDeployment is the interval used while waiting for the Deployments to become healthy or deleted.
	IntervalWaitForDeployment = 5 * time.Second
	// TimeoutWaitForDeployment is the timeout used while waiting for the Deployments to become healthy or deleted.
	TimeoutWaitForDeployment = 3 * time.Minute
	// Until is an alias for retry.Until. Exposed for tests.
	Until = retry.Until
)

func (k *kubeControllerManager) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForDeployment)
	defer cancel()

	return Until(timeoutCtx, IntervalWaitForDeployment, health.IsDeploymentUpdated(k.seedClient.APIReader(), k.emptyDeployment()))
}

func (k *kubeControllerManager) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForDeployment)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, k.seedClient.Client(), k.namespace, ManagedResourceName)
}

func (k *kubeControllerManager) WaitForControllerToBeActive(ctx context.Context) error {
	const (
		pollInterval = 5 * time.Second
		pollTimeout  = 90 * time.Second
	)

	// Check whether the kube-controller-manager deployment exists
	if err := k.seedClient.Client().Get(ctx, client.ObjectKey{Namespace: k.namespace, Name: v1beta1constants.DeploymentNameKubeControllerManager}, &appsv1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("kube controller manager deployment not found: %w", err)
		}
		return err
	}

	return retry.UntilTimeout(ctx, pollInterval, pollTimeout, func(ctx context.Context) (done bool, err error) {
		podList := &corev1.PodList{}
		err = k.seedClient.Client().List(ctx, podList,
			client.InNamespace(k.namespace),
			client.MatchingLabels(map[string]string{
				v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
				v1beta1constants.LabelRole: v1beta1constants.LabelControllerManager,
			}))
		if err != nil {
			return retry.SevereError(fmt.Errorf("could not check whether controller %s is active: %w", v1beta1constants.DeploymentNameKubeControllerManager, err))
		}

		// Check that one replica of the controller exists.
		if len(podList.Items) < 1 {
			k.log.Info("Waiting for kube-controller-manager to have at least one replica")
			return retry.MinorError(fmt.Errorf("controller %s is not active", v1beta1constants.DeploymentNameKubeControllerManager))
		}

		// Check that the existing replicas are not getting deleted.
		for _, pod := range podList.Items {
			if pod.DeletionTimestamp != nil {
				k.log.Info("Waiting for a new replica of kube-controller-manager")
				return retry.MinorError(fmt.Errorf("controller %s is not active", v1beta1constants.DeploymentNameKubeControllerManager))
			}
		}

		// Check if the controller is active by reading its leader election record.
		lock := resourcelock.LeasesResourceLock

		leaderElectionRecord, err := kubernetesutils.ReadLeaderElectionRecord(ctx, k.shootClient, lock, metav1.NamespaceSystem, v1beta1constants.DeploymentNameKubeControllerManager)
		if err != nil {
			return retry.SevereError(fmt.Errorf("could not check whether controller %s is active: %w", v1beta1constants.DeploymentNameKubeControllerManager, err))
		}

		lastRenew := metav1.Now().UTC().Sub(leaderElectionRecord.RenewTime.UTC())
		defaultRenewalTime := 2 * time.Second
		if lastRenew <= defaultRenewalTime*2 {
			return retry.Ok()
		}

		k.log.Info("Waiting for kube-controller-manager to be active")
		return retry.MinorError(fmt.Errorf("controller %s is not active", v1beta1constants.DeploymentNameKubeControllerManager))
	})
}
