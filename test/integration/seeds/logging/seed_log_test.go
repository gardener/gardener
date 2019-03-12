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

package logging_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"html/template"
	"path/filepath"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/gardener/gardener/test/integration/shoots"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	kubeconfig        = flag.String("kubeconfig", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
	shootName         = flag.String("shootName", "", "the name of the shoot we want to test")
	shootNamespace    = flag.String("shootNamespace", "", "the namespace name that the shoot resides in")
	testShootsPrefix  = flag.String("prefix", "", "prefix to use for test shoots")
	shootTestYamlPath = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
	logLevel          = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	logsCount         = flag.Uint64("logsCount", 10000, "the logs count to be logged by the logger application")
	cleanup           = flag.Bool("cleanup", false, "deletes the newly created / existing test shoot after the test suite is done")
)

const (
	LoggingAppTemplateName = "logger-app.yaml.tpl"

	InitializationTimeout           = 15 * time.Minute
	FinalizationTimeout             = 30 * time.Minute
	KibanaAvailableTimeout          = 10 * time.Second
	GetLogsFromElasticsearchTimeout = 5 * time.Minute

	FluentBit = "fluent-bit"
	Fluentd   = "fluentd-es"
	Garden    = "garden"
	Logger    = "logger"
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

var _ = Describe("Seed logging testing", func() {
	var (
		gardenTestOperation *GardenerTestOperation
		shootGardenerTest   *ShootGardenerTest
		seedLogTestLogger   *logrus.Logger
		shootSeedNamespace  string
	)

	CBeforeSuite(func(ctx context.Context) {
		validateFlags()

		seedLogTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)
		k8sGardenClient, err := kubernetes.NewClientFromFile("", *kubeconfig, client.Options{
			Scheme: kubernetes.GardenScheme,
		})
		Expect(err).NotTo(HaveOccurred())

		// Checks whether required logging resources are present.
		// If not, probably the logging feature gate is not enabled.
		hasRequiredResources := func(ctx context.Context, k8sSeedClient kubernetes.Interface) (bool, error) {
			fluentBit := &appsv1.DaemonSet{}
			if err := k8sSeedClient.Client().Get(ctx, client.ObjectKey{Namespace: Garden, Name: FluentBit}, fluentBit); err != nil {
				return false, err
			}

			fluentd := &appsv1.StatefulSet{}
			if err := k8sSeedClient.Client().Get(ctx, client.ObjectKey{Namespace: Garden, Name: Fluentd}, fluentd); err != nil {
				return false, err
			}

			return true, nil
		}

		checkRequiredResources := func(ctx context.Context, k8sSeedClient kubernetes.Interface) {
			isLoggingEnabled, err := hasRequiredResources(ctx, k8sSeedClient)
			if !isLoggingEnabled {
				message := fmt.Sprintf("Error occurred checking for required logging resources in the seed %s namespace. Ensure that the logging feature gate is enabled: %s", Garden, err.Error())
				Fail(message)
			}
		}

		// check if a shoot spec is provided, if yes create a shoot object from it and use it for testing
		if StringSet(*shootTestYamlPath) {
			*cleanup = true
			// parse shoot yaml into shoot object and generate random test names for shoots
			_, shootObject, err := CreateShootTestArtifacts(*shootTestYamlPath, *testShootsPrefix)
			Expect(err).NotTo(HaveOccurred())

			seed := &v1beta1.Seed{}
			err = k8sGardenClient.Client().Get(ctx, client.ObjectKey{Name: *shootObject.Spec.Cloud.Seed}, seed)
			Expect(err).NotTo(HaveOccurred())

			seedSecretRef := seed.Spec.SecretRef
			seedClient, err := kubernetes.NewClientFromSecret(k8sGardenClient, seedSecretRef.Namespace, seedSecretRef.Name, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			By("Checking for required logging resources")
			checkRequiredResources(ctx, seedClient)

			shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, shootObject, seedLogTestLogger)
			Expect(err).NotTo(HaveOccurred())

			targetTestShoot, err := shootGardenerTest.CreateShoot(ctx)
			Expect(err).NotTo(HaveOccurred())

			gardenTestOperation, err = NewGardenTestOperation(ctx, shootGardenerTest.GardenClient, seedLogTestLogger, targetTestShoot)
			Expect(err).NotTo(HaveOccurred())
		}

		if StringSet(*shootName) {
			shoot := &v1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
			gardenTestOperation, err = NewGardenTestOperation(ctx, k8sGardenClient, seedLogTestLogger, shoot)
			Expect(err).NotTo(HaveOccurred())

			By("Checking for required logging resources")
			checkRequiredResources(ctx, gardenTestOperation.SeedClient)
		}

		shootSeedNamespace = gardenTestOperation.ShootSeedNamespace()
	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		deleteResource := func(ctx context.Context, resource runtime.Object) error {
			err := gardenTestOperation.SeedClient.Client().Delete(ctx, resource)
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		if shootSeedNamespace != "" {
			By("Cleaning up logger app resources")
			loggerDeploymentToDelete := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: shootSeedNamespace,
					Name:      Logger,
				},
			}

			err := deleteResource(ctx, loggerDeploymentToDelete)
			Expect(err).NotTo(HaveOccurred())
		}

		if StringSet(*shootTestYamlPath) && *cleanup && shootGardenerTest != nil {
			By("Cleaning up test shoot")
			err := shootGardenerTest.DeleteShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
		}
	}, FinalizationTimeout)

	CIt("Kibana should be available", func(ctx context.Context) {
		err := gardenTestOperation.KibanaDashboardAvailable(ctx)
		Expect(err).NotTo(HaveOccurred())
	}, KibanaAvailableTimeout)

	CIt("should get container logs from elasticsearch", func(ctx context.Context) {
		By("Calculate expected logs count")
		search, err := gardenTestOperation.GetElasticsearchLogs(ctx, shootSeedNamespace, Logger, gardenTestOperation.SeedClient)
		Expect(err).NotTo(HaveOccurred())
		expectedLogsCount := search.Hits.Total + *logsCount
		seedLogTestLogger.Debugf("expected logs count is %d", expectedLogsCount)

		By("Deploy the logger application")
		var loggingAppTpl *template.Template
		loggingAppTpl = template.Must(template.ParseFiles(filepath.Join(TemplateDir, LoggingAppTemplateName)))

		loggerParams := struct {
			HelmDeployNamespace string
			LogsCount           *uint64
		}{
			shootSeedNamespace,
			logsCount,
		}

		var writer bytes.Buffer
		err = loggingAppTpl.Execute(&writer, loggerParams)
		Expect(err).NotTo(HaveOccurred())

		// Apply the logger app deployment to shoot seed namespace
		manifestReader := kubernetes.NewManifestReader(writer.Bytes())
		err = gardenTestOperation.SeedClient.Applier().ApplyManifest(ctx, manifestReader, kubernetes.DefaultApplierOptions)
		Expect(err).NotTo(HaveOccurred())

		By("Wait until logger application is ready")
		loggerLabels := labels.SelectorFromSet(labels.Set(map[string]string{
			"app": Logger,
		}))
		err = gardenTestOperation.WaitUntilDeploymentsWithLabelsIsReady(ctx, loggerLabels, shootSeedNamespace, gardenTestOperation.SeedClient)
		Expect(err).NotTo(HaveOccurred())

		By("Verify elasticsearch received logger application logs")
		err = gardenTestOperation.WaitUntilElasticsearchReceivesLogs(ctx, shootSeedNamespace, Logger, expectedLogsCount, gardenTestOperation.SeedClient)
		Expect(err).NotTo(HaveOccurred())
	}, GetLogsFromElasticsearchTimeout)
})
