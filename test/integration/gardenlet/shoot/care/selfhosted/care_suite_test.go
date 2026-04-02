// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhosted_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	druidcorecrds "github.com/gardener/etcd-druid/api/core/v1alpha1/crds"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	"github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
	"github.com/gardener/gardener/test/utils/namespacefinalizer"
)

func TestCare(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Gardenlet Self-Hosted Shoot Care Suite")
}

const testID = "selfhosted-shoot-care-controller-test"

var _ = BeforeEach(func() {
	SetDefaultEventuallyTimeout(30 * time.Second)
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
})

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

	// gardenNamespace is the "garden" namespace where self-hosted shoots must reside.
	gardenNamespace *corev1.Namespace
	// cpNamespace is the seed-side control plane namespace for the shoot (shoot.Status.TechnicalID).
	cpNamespace *corev1.Namespace
	// project is required so the operation builder can resolve the "garden" namespace to a Project.
	project *gardencorev1beta1.Project

	testRunID   string
	shootName   string
	projectName string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	features.RegisterFeatureGates()

	var err error
	By("Fetch Etcd CRD")
	k8sVersion, err := gardenerenvtest.GetK8SVersion()
	Expect(err).NotTo(HaveOccurred())
	etcdCRDs, err := druidcorecrds.GetAll(k8sVersion.String())
	Expect(err).NotTo(HaveOccurred())
	etcdCRDYAML, ok := etcdCRDs[druidcorecrds.ResourceNameEtcd]
	Expect(ok).To(BeTrue())
	etcdCRD, err := kubernetesutils.DecodeCRD(etcdCRDYAML)
	Expect(err).NotTo(HaveOccurred())

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "resource-manager", "10-crd-resources.gardener.cloud_managedresources.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_backupentries.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_clusters.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_containerruntimes.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_controlplanes.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_dnsrecords.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_extensions.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_infrastructures.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_networks.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_operatingsystemconfigs.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_workers.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-monitoring.coreos.com_prometheuses.yaml"),
				},
				CRDs: []*apiextensionsv1.CustomResourceDefinition{etcdCRD},
			},
			ErrorIfCRDPathMissing: true,
		},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootDNS,ShootQuotaValidator,ShootTolerationRestriction,ShootValidator,ShootMutator"},
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
		extensionsv1alpha1.AddToScheme,
		resourcesv1alpha1.AddToScheme,
		druidcorev1alpha1.AddToScheme,
		monitoringv1.AddToScheme,
	)
	testScheme := runtime.NewScheme()
	Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
	shootName = "shoot-" + testRunID
	projectName = "p-" + testRunID

	// Self-hosted shoots must be in the "garden" namespace.
	// The namespace must also carry a ProjectName label so the operation builder can resolve it to a Project.
	By("Create garden Namespace")
	gardenNamespaceName := v1beta1constants.GardenNamespace
	gardenNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: gardenNamespaceName,
			Labels: map[string]string{
				testID:                       testRunID,
				v1beta1constants.ProjectName: projectName,
			},
		},
	}
	// The garden namespace may already exist in the envtest environment; ignore AlreadyExists.
	if err := testClient.Create(ctx, gardenNamespace); client.IgnoreAlreadyExists(err) != nil {
		Expect(err).NotTo(HaveOccurred())
	}
	// Ensure the labels are present for cache selector and project lookup.
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenNamespace), gardenNamespace)).To(Succeed())
		patch := client.MergeFrom(gardenNamespace.DeepCopy())
		if gardenNamespace.Labels == nil {
			gardenNamespace.Labels = map[string]string{}
		}
		gardenNamespace.Labels[testID] = testRunID
		gardenNamespace.Labels[v1beta1constants.ProjectName] = projectName
		g.Expect(testClient.Patch(ctx, gardenNamespace, patch)).To(Succeed())
	}).Should(Succeed())

	By("Create Project")
	gardenNamespaceName = gardenNamespace.Name
	project = &gardencorev1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:   projectName,
			Labels: map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.ProjectSpec{
			Namespace: &gardenNamespaceName,
		},
	}
	Expect(testClient.Create(ctx, project)).To(Succeed())
	log.Info("Created Project for test", "project", project.Name)

	DeferCleanup(func() {
		By("Delete Project")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, project))).To(Succeed())
	})

	// The control plane namespace is the seed-side namespace for the shoot's technical ID.
	cpNamespaceName := "cp-" + testRunID
	By("Create control plane Namespace")
	cpNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   cpNamespaceName,
			Labels: map[string]string{testID: testRunID},
		},
	}
	Expect(testClient.Create(ctx, cpNamespace)).To(Succeed())
	log.Info("Created control plane Namespace for test", "namespaceName", cpNamespace.Name)

	DeferCleanup(func() {
		By("Delete control plane Namespace")
		Expect(testClient.Delete(ctx, cpNamespace)).To(Or(Succeed(), BeNotFoundError()))
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

	// We create the control plane namespace and delete it after every test, so let's ensure it gets finalized.
	Expect((&namespacefinalizer.Reconciler{}).AddToManager(mgr)).To(Succeed())

	By("Setup field indexes")
	Expect(indexer.AddManagedSeedShootName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddControllerInstallationShootRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())

	By("Create test clientset")
	testClientSet, err = kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(mgr.GetClient()),
		kubernetes.WithRuntimeCache(mgr.GetCache()),
	)
	Expect(err).NotTo(HaveOccurred())

	// The shoot client map key uses the shoot's namespace (garden) and name.
	shootClientMap = fakeclientmap.NewClientMapBuilder().WithClientSetForKey(
		keys.ForShoot(&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{
			Name:      shootName,
			Namespace: v1beta1constants.GardenNamespace,
		}}),
		testClientSet,
	).Build()
	fakeClock = testclock.NewFakeClock(time.Now().Round(time.Second))

	By("Register controller")
	// No SeedConfig: self-hosted shoots do not have a SeedConfig in the gardenlet configuration.
	Expect((&care.Reconciler{
		SeedClientSet:  testClientSet,
		ShootClientMap: shootClientMap,
		Config: gardenletconfigv1alpha1.GardenletConfiguration{
			Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
				ShootCare: &gardenletconfigv1alpha1.ShootCareControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: 500 * time.Millisecond},
				},
			},
		},
		Clock: fakeClock,
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
