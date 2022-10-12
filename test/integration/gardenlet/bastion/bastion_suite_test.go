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

package bastion_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/bastion"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"k8s.io/apimachinery/pkg/util/uuid"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestControllerInstallationCare(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bastion Controller Integration Test Suite")
}

const (
	testID              = "bastion-controller-test"
	seedClusterIdentity = "seed"
	syncPeriod          = 100 * time.Millisecond
)

var (
	testRunID string

	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client
	mgrClient  client.Reader

	gardenNamespace *corev1.Namespace
	shootNamespace  *corev1.Namespace
	seed            *gardencorev1beta1.Seed
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("starting test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{filepath.Join("..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_bastions.yaml")},
			},
			ErrorIfCRDPathMissing: true,
		},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,Bastion,DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootDNS,ShootQuotaValidator,ShootTolerationRestriction,ShootValidator"},
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

	By("creating testClient")
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
	log.Info("Using test run ID for test", "testRunID", testRunID)

	By("creating seed")
	seed = &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "seed-",
			Labels:       map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.SeedSpec{
			Provider: gardencorev1beta1.SeedProvider{
				Region: "region",
				Type:   "providerType",
			},
			Networks: gardencorev1beta1.SeedNetworks{
				Pods:     "10.0.0.0/16",
				Services: "10.1.0.0/16",
				Nodes:    pointer.String("10.2.0.0/16"),
			},
			DNS: gardencorev1beta1.SeedDNS{
				IngressDomain: pointer.String("someingress.example.com"),
			},
		},
	}
	Expect(testClient.Create(ctx, seed)).To(Succeed())
	log.Info("Created Seed for test", "seed", seed.Name)

	patch := client.MergeFrom(seed.DeepCopy())
	seed.Status.ClusterIdentity = pointer.String(seedClusterIdentity)
	Expect(testClient.Status().Patch(ctx, seed, patch)).To(Succeed())

	DeferCleanup(func() {
		By("deleting seed")
		Expect(testClient.Delete(ctx, seed)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("creating garden namespace for test")
	gardenNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "garden-",
		},
	}

	Expect(testClient.Create(ctx, gardenNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", gardenNamespace.Name)
	testRunID = gardenNamespace.Name

	DeferCleanup(func() {
		By("deleting test namespace")
		Expect(testClient.Delete(ctx, gardenNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("creating shoot namespace for test")
	shootNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "shoot--",
		},
	}

	Expect(testClient.Create(ctx, shootNamespace)).To(Succeed())
	log.Info("Created shoot Namespace for test", "namespaceName", shootNamespace.Name)

	DeferCleanup(func() {
		By("deleting shoot namespace")
		Expect(testClient.Delete(ctx, shootNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             kubernetes.GardenScheme,
		MetricsBindAddress: "0",
		NewCache:           cache.BuilderWithOptions(cache.Options{}),
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	Expect(resourcesv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())

	By("starting manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	By("registering controller")
	c, err := bastion.NewBastionController(ctx, mgr.GetLogger(), mgr, mgr, &config.GardenletConfiguration{
		Controllers: &config.GardenletControllerConfiguration{
			Bastion: &config.BastionControllerConfiguration{
				ConcurrentSyncs: pointer.Int(5),
			},
		},
		SeedConfig: &config.SeedConfig{
			SeedTemplate: gardencore.SeedTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: seed.Name,
				},
			},
		},
	})
	Expect(err).To(Succeed())

	By("starting controller")
	go func() {
		defer GinkgoRecover()
		c.Run(mgrContext, 5)
	}()

	DeferCleanup(func() {
		By("stopping manager")
		mgrCancel()
	})
})
