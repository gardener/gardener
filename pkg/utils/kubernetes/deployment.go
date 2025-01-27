// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/retry"
)

// ValidDeploymentContainerImageVersion validates compliance of a deployment container image to a minimum version
func ValidDeploymentContainerImageVersion(deploymentToCheck *appsv1.Deployment, containerName, minimumVersion string) (bool, error) {
	containers := deploymentToCheck.Spec.Template.Spec.Containers
	getContainer := func(container string) (*corev1.Container, error) {
		for _, container := range containers {
			if container.Name == containerName {
				return &container, nil
			}
		}
		return nil, fmt.Errorf("container %q does not belong to this deployment", container)
	}

	containerToCheck, err := getContainer(containerName)
	if err != nil {
		return false, err
	}
	actualVersion, err := semver.NewVersion(strings.Split(containerToCheck.Image, ":")[1])
	if err != nil {
		return false, err
	}
	minVersion, err := semver.NewVersion(minimumVersion)
	if err != nil {
		return false, err
	}
	if actualVersion.LessThan(minVersion) {
		return false, nil
	}

	return true, nil
}

// CurrentReplicaCountForDeployment returns the current replicaCount for the given deployment.
func CurrentReplicaCountForDeployment(ctx context.Context, c client.Client, namespace, deploymentName string) (int32, error) {
	deployment := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: deploymentName}, deployment); err != nil && !apierrors.IsNotFound(err) {
		return 0, err
	}
	if deployment.Spec.Replicas == nil {
		return 0, nil
	}
	return *deployment.Spec.Replicas, nil
}

// HasDeploymentRolloutCompleted checks for the number of updated &
// available replicas to be equal to the deployment's desired replicas count.
// Thus confirming a successful rollout of the deployment.
func HasDeploymentRolloutCompleted(ctx context.Context, c client.Client, namespace, name string) (bool, error) {
	var (
		deployment      = &appsv1.Deployment{}
		desiredReplicas = int32(0)
	)

	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
		return retry.SevereError(err)
	}

	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}

	if deployment.Generation != deployment.Status.ObservedGeneration {
		return retry.MinorError(fmt.Errorf("%q not observed at latest generation (%d/%d)", name,
			deployment.Status.ObservedGeneration, deployment.Generation))
	}

	if deployment.Status.Replicas == desiredReplicas && deployment.Status.UpdatedReplicas == desiredReplicas && deployment.Status.AvailableReplicas == desiredReplicas {
		return retry.Ok()
	}

	return retry.MinorError(fmt.Errorf("deployment %q currently has Updated/Available: %d/%d replicas. Desired: %d", name, deployment.Status.UpdatedReplicas, deployment.Status.AvailableReplicas, desiredReplicas))
}

// WaitUntilDeploymentRolloutIsComplete waits for the number of updated &
// available replicas to be equal to the deployment's desired replicas count.
// It keeps retrying until timeout
func WaitUntilDeploymentRolloutIsComplete(ctx context.Context, client client.Client, namespace string, name string, interval, timeout time.Duration) error {
	return retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		return HasDeploymentRolloutCompleted(ctx, client, namespace, name)
	})
}
