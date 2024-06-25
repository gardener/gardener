package virtualcluster_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/operator/controller/extension"
	"github.com/gardener/gardener/pkg/operator/features"
	operatorwebhook "github.com/gardener/gardener/pkg/operator/webhook"
	defaultingwebhook "github.com/gardener/gardener/pkg/operator/webhook/defaulting"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
	"github.com/gardener/gardener/test/framework"
)

func TestVirtualCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Virtualcluster Suite")
}

const testID = "garden-extension-virtualcluster-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	// testEnv       *envtest.Environment
	testEnv       *gardenerenvtest.GardenerTestEnvironment
	testClient    client.Client
	testClientSet kubernetes.Interface
	mgrClient     client.Client

	testRunID     string
	testNamespace *corev1.Namespace
	gardenName    string
	extensionName = "provider-local"
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	features.RegisterFeatureGates()

	wh := operatorwebhook.GetMutatingWebhookConfiguration("service", "")
	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			WebhookInstallOptions: envtest.WebhookInstallOptions{
				MutatingWebhooks: []*v1.MutatingWebhookConfiguration{
					wh,
				},
			},
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join("..", "..", "..", "..", "..", "example", "operator", "10-crd-operator.gardener.cloud_gardens.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "operator", "10-crd-operator.gardener.cloud_extensions.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "resource-manager", "10-crd-resources.gardener.cloud_managedresources.yaml"),
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
	httpClient, err := rest.HTTPClientFor(restConfig)
	Expect(err).NotTo(HaveOccurred())
	mapper, err := apiutil.NewDynamicRESTMapper(restConfig, httpClient)
	Expect(err).NotTo(HaveOccurred())

	mgr, err := manager.New(restConfig, manager.Options{
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    testEnv.WebhookInstallOptions.LocalServingPort,
			Host:    testEnv.WebhookInstallOptions.LocalServingHost,
			CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			Mapper: mapper,
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
	testClientSet, err = kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(mgr.GetClient()),
		kubernetes.WithRuntimeCache(mgr.GetCache()),
	)
	Expect(err).NotTo(HaveOccurred())

	gardenClientMap := fakeclientmap.NewClientMapBuilder().WithClientSetForKey(keys.ForGarden(&operatorv1alpha1.Garden{ObjectMeta: metav1.ObjectMeta{Name: gardenName}}), testClientSet).Build()

	By("Register controller")
	Expect(extension.AddToManager(ctx, mgr, &config.OperatorConfiguration{
		Controllers: config.ControllerConfiguration{},
	}, gardenClientMap)).To(Succeed())

	By("Register defaulting webhook")
	Expect(defaultingwebhook.AddToManager(mgr)).To(Succeed())
	DeferCleanup(func() {
		By("Delete defaulting webhook")
		Expect(testClient.Delete(ctx, wh)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Start manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	Eventually(func() error {
		checker := mgr.GetWebhookServer().StartedChecker()
		return checker(&http.Request{})
	}).Should(BeNil())

	DeferCleanup(func() {
		By("Stop manager")
		mgrCancel()
	})
})
