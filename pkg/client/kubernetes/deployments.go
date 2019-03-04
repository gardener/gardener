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
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetDeployment returns a Deployment object.
func (c *Clientset) GetDeployment(namespace, name string) (*appsv1.Deployment, error) {
	return c.Kubernetes().AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
}

// ListDeployments returns the list of Deployments in the given <namespace>.
func (c *Clientset) ListDeployments(namespace string, listOptions metav1.ListOptions) (*appsv1.DeploymentList, error) {
	deployments, err := c.Kubernetes().AppsV1().Deployments(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}
	sort.Slice(deployments.Items, func(i, j int) bool {
		return deployments.Items[i].ObjectMeta.CreationTimestamp.Before(&deployments.Items[j].ObjectMeta.CreationTimestamp)
	})
	return deployments, nil
}

// PatchDeployment patches a Deployment object.
func (c *Clientset) PatchDeployment(namespace, name string, body []byte) (*appsv1.Deployment, error) {
	return c.Kubernetes().AppsV1().Deployments(namespace).Patch(name, types.JSONPatchType, body)
}

// DeleteDeployment deletes a Deployment object.
func (c *Clientset) DeleteDeployment(namespace, name string) error {
	return c.Kubernetes().AppsV1().Deployments(namespace).Delete(name, &defaultDeleteOptions)
}
