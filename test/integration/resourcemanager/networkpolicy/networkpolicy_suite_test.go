// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/logger"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/utils/namespacefinalizer"
)

func TestNetworkPolicy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ResourceManager NetworkPolicy Suite")
}

const (
	// testID is used for generating test namespace names and other IDs
	testID    = "networkpolicy-controller-test"
	finalizer = "test.gardener.cloud/integration"
)

var (
	ctx       = context.Background()
	log       logr.Logger
	logBuffer bytes.Buffer

	restConfig *rest.Config
	testEnv    *envtest.Environment
	testClient client.Client
	mgrClient  client.Client

	testRunID string

	ingressControllerNamespace   = "ingress-ctrl-ns"
	ingressControllerPodSelector = metav1.LabelSelector{MatchLabels: map[string]string{"app": "ingress-controller"}}
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(io.MultiWriter(GinkgoWriter, &logBuffer))))
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

	By("Create test clients")
	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:16]
	log.Info("Using test run ID for test", "testRunID", testRunID)

	testClient, err = client.New(restConfig, client.Options{Scheme: kubernetesscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  kubernetesscheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("Register controller")
	Expect((&networkpolicy.Reconciler{
		Config: resourcemanagerconfigv1alpha1.NetworkPolicyControllerConfig{
			ConcurrentSyncs:    ptr.To(5),
			NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{testID: testRunID}}},
			IngressControllerSelector: &resourcemanagerconfigv1alpha1.IngressControllerSelector{
				Namespace:   ingressControllerNamespace,
				PodSelector: ingressControllerPodSelector,
			},
		},
	}).AddToManager(mgr, mgr)).To(Succeed())

	// We create and delete namespace in every test, so let's ensure they get finalized.
	Expect((&namespacefinalizer.Reconciler{Exceptions: sets.New(finalizer)}).AddToManager(mgr)).To(Succeed())

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
