// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package csrapprover_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	userpkg "k8s.io/apiserver/pkg/authentication/user"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/logger"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/csrapprover"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

func TestCSRApprover(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ResourceManager CSRApprover Suite")
}

// testID is used for generating test namespace names and other IDs
const testID = "csr-autoapprove-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig            *rest.Config
	testEnv               *envtest.Environment
	testClientKubelet     client.Client
	testClientNodeAgent   client.Client
	testClientBootstrap   client.Client
	testClientNodeAgentSA client.Client
	mgrClient             client.Client

	testNamespace *corev1.Namespace
	testRunID     string

	machineName         string
	nodeName            string
	userNameKubelet     string
	userNameNodeAgent   string
	userNameBootstrap   string
	userNameNodeAgentSA string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{filepath.Join("testdata", "crd-machines.yaml")},
		},
		ErrorIfCRDPathMissing: true,
	}

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

	machineName = "machine-" + testRunID
	nodeName = "node-" + testRunID
	userNameKubelet = "system:node:" + nodeName
	userNameNodeAgent = "gardener.cloud:node-agent:machine:" + machineName
	userNameBootstrap = "system:bootstrap:" + testRunID
	userNameNodeAgentSA = "system:serviceaccount:kube-system:gardener-node-agent"

	createClient := func(userName string) client.Client {
		// We have to "fake" that our test client is the kubelet or gardener-node-agent user because the .spec.username
		// field in CSRs will also be overwritten by the kube-apiserver to the user who created it. This would always
		// fail the constraints of this controller.
		user, err := testEnv.AddUser(
			envtest.User{Name: userName, Groups: []string{userpkg.SystemPrivilegedGroup}},
			&rest.Config{QPS: 1000.0, Burst: 2000.0},
		)
		Expect(err).NotTo(HaveOccurred())

		testClient, err := client.New(user.Config(), client.Options{Scheme: resourcemanagerclient.CombinedScheme})
		Expect(err).NotTo(HaveOccurred())
		return testClient
	}

	testClientKubelet = createClient(userNameKubelet)
	testClientNodeAgent = createClient(userNameNodeAgent)
	testClientBootstrap = createClient(userNameBootstrap)
	testClientNodeAgentSA = createClient(userNameNodeAgentSA)

	By("Create test Namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
			GenerateName: testID + "-",
		},
	}
	Expect(testClientKubelet.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

	DeferCleanup(func() {
		By("Delete test Namespace")
		Expect(testClientKubelet.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  resourcemanagerclient.CombinedScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			DefaultNamespaces:    map[string]cache.Config{testNamespace.Name: {}},
			DefaultLabelSelector: labels.SelectorFromSet(labels.Set{testID: testRunID}),
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())
	mgrClient = mgr.GetClient()

	By("Register controller")
	kubernetesClient, err := kubernetesclientset.NewForConfig(restConfig)
	Expect(err).NotTo(HaveOccurred())

	Expect((&csrapprover.Reconciler{
		CertificatesClient: kubernetesClient.CertificatesV1().CertificateSigningRequests(),
		Config: resourcemanagerconfigv1alpha1.CSRApproverControllerConfig{
			ConcurrentSyncs:  ptr.To(5),
			MachineNamespace: testNamespace.Name,
		},
	}).AddToManager(mgr, mgr, mgr)).To(Succeed())

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
