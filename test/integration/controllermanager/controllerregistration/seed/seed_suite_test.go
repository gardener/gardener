// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

func TestControllerRegistration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ControllerManager ControllerRegistration Suite")
}

// testID is used for generating test namespace names and other IDs
const testID = "controllerregistration-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client
	mgrClient  client.Client

	testNamespace *corev1.Namespace
	testRunID     string

	providerType               = "provider"
	seedName                   string
	seed                       *gardencorev1beta1.Seed
	seedNamespace              *corev1.Namespace
	seedSecret                 *corev1.Secret
	seedControllerRegistration *gardencorev1beta1.ControllerRegistration
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootDNS,ShootQuotaValidator,ShootTolerationRestriction,ShootValidator,ControllerRegistrationResources"},
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
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create test Namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
			GenerateName: testID + "-",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
	testRunID = testNamespace.Name

	DeferCleanup(func() {
		By("Delete test Namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  kubernetes.GardenScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&gardencorev1beta1.ControllerRegistration{}: {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
				&gardencorev1beta1.Seed{}:                   {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
				&gardencorev1beta1.BackupBucket{}:           {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
				&gardencorev1beta1.BackupEntry{}:            {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
				&gardencorev1beta1.Shoot{}:                  {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
			},
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("Setup field indexes")
	Expect(indexer.AddBackupBucketSeedName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddBackupEntrySeedName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddShootSeedName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddShootStatusSeedName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddControllerInstallationSeedRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddControllerInstallationRegistrationRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())

	By("Register controller")
	Expect(controllerregistration.AddToManager(mgr, controllermanagerconfigv1alpha1.ControllerManagerConfiguration{
		Controllers: controllermanagerconfigv1alpha1.ControllerManagerControllerConfiguration{
			ControllerRegistration: &controllermanagerconfigv1alpha1.ControllerRegistrationControllerConfiguration{
				ConcurrentSyncs: ptr.To(5),
			},
		},
	})).To(Succeed())

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

	seedControllerRegistration = &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ctrlreg-seed-",
			Labels:       map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.ControllerRegistrationSpec{
			Resources: []gardencorev1beta1.ControllerResource{
				{Kind: extensionsv1alpha1.DNSRecordResource, Type: providerType},
				{Kind: extensionsv1alpha1.ControlPlaneResource, Type: providerType},
				{Kind: extensionsv1alpha1.InfrastructureResource, Type: providerType},
				{Kind: extensionsv1alpha1.WorkerResource, Type: providerType},
			},
		},
	}

	By("Create ControllerRegistration")
	Expect(testClient.Create(ctx, seedControllerRegistration)).To(Succeed())
	log.Info("Created ControllerRegistration for seed", "controllerRegistration", client.ObjectKeyFromObject(seedControllerRegistration))

	DeferCleanup(func() {
		By("Delete seed ControllerRegistration")
		Expect(testClient.Delete(ctx, seedControllerRegistration)).To(Or(Succeed(), BeNotFoundError()))

		By("Wait until manager has observed controllerregistration deletion")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(seedControllerRegistration), seedControllerRegistration)
		}).Should(BeNotFoundError())
	})

	seedName = "seed-ctrl-reg-test-" + utils.ComputeSHA256Hex([]byte(testID + uuid.NewUUID()))[:8]
	seedNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: gardenerutils.ComputeGardenNamespace(seedName),
		},
	}

	seedSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seed-secret",
			Namespace: seedNamespace.Name,
			Labels:    map[string]string{"gardener.cloud/role": "global-monitoring"},
		},
	}

	By("Create Seed Namespace")
	Expect(testClient.Create(ctx, seedNamespace)).To(Succeed())
	log.Info("Created Seed Namespace for test", "namespace", client.ObjectKeyFromObject(seedNamespace))

	By("Create Seed Secret")
	Expect(testClient.Create(ctx, seedSecret)).To(Succeed())
	log.Info("Created Seed Secret for test", "secret", client.ObjectKeyFromObject(seedSecret))

	DeferCleanup(func() {
		By("Delete Seed Secret")
		Expect(testClient.Delete(ctx, seedSecret)).To(Or(Succeed(), BeNotFoundError()))

		By("Delete Seed Namespace")
		Expect(testClient.Delete(ctx, seedNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	seed = &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name:   seedName,
			Labels: map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.SeedSpec{
			Provider: gardencorev1beta1.SeedProvider{
				Region: "region",
				Type:   providerType,
			},
			Ingress: &gardencorev1beta1.Ingress{
				Domain: "seed.example.com",
				Controller: gardencorev1beta1.IngressController{
					Kind: "nginx",
				},
			},
			DNS: gardencorev1beta1.SeedDNS{
				Provider: &gardencorev1beta1.SeedDNSProvider{
					Type: providerType,
					SecretRef: corev1.SecretReference{
						Name:      "some-secret",
						Namespace: "some-namespace",
					},
				},
			},
			Settings: &gardencorev1beta1.SeedSettings{
				Scheduling: &gardencorev1beta1.SeedSettingScheduling{Visible: true},
			},
			Networks: gardencorev1beta1.SeedNetworks{
				Pods:     "10.0.0.0/16",
				Services: "10.1.0.0/16",
				Nodes:    ptr.To("10.2.0.0/16"),
				ShootDefaults: &gardencorev1beta1.ShootNetworks{
					Pods:     ptr.To("100.128.0.0/11"),
					Services: ptr.To("100.72.0.0/13"),
				},
			},
		},
	}

	By("Create Seed")
	Expect(testClient.Create(ctx, seed)).To(Succeed())
	log.Info("Created Seed for test", "seed", client.ObjectKeyFromObject(seed))

	DeferCleanup(func() {
		By("Delete Seed")
		Expect(testClient.Delete(ctx, seed)).To(Or(Succeed(), BeNotFoundError()))

		By("Wait until manager has observed seed deletion")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
		}).Should(BeNotFoundError())
	})

	Eventually(func(g Gomega) []string {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
		return seed.Finalizers
	}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))
})

var _ = AfterSuite(func() {
	By("Delete Seed")
	Expect(testClient.Delete(ctx, seed)).To(Succeed())

	By("Expect ControllerInstallation to be deleted")
	Eventually(func(g Gomega) {
		controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
		g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
			core.RegistrationRefName: seedControllerRegistration.Name,
			core.SeedRefName:         seed.Name,
		})).To(Succeed())
		g.Expect(controllerInstallationList.Items).To(BeEmpty())
	}).Should(Succeed())
})
