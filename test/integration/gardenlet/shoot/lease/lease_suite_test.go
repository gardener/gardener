// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesclientset "k8s.io/client-go/kubernetes"
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/lease"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

func TestLease(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Gardenlet Shoot Lease Suite")
}

const testID = "shoot-lease-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client

	testNamespace *corev1.Namespace
	testRunID     string

	shoot         *gardencorev1beta1.Shoot
	fakeClock     *testclock.FakeClock
	healthManager healthz.Manager
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootDNS,ShootQuotaValidator,ShootTolerationRestriction,ShootValidator,ShootMutator"},
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
		resourcesv1alpha1.AddToScheme,
	)
	testScheme := runtime.NewScheme()
	Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create test Namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "garden-",
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
	testRunID = testNamespace.Name

	DeferCleanup(func() {
		By("Delete test Namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Create Shoot")
	shoot = &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "shoot-",
			Namespace:    testNamespace.Name,
			Labels:       map[string]string{testID: testRunID},
		},
		Spec: gardencorev1beta1.ShootSpec{
			CredentialsBindingName: ptr.To("my-provider-account"),
			CloudProfile:           &gardencorev1beta1.CloudProfileReference{Name: "test-cloudprofile", Kind: "CloudProfile"},
			Region:                 "foo-region",
			Provider: gardencorev1beta1.Provider{
				Type: "test",
				Workers: []gardencorev1beta1.Worker{
					{
						Name:    "cpu-worker",
						Minimum: 2,
						Maximum: 2,
						Machine: gardencorev1beta1.Machine{
							Type: "large",
							Image: &gardencorev1beta1.ShootMachineImage{
								Name:    "some-image",
								Version: ptr.To("1.0.0"),
							},
						},
					},
				},
			},
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version: "1.31.1",
			},
			Networking: &gardencorev1beta1.Networking{
				Type: ptr.To("foo-networking"),
			},
		},
	}
	Expect(testClient.Create(ctx, shoot)).To(Succeed())
	log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

	DeferCleanup(func() {
		By("Delete Shoot")
		Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  testScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{testNamespace.Name: {}},
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	By("Create Kubernetes clientset")
	kubernetesClient, err := kubernetesclientset.NewForConfig(mgr.GetConfig())
	Expect(err).NotTo(HaveOccurred())

	By("Register controller")
	fakeClock = testclock.NewFakeClock(time.Now())
	healthManager = healthz.NewDefaultHealthz()

	Expect(lease.AddToManager(mgr, mgr, kubernetesClient.RESTClient(), healthManager, fakeClock)).To(Succeed())

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
