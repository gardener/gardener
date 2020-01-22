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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	scheduler "github.com/gardener/gardener/pkg/scheduler/controller/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubeconfig                = "kubeconfig"
	loggingIngressCredentials = "logging-ingress-credentials"
	password                  = "password"
	token                     = "token"
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
func (o *GardenerTestOperation) GetPodsByLabels(ctx context.Context, labelsSelector labels.Selector, c kubernetes.Interface, namespace string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := c.Client().List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: labelsSelector})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// getAdminToken gets the admin token for authenticating against the api
func (o *GardenerTestOperation) getAdminToken(ctx context.Context) (string, error) {
	return GetObjectFromSecret(ctx, o.SeedClient, o.ShootSeedNamespace(), common.KubecfgSecretName, token)
}

func (o *GardenerTestOperation) getLoggingPassword(ctx context.Context) (string, error) {
	return GetObjectFromSecret(ctx, o.SeedClient, o.ShootSeedNamespace(), loggingIngressCredentials, password)
}

func (o *GardenerTestOperation) dashboardAvailableWithToken(ctx context.Context, url, token string) error {
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

	bearerToken := fmt.Sprintf("Bearer %s", token)
	httpRequest.Header.Set("Authorization", bearerToken)
	httpRequest = httpRequest.WithContext(ctx)

	r, err := httpClient.Do(httpRequest)
	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("dashboard unavailable")
	}

	return nil
}

func (o *GardenerTestOperation) dashboardAvailableWithBasicAuth(ctx context.Context, url, userName, password string) error {
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
	httpRequest = httpRequest.WithContext(ctx)

	r, err := httpClient.Do(httpRequest)
	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("dashboard unavailable")
	}

	return nil
}

func (s *ShootGardenerTest) mergePatch(ctx context.Context, oldShoot, newShoot *gardencorev1beta1.Shoot) error {
	patchBytes, err := kutil.CreateTwoWayMergePatch(oldShoot, newShoot)
	if err != nil {
		return fmt.Errorf("failed to patch bytes: %v", err)
	}

	_, err = s.GardenClient.GardenCore().CoreV1beta1().Shoots(s.Shoot.Namespace).Patch(s.Shoot.Name, types.StrategicMergePatchType, patchBytes)
	return err
}

func getDeploymentListByLabels(ctx context.Context, labelsSelector labels.Selector, namespace string, c kubernetes.Interface) (*appsv1.DeploymentList, error) {
	deploymentList := &appsv1.DeploymentList{}
	if err := c.Client().List(ctx, deploymentList,
		client.MatchingLabelsSelector{Selector: labelsSelector}); err != nil {
		return nil, err
	}

	return deploymentList, nil
}

// ShootCreationCompleted checks if a shoot is successfully reconciled.
func ShootCreationCompleted(newShoot *gardencorev1beta1.Shoot) bool {
	if newShoot.Generation != newShoot.Status.ObservedGeneration {
		return false
	}
	if len(newShoot.Status.Conditions) == 0 && newShoot.Status.LastOperation == nil {
		return false
	}

	for _, condition := range newShoot.Status.Conditions {
		if condition.Status != gardencorev1beta1.ConditionTrue {
			return false
		}
	}

	if newShoot.Status.LastOperation != nil {
		if newShoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate ||
			newShoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeReconcile {
			if newShoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
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
func plantCreationSuccessful(plantStatus *gardencorev1beta1.PlantStatus) bool {
	if len(plantStatus.Conditions) == 0 {
		return false
	}

	for _, condition := range plantStatus.Conditions {
		if condition.Status != gardencorev1beta1.ConditionTrue {
			return false
		}
	}

	if len(plantStatus.ClusterInfo.Kubernetes.Version) == 0 || len(plantStatus.ClusterInfo.Cloud.Type) == 0 || len(plantStatus.ClusterInfo.Cloud.Region) == 0 {
		return false
	}

	return true
}

// plantReconciledWithStatusUnknown determines, based on the plant status.condition and status.ClusterInfo, if the PlantStatus is 'unknown'
func plantReconciledWithStatusUnknown(plantStatus *gardencorev1beta1.PlantStatus) bool {
	if len(plantStatus.Conditions) == 0 {
		return false
	}

	for _, condition := range plantStatus.Conditions {
		if condition.Status != gardencorev1beta1.ConditionFalse && condition.Status != gardencorev1beta1.ConditionUnknown {
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

func shootIsScheduledSuccessfully(newSpec *gardencorev1beta1.ShootSpec) bool {
	return newSpec.SeedName != nil
}

func setHibernation(shoot *gardencorev1beta1.Shoot, hibernated bool) {
	if shoot.Spec.Hibernation != nil {
		shoot.Spec.Hibernation.Enabled = &hibernated
	}
	shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
		Enabled: &hibernated,
	}
}

// NewClientFromServiceAccount returns a kubernetes client for a service account.
func NewClientFromServiceAccount(ctx context.Context, k8sClient kubernetes.Interface, account *corev1.ServiceAccount) (kubernetes.Interface, error) {
	secret := &corev1.Secret{}
	err := k8sClient.Client().Get(ctx, client.ObjectKey{Namespace: account.Namespace, Name: account.Secrets[0].Name}, secret)
	if err != nil {
		return nil, err
	}

	serviceAccountConfig := &rest.Config{
		Host: k8sClient.RESTConfig().Host,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: false,
			CAData:   secret.Data["ca.crt"],
		},
		BearerToken: string(secret.Data["token"]),
	}

	return kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(serviceAccountConfig),
		kubernetes.WithClientOptions(
			client.Options{
				Scheme: kubernetes.GardenScheme,
			}),
	)
}

// GetDeploymentReplicas gets the spec.Replicas count from a deployment
func GetDeploymentReplicas(ctx context.Context, client client.Client, namespace, name string) (*int32, error) {
	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, kutil.Key(namespace, name), deployment); err != nil {
		return nil, err
	}
	replicas := deployment.Spec.Replicas
	return replicas, nil
}

// WaitUntilDeploymentScaled waits until the deployment has the desired replica count in the status
func WaitUntilDeploymentScaled(ctx context.Context, client client.Client, namespace, name string, desiredReplicas int32) error {
	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		deployment := &appsv1.Deployment{}
		if err := client.Get(ctx, kutil.Key(namespace, name), deployment); err != nil {
			return retry.SevereError(err)
		}
		if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != desiredReplicas {
			return retry.SevereError(fmt.Errorf("waiting for deployment scale failed. spec.replicas does not match the desired replicas"))
		}

		if deployment.Status.Replicas == desiredReplicas && deployment.Status.AvailableReplicas == desiredReplicas {
			return retry.Ok()
		}

		return retry.MinorError(fmt.Errorf("deployment currently has '%d' replicas. Desired: %d", deployment.Status.AvailableReplicas, desiredReplicas))
	})
}

// setup the integration test environment by manipulation the Gardener Components (namespace garden) in the garden cluster
func scaleGardenerComponentForIntegrationTests(setupContextTimeout time.Duration, client client.Client, desiredReplicas *int32, name string) (*int32, error) {
	if desiredReplicas == nil {
		return nil, nil
	}

	ctxSetup, cancelCtxSetup := context.WithTimeout(context.Background(), setupContextTimeout)
	defer cancelCtxSetup()

	replicas, err := GetDeploymentReplicas(ctxSetup, client, "garden", name)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve the replica count of the %s deployment: '%v'", name, err)
	}
	if replicas == nil || *replicas == *desiredReplicas {
		return nil, nil
	}
	// scale the scheduler deployment
	if err := kubernetes.ScaleDeployment(ctxSetup, client, kutil.Key("garden", name), *desiredReplicas); err != nil {
		return nil, fmt.Errorf("failed to scale the replica count of the %s deployment: '%v'", name, err)
	}

	// wait until scaled
	if err := WaitUntilDeploymentScaled(ctxSetup, client, "garden", name, *desiredReplicas); err != nil {
		return nil, fmt.Errorf("failed to wait until the %s deployment is scaled: '%v'", name, err)
	}
	return replicas, nil
}

// ScaleGardenerScheduler scales the gardener-scheduler to the desired replicas
func ScaleGardenerScheduler(setupContextTimeout time.Duration, client client.Client, desiredReplicas *int32) (*int32, error) {
	return scaleGardenerComponentForIntegrationTests(setupContextTimeout, client, desiredReplicas, "gardener-scheduler")
}

// ScaleGardenerControllerManager scales the gardener-controller-manager to the desired replicas
func ScaleGardenerControllerManager(setupContextTimeout time.Duration, client client.Client, desiredReplicas *int32) (*int32, error) {
	return scaleGardenerComponentForIntegrationTests(setupContextTimeout, client, desiredReplicas, "gardener-controller-manager")
}
