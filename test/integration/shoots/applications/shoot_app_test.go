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

/**
	Overview
		- Tests the workload deployment on top of a Shoot

	AfterSuite
		- Cleanup Workload in Shoot

	Test: Create Redis Deployment
	Expected Output
		- Redis Deployment is ready

	Test: Deploy Guestbook Application
	Expected Output
		- Guestbook application should be functioning
 **/

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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kubeconfig     = flag.String("kubecfg", "", "the path to the kubeconfig  of the garden cluster that will be used for integration tests")
	shootName      = flag.String("shoot-name", "", "the name of the shoot we want to test")
	shootNamespace = flag.String("shoot-namespace", "", "the namespace name that the shoot resides in")
	logLevel       = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	downloadPath   = flag.String("download-path", "/tmp/test", "the path to which you download the kubeconfig")
)

const (
	GuestbookAppTimeout       = 1800 * time.Second
	DownloadKubeconfigTimeout = 600 * time.Second
	DashboardAvailableTimeout = 60 * time.Minute
	InitializationTimeout     = 600 * time.Second
	FinalizationTimeout       = 1800 * time.Second
	DumpStateTimeout          = 5 * time.Minute

	GuestBook             = "guestbook"
	RedisMaster           = "redis-master"
	RedisSlave            = "redis-slave"
	GuestBookTemplateName = "guestbook-app.yaml.tpl"

	helmDeployNamespace = metav1.NamespaceDefault
	RedisChart          = "stable/redis"
	RedisChartVersion   = "9.2.0"
)

func validateFlags() {
	if !StringSet(*shootName) {
		Fail("You should specify a shootName to test against")
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
		shootAppTestLogger  *logrus.Logger
		guestBooktpl        *template.Template
		resourcesDir        = filepath.Join("..", "..", "resources")
		chartRepo           = filepath.Join(resourcesDir, "charts")
	)

	CBeforeSuite(func(ctx context.Context) {
		// validate flags
		validateFlags()
		shootAppTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		var err error
		shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, nil, shootAppTestLogger)
		Expect(err).NotTo(HaveOccurred())

		shoot := &gardencorev1alpha1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
		shootTestOperations, err = NewGardenTestOperationWithShoot(ctx, shootGardenerTest.GardenClient, shootAppTestLogger, shoot)
		Expect(err).NotTo(HaveOccurred())

		templateFilepath := filepath.Join(TemplateDir, GuestBookTemplateName)

		if !FileExists(templateFilepath) {
			Fail(fmt.Sprintf("could not find Guest book template in '%s'", templateFilepath))
		}

		guestBooktpl = template.Must(template.ParseFiles(templateFilepath))
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
						Name:      RedisSlave,
					},
				}

				redisSlaveStatefulSetToDelete = &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: helmDeployNamespace,
						Name:      RedisSlave,
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
	}, FinalizationTimeout)

	CAfterEach(func(ctx context.Context) {
		shootTestOperations.AfterEach(ctx)
	}, DumpStateTimeout)

	CIt("should download shoot kubeconfig successfully", func(ctx context.Context) {
		err := shootTestOperations.DownloadKubeconfig(ctx, shootTestOperations.SeedClient, shootTestOperations.ShootSeedNamespace(), gardencorev1alpha1.GardenerName, *downloadPath)
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Shoot Kubeconfig downloaded successfully to %s", *downloadPath))
	}, DownloadKubeconfigTimeout)

	CIt("should deploy guestbook app successfully", func(ctx context.Context) {
		shoot := shootTestOperations.Shoot
		if !shoot.Spec.Addons.NginxIngress.Enabled {
			Fail("The test requires .spec.kubernetes.addons.nginx-ingress.enabled to be true")
		} else if shoot.Spec.Kubernetes.AllowPrivilegedContainers == nil || !*shoot.Spec.Kubernetes.AllowPrivilegedContainers {
			Fail("The test requires .spec.kubernetes.allowPrivilegedContainers to be true")
		}

		ctx = context.WithValue(ctx, "name", "guestbook app")

		helm := Helm(resourcesDir)
		err := EnsureDirectories(helm)
		Expect(err).NotTo(HaveOccurred())

		By("Downloading chart artifacts")
		err = shootTestOperations.DownloadChartArtifacts(ctx, helm, chartRepo, RedisChart, RedisChartVersion)
		Expect(err).NotTo(HaveOccurred())

		By("Applying redis chart")
		if shoot.Spec.Provider.Type == "alicloud" {
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
			fmt.Sprintf("guestbook.ingress.%s", *shoot.Spec.DNS.Domain),
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
		guestBookAppURL := fmt.Sprintf("http://guestbook.ingress.%s", *shoot.Spec.DNS.Domain)
		pushString := fmt.Sprintf("foobar-%s", shoot.Name)
		pushURL := fmt.Sprintf("%s/rpush/guestbook/%s", guestBookAppURL, pushString)
		pullURL := fmt.Sprintf("%s/lrange/guestbook", guestBookAppURL)

		// Check availability of the guestbook app
		err = shootTestOperations.WaitUntilGuestbookAppIsAvailable(ctx, []string{guestBookAppURL, pushURL, pullURL})
		Expect(err).NotTo(HaveOccurred())

		// Push foobar-<shoot-name> to the guestbook app
		_, err = shootTestOperations.HTTPGet(ctx, pushURL)
		Expect(err).NotTo(HaveOccurred())

		// Pull foobar
		pullResponse, err := shootTestOperations.HTTPGet(ctx, pullURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(pullResponse.StatusCode).To(Equal(http.StatusOK))

		responseBytes, err := ioutil.ReadAll(pullResponse.Body)
		Expect(err).NotTo(HaveOccurred())

		// test if foobar-<shoot-name> was pulled successfully
		bodyString := string(responseBytes)
		Expect(bodyString).To(ContainSubstring(fmt.Sprintf("foobar-%s", shoot.Name)))
		By("Guestbook app was deployed successfully!")

	}, GuestbookAppTimeout)

	CIt("Dashboard should be available", func(ctx context.Context) {
		shoot := shootTestOperations.Shoot
		if !shoot.Spec.Addons.KubernetesDashboard.Enabled {
			Fail("The test requires .spec.addons.kubernetes-dashboard.enabled to be be true")
		}

		err := shootTestOperations.DashboardAvailable(ctx)
		Expect(err).NotTo(HaveOccurred())
	}, DashboardAvailableTimeout)

})
