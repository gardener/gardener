// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
func CurrentReplicaCountForDeployment(ctx context.Context, client client.Client, namespace, deploymentName string) (int32, error) {
	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, Key(namespace, deploymentName), deployment); err != nil && !apierrors.IsNotFound(err) {
		return 0, err
	}
	if deployment.Spec.Replicas == nil {
		return 0, nil
	}
	return *deployment.Spec.Replicas, nil
}
