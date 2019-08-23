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

	"github.com/onsi/ginkgo"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	shootop "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/helm/pkg/repo"
	apiregistrationscheme "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/scheme"
	metricsscheme "k8s.io/metrics/pkg/client/clientset/versioned/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultPollInterval  = 5 * time.Second
	dashboardUserName    = "admin"
	loggingUserName      = "admin"
	elasticsearchLogging = "elasticsearch-logging"
	elasticsearchPort    = 9200
)

// NewGardenTestOperation initializes a new test operation from created shoot Objects that can be used to issue commands against seeds and shoots
func NewGardenTestOperation(ctx context.Context, k8sGardenClient kubernetes.Interface, logger logrus.FieldLogger, shoot *v1beta1.Shoot) (*GardenerTestOperation, error) {
	var (
		seedClient  kubernetes.Interface
		shootClient kubernetes.Interface

		seed             = &v1beta1.Seed{}
		seedCloudProfile = &v1beta1.CloudProfile{}
		project          = &v1beta1.Project{}
	)
	if shoot != nil {
		if err := k8sGardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, shoot); err != nil {
			return nil, errors.Wrap(err, "could not get Shoot in Garden cluster")
		}

		err := k8sGardenClient.Client().Get(ctx, client.ObjectKey{Name: *shoot.Spec.Cloud.Seed}, seed)
		if err != nil {
			return nil, errors.Wrap(err, "could not get Seed from Shoot in Garden cluster")
		}

		err = k8sGardenClient.Client().Get(ctx, client.ObjectKey{Name: seed.Spec.Cloud.Profile}, seedCloudProfile)
		if err != nil {
			return nil, errors.Wrap(err, "could not get Seed's CloudProvider in Garden cluster")
		}

		ns := &corev1.Namespace{}
		if err := k8sGardenClient.Client().Get(ctx, client.ObjectKey{Name: shoot.Namespace}, ns); err != nil {
			return nil, errors.Wrap(err, "could not get the Shoot namespace in Garden cluster")
		}

		if ns.Labels == nil {
			return nil, fmt.Errorf("namespace %q does not have any labels", ns.Name)
		}
		projectName, ok := ns.Labels[common.ProjectName]
		if !ok {
			return nil, fmt.Errorf("namespace %q did not contain a project label", ns.Name)
		}

		if err := k8sGardenClient.Client().Get(ctx, client.ObjectKey{Name: projectName}, project); err != nil {
			return nil, errors.Wrap(err, "could not get Project in Garden cluster")
		}

		seedSecretRef := seed.Spec.SecretRef
		seedClient, err = kubernetes.NewClientFromSecret(k8sGardenClient, seedSecretRef.Namespace, seedSecretRef.Name, kubernetes.WithClientOptions(
			client.Options{
				Scheme: kubernetes.SeedScheme,
			}),
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not construct Seed client")
		}

		shootScheme := runtime.NewScheme()
		shootSchemeBuilder := runtime.NewSchemeBuilder(
			corescheme.AddToScheme,
			apiextensionsscheme.AddToScheme,
			apiregistrationscheme.AddToScheme,
			metricsscheme.AddToScheme,
		)
		err = shootSchemeBuilder.AddToScheme(shootScheme)
		if err != nil {
			return nil, errors.Wrap(err, "could not add schemes to shoot scheme")
		}
		shootClient, err = kubernetes.NewClientFromSecret(seedClient, shootop.ComputeTechnicalID(project.Name, shoot), v1beta1.GardenerName, kubernetes.WithClientOptions(
			client.Options{
				Scheme: shootScheme,
			}),
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not construct Shoot client")
		}
	}

	return &GardenerTestOperation{
		Logger: logger,

		GardenClient: k8sGardenClient,
		SeedClient:   seedClient,
		ShootClient:  shootClient,

		Seed:             seed,
		SeedCloudProfile: seedCloudProfile,
		Shoot:            shoot,
		Project:          project,
	}, nil
}

// ShootSeedNamespace gets the shoot namespace in the seed
func (o *GardenerTestOperation) ShootSeedNamespace() string {
	return shootop.ComputeTechnicalID(o.Project.Name, o.Shoot)
}

// SeedCloudProvider gets the Seed cluster's CloudProvider
func (o *GardenerTestOperation) SeedCloudProvider() (v1beta1.CloudProvider, error) {
	return helper.DetermineCloudProviderInProfile(o.SeedCloudProfile.Spec)
}

// DownloadKubeconfig downloads the shoot Kubeconfig
func (o *GardenerTestOperation) DownloadKubeconfig(ctx context.Context, client kubernetes.Interface, namespace, name, downloadPath string) error {
	kubeconfig, err := GetObjectFromSecret(ctx, client, namespace, name, kubeconfig)
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

// WaitUntilPodIsRunning waits until the pod with <podName> is running
func (o *GardenerTestOperation) WaitUntilPodIsRunning(ctx context.Context, podName, podNamespace string, c kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		pod := &corev1.Pod{}
		if err := c.Client().Get(ctx, client.ObjectKey{Namespace: podNamespace, Name: podName}, pod); err != nil {
			return retry.SevereError(err)
		}
		if !health.IsPodReady(pod) {
			o.Logger.Infof("Waiting for %s to be ready!!", podName)
			return retry.MinorError(fmt.Errorf(`pod "%s/%s" is not ready: %v`, podNamespace, podName, err))
		}

		return retry.Ok()

	})
}

// WaitUntilPodIsRunningWithLabels waits until the pod with <podLabels> is running
func (o *GardenerTestOperation) WaitUntilPodIsRunningWithLabels(ctx context.Context, labels labels.Selector, podNamespace string, c kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		pod, err := o.GetFirstRunningPodWithLabels(ctx, labels, podNamespace, c)

		if err != nil {
			return retry.SevereError(err)
		}

		if !health.IsPodReady(pod) {
			o.Logger.Infof("Waiting for %s to be ready!!", pod.GetName())
			return retry.MinorError(fmt.Errorf(`pod "%s/%s" is not ready: %v`, pod.GetNamespace(), pod.GetName(), err))
		}

		return retry.Ok()

	})
}

// WaitUntilDeploymentIsRunning waits until the deployment with <deploymentName> is running
func (o *GardenerTestOperation) WaitUntilDeploymentIsRunning(ctx context.Context, deploymentName, deploymentNamespace string, c kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		deployment := &appsv1.Deployment{}
		if err := c.Client().Get(ctx, client.ObjectKey{Namespace: deploymentNamespace, Name: deploymentName}, deployment); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckDeployment(deployment); err != nil {
			o.Logger.Infof("Waiting for %s to be ready!!", deploymentName)
			return retry.MinorError(fmt.Errorf("deployment %q is not healthy: %v", deploymentName, err))
		}

		return retry.Ok()
	})
}

// WaitUntilStatefulSetIsRunning waits until the stateful set with <statefulSetName> is running
func (o *GardenerTestOperation) WaitUntilStatefulSetIsRunning(ctx context.Context, statefulSetName, statefulSetNamespace string, c kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		statefulSet := &appsv1.StatefulSet{}
		if err := c.Client().Get(ctx, client.ObjectKey{Namespace: statefulSetNamespace, Name: statefulSetName}, statefulSet); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckStatefulSet(statefulSet); err != nil {
			o.Logger.Infof("Waiting for %s to be ready!!", statefulSetName)
			return retry.MinorError(fmt.Errorf("stateful set %s is not healthy: %v", statefulSetName, err))
		}

		o.Logger.Infof("%s is now ready!!", statefulSetName)
		return retry.Ok()
	})
}

// WaitUntilDeploymentsWithLabelsIsReady wait until pod with labels <podLabels> is running
func (o *GardenerTestOperation) WaitUntilDeploymentsWithLabelsIsReady(ctx context.Context, deploymentLabels labels.Selector, namespace string, client kubernetes.Interface) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		var deployments *appsv1.DeploymentList

		deployments, err = getDeploymentListByLabels(ctx, deploymentLabels, namespace, client)
		if err != nil {
			if apierrors.IsNotFound(err) {
				o.Logger.Infof("Waiting for deployments with labels: %v to be ready!!", deploymentLabels.String())
				return retry.MinorError(fmt.Errorf("no deployments with labels %s exist", deploymentLabels.String()))
			}
			return retry.SevereError(err)
		}

		for _, deployment := range deployments.Items {
			err = health.CheckDeployment(&deployment)
			if err != nil {
				o.Logger.Infof("Waiting for deployments with labels: %v to be ready!!", deploymentLabels)
				return retry.MinorError(fmt.Errorf("deployment %s is not healthy: %v", deployment.Name, err))
			}
		}
		return retry.Ok()
	})
}

// WaitUntilGuestbookAppIsAvailable waits until the guestbook app is available and ready to serve requests
func (o *GardenerTestOperation) WaitUntilGuestbookAppIsAvailable(ctx context.Context, guestbookAppUrls []string) error {
	return retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (done bool, err error) {
		for _, guestbookAppURL := range guestbookAppUrls {
			response, err := o.HTTPGet(ctx, guestbookAppURL)
			if err != nil {
				return retry.SevereError(err)
			}

			if response.StatusCode != http.StatusOK {
				o.Logger.Infof("Guestbook app url: %q is not available yet", guestbookAppURL)
				return retry.MinorError(fmt.Errorf("guestbook app url %q returned status %s", guestbookAppURL, response.Status))
			}

			responseBytes, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return retry.SevereError(err)
			}

			bodyString := string(responseBytes)
			if strings.Contains(bodyString, "404") || strings.Contains(bodyString, "503") {
				o.Logger.Infof("Guestbook app is not ready yet")
				return retry.MinorError(fmt.Errorf("guestbook response body contained an error code"))
			}
		}
		o.Logger.Infof("Rejoice, the guestbook app urls are available now!")
		return retry.Ok()
	})
}

// GetCloudProvider returns the cloud provider for the shoot
func (o *GardenerTestOperation) GetCloudProvider() (v1beta1.CloudProvider, error) {
	return helper.DetermineCloudProviderInShoot(o.Shoot.Spec.Cloud)
}

// DownloadChartArtifacts downloads a helm chart from helm stable repo url available in resources/repositories
func (o *GardenerTestOperation) DownloadChartArtifacts(ctx context.Context, helm Helm, chartRepoDestination, chartNameToDownload, chartVersionToDownload string) error {
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
		chartPath, err = downloadChart(ctx, chartNameToDownload, chartVersionToDownload, chartRepoDestination, stableRepo.URL, HelmAccess{
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
func (o *GardenerTestOperation) DeployChart(ctx context.Context, namespace, chartRepoDestination, chartNameToDeploy string, values map[string]interface{}) error {
	renderer, err := chartrenderer.NewForConfig(o.ShootClient.RESTConfig())
	if err != nil {
		return err
	}
	applier, err := kubernetes.NewApplierForConfig(o.ShootClient.RESTConfig())
	if err != nil {
		return err
	}
	chartApplier := kubernetes.NewChartApplier(renderer, applier)

	chartPathToRender := filepath.Join(chartRepoDestination, chartNameToDeploy)
	return chartApplier.ApplyChartInNamespace(ctx, chartPathToRender, namespace, chartNameToDeploy, values, nil)
}

// AfterEach greps all necessary logs and state of the cluster if the test failed
func (o *GardenerTestOperation) AfterEach(ctx context.Context) {
	if !ginkgo.CurrentGinkgoTestDescription().Failed {
		return
	}

	// dump shoot state if shoot is defined
	if o.Shoot != nil && o.ShootClient != nil {
		ctxIdentifier := fmt.Sprintf("[SHOOT %s]", o.Shoot.Name)
		o.Logger.Info(ctxIdentifier)
		if err := o.dumpDefaultResourcesInAllNamespaces(ctx, ctxIdentifier, o.ShootClient); err != nil {
			o.Logger.Errorf("unable to dump resources from all namespaces in shoot %s: %s", o.Shoot.Name, err.Error())
		}
		if err := o.dumpNodes(ctx, ctxIdentifier, o.ShootClient); err != nil {
			o.Logger.Errorf("unable to dump information of nodes from shoot %s: %s", o.Shoot.Name, err.Error())
		}

		// dump controlplane in the shootnamespace
		if o.Seed != nil && o.SeedClient != nil {
			if err := o.dumpControlplaneInSeed(ctx, o.SeedClient, o.Seed, o.ShootSeedNamespace()); err != nil {
				o.Logger.Errorf("unable to dump controlplane of %s in seed %s: %v", o.Shoot.Name, o.Seed.Name, err)
			}
		}
	}

	// dump gardener status
	if o.GardenClient != nil {
		ctxIdentifier := "[GARDENER]"
		o.Logger.Info(ctxIdentifier)
		if o.Shoot != nil {
			err := o.dumpEventsInNamespace(ctx, ctxIdentifier, o.GardenClient, *o.Project.Spec.Namespace, func(event corev1.Event) bool {
				return event.InvolvedObject.Name == o.Shoot.Name
			})
			if err != nil {
				o.Logger.Errorf("unable to dump Events from project namespace %s in gardener: %s", *o.Project.Spec.Namespace, err.Error())
			}
			return
		}

		err := o.dumpEventsInAllNamespace(ctx, ctxIdentifier, o.GardenClient)
		if err != nil {
			o.Logger.Errorf("unable to dump Events from namespaces gardener: %s", err.Error())
		}
	}
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
	pod, err := o.GetFirstRunningPodWithLabels(ctx, podLabels, namespace, client)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewPodExecutor(client.RESTConfig()).Execute(ctx, pod.Namespace, pod.Name, podContainer, command)
}

// GetDashboardPodIP gets the dashboard IP
func (o *GardenerTestOperation) GetDashboardPodIP(ctx context.Context) (string, error) {
	dashboardLabels := labels.SelectorFromSet(labels.Set(map[string]string{
		"app": "kubernetes-dashboard",
	}))

	dashboardPod, err := o.GetFirstRunningPodWithLabels(ctx, dashboardLabels, metav1.NamespaceSystem, o.ShootClient)
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
	index := fmt.Sprintf("logstash-admin-%d.%02d.%02d", now.Year(), now.Month(), now.Day())
	loggingPassword, err := o.getLoggingPassword(ctx)

	if err != nil {
		return nil, err
	}

	command := fmt.Sprintf("curl http://localhost:%d/%s/_search?q=kubernetes.pod_name:%s --user %s:%s", elasticsearchPort, index, podName, loggingUserName, loggingPassword)
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
	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		search, err := o.GetElasticsearchLogs(ctx, elasticsearchNamespace, podName, client)
		if err != nil {
			return retry.SevereError(err)
		}

		actual := search.Hits.Total
		if expected > actual {
			o.Logger.Infof("Waiting to receive %d logs, currently received %d", expected, actual)
			return retry.MinorError(fmt.Errorf("received only %d/%d logs", actual, expected))
		} else if expected < search.Hits.Total {
			return retry.SevereError(fmt.Errorf("expected to receive %d logs but was %d", expected, actual))
		}

		o.Logger.Infof("Received all of %d logs", actual)
		return retry.Ok()
	})
}
