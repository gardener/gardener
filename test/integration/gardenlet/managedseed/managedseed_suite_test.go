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

package managedseed_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/charts"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	gardenerenvtest "github.com/gardener/gardener/pkg/envtest"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	"github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/utils/namespacefinalizer"
)

func TestManagedSeed(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManagedSeed Controller Integration Test Suite")
}

const testID = "managedseed-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig     *rest.Config
	testEnv        *gardenerenvtest.GardenerTestEnvironment
	testClient     client.Client
	testClientSet  kubernetes.Interface
	shootClientMap clientmap.ClientMap
	mgrClient      client.Client

	testRunID             string
	shootName             string
	gardenNamespaceShoot  string
	gardenNamespaceGarden *corev1.Namespace
	seed                  *gardencorev1beta1.Seed
	err                   error
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	features.RegisterFeatureGates()

	By("starting test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{
				"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,SeedValidator,ShootDNS,ShootQuotaValidator,ShootTolerationRestriction,ManagedSeedShoot,ManagedSeed,ShootManagedSeed,ShootDNS,ShootValidator,SeedValidator",
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

	scheme := kubernetes.GardenScheme
	Expect(resourcesv1alpha1.AddToScheme(scheme)).To(Succeed())

	By("creating test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
	log.Info("Using test run ID for test", "testRunID", testRunID)

	By("creating seed")
	seed = &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "seed-",
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

	DeferCleanup(func() {
		By("deleting seed")
		Expect(testClient.Delete(ctx, seed)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("creating garden namespace for test")
	gardenNamespaceGarden = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "garden-",
		},
	}

	Expect(testClient.Create(ctx, gardenNamespaceGarden)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", gardenNamespaceGarden.Name)

	DeferCleanup(func() {
		By("deleting garden namespace")
		Expect(testClient.Delete(ctx, gardenNamespaceGarden)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		Namespace:          gardenNamespaceGarden.Name,
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: map[client.Object]cache.ObjectSelector{
				&seedmanagementv1alpha1.ManagedSeed{}: {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
				&gardencorev1beta1.Shoot{}:            {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
				&gardencorev1beta1.SecretBinding{}:    {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
				&corev1.Secret{}:                      {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
			},
		}),
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	// The managedseed controller waits for namespaces to be gone, so we need to finalize them as envtest doesn't run the
	// namespace controller.
	Expect((&namespacefinalizer.Reconciler{}).AddToManager(mgr)).To(Succeed())

	By("creating test clientset")
	testClientSet, err = kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(mgr.GetClient()),
		kubernetes.WithRuntimeCache(mgr.GetCache()),
	)
	Expect(err).NotTo(HaveOccurred())

	By("registering controller")
	chartsPath := filepath.Join("..", "..", "..", "..", charts.Path)
	imageVector, err := imagevector.ReadGlobalImageVectorWithEnvOverride(filepath.Join(chartsPath, "images.yaml"))
	Expect(err).NotTo(HaveOccurred())

	shootName = "shoot-" + testRunID
	shootClientMap = fakeclientmap.NewClientMapBuilder().WithClientSetForKey(keys.ForShoot(&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: gardenNamespaceGarden.Name}}), testClientSet).Build()

	cfg := config.GardenletConfiguration{
		Controllers: &config.GardenletControllerConfiguration{
			ManagedSeed: &config.ManagedSeedControllerConfiguration{
				WaitSyncPeriod:   &metav1.Duration{Duration: 5 * time.Millisecond},
				ConcurrentSyncs:  pointer.Int(5),
				SyncJitterPeriod: &metav1.Duration{Duration: 50 * time.Millisecond},
			},
		},
		SeedConfig: &config.SeedConfig{
			SeedTemplate: gardencore.SeedTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: seed.Name,
				},
			},
		},
	}

	gardenNamespaceShoot = "test-" + testRunID
	Expect((&managedseed.Reconciler{
		Config:                *cfg.Controllers.ManagedSeed,
		ChartsPath:            chartsPath,
		GardenNamespaceGarden: gardenNamespaceGarden.Name,
		GardenNamespaceShoot:  gardenNamespaceShoot,
		// limit exponential backoff in tests
		RateLimiter: workqueue.NewWithMaxWaitRateLimiter(workqueue.DefaultControllerRateLimiter(), 100*time.Millisecond),
	}).AddToManager(mgr, cfg, mgr, mgr, shootClientMap, imageVector)).To(Succeed())

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
})
