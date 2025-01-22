// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gardener/gardener/test/utils/namespacefinalizer"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension"
	"github.com/gardener/gardener/pkg/operator/features"
	ocifake "github.com/gardener/gardener/pkg/utils/oci/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
	"github.com/gardener/gardener/test/framework"
)

func TestExtension(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Operator Extension Suite")
}

const testID = "extension-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client
	mgrClient  client.Client

	testRunID     string
	testNamespace *corev1.Namespace
	gardenName    string

	fakeRegistry                           *ocifake.Registry
	ociRepositoryProviderLocalChart        gardencorev1.OCIRepository
	ociRepositoryAdmissionApplicationChart gardencorev1.OCIRepository
	ociRepositoryAdmissionRuntimeChart     gardencorev1.OCIRepository
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
					filepath.Join("..", "..", "..", "..", "..", "example", "operator", "10-crd-operator.gardener.cloud_gardens.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "operator", "10-crd-operator.gardener.cloud_extensions.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "resource-manager", "10-crd-resources.gardener.cloud_managedresources.yaml"),
				},
			},
			ErrorIfCRDPathMissing: true,
		},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{},
	}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("Create test client")
	scheme := runtime.NewScheme()
	framework.Must(operatorclient.AddRuntimeSchemeToScheme(scheme))
	framework.Must(operatorclient.AddVirtualSchemeToScheme(scheme))

	testClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create test Namespaces")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "garden-",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
	gardenName = testNamespace.Name
	testRunID = testNamespace.Name

	DeferCleanup(func() {
		By("Delete test Namespaces")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&operatorv1alpha1.Garden{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
				&operatorv1alpha1.Extension{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		},
	})

	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("Create test clientset")
	testClientSet, err := kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(mgr.GetClient()),
		kubernetes.WithRuntimeCache(mgr.GetCache()),
	)
	Expect(err).NotTo(HaveOccurred())

	gardenClientMap := fakeclientmap.NewClientMapBuilder().WithClientSetForKey(keys.ForGarden(&operatorv1alpha1.Garden{ObjectMeta: metav1.ObjectMeta{Name: gardenName}}), testClientSet).Build()

	By("Setup fake OCI registry with provider-local chart")
	fakeRegistry = ocifake.NewRegistry()

	ociRepositoryProviderLocalChart = gardencorev1.OCIRepository{Repository: ptr.To("provider-local"), Tag: ptr.To("test")}

	Expect(exec.Command("helm", "package", filepath.Join("..", "..", "..", "..", "..", "charts", "gardener", "provider-local"), "--destination", ".").Run()).To(Succeed())
	DeferCleanup(func() {
		Expect(os.Remove("gardener-extension-provider-local-v0.1.0.tgz")).To(Succeed())
	})
	providerLocalChart, err := os.ReadFile("gardener-extension-provider-local-v0.1.0.tgz")
	Expect(err).NotTo(HaveOccurred())

	fakeRegistry.AddArtifact(&ociRepositoryProviderLocalChart, providerLocalChart)

	By("Setup fake OCI registry with admission-local charts")
	ociRepositoryAdmissionApplicationChart = gardencorev1.OCIRepository{Repository: ptr.To("admission-local-application"), Tag: ptr.To("test")}
	ociRepositoryAdmissionRuntimeChart = gardencorev1.OCIRepository{Repository: ptr.To("admission-local-runtime"), Tag: ptr.To("test")}

	Expect(exec.Command("helm", "package", filepath.Join("..", "..", "..", "..", "..", "charts", "gardener", "admission-local", "charts", "application"), "--destination", ".").Run()).To(Succeed())
	DeferCleanup(func() {
		Expect(os.Remove("admission-local-application-0.1.0.tgz")).To(Succeed())
	})
	admissionLocalApplicationChart, err := os.ReadFile("admission-local-application-0.1.0.tgz")
	Expect(err).NotTo(HaveOccurred())

	Expect(exec.Command("helm", "package", filepath.Join("..", "..", "..", "..", "..", "charts", "gardener", "admission-local", "charts", "runtime"), "--destination", ".").Run()).To(Succeed())
	DeferCleanup(func() {
		Expect(os.Remove("admission-local-runtime-0.1.0.tgz")).To(Succeed())
	})
	admissionLocalRuntimeChart, err := os.ReadFile("admission-local-runtime-0.1.0.tgz")
	Expect(err).NotTo(HaveOccurred())

	fakeRegistry.AddArtifact(&ociRepositoryAdmissionApplicationChart, admissionLocalApplicationChart)
	fakeRegistry.AddArtifact(&ociRepositoryAdmissionRuntimeChart, admissionLocalRuntimeChart)

	By("Register controller")
	Expect((&extension.Reconciler{
		Config: config.OperatorConfiguration{
			Controllers: config.ControllerConfiguration{},
		},
		HelmRegistry:    fakeRegistry,
		GardenNamespace: testNamespace.Name,
		GardenClientMap: gardenClientMap,
	}).AddToManager(mgr)).Should(Succeed())

	// We create runtime-extension namespaces and delete them again, so let's ensure it gets finalized.
	Expect((&namespacefinalizer.Reconciler{}).AddToManager(mgr)).To(Succeed())

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
