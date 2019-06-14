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
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	scheduler "github.com/gardener/gardener/pkg/scheduler/controller"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubecfg                   = "kubecfg"
	kubeconfig                = "kubeconfig"
	loggingIngressCredentials = "logging-ingress-credentials"
	password                  = "password"
)

// GetFirstRunningPodWithLabels fetches the first running pod with the desired set of labels <labelsMap>
func (o *GardenerTestOperation) GetFirstRunningPodWithLabels(ctx context.Context, labelsMap labels.Selector, namespace string, client kubernetes.Interface) (*corev1.Pod, error) {
	var (
		podList *corev1.PodList
		err     error
	)
	podList, err = o.GetPodsByLabels(ctx, labelsMap, client, namespace)
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, ErrNoRunningPodsFound
	}

	for _, pod := range podList.Items {
		if health.IsPodReady(&pod) {
			return &pod, nil
		}
	}

	return nil, ErrNoRunningPodsFound
}

// GetPodsByLabels fetches all pods with the desired set of labels <labelsMap>
func (o *GardenerTestOperation) GetPodsByLabels(ctx context.Context, labelsMap labels.Selector, c kubernetes.Interface, namespace string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := c.Client().List(ctx, podList, client.UseListOptions(&client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelsMap,
	}))
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// getAdminPassword gets the admin password for authenticating against the api
func (o *GardenerTestOperation) getAdminPassword(ctx context.Context) (string, error) {
	return GetObjectFromSecret(ctx, o.SeedClient, o.ShootSeedNamespace(), kubecfg, password)
}

func (o *GardenerTestOperation) getLoggingPassword(ctx context.Context) (string, error) {
	return GetObjectFromSecret(ctx, o.SeedClient, o.ShootSeedNamespace(), loggingIngressCredentials, password)
}

func (o *GardenerTestOperation) dashboardAvailable(ctx context.Context, url, userName, password string) error {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := http.Client{
		Transport: transport,
		Timeout:   time.Duration(5 * time.Second),
	}

	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	httpRequest.SetBasicAuth(userName, password)
	httpRequest.WithContext(ctx)

	r, err := httpClient.Do(httpRequest)
	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("dashboard unavailable")
	}

	return nil
}

func (s *ShootGardenerTest) mergePatch(ctx context.Context, oldShoot, newShoot *v1beta1.Shoot) error {
	patchBytes, err := kutil.CreateTwoWayMergePatch(oldShoot, newShoot)
	if err != nil {
		return fmt.Errorf("failed to patch bytes")
	}

	_, err = s.GardenClient.Garden().GardenV1beta1().Shoots(s.Shoot.Namespace).Patch(s.Shoot.Name, types.StrategicMergePatchType, patchBytes)
	return err
}

func getDeploymentListByLabels(ctx context.Context, labelsMap labels.Selector, namespace string, c kubernetes.Interface) (*appsv1.DeploymentList, error) {
	deploymentList := &appsv1.DeploymentList{}
	err := c.Client().List(ctx, deploymentList, client.UseListOptions(&client.ListOptions{LabelSelector: labelsMap}))
	if err != nil {
		return nil, err
	}
	return deploymentList, nil
}

func shootCreationCompleted(newStatus *v1beta1.ShootStatus) bool {
	if len(newStatus.Conditions) == 0 && newStatus.LastOperation == nil {
		return false
	}

	for _, condition := range newStatus.Conditions {
		if condition.Status != gardencorev1alpha1.ConditionTrue {
			return false
		}
	}

	if newStatus.LastOperation != nil {
		if newStatus.LastOperation.Type == gardencorev1alpha1.LastOperationTypeCreate ||
			newStatus.LastOperation.Type == gardencorev1alpha1.LastOperationTypeReconcile {
			if newStatus.LastOperation.State != gardencorev1alpha1.LastOperationStateSucceeded {
				return false
			}
		}
	}

	return true
}

// GetObjectFromSecret returns object from secret
func GetObjectFromSecret(ctx context.Context, k8sClient kubernetes.Interface, namespace, secretName, objectKey string) (string, error) {
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

// plantCreationSuccessful determines, based on the plant condition and Cluster Info, if the Plant was reconciled successfully
func plantCreationSuccessful(plantStatus *gardencorev1alpha1.PlantStatus) bool {
	if len(plantStatus.Conditions) == 0 {
		return false
	}

	for _, condition := range plantStatus.Conditions {
		if condition.Status != gardencorev1alpha1.ConditionTrue {
			return false
		}
	}

	if len(plantStatus.ClusterInfo.Kubernetes.Version) == 0 || len(plantStatus.ClusterInfo.Cloud.Type) == 0 || len(plantStatus.ClusterInfo.Cloud.Region) == 0 {
		return false
	}

	return true
}

// plantReconciledWithStatusUnknown determines, based on the plant status.condition and status.ClusterInfo, if the PlantStatus is 'unknown'
func plantReconciledWithStatusUnknown(plantStatus *gardencorev1alpha1.PlantStatus) bool {
	if len(plantStatus.Conditions) == 0 {
		return false
	}

	for _, condition := range plantStatus.Conditions {
		if condition.Status != gardencorev1alpha1.ConditionFalse && condition.Status != gardencorev1alpha1.ConditionUnknown {
			return false
		}
	}

	if len(plantStatus.ClusterInfo.Kubernetes.Version) != 0 || len(plantStatus.ClusterInfo.Cloud.Type) != 0 && len(plantStatus.ClusterInfo.Cloud.Region) != 0 {
		return false
	}

	return true
}

func shootIsUnschedulable(events []corev1.Event) bool {
	if len(events) == 0 {
		return false
	}

	for _, event := range events {
		if strings.Contains(event.Message, scheduler.MsgUnschedulable) {
			return true
		}
	}
	return false
}

func shootIsScheduledSuccessfully(newSpec *v1beta1.ShootSpec) bool {
	if newSpec.Cloud.Seed != nil {
		return true
	}
	return false
}
