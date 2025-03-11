// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	"github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
	"github.com/gardener/gardener/test/utils/namespacefinalizer"
)

func TestCare(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Gardenlet Shoot Care Suite")
}

const testID = "shoot-care-controller-test"

var (
	ctx       = context.Background()
	log       logr.Logger
	fakeClock *testclock.FakeClock

	restConfig     *rest.Config
	testEnv        *gardenerenvtest.GardenerTestEnvironment
	testClient     client.Client
	testClientSet  kubernetes.Interface
	shootClientMap clientmap.ClientMap
	mgrClient      client.Client

	project       *gardencorev1beta1.Project
	seed          *gardencorev1beta1.Seed
	testNamespace *corev1.Namespace
	testRunID     string
	projectName   string
	seedName      string
	shootName     string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	features.RegisterFeatureGates()

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join("..", "..", "..", "..", "..", "example", "resource-manager", "10-crd-resources.gardener.cloud_managedresources.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-druid.gardener.cloud_etcds.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_backupentries.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_clusters.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_containerruntimes.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_controlplanes.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_dnsrecords.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_extensions.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_infrastructures.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_networks.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_operatingsystemconfigs.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_workers.yaml"),
				},
			},
			ErrorIfCRDPathMissing: true,
		},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootDNS,ShootQuotaValidator,ShootTolerationRestriction,ShootValidator"},
		},
	}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	testSchemeBuilder := runtime.NewSchemeBuilder(
		kubernetes.AddGardenSchemeToScheme,
		extensionsv1alpha1.AddToScheme,
		resourcesv1alpha1.AddToScheme,
		druidcorev1alpha1.AddToScheme,
	)
	testScheme := runtime.NewScheme()
	Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
	projectName = "p-" + testRunID
	seedName = "seed-" + testRunID
	shootName = "shoot-" + testRunID

	By("Create test Namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "garden-" + testRunID,
			Labels: map[string]string{
				testID:                       testRunID,
				v1beta1constants.ProjectName: projectName,
			},
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

	DeferCleanup(func() {
		By("Delete test Namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	project = &gardencorev1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:   projectName,
			Labels: map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.ProjectSpec{
			Namespace: &testNamespace.Name,
		},
	}

	By("Create Project")
	Expect(testClient.Create(ctx, project)).To(Succeed())
	log.Info("Created Project for test", "project", project.Name)

	DeferCleanup(func() {
		By("Delete Project")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, project))).To(Succeed())
	})

	seed = &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name:   seedName,
			Labels: map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.SeedSpec{
			Provider: gardencorev1beta1.SeedProvider{
				Region: "region",
				Type:   "providerType",
				Zones:  []string{"a", "b", "c"},
			},
			Ingress: &gardencorev1beta1.Ingress{
				Domain: "seed.example.com",
				Controller: gardencorev1beta1.IngressController{
					Kind: "nginx",
				},
			},
			DNS: gardencorev1beta1.SeedDNS{
				Provider: &gardencorev1beta1.SeedDNSProvider{
					Type: "providerType",
					SecretRef: corev1.SecretReference{
						Name:      "some-secret",
						Namespace: "some-namespace",
					},
				},
			},
			Networks: gardencorev1beta1.SeedNetworks{
				Pods:     "10.0.0.0/16",
				Services: "10.1.0.0/16",
				Nodes:    ptr.To("10.2.0.0/16"),
			},
		},
	}

	By("Create Seed")
	Expect(testClient.Create(ctx, seed)).To(Succeed())
	log.Info("Created Seed for test", "seed", seed.Name)

	DeferCleanup(func() {
		By("Delete Project")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  testScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			DefaultLabelSelector: labels.SelectorFromSet(labels.Set{testID: testRunID}),
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	// We create the seed namespace in the garden and delete it after every test, so let's ensure it gets finalized.
	Expect((&namespacefinalizer.Reconciler{}).AddToManager(mgr)).To(Succeed())

	By("Setup field indexes")
	Expect(indexer.AddManagedSeedShootName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddControllerInstallationSeedRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())

	By("Create test clientset")
	testClientSet, err = kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(mgr.GetClient()),
		kubernetes.WithRuntimeCache(mgr.GetCache()),
	)
	Expect(err).NotTo(HaveOccurred())

	shootClientMap = fakeclientmap.NewClientMapBuilder().WithClientSetForKey(keys.ForShoot(&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: testNamespace.Name}}), testClientSet).Build()
	fakeClock = testclock.NewFakeClock(time.Now().Round(time.Second))

	By("Register controller")
	Expect((&care.Reconciler{
		SeedClientSet:  testClientSet,
		ShootClientMap: shootClientMap,
		Config: gardenletconfigv1alpha1.GardenletConfiguration{
			Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
				ShootCare: &gardenletconfigv1alpha1.ShootCareControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: 500 * time.Millisecond},
				},
			},
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: seedName,
					},
				},
			},
		},
		Clock:    fakeClock,
		SeedName: seedName,
	}).AddToManager(mgr, mgr)).To(Succeed())

	By("Start manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	DeferCleanup(func() {
		By("Stop manager")
		mgrCancel()
	})
})
