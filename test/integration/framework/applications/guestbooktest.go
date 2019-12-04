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
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/test/integration/framework"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ginkgo "github.com/onsi/ginkgo"
	gomega "github.com/onsi/gomega"
)

const (
	// GuestBook is the name of the guestbook k8s resources
	GuestBook = "guestbook"

	// RedisMaster is the name of the redis master deployed by the helm chart
	RedisMaster = "redis-master"

	guestBookTemplateName = "guestbook-app.yaml.tpl"
	helmDeployNamespace   = metav1.NamespaceDefault
	redisChart            = "stable/redis"
	redisChartVersion     = "10.2.1"
)

// GuestBookTest is simple application tests.
// It deploys a guestbook application with a redis backend and a frontend
// that is exposed via an ingress.
type GuestBookTest struct {
	resourcesDir string
	chartRepo    string

	guestBookTpl *template.Template
}

// NewGuestBookTest creates a new guestbook application test
// It takes the path to the test resources.
func NewGuestBookTest(resourcesDir string) (*GuestBookTest, error) {
	if _, err := os.Stat(resourcesDir); err != nil {
		return nil, fmt.Errorf("respurces directory %s does not exist", resourcesDir)
	}

	templateFilepath := filepath.Join(resourcesDir, "templates", guestBookTemplateName)
	if _, err := os.Stat(templateFilepath); err != nil {
		return nil, fmt.Errorf("could not find Guest book template in '%s'", templateFilepath)
	}

	guestBooktpl := template.Must(template.ParseFiles(templateFilepath))
	return &GuestBookTest{
		resourcesDir: resourcesDir,
		chartRepo:    filepath.Join(resourcesDir, "charts"),
		guestBookTpl: guestBooktpl,
	}, nil
}

// WaitUntilPrerequisitesAreReady waits until the redis master is ready.
func (t *GuestBookTest) WaitUntilPrerequisitesAreReady(ctx context.Context, gardenerTestOperations *framework.GardenerTestOperation) {
	err := gardenerTestOperations.WaitUntilStatefulSetIsRunning(ctx, RedisMaster, helmDeployNamespace, gardenerTestOperations.ShootClient)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

// DeployGuestBookApp deploys the redis helmchart and the guestbook app application
func (t *GuestBookTest) DeployGuestBookApp(ctx context.Context, gardenerTestOperations *framework.GardenerTestOperation) {
	shoot := gardenerTestOperations.Shoot
	if !shoot.Spec.Addons.NginxIngress.Enabled {
		ginkgo.Fail("The test requires .spec.kubernetes.addons.nginx-ingress.enabled to be true")
	} else if shoot.Spec.Kubernetes.AllowPrivilegedContainers == nil || !*shoot.Spec.Kubernetes.AllowPrivilegedContainers {
		ginkgo.Fail("The test requires .spec.kubernetes.allowPrivilegedContainers to be true")
	}

	helm := framework.Helm(t.resourcesDir)
	err := framework.EnsureDirectories(helm)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.By("Downloading chart artifacts")
	err = gardenerTestOperations.DownloadChartArtifacts(ctx, helm, t.chartRepo, redisChart, redisChartVersion)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.By("Applying redis chart")
	// redis-slaves are not required for test success
	chartOverrides := map[string]interface{}{
		"cluster": map[string]interface{}{
			"enabled": false,
		},
	}
	if shoot.Spec.Provider.Type == "alicloud" {
		// AliCloud requires a minimum of 20 GB for its PVCs
		chartOverrides["master"] = map[string]interface{}{
			"persistence": map[string]interface{}{
				"size": "20Gi",
			}}
	}
	err = gardenerTestOperations.DeployChart(ctx, helmDeployNamespace, t.chartRepo, "redis", chartOverrides)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	t.WaitUntilPrerequisitesAreReady(ctx, gardenerTestOperations)

	guestBookParams := struct {
		HelmDeployNamespace string
		ShootDNSHost        string
	}{
		helmDeployNamespace,
		fmt.Sprintf("guestbook.ingress.%s", *shoot.Spec.DNS.Domain),
	}

	ginkgo.By("Deploy the guestbook application")
	var writer bytes.Buffer
	err = t.guestBookTpl.Execute(&writer, guestBookParams)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// Apply the guestbook app resources to shoot
	manifestReader := kubernetes.NewManifestReader(writer.Bytes())
	err = gardenerTestOperations.ShootClient.Applier().ApplyManifest(ctx, manifestReader, kubernetes.DefaultApplierOptions)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.By("Guestbook app was deployed successfully!")
}

// Test tests that a deployed guestbook application is working correctly
func (t *GuestBookTest) Test(ctx context.Context, gardenerTestOperations *framework.GardenerTestOperation) {
	shoot := gardenerTestOperations.Shoot

	// define guestbook app urls
	guestBookAppURL := fmt.Sprintf("http://guestbook.ingress.%s", *shoot.Spec.DNS.Domain)
	pushString := fmt.Sprintf("foobar-%s", shoot.Name)
	pushURL := fmt.Sprintf("%s/rpush/guestbook/%s", guestBookAppURL, pushString)
	pullURL := fmt.Sprintf("%s/lrange/guestbook", guestBookAppURL)

	// Check availability of the guestbook app
	err := gardenerTestOperations.WaitUntilGuestbookAppIsAvailable(ctx, []string{guestBookAppURL, pushURL, pullURL})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// Push foobar-<shoot-name> to the guestbook app
	_, err = gardenerTestOperations.HTTPGet(ctx, pushURL)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// Pull foobar
	pullResponse, err := gardenerTestOperations.HTTPGet(ctx, pullURL)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(pullResponse.StatusCode).To(gomega.Equal(http.StatusOK))

	responseBytes, err := ioutil.ReadAll(pullResponse.Body)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// test if foobar-<shoot-name> was pulled successfully
	bodyString := string(responseBytes)
	gomega.Expect(bodyString).To(gomega.ContainSubstring(fmt.Sprintf("foobar-%s", shoot.Name)))
}

// Cleanup cleans up all resources depoyed by the guestbook test
func (t *GuestBookTest) Cleanup(ctx context.Context, shootTestOperations *framework.GardenerTestOperation) {
	// Clean up shoot
	ginkgo.By("Cleaning up guestbook app resources")
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
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = deleteResource(ctx, guestBookDeploymentToDelete)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = deleteResource(ctx, guestBookServiceToDelete)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
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

		)

		err := deleteResource(ctx, redisMasterServiceToDelete)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = deleteResource(ctx, redisMasterStatefulSetToDelete)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

	}
	cleanupGuestbook()
	cleanupRedis()

	err := os.RemoveAll(filepath.Join(t.resourcesDir, "charts"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	err = os.RemoveAll(filepath.Join(t.resourcesDir, "repository", "cache"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.By("redis and the guestbook app have been cleaned up!")
}
