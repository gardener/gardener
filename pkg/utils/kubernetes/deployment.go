// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
