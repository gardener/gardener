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

package framework

import (
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/labels"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	kubecfg    = "kubecfg"
	kubeconfig = "kubeconfig"
	password   = "password"
)

// getFirstRunningPodWithLabels fetches the first running pod with the desired set of labels <labelsMap>
func (o *GardenerTestOperation) getFirstRunningPodWithLabels(ctx context.Context, labelsMap labels.Selector, namespace string, client kubernetes.Interface) (*corev1.Pod, error) {
	var (
		podList *corev1.PodList
		err     error
	)
	podList, err = getPodsByLabels(ctx, labelsMap, client, namespace)
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, ErrNoRunningPodsFound
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return &pod, nil
		}
	}

	return nil, ErrNoRunningPodsFound
}

// getAdminPassword gets the admin password for authenticating against the api
func (o *GardenerTestOperation) getAdminPassword(ctx context.Context) (string, error) {
	return getObjectFromSecret(ctx, o.SeedClient, o.ShootSeedNamespace(), kubecfg, password)
}

func (s *ShootGardenerTest) mergePatch(ctx context.Context, oldShoot, newShoot *v1beta1.Shoot) error {
	patchBytes, err := kubernetesutils.CreateTwoWayMergePatch(oldShoot, newShoot)
	if err != nil {
		return fmt.Errorf("failed to patch bytes")
	}

	_, err = s.GardenClient.Garden().GardenV1beta1().Shoots(s.Shoot.Namespace).Patch(s.Shoot.Name, types.StrategicMergePatchType, patchBytes)
	return err
}

func getPodsByLabels(ctx context.Context, labelsMap labels.Selector, c kubernetes.Interface, namespace string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := c.Client().List(ctx, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelsMap,
	}, podList)
	if err != nil {
		return nil, err
	}
	return podList, nil
}

func getDeploymentListByLabels(ctx context.Context, labelsMap labels.Selector, namespace string, c kubernetes.Interface) (*appsv1.DeploymentList, error) {
	deploymentList := &appsv1.DeploymentList{}
	err := c.Client().List(ctx,
		&client.ListOptions{LabelSelector: labelsMap}, deploymentList)
	if err != nil {
		return nil, err
	}
	return deploymentList, nil
}

func shootCreationCompleted(newStatus *v1beta1.ShootStatus) bool {
	if len(newStatus.Conditions) == 0 {
		return false
	}

	for _, condition := range newStatus.Conditions {
		if condition.Status != gardenv1beta1.ConditionTrue {
			return false
		}
	}

	if newStatus.LastOperation != nil {
		if newStatus.LastOperation.Type == v1beta1.ShootLastOperationTypeCreate ||
			newStatus.LastOperation.Type == v1beta1.ShootLastOperationTypeReconcile {
			if newStatus.LastOperation.State != v1beta1.ShootLastOperationStateSucceeded {
				return false
			}
		}
	}
	return true

}

func getObjectFromSecret(ctx context.Context, k8sClient kubernetes.Interface, namespace, secretName, objectKey string) (string, error) {
	secret := &corev1.Secret{}
	err := k8sClient.Client().Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, secret)
	if err != nil {
		return "", err
	}

	if _, ok := secret.Data[objectKey]; ok {
		return string(secret.Data[objectKey]), nil
	}
	return "", fmt.Errorf("secret %s/%s did not contain object key %q", namespace, secretName, objectKey)
}

// Exists checks if a path exists
func Exists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
