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
// limitations under the License.

package framework

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/gardener/gardener/pkg/chartrenderer"

	"k8s.io/helm/pkg/repo"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/client-go/tools/cache"
)

const (
	defaultPollInterval = 5 * time.Second
	dashboardUserName   = "admin"
)

// NewGardenTestOperationFromObject  initializes a new test operation from created shoot Objects that can be used to issue commands against seeds and shoots
func NewGardenTestOperationFromObject(shootGardenerTest *ShootGardenerTest, shootObject *v1beta1.Shoot, tillerOptions *TillerOptions) (*GardenerTestOperation, error) {
	// Safety checks
	if shootGardenerTest == nil {
		return nil, ErrEmptyShootGardenerTest
	}

	// Start generating the GardenTestOperation
	ctx := context.Background()
	shootGardenerTest.K8sGardenInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(),
		shootGardenerTest.ShootInformer.Informer().HasSynced,
		shootGardenerTest.SeedInformer.Informer().HasSynced,
		shootGardenerTest.ProjectInfomer.Informer().HasSynced,
		shootGardenerTest.CloudProfileInformer.Informer().HasSynced,
		shootGardenerTest.SecretBindingInformer.Informer().HasSynced) {
		panic("Timed out waiting for Garden caches to sync")
	}

	gardenv1beta1Informers := shootGardenerTest.K8sGardenInformers.Garden().V1beta1()

	seedName := shootObject.Spec.Cloud.Seed
	seedCluster, err := shootGardenerTest.SeedInformer.Lister().Get(*seedName)
	if err != nil {
		return nil, err
	}

	seedObj, err := seed.New(shootGardenerTest.K8sGardenClient, gardenv1beta1Informers, seedCluster)
	if err != nil {
		return nil, err
	}

	gardenObj, err := garden.New(shootGardenerTest.ProjectLister, shootObject.Namespace)
	if err != nil {
		return nil, err
	}

	shootObj, err := shootpkg.New(shootGardenerTest.K8sGardenClient, gardenv1beta1Informers, shootObject, gardenObj.Project.Name, "")
	operation := &GardenerTestOperation{
		ShootGardenerTest: shootGardenerTest,
		Seed:              seedObj,
		Shoot:             shootObj,
	}

	err = operation.initializeSeedClient()
	if err != nil {
		return nil, err
	}

	err = operation.initializeShootClient()
	if err != nil {
		return nil, err
	}
	return operation, nil
}

// NewGardenTestOperationFromName initializes a new test operation for existing shoots that can be used to issue commands against seeds and shoots
func NewGardenTestOperationFromName(shootGardenerTest *ShootGardenerTest, namespace, shootName string, tillerOptions *TillerOptions) (*GardenerTestOperation, error) {
	// Safety checks
	if shootGardenerTest == nil {
		return nil, ErrEmptyShootGardenerTest
	}
	if len(namespace) == 0 || len(shootName) == 0 {
		return nil, ErrEmptyShootNamespaceNames
	}

	gardenObj, err := garden.New(shootGardenerTest.ProjectLister, namespace)
	if err != nil {
		return nil, err
	}

	shoot, err := shootGardenerTest.ShootLister.Shoots(namespace).Get(shootName)
	if err != nil {
		return nil, err
	}

	seedCluster, err := shootGardenerTest.SeedInformer.Lister().Get(*shoot.Spec.Cloud.Seed)
	if err != nil {
		return nil, err
	}

	gardenv1beta1Informers := shootGardenerTest.K8sGardenInformers.Garden().V1beta1()
	seedObj, err := seed.New(shootGardenerTest.K8sGardenClient, gardenv1beta1Informers, seedCluster)
	if err != nil {
		return nil, err
	}

	// set shootGardenerTest.Shoot for operations such as create and delete shoots
	shootGardenerTest.Shoot = shoot

	shootObj, err := shootpkg.New(shootGardenerTest.K8sGardenClient, gardenv1beta1Informers, shoot, gardenObj.Project.Name, "")

	operation := &GardenerTestOperation{
		ShootGardenerTest: shootGardenerTest,
		Seed:              seedObj,
		Shoot:             shootObj,
	}

	err = operation.initializeSeedClient()
	if err != nil {
		return nil, err
	}

	err = operation.initializeShootClient()
	if err != nil {
		return nil, err
	}
	return operation, nil
}

// DownloadKubeconfig downloads the shoot Kubeconfig
func (o *GardenerTestOperation) DownloadKubeconfig(ctx context.Context, downloadPath string) error {
	_, err := getObjectFromSecret(ctx, o.K8sSeedClient, o.Shoot.SeedNamespace, v1beta1.GardenerName, kubeconfig)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(downloadPath, []byte(kubeconfig), 0)
	if err != nil {
		return err

	}
	return nil
}

// DashboardAvailable checks if the dashboard is available
func (o *GardenerTestOperation) DashboardAvailable(ctx context.Context) error {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := http.Client{
		Transport: transport,
		Timeout:   time.Duration(5 * time.Second),
	}

	url := fmt.Sprintf("https://api.%s/api/v1/namespaces/kube-system/services/https:kubernetes-dashboard:/proxy", *o.Shoot.Info.Spec.DNS.Domain)
	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	dashboardPassword, err := o.getAdminPassword(ctx)
	if err != nil {
		return err
	}

	httpRequest.SetBasicAuth(dashboardUserName, dashboardPassword)
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

// HTTPGet performs an HTTP GET request with context
func (o *GardenerTestOperation) HTTPGet(ctx context.Context, url string) (*http.Response, error) {
	httpClient := http.Client{}
	httpRequest, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	httpRequest = httpRequest.WithContext(ctx)
	return httpClient.Do(httpRequest)
}

// WaitUntilDeploymentIsRunning waits until the deployment with <podName> is running
func (o *GardenerTestOperation) WaitUntilDeploymentIsRunning(ctx context.Context, deploymentName, deploymentNamespace string, c kubernetes.Interface) error {
	return wait.PollImmediateUntil(defaultPollInterval, func() (bool, error) {
		deployment := &appsv1.Deployment{}
		if err := c.Client().Get(ctx, client.ObjectKey{Namespace: deploymentNamespace, Name: deploymentName}, deployment); err != nil {
			return false, err
		}

		if err := health.CheckDeployment(deployment); err != nil {
			o.ShootGardenerTest.Logger.Infof("Waiting for %s to be ready!!", deploymentName)
			return false, nil
		}

		return true, nil

	}, ctx.Done())
}

// WaitUntilStatefulSetIsRunning waits until the deployment with <podName> is running
func (o *GardenerTestOperation) WaitUntilStatefulSetIsRunning(ctx context.Context, statefulSetName, statefulSetNamespace string, c kubernetes.Interface) error {
	return wait.PollImmediateUntil(defaultPollInterval, func() (bool, error) {
		statefulSet := &appsv1.StatefulSet{}
		if err := c.Client().Get(ctx, client.ObjectKey{Namespace: statefulSetNamespace, Name: statefulSetName}, statefulSet); err != nil {
			return false, err
		}

		if err := health.CheckStatefulSet(statefulSet); err != nil {
			o.ShootGardenerTest.Logger.Infof("Waiting for %s to be ready!!", statefulSetName)
			return false, nil
		}
		o.ShootGardenerTest.Logger.Infof("%s is now ready!!", statefulSetName)
		return true, nil

	}, ctx.Done())
}

// WaitUntilDeploymentsWithLabelsIsReady wait until pod with labels <podLabels> is running
func (o *GardenerTestOperation) WaitUntilDeploymentsWithLabelsIsReady(ctx context.Context, deploymentLabels labels.Selector, namespace string, client kubernetes.Interface) error {
	return wait.PollImmediateUntil(defaultPollInterval, func() (bool, error) {
		var (
			deployments *appsv1.DeploymentList
			err         error
		)

		deployments, err = getDeploymentListByLabels(ctx, deploymentLabels, namespace, client)
		if err != nil {
			if apierrors.IsNotFound(err) {
				o.ShootGardenerTest.Logger.Infof("Waiting for deployments with labels: %v to be ready!!", deploymentLabels.String())
				return false, nil
			}
			return false, err
		}

		for _, deployment := range deployments.Items {
			err = health.CheckDeployment(&deployment)
			if err != nil {
				o.ShootGardenerTest.Logger.Infof("Waiting for deployments with labels: %v to be ready!!", deploymentLabels)
				return false, nil
			}
		}
		return true, nil
	}, ctx.Done())
}

// WaitUntilGuestbookAppIsAvailable waits until the guestbook app is available and ready to serve requests
func (o *GardenerTestOperation) WaitUntilGuestbookAppIsAvailable(ctx context.Context, guestbookAppUrls []string) error {
	return wait.PollImmediateUntil(defaultPollInterval, func() (bool, error) {
		for _, guestbookAppURL := range guestbookAppUrls {
			response, err := o.HTTPGet(ctx, guestbookAppURL)
			if err != nil {
				return false, err
			}
			responseBytes, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return false, err
			}

			bodyString := string(responseBytes)

			if response.StatusCode != http.StatusOK {
				o.ShootGardenerTest.Logger.Infof("Guestbook app url: %q is not available yet", guestbookAppURL)
				return false, nil
			}

			if strings.Contains(bodyString, "404") || strings.Contains(bodyString, "503") {
				o.ShootGardenerTest.Logger.Infof("Guestbook app is not ready yet")
				return false, nil
			}
		}
		o.ShootGardenerTest.Logger.Infof("Rejoice, the guestbook app urls are available now!")
		return true, nil
	}, ctx.Done())
}

// DownloadAndDeployHelmChart downloads a helm chart from helm stable repo url available in resources/repositories
// and deploys it on the test shoot
func (o *GardenerTestOperation) DownloadAndDeployHelmChart(ctx context.Context, helm Helm, namespace, chartNameToDownload string) error {
	chartRepo := filepath.Join(ResourcesDir, "charts")
	exists, err := Exists(chartRepo)
	if err != nil {
		return err
	}
	if !exists {
		if err := os.MkdirAll(chartRepo, 0755); err != nil {
			return err
		}
	}

	rf, err := repo.LoadRepositoriesFile(helm.RepositoryFile())
	if err != nil {
		return err
	}

	if len(rf.Repositories) == 0 {
		return ErrNoRepositoriesFound
	}

	stableRepo := rf.Repositories[0]
	var chartPath string

	chartDownloaded, err := Exists(filepath.Join(chartRepo, strings.Split(chartNameToDownload, "/")[1]))
	if err != nil {
		return err
	}

	if !chartDownloaded {
		chartPath, err = downloadChart(ctx, chartNameToDownload, chartRepo, stableRepo.URL, HelmAccess{
			HelmPath: helm,
		})
		if err != nil {
			return err
		}
		o.ShootGardenerTest.Logger.Infof("Chart downloaded to %s", chartPath)
	}

	renderer, err := chartrenderer.New(o.K8sShootClient)
	if err != nil {
		return err
	}

	chartName := strings.Split(chartNameToDownload, "/")[1]
	chartPathToRender := filepath.Join(chartRepo, chartName)

	o.ShootGardenerTest.Logger.Infof("Applying Chart %s", chartPathToRender)
	return common.ApplyChartInNamespace(ctx, o.K8sShootClient, renderer, chartPathToRender, chartName, namespace, nil, nil)

}

// EnsureDirectories creates the repository directory which holds the repositories.yaml config file
func (o *GardenerTestOperation) EnsureDirectories(helm Helm) error {
	configDirectories := []string{
		helm.String(),
		helm.Repository(),
	}
	for _, p := range configDirectories {
		fi, err := os.Stat(p)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return fmt.Errorf("%s must be a directory", p)
		}
	}
	return nil
}

// PodExecByLabel executes a command inside pods filtered by label
func (o *GardenerTestOperation) PodExecByLabel(ctx context.Context, podLabels labels.Selector, podContainer, command, namespace string, client kubernetes.Interface) error {
	pod, err := o.getFirstRunningPodWithLabels(ctx, podLabels, namespace, client)
	if err != nil {
		return err
	}

	_, err = kubernetes.NewPodExecutor(o.K8sSeedClient.RESTConfig()).Execute(ctx, pod.Namespace, pod.Name, podContainer, command)
	return err
}

// GetFirstNodeInternalIP gets the internal IP of the first node
func (o *GardenerTestOperation) GetFirstNodeInternalIP(ctx context.Context) (string, error) {
	nodes := &corev1.NodeList{}
	err := o.K8sShootClient.Client().List(ctx, &client.ListOptions{}, nodes)
	if err != nil {
		return "", err
	}

	if len(nodes.Items) > 0 {
		firstNode := nodes.Items[0]
		for _, address := range firstNode.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				return address.Address, nil
			}
		}
	}
	return "", ErrNoInternalIPsForNodeWasFound
}

// GetDashboardPodIP gets the dashboard IP
func (o *GardenerTestOperation) GetDashboardPodIP(ctx context.Context) (string, error) {
	dashboardLabels := labels.SelectorFromSet(labels.Set(map[string]string{
		"app": "kubernetes-dashboard",
	}))

	dashboardPod, err := o.getFirstRunningPodWithLabels(ctx, dashboardLabels, metav1.NamespaceSystem, o.K8sShootClient)
	if err != nil {
		return "", err
	}

	return dashboardPod.Status.PodIP, nil
}
