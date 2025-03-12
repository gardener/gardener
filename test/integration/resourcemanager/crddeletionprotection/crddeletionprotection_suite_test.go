// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeletionprotection_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cmdutils "github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	"github.com/gardener/gardener/pkg/logger"
	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/crddeletionprotection"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

func TestCRDDeletionProtection(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ResourceManager CRDDeletionProtection Suite")
}

const testID = "crddeletionprotection-webhook-test"

var (
	ctx = context.Background()
	log logr.Logger

	restConfig *rest.Config
	testEnv    *envtest.Environment
	testClient client.Client

	testNamespace *corev1.Namespace
)

var _ = BeforeSuite(func() {
	cmdutils.DeduplicateWarnings()

	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	By("Start test environment")
	testEnv = &envtest.Environment{
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			ValidatingWebhooks: []*admissionregistrationv1.ValidatingWebhookConfiguration{getValidatingWebhookConfig()},
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

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: resourcemanagerclient.SourceScheme})
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

	DeferCleanup(func() {
		By("Delete test Namespace")
		Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
	})

	By("Setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme: kubernetes.SeedScheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    testEnv.WebhookInstallOptions.LocalServingPort,
			Host:    testEnv.WebhookInstallOptions.LocalServingHost,
			CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
		Metrics: metricsserver.Options{BindAddress: "0"},
		Controller: controllerconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	By("Register webhooks")
	Expect((&crddeletionprotection.Handler{
		Logger:       log,
		SourceReader: mgr.GetAPIReader(),
		Decoder:      admission.NewDecoder(mgr.GetScheme()),
	}).AddToManager(mgr)).To(Succeed())

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

func getValidatingWebhookConfig() *admissionregistrationv1.ValidatingWebhookConfiguration {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener-resource-manager",
		},
		Webhooks: resourcemanager.GetCRDDeletionProtectionValidatingWebhooks(nil, func(_ *corev1.Secret, path string) admissionregistrationv1.WebhookClientConfig {
			return admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Path: &path,
				},
			}
		}),
	}
}

func getEnvTestK8SVersion() (string, error) {
	k8sVersion, ok := os.LookupEnv("ENVTEST_K8S_VERSION")
	if !ok {
		return "", errors.New("error fetching k8s version from environment")
	}
	return k8sVersion, nil
}
