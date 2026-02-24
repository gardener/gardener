// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

func TestShoot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ControllerManager ControllerRegistration ControllerInstallation Shoot Suite")
}

// testID is used for generating test namespace names and other IDs
const testID = "controllerinstallation-shoot-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client
	mgrClient  client.Client

	testNamespace *corev1.Namespace
	testRunID     string

	providerType           = "provider"
	shoot                  *gardencorev1beta1.Shoot
	controllerRegistration *gardencorev1beta1.ControllerRegistration
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootDNS,ShootQuotaValidator,ShootTolerationRestriction,ShootValidator,ControllerRegistrationResources,ShootMutator"},
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
	Expect(indexer.AddBackupBucketShootRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddBackupBucketShootRefNamespace(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddBackupEntryShootRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddBackupEntryShootRefNamespace(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddControllerInstallationSeedRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddControllerInstallationShootRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddControllerInstallationShootRefNamespace(ctx, mgr.GetFieldIndexer())).To(Succeed())
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

	controllerRegistration = &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ctrlreg-shoot-",
			Labels:       map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.ControllerRegistrationSpec{
			Resources: []gardencorev1beta1.ControllerResource{
				{Kind: extensionsv1alpha1.ControlPlaneResource, Type: providerType},
				{Kind: extensionsv1alpha1.InfrastructureResource, Type: providerType},
				{Kind: extensionsv1alpha1.WorkerResource, Type: providerType},
				{Kind: extensionsv1alpha1.NetworkResource, Type: providerType},
				{Kind: extensionsv1alpha1.ContainerRuntimeResource, Type: providerType},
				{Kind: extensionsv1alpha1.DNSRecordResource, Type: providerType},
				{Kind: extensionsv1alpha1.OperatingSystemConfigResource, Type: providerType},
				{Kind: extensionsv1alpha1.ExtensionResource, Type: providerType},
			},
		},
	}

	By("Create ControllerRegistration")
	Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
	log.Info("Created ControllerRegistration for shoot", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

	DeferCleanup(func() {
		By("Delete ControllerRegistration")
		Expect(testClient.Delete(ctx, controllerRegistration)).To(Or(Succeed(), BeNotFoundError()))

		By("Wait until manager has observed ControllerRegistration deletion")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
		}).Should(BeNotFoundError())
	})

	shoot = &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testID + "-",
			Namespace:    testNamespace.Name,
			Labels:       map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.ShootSpec{
			CredentialsBindingName: ptr.To("my-provider-account"),
			CloudProfile:           &gardencorev1beta1.CloudProfileReference{Kind: "CloudProfile", Name: "test-cloudprofile"},
			Region:                 "foo-region",
			Provider: gardencorev1beta1.Provider{
				Type: providerType,
				Workers: []gardencorev1beta1.Worker{{
					Name:         "controlplane",
					ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
					Minimum:      1,
					Maximum:      1,
					Machine: gardencorev1beta1.Machine{
						Type:  "large",
						Image: &gardencorev1beta1.ShootMachineImage{Name: providerType, Version: ptr.To("0.0.0")},
					},
					CRI: &gardencorev1beta1.CRI{
						Name:              gardencorev1beta1.CRINameContainerD,
						ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: providerType}},
					},
				}},
			},
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version: "1.31.1",
			},
			Networking: &gardencorev1beta1.Networking{
				Type: &providerType,
			},
			Extensions: []gardencorev1beta1.Extension{{
				Type: providerType,
			}},
			DNS: &gardencorev1beta1.DNS{
				Providers: []gardencorev1beta1.DNSProvider{{
					Type:           &providerType,
					CredentialsRef: &autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"},
				}},
			},
		},
	}

	By("Create Shoot")
	Expect(testClient.Create(ctx, shoot)).To(Succeed())
	log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

	DeferCleanup(func() {
		By("Delete Shoot")
		Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))

		By("Wait until manager has observed Shoot deletion")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(BeNotFoundError())

		By("Expect ControllerInstallation to be deleted")
		Eventually(func(g Gomega) []gardencorev1beta1.ControllerInstallation {
			controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
			g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
				core.RegistrationRefName: controllerRegistration.Name,
				core.ShootRefName:        shoot.Name,
				core.ShootRefNamespace:   shoot.Namespace,
			})).To(Succeed())
			return controllerInstallationList.Items
		}).Should(BeEmpty())
	})

	Eventually(func(g Gomega) []string {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
		return shoot.Finalizers
	}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))
})
