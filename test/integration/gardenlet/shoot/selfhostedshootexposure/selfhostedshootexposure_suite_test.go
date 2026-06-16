// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/api/indexer"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/selfhostedshootexposure"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

func TestSelfHostedShootExposure(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Gardenlet Shoot SelfHostedShootExposure Suite")
}

const (
	testID = "shoot-selfhostedshootexposure-controller-test"
	// shootName is fixed because the controller is configured with a single ShootKey at startup.
	shootName = "root"
)

var (
	ctx       = context.Background()
	log       logr.Logger
	fakeClock *testclock.FakeClock

	restConfig *rest.Config
	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client

	testRunID string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	SetDefaultEventuallyTimeout(10 * time.Second)
	SetDefaultConsistentlyDuration(time.Second)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_dnsrecords.yaml"),
					filepath.Join("..", "..", "..", "..", "..", "example", "seed-crds", "10-crd-extensions.gardener.cloud_selfhostedshootexposures.yaml"),
				},
			},
			ErrorIfCRDPathMissing: true,
		},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{
				"--disable-admission-plugins=DeletionConfirmation,ResourceReferenceManager,ExtensionValidator,ShootDNS,ShootQuotaValidator,ShootTolerationRestriction,ShootValidator,ShootMutator",
			},
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
	)
	testScheme := runtime.NewScheme()
	Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())

	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

	By("Ensure garden Namespace exists (self-hosted shoots must live in the garden namespace)")
	Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.GardenNamespace}})).To(Or(Succeed(), MatchError(apierrors.IsAlreadyExists, "already exists")))

	By("Ensure kube-system Namespace exists (control-plane namespace of a self-hosted shoot)")
	Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem}})).To(Or(Succeed(), MatchError(apierrors.IsAlreadyExists, "already exists")))

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  testScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Controller: controllerconfig.Controller{
			SkipNameValidation: new(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	By("Register field indexes the controller lists ControllerInstallations by")
	Expect(indexer.AddControllerInstallationShootRefName(ctx, mgr.GetFieldIndexer())).To(Succeed())
	Expect(indexer.AddControllerInstallationShootRefNamespace(ctx, mgr.GetFieldIndexer())).To(Succeed())

	fakeClock = testclock.NewFakeClock(time.Now().Round(time.Second))

	By("Register controller")
	Expect((&selfhostedshootexposure.Reconciler{
		ShootKey: client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: shootName},
		Clock:    fakeClock,
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
