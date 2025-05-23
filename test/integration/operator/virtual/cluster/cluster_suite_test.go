// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	virtualcluster "github.com/gardener/gardener/pkg/operator/controller/virtual/cluster"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
)

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration Operator Virtual Cluster Suite")
}

const testID = "garden-access-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	testEnv    *gardenerenvtest.GardenerTestEnvironment
	restConfig *rest.Config
	ctrl       *gomock.Controller
	testRunID  string

	channel chan event.TypedGenericEvent[*rest.Config]

	mockManager             *mockmanager.MockManager
	virtualCluster          cluster.Cluster
	virtualClientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{
		GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
			Args: []string{
				"--disable-admission-plugins=ResourceReferenceManager,ExtensionValidator,SeedValidator",
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
		ctrl.Finish()
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  operatorclient.RuntimeScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Secret{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
			},
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})

	Expect(err).NotTo(HaveOccurred())

	channel = make(chan event.TypedGenericEvent[*rest.Config])
	ctrl = gomock.NewController(GinkgoT())
	mockManager = mockmanager.NewMockManager(ctrl)
	virtualClientConnection = componentbaseconfigv1alpha1.ClientConnectionConfiguration{
		AcceptContentTypes: "application/json",
		ContentType:        "application/json",
		Burst:              42,
		QPS:                321,
	}

	By("Register controller")
	Expect((&virtualcluster.Reconciler{
		Manager: mockManager,
		StoreCluster: func(cluster cluster.Cluster) {
			virtualCluster = cluster
		},
		VirtualClientConnection: virtualClientConnection,
	}).AddToManager(mgr, channel)).To(Succeed())

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
