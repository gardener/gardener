// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
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

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/controllerinstallation"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	ocifake "github.com/gardener/gardener/pkg/utils/oci/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
	"github.com/gardener/gardener/test/utils/namespacefinalizer"
)

func TestControllerInstallation(t *testing.T) {
	gardenletfeatures.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Gardenlet ControllerInstallation Main Suite")
}

const (
	testID              = "controllerinstallation-controller-test"
	seedClusterIdentity = "seed"
)

var (
	// Prevent testRunID from being able to be interpreted as number, see https://github.com/gardener/gardener/issues/6786
	// for more details about the reasoning.
	testRunID = testID + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

	ctx = context.Background()
	log logr.Logger

	restConfig    *rest.Config
	testEnv       *gardenerenvtest.GardenerTestEnvironment
	testClient    client.Client
	testClientSet kubernetes.Interface
	mgrClient     client.Client
	fakeRegistry  *ocifake.Registry

	seed                  *gardencorev1beta1.Seed
	seedNamespace         *corev1.Namespace
	gardenNamespace       *corev1.Namespace
	identity              = &gardencorev1beta1.Gardener{Version: "1.2.3"}
	gardenClusterIdentity = "test-garden"
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	log.Info("Using test run ID for test", "testRunID", testRunID)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join("..", "..", "..", "..", "..", "example", "resource-manager", "10-crd-resources.gardener.cloud_managedresources.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "operator", "10-crd-operator.gardener.cloud_gardens.yaml"),
				},
			},
			ErrorIfCRDPathMissing: true,
		},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootQuotaValidator,ShootValidator,ShootTolerationRestriction,ShootDNS"},
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
		kubernetes.AddSeedSchemeToScheme,
		resourcesv1alpha1.AddToScheme,
	)
	testScheme := runtime.NewScheme()
	Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create seed")
	seed = &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "seed-",
			Labels:       map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.SeedSpec{
			Provider: gardencorev1beta1.SeedProvider{
				Region: "region",
				Type:   "providerType",
				Zones:  []string{"a", "b", "c"},
			},
			Ingress: &gardencorev1beta1.Ingress{
				Domain: "someingress.example.com",
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
	Expect(testClient.Create(ctx, seed)).To(Succeed())
	log.Info("Created Seed for test", "seed", seed.Name)

	patch := client.MergeFrom(seed.DeepCopy())
	seed.Status.ClusterIdentity = ptr.To(seedClusterIdentity)
	Expect(testClient.Status().Patch(ctx, seed, patch)).To(Succeed())

	DeferCleanup(func() {
		By("Delete seed")
		Expect(testClient.Delete(ctx, seed)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Create seed namespace")
	seedNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "seed-" + seed.Name,
		},
	}
	Expect(testClient.Create(ctx, seedNamespace)).To(Succeed())
	log.Info("Created seed namespace for test", "namespaceName", seedNamespace)

	DeferCleanup(func() {
		By("Delete seed namespace")
		Expect(testClient.Delete(ctx, seedNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Create garden namespace")
	gardenNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "garden",
		},
	}
	Expect(testClient.Create(ctx, gardenNamespace)).To(Succeed())
	log.Info("Created garden namespace for test", "namespaceName", gardenNamespace)

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  testScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&gardencorev1beta1.ControllerInstallation{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
					Field: fields.SelectorFromSet(fields.Set{gardencore.SeedRefName: seed.Name}),
				},
				&gardencorev1beta1.ControllerRegistration{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
				&gardencorev1.ControllerDeployment{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
				&gardencorev1beta1.Seed{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.Secret{}, // applied because of operations on managed resources and their secrets
				},
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

	// The controller waits for namespaces to be gone, so we need to finalize them as envtest doesn't run the namespace
	// controller.
	Expect((&namespacefinalizer.Reconciler{}).AddToManager(mgr)).To(Succeed())

	fakeRegistry = ocifake.NewRegistry()
	By("Register controller")
	Expect((&controllerinstallation.Reconciler{
		HelmRegistry:  fakeRegistry,
		SeedClientSet: testClientSet,
		Config: gardenletconfigv1alpha1.GardenletConfiguration{
			Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
				ControllerInstallation: &gardenletconfigv1alpha1.ControllerInstallationControllerConfiguration{
					ConcurrentSyncs: ptr.To(5),
				},
			},
		},
		Identity:              identity,
		GardenClusterIdentity: gardenClusterIdentity,
	}).AddToManager(ctx, mgr, mgr)).To(Succeed())

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

	DeferCleanup(func() {
		By("Delete garden namespace")
		Expect(testClient.Delete(ctx, gardenNamespace)).To(Or(Succeed(), BeNotFoundError()))

		// Wait for garden namespace to be finalized after deleting it.
		// Otherwise, rerunning the test suite against the same test env is not possible.
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenNamespace), gardenNamespace)).To(BeNotFoundError())
		}).Should(Succeed())
	})
})
