// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificatesigningrequest_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	certificatesigningrequestcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/certificatesigningrequest"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

func TestCertificateSigningRequest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ControllerManager CertificateSigningRequest Suite")
}

// testID is used for generating test namespace names and other IDs
const testID = "csr-autoapprove-controller-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig       *rest.Config
	testEnv          *gardenerenvtest.GardenerTestEnvironment
	testClient       client.Client
	kubernetesClient *kubernetesclientset.Clientset

	testNamespace *corev1.Namespace
	testRunID     string

	bootstrapTokenID = "123abc"
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &gardenerenvtest.GardenerTestEnvironment{}

	var err error
	restConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("Create test clients")
	// envtest.Environment.AddUser doesn't work when running against an existing cluster
	// use impersonation instead to simulate different user
	userConfig := rest.CopyConfig(restConfig)
	userConfig.Impersonate = rest.ImpersonationConfig{
		UserName: "system:bootstrap:" + bootstrapTokenID,
		Groups: []string{
			"system:bootstrappers",
			"system:masters", // added to be able to create stuff without additional RBAC rules
		},
	}
	testClient, err = client.New(userConfig, client.Options{Scheme: kubernetes.GardenScheme})
	Expect(err).NotTo(HaveOccurred())
	kubernetesClient, err = kubernetesclientset.NewForConfig(userConfig)
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

	By("Create RBAC for bootstrap tokens")
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gardener.cloud:system:seed-bootstrapper-test-" + testRunID,
			Labels: map[string]string{testID: testRunID},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"certificates.k8s.io"},
				Resources: []string{"certificatesigningrequests"},
				Verbs:     []string{"create", "get"},
			},
			{
				APIGroups: []string{"certificates.k8s.io"},
				Resources: []string{"certificatesigningrequests/seedclient", "certificatesigningrequests/shootclient"},
				Verbs:     []string{"create"},
			},
		},
	}
	Expect(testClient.Create(ctx, clusterRole)).To(Succeed())

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gardener.cloud:system:seed-bootstrapper-test-" + testRunID,
			Labels: map[string]string{testID: testRunID},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.GroupKind,
				Name:     "system:bootstrappers",
				APIGroup: rbacv1.GroupName,
			},
		},
	}
	Expect(testClient.Create(ctx, clusterRoleBinding)).To(Succeed())

	DeferCleanup(func() {
		By("Delete RBAC resources")
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, clusterRoleBinding))).To(Succeed())
		Expect(client.IgnoreNotFound(testClient.Delete(ctx, clusterRole))).To(Succeed())
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:  kubernetes.GardenScheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				testNamespace.Name: {},
				"kube-system":      {},
			},
			ByObject: map[client.Object]cache.ByObject{
				&certificatesv1.CertificateSigningRequest{}: {
					Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
				},
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

	By("Register controller")
	Expect((&certificatesigningrequestcontroller.Reconciler{
		CertificatesClient: kubernetesClient.CertificatesV1().CertificateSigningRequests(),
		Config: controllermanagerconfigv1alpha1.CertificateSigningRequestControllerConfiguration{
			ConcurrentSyncs: ptr.To(5),
		},
	}).AddToManager(mgr)).To(Succeed())

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
