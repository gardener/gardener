// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	"k8s.io/client-go/rest"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/logger"
	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/nodeagentauthorizer"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	netutils "github.com/gardener/gardener/pkg/utils/net"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/framework"
)

func TestNodeAgentAuthorizer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ResourceManager NodeAgentAuthorizer Suite")
}

const (
	testID      = "nodeagentauthorizer-webhook-test"
	nodeName    = "foo-node"
	machineName = "foo-machine"
)

var (
	ctx = context.Background()
	log logr.Logger

	testRestConfig                 *rest.Config
	testRestConfigNodeAgentMachine *rest.Config
	testRestConfigNodeAgentNode    *rest.Config
	testEnv                        *envtest.Environment
	testClient                     client.Client
	testClientNodeAgentMachine     client.Client
	testClientNodeAgentNode        client.Client

	testRunID     string
	testNamespace *corev1.Namespace
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)
	testRunID = utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:16]

	// determine a unique namespace name for test environment
	testNamespaceName := testID + "-" + testRunID[:8]

	By("Create kubeconfig file for the authorization webhook")
	webhookAddress, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("localhost", "0"))
	Expect(err).NotTo(HaveOccurred())
	webhookPort, _, err := netutils.SuggestPort("")
	Expect(err).ToNot(HaveOccurred())
	kubeconfigFileName, err := createKubeconfigFileForAuthorizationWebhook(webhookAddress.IP.String(), webhookPort)
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		By("Delete kubeconfig file for authorization webhook")
		Expect(os.Remove(kubeconfigFileName)).To(Succeed())
	})

	By("Create authorization configuration file")
	authorizerConfigFileName, err := createAuthorizationConfigurationFile(kubeconfigFileName)
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		By("Delete authorization configuration file")
		Expect(os.Remove(authorizerConfigFileName)).To(Succeed())
	})

	By("Start test environment")
	Expect(framework.FileExists(kubeconfigFileName)).To(BeTrue())
	testAPIServer := &envtest.APIServer{}
	testAPIServer.Configure().
		Set("authorization-config", authorizerConfigFileName).
		Disable("authorization-mode").
		Disable("authorization-webhook-cache-authorized-ttl").
		Disable("authorization-webhook-cache-unauthorized-ttl")

	testEnv = &envtest.Environment{
		ControlPlane: envtest.ControlPlane{
			APIServer: testAPIServer,
		},
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join("..", "..", "..", "..", "example", "seed-crds", "10-crd-machine.sapcloud.io_machines.yaml"),
			},
		},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			LocalServingHost: webhookAddress.IP.String(),
			LocalServingPort: webhookPort,
		},
	}

	testRestConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(testRestConfig).NotTo(BeNil())

	DeferCleanup(func() {
		By("Stop target environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("Create target clients")
	testClient, err = client.New(testRestConfig, client.Options{Scheme: resourcemanagerclient.CombinedScheme})
	Expect(err).NotTo(HaveOccurred())

	userNameNodeAgentMachine := "gardener.cloud:node-agent:machine:" + machineName
	userMachine, err := testEnv.AddUser(
		envtest.User{Name: userNameNodeAgentMachine, Groups: []string{v1beta1constants.NodeAgentsGroup}},
		&rest.Config{QPS: 1000.0, Burst: 2000.0},
	)
	Expect(err).NotTo(HaveOccurred())
	testRestConfigNodeAgentMachine = userMachine.Config()

	testClientNodeAgentMachine, err = client.New(userMachine.Config(), client.Options{Scheme: resourcemanagerclient.CombinedScheme})
	Expect(err).NotTo(HaveOccurred())

	userNameNodeAgentNode := "gardener.cloud:node-agent:machine:" + nodeName
	userNode, err := testEnv.AddUser(
		envtest.User{Name: userNameNodeAgentNode, Groups: []string{v1beta1constants.NodeAgentsGroup}},
		&rest.Config{QPS: 1000.0, Burst: 2000.0},
	)
	Expect(err).NotTo(HaveOccurred())
	testRestConfigNodeAgentNode = userNode.Config()

	testClientNodeAgentNode, err = client.New(userNode.Config(), client.Options{Scheme: resourcemanagerclient.CombinedScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create test Namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
			Name: testNamespaceName,
		},
	}
	Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
	log.Info("Created test Namespace in test cluster", "namespaceName", testNamespace.Name)

	DeferCleanup(func() {
		By("Delete test Namespace from test cluster")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})
})

func createKubeconfigFileForAuthorizationWebhook(address string, port int) (string, error) {
	kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(
		"authorization-webhook",
		clientcmdv1.Cluster{
			Server:                fmt.Sprintf("https://%s:%d%s", address, port, nodeagentauthorizer.WebhookPath),
			InsecureSkipTLSVerify: true,
		},
		clientcmdv1.AuthInfo{},
	))
	if err != nil {
		return "", err
	}

	kubeConfigFile, err := os.CreateTemp("", "kubeconfig-nodeagentauthorizer-")
	if err != nil {
		return "", err
	}

	return kubeConfigFile.Name(), os.WriteFile(kubeConfigFile.Name(), kubeconfig, 0600)
}

func createAuthorizationConfigurationFile(kubeconfigFileName string) (string, error) {
	authorizationConfiguration := &apiserverv1beta1.AuthorizationConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiserverv1beta1.ConfigSchemeGroupVersion.String(),
			Kind:       "AuthorizationConfiguration",
		},
		Authorizers: []apiserverv1beta1.AuthorizerConfiguration{
			{Type: "RBAC", Name: "rbac"},
			{
				Type: "Webhook",
				Name: "node-agent-authorizer",
				Webhook: &apiserverv1beta1.WebhookConfiguration{
					// Set TTL to a very low value since it cannot be set to 0 because of defaulting.
					// See https://github.com/kubernetes/apiserver/blob/3658357fea9fa8b36173d072f2d548f135049e05/pkg/apis/apiserver/v1/defaults.go#L52-L59
					AuthorizedTTL:                            metav1.Duration{Duration: 1 * time.Nanosecond},
					UnauthorizedTTL:                          metav1.Duration{Duration: 1 * time.Nanosecond},
					Timeout:                                  metav1.Duration{Duration: 1 * time.Second},
					FailurePolicy:                            apiserverv1beta1.FailurePolicyDeny,
					SubjectAccessReviewVersion:               "v1",
					MatchConditionSubjectAccessReviewVersion: "v1",
					MatchConditions: []apiserverv1beta1.WebhookMatchCondition{{
						Expression: fmt.Sprintf("'%s' in request.groups", v1beta1constants.NodeAgentsGroup),
					}},
					ConnectionInfo: apiserverv1beta1.WebhookConnectionInfo{
						Type:           apiserverv1beta1.AuthorizationWebhookConnectionInfoTypeKubeConfigFile,
						KubeConfigFile: ptr.To(kubeconfigFileName),
					},
				},
			},
		},
	}

	authorizationConfigurationRaw, err := runtime.Encode(apiserver.ConfigCodec, authorizationConfiguration)
	if err != nil {
		return "", fmt.Errorf("unable to encode authorization configuration: %w", err)
	}

	authorizerConfigFile, err := os.CreateTemp("", "authorizer-configuration-")
	if err != nil {
		return "", err
	}

	return authorizerConfigFile.Name(), os.WriteFile(authorizerConfigFile.Name(), authorizationConfigurationRaw, 0600)
}
