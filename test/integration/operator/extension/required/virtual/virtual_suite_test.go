// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	requiredvirtual "github.com/gardener/gardener/pkg/operator/controller/extension/required/virtual"
	"github.com/gardener/gardener/pkg/utils"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

func TestExtensionRequiredVirtual(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Operator Extension Required Virtual Suite")
}

const testID = "extension-required-virtual-test"

var (
	ctx = context.Background()
	log logr.Logger

	testEnv    *gardenerenvtest.GardenerTestEnvironment
	testClient client.Client
	mgrClient  client.Client

	testRunID string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	randomSuffix, err := utils.GenerateRandomString(8)
	Expect(err).NotTo(HaveOccurred())
	testRunID = "garden-" + strings.ToLower(randomSuffix)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		Environment: &envtest.Environment{
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join("..", "..", "..", "..", "..", "..", "example", "operator", "10-crd-operator.gardener.cloud_extensions.yaml"),
				},
			},
			ErrorIfCRDPathMissing: true,
		},
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{
				"--disable-admission-plugins=ResourceReferenceManager,ExtensionValidator,SeedValidator",
			},
		},
	}

	restConfig, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("Create test client")
	scheme := runtime.NewScheme()
	Expect(operatorclient.AddRuntimeSchemeToScheme(scheme)).To(Succeed())
	Expect(operatorclient.AddVirtualSchemeToScheme(scheme)).To(Succeed())

	testClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&gardencorev1beta1.ControllerInstallation{}: {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
				&operatorv1alpha1.Extension{}:               {Label: labels.SelectorFromSet(labels.Set{testID: testRunID})},
			},
		},
	})

	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("Index Setup")
	Expect(indexer.AddControllerInstallationRegistrationRefName(ctx, mgr.GetFieldIndexer())).NotTo(HaveOccurred())

	By("Register Controller")
	Expect((&requiredvirtual.Reconciler{
		Config:        operatorconfigv1alpha1.ExtensionRequiredVirtualControllerConfiguration{ConcurrentSyncs: ptr.To(5)},
		RuntimeClient: mgr.GetClient(),
		VirtualClient: mgr.GetClient(),
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
