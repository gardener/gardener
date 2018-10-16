// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"strings"

	"github.com/Masterminds/semver"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
		return nil, fmt.Errorf("Container %q does not belong to this deployment", container)
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

// DeploymentSource is a function that produces a slice of Deployments or an error.
type DeploymentSource func() ([]*appsv1.Deployment, error)

// DeploymentLister is a lister of Deployments.
type DeploymentLister interface {
	// List lists all Deployments that match the given selector.
	List(selector labels.Selector) ([]*appsv1.Deployment, error)
	// Deployments yields a DeploymentNamespaceLister for the given namespace.
	Deployments(namespace string) DeploymentNamespaceLister
}

// DeploymentNamespaceLister is a lister of deployments for a specific namespace.
type DeploymentNamespaceLister interface {
	// List lists all Deployments that match the given selector in the current namespace.
	List(selector labels.Selector) ([]*appsv1.Deployment, error)
	// Get retrieves the Deployment with the given name in the current namespace.
	Get(name string) (*appsv1.Deployment, error)
}

type deploymentLister struct {
	source DeploymentSource
}

type deploymentNamespaceLister struct {
	source    DeploymentSource
	namespace string
}

// NewDeploymentLister creates a new DeploymentLister from the given DeploymentSource.
func NewDeploymentLister(source DeploymentSource) DeploymentLister {
	return &deploymentLister{source: source}
}

func filterDeployments(source DeploymentSource, filter func(*appsv1.Deployment) bool) ([]*appsv1.Deployment, error) {
	deployments, err := source()
	if err != nil {
		return nil, err
	}

	var out []*appsv1.Deployment
	for _, deployment := range deployments {
		if filter(deployment) {
			out = append(out, deployment)
		}
	}
	return out, nil
}

func (d *deploymentLister) List(selector labels.Selector) ([]*appsv1.Deployment, error) {
	return filterDeployments(d.source, func(deployment *appsv1.Deployment) bool {
		return selector.Matches(labels.Set(deployment.Labels))
	})
}

func (d *deploymentLister) Deployments(namespace string) DeploymentNamespaceLister {
	return &deploymentNamespaceLister{
		source:    d.source,
		namespace: namespace,
	}
}

func (d *deploymentNamespaceLister) Get(name string) (*appsv1.Deployment, error) {
	deployments, err := filterDeployments(d.source, func(deployment *appsv1.Deployment) bool {
		return deployment.Namespace == d.namespace && deployment.Name == name
	})
	if err != nil {
		return nil, err
	}

	if len(deployments) == 0 {
		return nil, apierrors.NewNotFound(appsv1.Resource("Deployments"), name)
	}
	return deployments[0], nil
}

func (d *deploymentNamespaceLister) List(selector labels.Selector) ([]*appsv1.Deployment, error) {
	return filterDeployments(d.source, func(deployment *appsv1.Deployment) bool {
		return deployment.Namespace == d.namespace && selector.Matches(labels.Set(deployment.Labels))
	})
}
