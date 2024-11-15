// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/nodeagentauthorizer"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/framework"
)

func TestNodeAgentAuthorizer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ResourceManager NodeAgentAuthorizer Suite")
}

const testID = "nodeagentauthorizer-webhook-test"

var (
	ctx = context.Background()
	log logr.Logger

	sourceRestConfig *rest.Config
	sourceEnv        *envtest.Environment
	sourceClient     client.Client

	targetRestConfig          *rest.Config
	targetRestConfigNodeAgent *rest.Config
	targetEnv                 *envtest.Environment
	targetClient              client.Client
	targetClientNodeAgent     client.Client

	testRunID     string
	testNamespace *corev1.Namespace

	machineName       string
	userNameNodeAgent string
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)
	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:16]

	// determine a unique namespace name for test environment
	testNamespaceName := testID + "-" + testRunID[:8]

	By("Start source environment")
	sourceEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join("..", "..", "..", "..", "example", "seed-crds", "10-crd-machine.sapcloud.io_machines.yaml"),
			},
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	sourceRestConfig, err = sourceEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(sourceRestConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop source environment")
		Expect(sourceEnv.Stop()).To(Succeed())
	})

	By("Create source client")
	sourceClient, err = client.New(sourceRestConfig, client.Options{Scheme: resourcemanagerclient.CombinedScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create test Namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
			Name: testNamespaceName,
		},
	}
	Expect(sourceClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created test Namespace in source cluster", "namespaceName", testNamespace.Name)

	DeferCleanup(func() {
		By("Delete test Namespace from source cluster")
		Expect(sourceClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Create kubeconfig file for source cluster")
	kubeconfigFileName, err := createKubeconfigFileForAuthorizationWebhook(sourceEnv.WebhookInstallOptions)
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		By("Delete kubeconfig file for source cluster")
		Expect(os.Remove(kubeconfigFileName)).To(Succeed())
	})

	By("Start target environment")
	Expect(framework.FileExists(kubeconfigFileName)).To(BeTrue())
	testAPIServer := &envtest.APIServer{}
	testAPIServer.Configure().
		Set("authorization-mode", "RBAC", "Webhook").
		Set("authorization-webhook-config-file", kubeconfigFileName).
		Set("authorization-webhook-cache-authorized-ttl", "0").
		Set("authorization-webhook-cache-unauthorized-ttl", "0")

	targetEnv = &envtest.Environment{
		ControlPlane: envtest.ControlPlane{
			APIServer: testAPIServer,
		},
	}

	targetRestConfig, err = targetEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(targetRestConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop target environment")
		Expect(targetEnv.Stop()).To(Succeed())
	})

	By("Create target clients")
	targetClient, err = client.New(targetRestConfig, client.Options{Scheme: resourcemanagerclient.CombinedScheme})
	Expect(err).NotTo(HaveOccurred())

	machineName = "machine-" + testRunID
	userNameNodeAgent = "gardener.cloud:node-agent:machine:" + machineName

	user, err := targetEnv.AddUser(
		envtest.User{Name: userNameNodeAgent, Groups: []string{v1beta1constants.NodeAgentsGroup}},
		&rest.Config{QPS: 1000.0, Burst: 2000.0},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(user).NotTo(BeNil())
	targetRestConfigNodeAgent = user.Config()

	targetClientNodeAgent, err = client.New(user.Config(), client.Options{Scheme: resourcemanagerclient.CombinedScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Setup manager")
	mgr, err := manager.New(sourceRestConfig, manager.Options{
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    sourceEnv.WebhookInstallOptions.LocalServingPort,
			Host:    sourceEnv.WebhookInstallOptions.LocalServingHost,
			CertDir: sourceEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{testNamespace.Name: {}},
		},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	By("Register webhook")
	Expect((&nodeagentauthorizer.Webhook{
		Logger: log,
		Config: config.NodeAgentAuthorizerWebhookConfig{
			Enabled:          true,
			MachineNamespace: testNamespaceName,
		},
	}).AddToManager(mgr, sourceClient, targetClient)).To(Succeed())

	By("Start manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	// Wait for the webhook server to start
	Eventually(func() error {
		checker := mgr.GetWebhookServer().StartedChecker()
		return checker(&http.Request{})
	}).Should(Succeed())

	DeferCleanup(func() {
		By("Stop manager")
		mgrCancel()
	})
})

func createKubeconfigFileForAuthorizationWebhook(webhookInstallOptions envtest.WebhookInstallOptions) (string, error) {
	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters["authorization-webhook"] = &clientcmdapi.Cluster{
		Server:                   fmt.Sprintf("https://%s:%d%s", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort, nodeagentauthorizer.WebhookPath),
		CertificateAuthorityData: webhookInstallOptions.LocalServingCAData,
	}
	contexts := make(map[string]*clientcmdapi.Context)
	contexts["authorization-webhook"] = &clientcmdapi.Context{
		Cluster:  "authorization-webhook",
		AuthInfo: "authorization-webhook",
	}
	authinfos := make(map[string]*clientcmdapi.AuthInfo)
	clientConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: "authorization-webhook",
		AuthInfos:      authinfos,
	}
	kubeConfigFile, err := os.CreateTemp("", "kubeconfig-nodeagentauthorizer-")
	if err != nil {
		return "", err
	}
	if err := clientcmd.WriteToFile(clientConfig, kubeConfigFile.Name()); err != nil {
		return "", err
	}
	return kubeConfigFile.Name(), nil
}
