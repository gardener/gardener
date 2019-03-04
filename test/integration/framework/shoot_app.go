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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	shootop "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/helm/pkg/repo"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultPollInterval  = 5 * time.Second
	dashboardUserName    = "admin"
	elasticsearchLogging = "elasticsearch-logging"
	elasticsearchPort    = 9200
)

// NewGardenTestOperation initializes a new test operation from created shoot Objects that can be used to issue commands against seeds and shoots
func NewGardenTestOperation(ctx context.Context, k8sGardenClient kubernetes.Interface, logger logrus.FieldLogger, shoot *v1beta1.Shoot) (*GardenerTestOperation, error) {
	if err := k8sGardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, shoot); err != nil {
		return nil, err
	}

	seed := &v1beta1.Seed{}
	err := k8sGardenClient.Client().Get(ctx, client.ObjectKey{Name: *shoot.Spec.Cloud.Seed}, seed)
	if err != nil {
		return nil, err
	}

	ns := &corev1.Namespace{}
	if err := k8sGardenClient.Client().Get(ctx, client.ObjectKey{Name: shoot.Namespace}, ns); err != nil {
		return nil, err
	}

	if ns.Labels == nil {
		return nil, fmt.Errorf("namespace %q does not have any labels", ns.Name)
	}
	projectName, ok := ns.Labels[common.ProjectName]
	if !ok {
		return nil, fmt.Errorf("namespace %q did not contain a project label", ns.Name)
	}

	project := &v1beta1.Project{}
	if err := k8sGardenClient.Client().Get(ctx, client.ObjectKey{Name: projectName}, project); err != nil {
		return nil, err
	}

	seedSecretRef := seed.Spec.SecretRef
	seedClient, err := kubernetes.NewClientFromSecret(k8sGardenClient, seedSecretRef.Namespace, seedSecretRef.Name, client.Options{})
	if err != nil {
		return nil, err
	}

	k8sShootClient, err := kubernetes.NewClientFromSecret(seedClient, shootop.ComputeTechnicalID(project.Name, shoot), v1beta1.GardenerName, client.Options{})
	if err != nil {
		return nil, err
	}

	return &GardenerTestOperation{
		Logger: logger,

		GardenClient: k8sGardenClient,
		SeedClient:   seedClient,
		ShootClient:  k8sShootClient,

		Seed:    seed,
		Shoot:   shoot,
		Project: project,
	}, nil
}

// ShootSeedNamespace gets the shoot namespace in the seed
func (o *GardenerTestOperation) ShootSeedNamespace() string {
	return shootop.ComputeTechnicalID(o.Project.Name, o.Shoot)
}

// DownloadKubeconfig downloads the shoot Kubeconfig
func (o *GardenerTestOperation) DownloadKubeconfig(ctx context.Context, client kubernetes.Interface, namespace, name, downloadPath string) error {
	kubeconfig, err := getObjectFromSecret(ctx, client, namespace, name, kubeconfig)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(downloadPath, []byte(kubeconfig), 0755)
	if err != nil {
		return err
	}
	return nil
}

// DashboardAvailable checks if the kubernetes dashboard is available
func (o *GardenerTestOperation) DashboardAvailable(ctx context.Context) error {
	url := fmt.Sprintf("https://api.%s/api/v1/namespaces/kube-system/services/https:kubernetes-dashboard:/proxy", *o.Shoot.Spec.DNS.Domain)
	dashboardPassword, err := o.getAdminPassword(ctx)
	if err != nil {
		return err
	}

	return o.dashboardAvailable(ctx, url, dashboardUserName, dashboardPassword)
}

// KibanaDashboardAvailable checks if Kibana instance in shoot seed namespace is available
func (o *GardenerTestOperation) KibanaDashboardAvailable(ctx context.Context) error {
	url := fmt.Sprintf("https://k.%s.%s.%s/api/status", o.Shoot.Name, o.Project.Name, o.Seed.Spec.IngressDomain)
	loggingPassword, err := o.getLoggingPassword(ctx)
	if err != nil {
		return err
	}

	return o.dashboardAvailable(ctx, url, dashboardUserName, loggingPassword)
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

// WaitUntilDeploymentIsRunning waits until the deployment with <deploymentName> is running
func (o *GardenerTestOperation) WaitUntilDeploymentIsRunning(ctx context.Context, deploymentName, deploymentNamespace string, c kubernetes.Interface) error {
	return wait.PollImmediateUntil(defaultPollInterval, func() (bool, error) {
		deployment := &appsv1.Deployment{}
		if err := c.Client().Get(ctx, client.ObjectKey{Namespace: deploymentNamespace, Name: deploymentName}, deployment); err != nil {
			return false, err
		}

		if err := health.CheckDeployment(deployment); err != nil {
			o.Logger.Infof("Waiting for %s to be ready!!", deploymentName)
			return false, nil
		}

		return true, nil

	}, ctx.Done())
}

// WaitUntilStatefulSetIsRunning waits until the stateful set with <statefulSetName> is running
func (o *GardenerTestOperation) WaitUntilStatefulSetIsRunning(ctx context.Context, statefulSetName, statefulSetNamespace string, c kubernetes.Interface) error {
	return wait.PollImmediateUntil(defaultPollInterval, func() (bool, error) {
		statefulSet := &appsv1.StatefulSet{}
		if err := c.Client().Get(ctx, client.ObjectKey{Namespace: statefulSetNamespace, Name: statefulSetName}, statefulSet); err != nil {
			return false, err
		}

		if err := health.CheckStatefulSet(statefulSet); err != nil {
			o.Logger.Infof("Waiting for %s to be ready!!", statefulSetName)
			return false, nil
		}
		o.Logger.Infof("%s is now ready!!", statefulSetName)
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
				o.Logger.Infof("Waiting for deployments with labels: %v to be ready!!", deploymentLabels.String())
				return false, nil
			}
			return false, err
		}

		for _, deployment := range deployments.Items {
			err = health.CheckDeployment(&deployment)
			if err != nil {
				o.Logger.Infof("Waiting for deployments with labels: %v to be ready!!", deploymentLabels)
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

			if response.StatusCode != http.StatusOK {
				o.Logger.Infof("Guestbook app url: %q is not available yet", guestbookAppURL)
				return false, nil
			}

			responseBytes, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return false, err
			}

			bodyString := string(responseBytes)
			if strings.Contains(bodyString, "404") || strings.Contains(bodyString, "503") {
				o.Logger.Infof("Guestbook app is not ready yet")
				return false, nil
			}
		}
		o.Logger.Infof("Rejoice, the guestbook app urls are available now!")
		return true, nil
	}, ctx.Done())
}

// DownloadChartArtifacts downloads a helm chart from helm stable repo url available in resources/repositories
func (o *GardenerTestOperation) DownloadChartArtifacts(ctx context.Context, helm Helm, chartRepoDestination, chartNameToDownload string) error {
	exists, err := Exists(chartRepoDestination)
	if err != nil {
		return err
	}

	if !exists {
		if err := os.MkdirAll(chartRepoDestination, 0755); err != nil {
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

	chartDownloaded, err := Exists(filepath.Join(chartRepoDestination, strings.Split(chartNameToDownload, "/")[1]))
	if err != nil {
		return err
	}

	if !chartDownloaded {
		chartPath, err = downloadChart(ctx, chartNameToDownload, chartRepoDestination, stableRepo.URL, HelmAccess{
			HelmPath: helm,
		})
		if err != nil {
			return err
		}
		o.Logger.Infof("Chart downloaded to %s", chartPath)
	}
	return nil
}

// DeployChart deploys it on the test shoot
func (o *GardenerTestOperation) DeployChart(ctx context.Context, namespace, chartRepoDestination, chartNameToDeploy string) error {
	renderer, err := chartrenderer.New(o.ShootClient.Kubernetes())
	if err != nil {
		return err
	}
	applier, err := kubernetes.NewApplierForConfig(o.ShootClient.RESTConfig())
	if err != nil {
		return err
	}
	chartApplier := kubernetes.NewChartApplier(renderer, applier)

	chartPathToRender := filepath.Join(chartRepoDestination, chartNameToDeploy)
	return chartApplier.ApplyChartInNamespace(ctx, chartPathToRender, namespace, chartNameToDeploy, nil, nil)
}

// EnsureDirectories creates the repository directory which holds the repositories.yaml config file
func EnsureDirectories(helm Helm) error {
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
func (o *GardenerTestOperation) PodExecByLabel(ctx context.Context, podLabels labels.Selector, podContainer, command, namespace string, client kubernetes.Interface) (io.Reader, error) {
	pod, err := o.getFirstRunningPodWithLabels(ctx, podLabels, namespace, client)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewPodExecutor(client.RESTConfig()).Execute(ctx, pod.Namespace, pod.Name, podContainer, command)
}

// GetFirstNodeInternalIP gets the internal IP of the first node
func (o *GardenerTestOperation) GetFirstNodeInternalIP(ctx context.Context) (string, error) {
	nodes := &corev1.NodeList{}
	err := o.ShootClient.Client().List(ctx, &client.ListOptions{}, nodes)
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

	dashboardPod, err := o.getFirstRunningPodWithLabels(ctx, dashboardLabels, metav1.NamespaceSystem, o.ShootClient)
	if err != nil {
		return "", err
	}

	return dashboardPod.Status.PodIP, nil
}

// GetElasticsearchLogs gets logs for <podName> from the elasticsearch instance in <elasticsearchNamespace>
func (o *GardenerTestOperation) GetElasticsearchLogs(ctx context.Context, elasticsearchNamespace, podName string, client kubernetes.Interface) (*SearchResponse, error) {
	elasticsearchLabels := labels.SelectorFromSet(labels.Set(map[string]string{
		"app":  elasticsearchLogging,
		"role": "logging",
	}))

	now := time.Now()
	index := fmt.Sprintf("logstash-%d.%02d.%02d", now.Year(), now.Month(), now.Day())
	command := fmt.Sprintf("curl http://localhost:%d/%s/_search?q=kubernetes.pod_name:%s", elasticsearchPort, index, podName)
	reader, err := o.PodExecByLabel(ctx, elasticsearchLabels, elasticsearchLogging,
		command, elasticsearchNamespace, client)
	if err != nil {
		return nil, err
	}

	search := &SearchResponse{}
	if err = json.NewDecoder(reader).Decode(search); err != nil {
		return nil, err
	}

	return search, nil
}

// WaitUntilElasticsearchReceivesLogs waits until the elasticsearch instance in <elasticsearchNamespace> receives <expected> logs from <podName>
func (o *GardenerTestOperation) WaitUntilElasticsearchReceivesLogs(ctx context.Context, elasticsearchNamespace, podName string, expected uint64, client kubernetes.Interface) error {
	return wait.PollImmediateUntil(5*time.Second, func() (bool, error) {
		search, err := o.GetElasticsearchLogs(ctx, elasticsearchNamespace, podName, client)
		if err != nil {
			return true, err
		}

		actual := search.Hits.Total
		if expected > actual {
			o.Logger.Infof("Waiting to receive %d logs, currently received %d", expected, actual)
			return false, nil
		} else if expected < search.Hits.Total {
			return true, fmt.Errorf("expected to receive %d logs but was %d", expected, actual)
		}

		o.Logger.Infof("Received all of %d logs", actual)
		return true, nil
	}, ctx.Done())
}
