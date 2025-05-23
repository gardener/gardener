// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	ocifake "github.com/gardener/gardener/pkg/utils/oci/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

func TestGardenlet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Gardenlet Gardenlet Suite")
}

const testID = "gardenlet-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig    *rest.Config
	testEnv       *gardenerenvtest.GardenerTestEnvironment
	testClient    client.Client
	testClientSet kubernetes.Interface
	mgrClient     client.Client

	testRunID           string
	gardenNamespaceSeed *corev1.Namespace
	seed                *gardencorev1beta1.Seed
	err                 error

	fakeRegistry  *ocifake.Registry
	ociRepository = gardencorev1.OCIRepository{Repository: ptr.To("gardenlet"), Tag: ptr.To("test")}
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	features.RegisterFeatureGates()

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{
				"--disable-admission-plugins=ResourceReferenceManager,ExtensionValidator,SeedValidator",
			},
		},
	}

	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	testSchemeBuilder := runtime.NewSchemeBuilder(
		kubernetes.AddGardenSchemeToScheme,
		resourcesv1alpha1.AddToScheme,
	)
	testScheme := runtime.NewScheme()
	Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
	log.Info("Using test run ID for test", "testRunID", testRunID)

	By("Create seed")
	seed = &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "seed-",
		},
		Spec: gardencorev1beta1.SeedSpec{
			Provider: gardencorev1beta1.SeedProvider{
				Region: "region",
				Type:   "providerType",
			},
			Ingress: &gardencorev1beta1.Ingress{
				Domain: "seed.example.com",
				Controller: gardencorev1beta1.IngressController{
					Kind: "nginx",
				},
			},
			DNS: gardencorev1beta1.SeedDNS{
				Provider: &gardencorev1beta1.SeedDNSProvider{
					Type: "provider",
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
	Expect(testClient.Create(ctx, seed)).To(Succeed())
	log.Info("Created Seed for test", "seed", seed.Name)

	DeferCleanup(func() {
		By("Delete seed")
		Expect(testClient.Delete(ctx, seed)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Create garden namespaces for test")
	gardenNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "garden"}}
	Expect(testClient.Create(ctx, gardenNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", gardenNamespace.Name)
	DeferCleanup(func() {
		By("Delete garden namespace")
		Expect(testClient.Delete(ctx, gardenNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	gardenNamespaceSeed = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "garden-"}}
	Expect(testClient.Create(ctx, gardenNamespaceSeed)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", gardenNamespaceSeed.Name)
	DeferCleanup(func() {
		By("Delete garden namespace for seed")
		Expect(testClient.Delete(ctx, gardenNamespaceSeed)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  testScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&seedmanagementv1alpha1.Gardenlet{}: {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
			},
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("Create test clientset")
	testClientSet, err = kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(mgr.GetClient()),
		kubernetes.WithRuntimeCache(mgr.GetCache()),
	)
	Expect(err).NotTo(HaveOccurred())

	By("Setup fake OCI registry with gardenlet chart")
	Expect(exec.Command("helm", "package", filepath.Join("..", "..", "..", "..", "charts", "gardener", "gardenlet"), "--destination", ".").Run()).To(Succeed())
	gardenletChart, err := os.ReadFile("gardenlet-0.1.0.tgz")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Remove("gardenlet-0.1.0.tgz")).To(Succeed())

	fakeRegistry = ocifake.NewRegistry()
	fakeRegistry.AddArtifact(&ociRepository, gardenletChart)

	By("Register controller")
	cfg := gardenletconfigv1alpha1.GardenletConfiguration{
		Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
			Gardenlet: &gardenletconfigv1alpha1.GardenletObjectControllerConfiguration{
				// This controller is pretty heavy-weight, so use a higher duration.
				SyncPeriod: &metav1.Duration{Duration: time.Minute},
			},
		},
		SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
			SeedTemplate: gardencorev1beta1.SeedTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: seed.Name,
				},
			},
		},
	}

	Expect((&gardenlet.Reconciler{
		Config:          cfg,
		GardenNamespace: gardenNamespaceSeed.Name,
		HelmRegistry:    fakeRegistry,
	}).AddToManager(mgr, mgr, testClientSet)).To(Succeed())

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
