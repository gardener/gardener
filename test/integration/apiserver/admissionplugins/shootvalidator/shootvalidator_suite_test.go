// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootvalidator_test

import (
	"context"
	"testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestShootValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ShootValidator Integration Test Suite")
}

// testID is used for generating test namespace names and other IDs
const testID = "shootvalidator-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client

	testNamespace *corev1.Namespace
	cloudProfile  *gardencorev1beta1.CloudProfile
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("starting test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{
				"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootDNS",
			},
		},
	}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("creating test clients")
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())

	By("creating test namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
			GenerateName: "garden-",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

	DeferCleanup(func() {
		By("deleting test namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("creating Project")
	project := &gardencorev1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
		},
		Spec: gardencorev1beta1.ProjectSpec{
			Namespace: &testNamespace.Name,
		},
	}
	Expect(testClient.Create(ctx, project)).To(Succeed())
	log.Info("Created Project for test", "project", client.ObjectKeyFromObject(project))

	DeferCleanup(func() {
		By("deleting Project")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, project))).To(Succeed())
	})

	By("creating Cloudprofile")
	cloudProfile = &gardencorev1beta1.CloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testID + "-",
		},
		Spec: gardencorev1beta1.CloudProfileSpec{
			Kubernetes: gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.21.1"}},
			},
			MachineImages: []gardencorev1beta1.MachineImage{
				{
					Name: "some-OS",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.1"},
							CRI:              []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
						},
					},
				},
			},
			MachineTypes: []gardencorev1beta1.MachineType{{Name: "large"}},
			Regions:      []gardencorev1beta1.Region{{Name: "region"}},
			Type:         "providerType",
		},
	}
	Expect(testClient.Create(ctx, cloudProfile)).To(Succeed())
	log.Info("Created CloudProfile for test", "cloudProfile", client.ObjectKeyFromObject(cloudProfile))

	DeferCleanup(func() {
		By("deleting CloudProfile")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, cloudProfile))).To(Succeed())
	})
})
