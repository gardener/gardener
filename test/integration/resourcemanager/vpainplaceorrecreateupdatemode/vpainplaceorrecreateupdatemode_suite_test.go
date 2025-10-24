// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpainplaceorrecreateupdatemode_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
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
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	"github.com/gardener/gardener/pkg/logger"
	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/vpainplaceorrecreateupdatemode"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

func TestVPAInPlaceUpdatesProfile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Integration ResourceManager VPAInPlaceUpdates Suite")
}

const testID = "vpa-in-place-or-recreate-update-mode-webhook-test"

var (
	ctx = context.Background()
	log logr.Logger

	testEnv    *envtest.Environment
	restConfig *rest.Config
	testClient client.Client

	testNamespace *corev1.Namespace
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName(testID)

	testNamespaceName := testID + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

	By("Start test environment")
	testEnv = &envtest.Environment{
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			MutatingWebhooks: getMutatingWebhookConfigurations(testNamespaceName),
		},
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "..", "pkg", "component", "autoscaling", "vpa", "templates", "crd-autoscaling.k8s.io_verticalpodautoscalers.yaml")},
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

	By("Create test client")
	testClient, err = client.New(restConfig, client.Options{Scheme: resourcemanagerclient.CombinedScheme})
	Expect(err).NotTo(HaveOccurred())

	By("Create test Namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespaceName,
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
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    testEnv.WebhookInstallOptions.LocalServingPort,
			Host:    testEnv.WebhookInstallOptions.LocalServingHost,
			CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
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
	Expect((&vpainplaceorrecreateupdatemode.Handler{
		Logger: log,
	}).AddToManager(mgr)).To(Succeed())

	By("Start manager")
	mgrContext, mgrCancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrContext)).To(Succeed())
	}()

	Eventually(func() error {
		checker := mgr.GetWebhookServer().StartedChecker()
		return checker(&http.Request{})
	}).Should(Succeed())

	DeferCleanup(func() {
		By("Stop manager")
		mgrCancel()
	})
})

func getMutatingWebhookConfigurations(namespaceName string) []*admissionregistrationv1.MutatingWebhookConfiguration {
	return []*admissionregistrationv1.MutatingWebhookConfiguration{
		{
			TypeMeta: metav1.TypeMeta{
				APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
				Kind:       "MutatingWebhookConfiguration",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener-resource-manager",
			},
			Webhooks: []admissionregistrationv1.MutatingWebhook{
				resourcemanager.NewInPlaceOrRecreateUpdateModeWebhook(&metav1.LabelSelector{
					MatchLabels: map[string]string{corev1.LabelMetadataName: namespaceName},
				}, nil, func(_ *corev1.Secret, path string) admissionregistrationv1.WebhookClientConfig {
					return admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Path: &path,
						},
					}
				}),
			},
		},
	}
}
