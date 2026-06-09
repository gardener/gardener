// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/inplaceupdate"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
)

func TestInPlaceUpdate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Gardenlet Shoot InPlaceUpdate Suite")
}

const testID = "shoot-inplace-update-test"

const (
	poolDefault           = "pool-default"
	poolDefaultSecretName = "pool-default-secret"

	poolMulti           = "pool-multi"
	poolMultiSecretName = "pool-multi-secret"

	poolControlPlane           = "pool-cp"
	poolControlPlaneSecretName = "pool-cp-secret"

	finalizerKeepAlive = "shoot-inplace-update-test.gardener.cloud/keep-alive"
)

var (
	ctx       = context.Background()
	log       logr.Logger
	fakeClock *testclock.FakeClock

	restConfig *rest.Config
	testEnv    *envtest.Environment
	testClient client.Client
	mgrClient  client.Client

	testNamespace *corev1.Namespace
	testRunID     string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &envtest.Environment{}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetes.SeedScheme})
	Expect(err).NotTo(HaveOccurred())

	testRunID = gardenerutils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

	By("Create control plane Namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cp-" + testRunID,
			Labels: map[string]string{testID: testRunID},
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created control plane Namespace for test", "namespace", testNamespace.Name, "testRunID", testRunID)

	DeferCleanup(func() {
		By("Delete control plane Namespace")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, testNamespace))).To(Succeed())
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  kubernetes.SeedScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Node{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
				&corev1.Pod{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: new(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("Setup field indexes")
	Expect(indexer.AddPodNodeName(ctx, mgr.GetFieldIndexer())).To(Succeed())

	fakeClock = testclock.NewFakeClock(time.Now().Round(time.Second))

	By("Register controller")
	workers := []gardencorev1beta1.Worker{
		{
			Name:           poolDefault,
			Maximum:        5,
			MaxUnavailable: new(intstr.FromInt32(1)),
		},
		{
			Name:           poolMulti,
			Maximum:        5,
			MaxUnavailable: new(intstr.FromInt32(2)),
		},
		{
			Name:           poolControlPlane,
			Maximum:        5,
			ControlPlane:   &gardencorev1beta1.WorkerControlPlane{},
			MaxUnavailable: new(intstr.FromString("99%")),
		},
	}

	Expect((&inplaceupdate.Reconciler{
		SeedClient:            mgr.GetClient(),
		Clock:                 fakeClock,
		Workers:               workers,
		ControlPlaneNamespace: testNamespace.Name,
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
