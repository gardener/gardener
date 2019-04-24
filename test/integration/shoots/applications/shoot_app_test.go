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

package applications

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	apiextensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"

	. "github.com/gardener/gardener/test/integration/shoots"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kubeconfig        = flag.String("kubeconfig", "", "the path to the kubeconfig  of the garden cluster that will be used for integration tests")
	shootName         = flag.String("shootName", "", "the name of the shoot we want to test")
	shootNamespace    = flag.String("shootNamespace", "", "the namespace name that the shoot resides in")
	testShootsPrefix  = flag.String("prefix", "", "prefix to use for test shoots")
	logLevel          = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	downloadPath      = flag.String("downloadPath", "/tmp/test", "the path to which you download the kubeconfig")
	shootTestYamlPath = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
	cleanup           = flag.Bool("cleanup", false, "deletes the newly created / existing test shoot after the test suite is done")
)

const (
	GuestbookAppTimeout       = 1800 * time.Second
	DownloadKubeconfigTimeout = 600 * time.Second
	DashboardAvailableTimeout = 60 * time.Minute
	InitializationTimeout     = 600 * time.Second
	FinalizationTimeout       = 1800 * time.Second

	GuestBook             = "guestbook"
	RedisMaster           = "redis-master"
	RedisSalve            = "redis-slave"
	APIServer             = "kube-apiserver"
	GuestBookTemplateName = "guestbook-app.yaml.tpl"

	helmDeployNamespace = metav1.NamespaceDefault
	RedisChart          = "stable/redis"
	RedisChartVersion   = "7.0.0"
)

func validateFlags() {
	if StringSet(*shootTestYamlPath) && StringSet(*shootName) {
		Fail("You can set either the shoot YAML path or specify a shootName to test against")
	}

	if !StringSet(*shootTestYamlPath) && !StringSet(*shootName) {
		Fail("You should either set the shoot YAML path or specify a shootName to test against")
	}

	if StringSet(*shootTestYamlPath) {
		if !FileExists(*shootTestYamlPath) {
			Fail("shoot yaml path is set but invalid")
		}
	}

	if !StringSet(*kubeconfig) {
		Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(*kubeconfig) {
		Fail("kubeconfig path does not exist")
	}
}

var _ = Describe("Shoot application testing", func() {
	var (
		shootGardenerTest   *ShootGardenerTest
		shootTestOperations *GardenerTestOperation
		cloudProvider       v1beta1.CloudProvider
		shootAppTestLogger  *logrus.Logger
		apiserverLabels     labels.Selector
		guestBooktpl        *template.Template
		targetTestShoot     *v1beta1.Shoot
		resourcesDir        = filepath.Join("..", "..", "resources")
		chartRepo           = filepath.Join(resourcesDir, "charts")
	)

	CBeforeSuite(func(ctx context.Context) {
		// validate flags
		validateFlags()
		shootAppTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		// check if a shoot spec is provided, if yes create a shoot object from it and use it for testing
		if StringSet(*shootTestYamlPath) {
			*cleanup = true
			// parse shoot yaml into shoot object and generate random test names for shoots
			_, shootObject, err := CreateShootTestArtifacts(*shootTestYamlPath, *testShootsPrefix)
			Expect(err).NotTo(HaveOccurred())

			shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, shootObject, shootAppTestLogger)
			Expect(err).NotTo(HaveOccurred())

			targetTestShoot, err = shootGardenerTest.CreateShoot(ctx)
			Expect(err).NotTo(HaveOccurred())

			shootTestOperations, err = NewGardenTestOperation(ctx, shootGardenerTest.GardenClient, shootAppTestLogger, targetTestShoot)
			Expect(err).NotTo(HaveOccurred())
		}

		if StringSet(*shootName) {
			var err error
			shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, nil, shootAppTestLogger)
			Expect(err).NotTo(HaveOccurred())

			shoot := &v1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
			shootTestOperations, err = NewGardenTestOperation(ctx, shootGardenerTest.GardenClient, shootAppTestLogger, shoot)
			Expect(err).NotTo(HaveOccurred())
		}
		var err error
		cloudProvider, err = shootTestOperations.GetCloudProvider()
		Expect(err).NotTo(HaveOccurred())

		apiserverLabels = labels.SelectorFromSet(labels.Set(map[string]string{
			"app":  "kubernetes",
			"role": "apiserver",
		}))
		guestBooktpl = template.Must(template.ParseFiles(filepath.Join(TemplateDir, GuestBookTemplateName)))
	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		// Clean up shoot
		By("Cleaning up guestbook app resources")
		deleteResource := func(ctx context.Context, resource runtime.Object) error {
			err := shootTestOperations.ShootClient.Client().Delete(ctx, resource)
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		cleanupGuestbook := func() {
			var (
				guestBookIngressToDelete = &apiextensions.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: helmDeployNamespace,
						Name:      GuestBook,
					}}

				guestBookDeploymentToDelete = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: helmDeployNamespace,
						Name:      GuestBook,
					},
				}

				guestBookServiceToDelete = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: helmDeployNamespace,
						Name:      GuestBook,
					},
				}
			)

			err := deleteResource(ctx, guestBookIngressToDelete)
			Expect(err).NotTo(HaveOccurred())

			err = deleteResource(ctx, guestBookDeploymentToDelete)
			Expect(err).NotTo(HaveOccurred())

			err = deleteResource(ctx, guestBookServiceToDelete)
			Expect(err).NotTo(HaveOccurred())
		}

		cleanupRedis := func() {
			var (
				redisMasterServiceToDelete = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: helmDeployNamespace,
						Name:      RedisMaster,
					},
				}
				redisMasterStatefulSetToDelete = &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: helmDeployNamespace,
						Name:      RedisMaster,
					},
				}

				redisSlaveServiceToDelete = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: helmDeployNamespace,
						Name:      RedisSalve,
					},
				}

				redisSlaveStatefulSetToDelete = &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: helmDeployNamespace,
						Name:      RedisSalve,
					},
				}
			)

			err := deleteResource(ctx, redisMasterServiceToDelete)
			Expect(err).NotTo(HaveOccurred())

			err = deleteResource(ctx, redisMasterStatefulSetToDelete)
			Expect(err).NotTo(HaveOccurred())

			err = deleteResource(ctx, redisSlaveServiceToDelete)
			Expect(err).NotTo(HaveOccurred())

			err = deleteResource(ctx, redisSlaveStatefulSetToDelete)
			Expect(err).NotTo(HaveOccurred())
		}
		cleanupGuestbook()
		cleanupRedis()

		err := os.RemoveAll(filepath.Join(resourcesDir, "charts"))
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(filepath.Join(resourcesDir, "repository", "cache"))
		Expect(err).NotTo(HaveOccurred())

		By("redis and the guestbook app have been cleaned up!")

		if *cleanup {
			By("Cleaning up test shoot")
			err := shootGardenerTest.DeleteShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
		}
	}, FinalizationTimeout)

	CIt("should download shoot kubeconfig successfully", func(ctx context.Context) {
		err := shootTestOperations.DownloadKubeconfig(ctx, shootTestOperations.SeedClient, shootTestOperations.ShootSeedNamespace(), v1beta1.GardenerName, *downloadPath)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Shoot Kubeconfig downloaded successfully to %s", *downloadPath))
	}, DownloadKubeconfigTimeout)

	CIt("should deploy guestbook app successfully", func(ctx context.Context) {
		ctx = context.WithValue(ctx, "name", "guestbook app")

		helm := Helm(resourcesDir)
		err := EnsureDirectories(helm)
		Expect(err).NotTo(HaveOccurred())

		By("Downloading chart artifacts")
		err = shootTestOperations.DownloadChartArtifacts(ctx, helm, chartRepo, RedisChart, RedisChartVersion)
		Expect(err).NotTo(HaveOccurred())

		By("Applying redis chart")
		if cloudProvider == v1beta1.CloudProviderAlicloud {
			// AliCloud requires a minimum of 20 GB for its PVCs
			err = shootTestOperations.DeployChart(ctx, helmDeployNamespace, chartRepo, "redis", map[string]interface{}{"master": map[string]interface{}{
				"persistence": map[string]interface{}{
					"size": "20Gi",
				},
			}})
			Expect(err).NotTo(HaveOccurred())
		} else {
			err = shootTestOperations.DeployChart(ctx, helmDeployNamespace, chartRepo, "redis", nil)
			Expect(err).NotTo(HaveOccurred())
		}

		err = shootTestOperations.WaitUntilStatefulSetIsRunning(ctx, "redis-master", helmDeployNamespace, shootTestOperations.ShootClient)
		Expect(err).NotTo(HaveOccurred())

		redisSlaveLabelSelector := labels.SelectorFromSet(labels.Set(map[string]string{
			"app":  "redis",
			"role": "slave",
		}))

		err = shootTestOperations.WaitUntilDeploymentsWithLabelsIsReady(ctx, redisSlaveLabelSelector, helmDeployNamespace, shootTestOperations.ShootClient)
		Expect(err).NotTo(HaveOccurred())

		guestBookParams := struct {
			HelmDeployNamespace string
			ShootDNSHost        string
		}{
			helmDeployNamespace,
			fmt.Sprintf("guestbook.ingress.%s", *shootTestOperations.Shoot.Spec.DNS.Domain),
		}

		By("Deploy the guestbook application")
		var writer bytes.Buffer
		err = guestBooktpl.Execute(&writer, guestBookParams)
		Expect(err).NotTo(HaveOccurred())

		// Apply the guestbook app resources to shoot
		manifestReader := kubernetes.NewManifestReader(writer.Bytes())
		err = shootTestOperations.ShootClient.Applier().ApplyManifest(ctx, manifestReader, kubernetes.DefaultApplierOptions)
		Expect(err).NotTo(HaveOccurred())

		// define guestbook app urls
		guestBookAppURL := fmt.Sprintf("http://guestbook.ingress.%s", *shootTestOperations.Shoot.Spec.DNS.Domain)
		pushString := fmt.Sprintf("foobar-%s", shootTestOperations.Shoot.Name)
		pushUrl := fmt.Sprintf("%s/rpush/guestbook/%s", guestBookAppURL, pushString)
		pullUrl := fmt.Sprintf("%s/lrange/guestbook", guestBookAppURL)

		// Check availability of the guestbook app
		err = shootTestOperations.WaitUntilGuestbookAppIsAvailable(ctx, []string{guestBookAppURL, pushUrl, pullUrl})
		Expect(err).NotTo(HaveOccurred())

		// Push foobar-<shoot-name> to the guestbook app
		_, err = shootTestOperations.HTTPGet(ctx, pushUrl)
		Expect(err).NotTo(HaveOccurred())

		// Pull foobar
		pullResponse, err := shootTestOperations.HTTPGet(ctx, pullUrl)
		Expect(err).NotTo(HaveOccurred())
		Expect(pullResponse.StatusCode).To(Equal(http.StatusOK))

		responseBytes, err := ioutil.ReadAll(pullResponse.Body)
		Expect(err).NotTo(HaveOccurred())

		// test if foobar-<shoot-name> was pulled successfully
		bodyString := string(responseBytes)
		Expect(bodyString).To(ContainSubstring(fmt.Sprintf("foobar-%s", shootTestOperations.Shoot.Name)))
		By("Guestbook app was deployed successfully!")

	}, GuestbookAppTimeout)

	CIt("Dashboard should be available", func(ctx context.Context) {
		err := shootTestOperations.DashboardAvailable(ctx)
		Expect(err).NotTo(HaveOccurred())
	}, DashboardAvailableTimeout)

	Context("Network Policy Testing", func() {
		var (
			NetworkPolicyTimeout = 1 * time.Minute
			ExecNCOnAPIServer    = func(ctx context.Context, host, port string) error {
				_, err := shootTestOperations.PodExecByLabel(ctx, apiserverLabels, APIServer,
					fmt.Sprintf("apt-get update && apt-get -y install netcat && nc -z -w5 %s %s", host, port), shootTestOperations.ShootSeedNamespace(), shootTestOperations.SeedClient)

				return err
			}

			ItShouldAllowTrafficTo = func(name, host, port string) {
				CIt(fmt.Sprintf("%s should allow connections", name), func(ctx context.Context) {
					Expect(ExecNCOnAPIServer(ctx, host, port)).NotTo(HaveOccurred())
				}, NetworkPolicyTimeout)
			}

			ItShouldBlockTrafficTo = func(name, host, port string) {
				CIt(fmt.Sprintf("%s should allow connections", name), func(ctx context.Context) {
					Expect(ExecNCOnAPIServer(ctx, host, port)).To(HaveOccurred())
				}, NetworkPolicyTimeout)
			}
		)

		ItShouldAllowTrafficTo("seed apiserver/external connection", "kubernetes.default", "443")
		ItShouldAllowTrafficTo("shoot etcd-main", "etcd-main-client", "2379")
		ItShouldAllowTrafficTo("shoot etcd-events", "etcd-events-client", "2379")

		CIt("should allow traffic to the shoot pod range", func(ctx context.Context) {
			dashboardIP, err := shootTestOperations.GetDashboardPodIP(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ExecNCOnAPIServer(ctx, dashboardIP, "8443")).NotTo(HaveOccurred())
		}, NetworkPolicyTimeout)

		CIt("should allow traffic to the shoot node range", func(ctx context.Context) {
			nodeIP, err := shootTestOperations.GetFirstNodeInternalIP(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ExecNCOnAPIServer(ctx, nodeIP, "10250")).NotTo(HaveOccurred())
		}, NetworkPolicyTimeout)

		ItShouldBlockTrafficTo("seed kubernetes dashboard", "kubernetes-dashboard.kube-system", "443")
		ItShouldBlockTrafficTo("shoot grafana", "grafana", "3000")
		ItShouldBlockTrafficTo("shoot kube-controller-manager", "kube-controller-manager", "10252")
		ItShouldBlockTrafficTo("shoot cloud-controller-manager", "cloud-controller-manager", "10253")
		ItShouldBlockTrafficTo("shoot machine-controller-manager", "machine-controller-manager", "10258")

		CIt("should block traffic to the metadataservice", func(ctx context.Context) {
			if cloudProvider == v1beta1.CloudProviderAlicloud {
				Expect(ExecNCOnAPIServer(ctx, "100.100.100.200", "80")).To(HaveOccurred())
			} else {
				Expect(ExecNCOnAPIServer(ctx, "169.254.169.254", "80")).To(HaveOccurred())
			}
		}, NetworkPolicyTimeout)
	})
})
