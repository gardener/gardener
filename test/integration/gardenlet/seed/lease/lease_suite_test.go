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

package lease_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	seedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	"github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

func TestSeedLease(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Seed Lease Controller Integration Test Suite")
}

const testID = "seed-lease-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig    *rest.Config
	testEnv       *gardenerenvtest.GardenerTestEnvironment
	testClient    client.Client
	testClientSet kubernetes.Interface

	testNamespace *corev1.Namespace
	testRunID     string

	seed          *gardencorev1beta1.Seed
	fakeClock     *testclock.FakeClock
	healthManager healthz.Manager
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("starting test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator"},
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

	scheme := kubernetes.GardenScheme
	Expect(resourcesv1alpha1.AddToScheme(scheme)).To(Succeed())

	By("creating test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())

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

	By("creating test namespace for test")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "gardener-system-seed-lease-",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
	testRunID = testNamespace.Name

	DeferCleanup(func() {
		By("deleting test namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             kubernetes.GardenScheme,
		MetricsBindAddress: "0",
		Namespace:          testNamespace.Name,
	})
	Expect(err).NotTo(HaveOccurred())

	By("creating test clientset")
	testClientSet, err = kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(mgr.GetClient()),
		kubernetes.WithRuntimeCache(mgr.GetCache()),
	)
	Expect(err).NotTo(HaveOccurred())

	By("starting manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	DeferCleanup(func() {
		By("stopping manager")
		mgrCancel()
	})

	By("starting controller")
	fakeClock = testclock.NewFakeClock(time.Now())
	healthManager = healthz.NewDefaultHealthz()

	// can be removed when the reconciler is tested individually
	features.RegisterFeatureGates()

	c, err := seedcontroller.NewSeedController(ctx, mgr.GetLogger(), mgr, testClientSet, healthManager, nil, nil, nil, &config.GardenletConfiguration{
		Controllers: &config.GardenletControllerConfiguration{
			Seed: &config.SeedControllerConfiguration{
				LeaseResyncSeconds: pointer.Int32(1),
			},
			// can be removed when the reconciler is tested individually
			SeedCare: &config.SeedCareControllerConfiguration{
				SyncPeriod: &metav1.Duration{Duration: time.Minute},
			},
		},
		SeedConfig: &config.SeedConfig{
			SeedTemplate: gardencore.SeedTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: seed.Name,
				},
			},
		},
	}, fakeClock, testNamespace.Name)
	Expect(err).To(Succeed())

	go func() {
		defer GinkgoRecover()
		c.Run(mgrContext, 0)
	}()
})
