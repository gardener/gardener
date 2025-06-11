// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificates_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/uuid"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/certificates"
	extensionscmdwebhook "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	extensionsshootwebhook "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	servicePort = 12345

	extensionType                   = "test"
	shootWebhookManagedResourceName = "extension-provider-test-shoot-webhooks"

	seedWebhookName, seedWebhookPath                       = "seed-webhook", "seed-path"
	shootMutatingWebhookName, shootMutatingWebhookPath     = "shoot-mutating-webhook", "shoot-mutation-path"
	shootValidatingWebhookName, shootValidatingWebhookPath = "shoot-validating-webhook", "shoot-validating-path"
)

var shootNamespaceSelector = map[string]string{"shoot.gardener.cloud/provider": extensionType}

var _ = Describe("Certificates tests", func() {
	var (
		err       error
		ok        bool
		mgr       manager.Manager
		codec     = newCodec(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		fakeClock *testclock.FakeClock

		extensionName          string
		extensionNamespace     *corev1.Namespace
		shootNamespaceTemplate *corev1.Namespace
		shootNamespace         *corev1.Namespace
		cluster                *extensionsv1alpha1.Cluster

		seedWebhook              admissionregistrationv1.MutatingWebhook
		shootMutatingWebhook     admissionregistrationv1.MutatingWebhook
		shootValidatingWebhook   admissionregistrationv1.ValidatingWebhook
		seedWebhookConfig        *admissionregistrationv1.MutatingWebhookConfiguration
		shootWebhookConfig       *extensionswebhook.Configs
		atomicShootWebhookConfig *atomic.Value
		defaultServer            *webhook.DefaultServer

		failurePolicyFail        = admissionregistrationv1.Fail
		matchPolicyExact         = admissionregistrationv1.Exact
		sideEffectsNone          = admissionregistrationv1.SideEffectClassNone
		reinvocationPolicy       = admissionregistrationv1.NeverReinvocationPolicy
		scope                    = admissionregistrationv1.AllScopes
		timeoutSeconds     int32 = 10
	)

	BeforeEach(func() {
		By("Create test Namespaces")
		extensionNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "webhook-certs-test-",
			},
		}
		Expect(testClient.Create(ctx, extensionNamespace)).To(Succeed())
		log.Info("Created extension Namespace for test", "namespaceName", extensionNamespace.Name)

		DeferCleanup(func() {
			By("Delete extension namespace")
			Expect(testClient.Delete(ctx, extensionNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		shootNamespaceTemplate = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "shoot--foo--",
				Labels: utils.MergeStringMaps(shootNamespaceSelector, map[string]string{
					"gardener.cloud/role": "shoot",
				}),
			},
		}
		shootNamespace = shootNamespaceTemplate.DeepCopy()
		Expect(testClient.Create(ctx, shootNamespace)).To(Succeed())
		log.Info("Created shoot Namespace for test", "namespaceName", shootNamespace.Name)

		DeferCleanup(func() {
			By("Delete shoot namespace")
			Expect(testClient.Delete(ctx, shootNamespace)).To(Or(Succeed(), BeNotFoundError()))

			By("Finalize shoot namespace and wait until it's gone")
			Eventually(func(g Gomega) error {
				// The tests create ManagedResources and Secrets in the shoot namespaces, but since kube-controller-manager
				// is not running in envtest, nobody cleans them up on namespace deletion. Hence, let's simply finalize
				// the namespace to get rid of it
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootNamespace), shootNamespace)).To(Succeed())
				shootNamespace.SetFinalizers(nil)
				shootNamespace.Spec.Finalizers = nil
				g.Expect(testClient.SubResource("finalize").Update(ctx, shootNamespace)).To(Or(Succeed(), BeNotFoundError()))

				return testClient.Get(ctx, client.ObjectKeyFromObject(shootNamespace), &corev1.Namespace{})
			}).Should(BeNotFoundError())
		})

		// use unique extension name for each test,for unique webhook config name
		extensionName = "provider-test-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

		fakeClock = testclock.NewFakeClock(time.Now())

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootNamespace.Name,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				CloudProfile: runtime.RawExtension{Object: &gardencorev1beta1.CloudProfile{}},
				Seed:         runtime.RawExtension{Object: &gardencorev1beta1.Seed{}},
				Shoot:        runtime.RawExtension{Object: &gardencorev1beta1.Shoot{}},
			},
		}

		shootMutatingWebhook = admissionregistrationv1.MutatingWebhook{
			Name: fmt.Sprintf("%s.%s.extensions.gardener.cloud", shootMutatingWebhookName, extensionType),
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL: ptr.To("https://gardener-extension-" + extensionName + "." + extensionNamespace.Name + ":443/" + shootMutatingWebhookPath),
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"serviceaccounts"}, Scope: &scope},
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
				},
			},
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			FailurePolicy:           &failurePolicyFail,
			MatchPolicy:             &matchPolicyExact,
			SideEffects:             &sideEffectsNone,
			TimeoutSeconds:          &timeoutSeconds,
			ReinvocationPolicy:      &reinvocationPolicy,
			NamespaceSelector:       &metav1.LabelSelector{},
			ObjectSelector:          &metav1.LabelSelector{},
		}
		shootValidatingWebhook = admissionregistrationv1.ValidatingWebhook{
			Name: fmt.Sprintf("%s.%s.extensions.gardener.cloud", shootValidatingWebhookName, extensionType),
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL: ptr.To("https://gardener-extension-" + extensionName + "." + extensionNamespace.Name + ":443/" + shootMutatingWebhookPath),
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"serviceaccounts"}, Scope: &scope},
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
				},
			},
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			FailurePolicy:           &failurePolicyFail,
			MatchPolicy:             &matchPolicyExact,
			SideEffects:             &sideEffectsNone,
			TimeoutSeconds:          &timeoutSeconds,
			NamespaceSelector:       &metav1.LabelSelector{},
			ObjectSelector:          &metav1.LabelSelector{},
		}

		shootWebhookConfig = &extensionswebhook.Configs{
			MutatingWebhookConfig: &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener-extension-" + extensionName + "-shoot"},
				Webhooks:   []admissionregistrationv1.MutatingWebhook{shootMutatingWebhook},
			},
			ValidatingWebhookConfig: &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener-extension-" + extensionName + "-shoot"},
				Webhooks:   []admissionregistrationv1.ValidatingWebhook{shootValidatingWebhook},
			},
		}
	})

	Context("run without seed webhook", func() {
		JustBeforeEach(func() {
			By("Setup manager")
			mgr, err = manager.New(restConfig, manager.Options{
				Scheme:  kubernetes.SeedScheme,
				Metrics: metricsserver.Options{BindAddress: "0"},
				Controller: controllerconfig.Controller{
					SkipNameValidation: ptr.To(true),
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Register webhooks")
			var (
				serverOptions = &extensionscmdwebhook.ServerOptions{
					Mode:        extensionswebhook.ModeService,
					ServicePort: servicePort,
					Namespace:   extensionNamespace.Name,
				}
				switchOptions = extensionscmdwebhook.NewSwitchOptions(
					extensionscmdwebhook.Switch(shootMutatingWebhookName, newShootMutatingWebhook),
				)
				webhookOptions = extensionscmdwebhook.NewAddToManagerOptions(extensionName, shootWebhookManagedResourceName, shootNamespaceSelector, serverOptions, switchOptions)
			)

			shootWebhookConfig.ValidatingWebhookConfig = nil
			Expect(webhookOptions.Complete()).To(Succeed())
			webhookConfig := webhookOptions.Completed()
			webhookConfig.Clock = fakeClock
			atomicShootWebhookConfig, err = webhookConfig.AddToManager(ctx, mgr, nil, false)
			Expect(err).NotTo(HaveOccurred())

			defaultServer, ok = mgr.GetWebhookServer().(*webhook.DefaultServer)
			Expect(ok).To(BeTrue())

			By("Verify certificates exist on disk")
			Eventually(func(g Gomega) {
				serverCert, err := os.ReadFile(filepath.Join(defaultServer.Options.CertDir, "tls.crt"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(serverCert).NotTo(BeEmpty())

				serverKey, err := os.ReadFile(filepath.Join(defaultServer.Options.CertDir, "tls.key"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(serverKey).NotTo(BeEmpty())
			}).Should(Succeed())

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

			By("Verify CA bundle was written in atomic shoot mutating webhook config")
			Eventually(func() []byte {
				val, ok := atomicShootWebhookConfig.Load().(*extensionswebhook.Configs)
				if !ok {
					return nil
				}
				if val.MutatingWebhookConfig == nil {
					return nil
				}
				return val.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle
			}).ShouldNot(BeEmpty())
		})

		Context("certificate rotation", func() {
			var shootNamespace2 *corev1.Namespace

			BeforeEach(func() {
				shootNamespace2 = shootNamespaceTemplate.DeepCopy()
				cluster2 := cluster.DeepCopy()

				By("Prepare existing shoot webhook resources")
				Expect(testClient.Create(ctx, cluster)).To(Succeed())
				Expect(extensionsshootwebhook.ReconcileWebhookConfig(ctx, testClient, shootNamespace.Name, shootWebhookManagedResourceName, *shootWebhookConfig, &extensions.Cluster{Shoot: &gardencorev1beta1.Shoot{}}, true)).To(Succeed())

				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, cluster)).To(Or(Succeed(), BeNotFoundError()))
				})

				DeferCleanup(test.WithVars(
					&certificates.DefaultSyncPeriod, 100*time.Millisecond,
					&secretsutils.GenerateKey, secretsutils.FakeGenerateKey,
					&secretsutils.Clock, fakeClock,
				))

				By("Prepare another shoot namespace that is incomplete", func() {
					Expect(testClient.Create(ctx, shootNamespace2)).To(Succeed())

					cluster2.Name = shootNamespace2.Name
					Expect(testClient.Create(ctx, cluster2)).To(Succeed())

					DeferCleanup(func() {
						Expect(testClient.Delete(ctx, cluster2)).To(Or(Succeed(), BeNotFoundError()))
						Expect(testClient.Delete(ctx, shootNamespace2)).To(Or(Succeed(), BeNotFoundError()))
					})
				})
			})

			It("should rotate the certificates and update the webhook configs", func() {
				var serverCert1 []byte

				By("Retrieve CA bundle (before first reconciliation)")
				Eventually(func(g Gomega) []byte {
					g.Expect(getShootWebhookConfig(codec, shootWebhookConfig, shootNamespace.Name)).To(Succeed())
					return shootWebhookConfig.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle
				}).Should(Not(BeEmpty()))

				By("Read generated server certificate from disk")
				Eventually(func(g Gomega) []byte {
					serverCert1, err = os.ReadFile(filepath.Join(defaultServer.Options.CertDir, "tls.crt"))
					g.Expect(err).NotTo(HaveOccurred())
					return serverCert1
				}).Should(Not(BeEmpty()))

				Eventually(func(g Gomega) []byte {
					serverKey1, err := os.ReadFile(filepath.Join(defaultServer.Options.CertDir, "tls.key"))
					g.Expect(err).NotTo(HaveOccurred())
					return serverKey1
				}).Should(Not(BeEmpty()))

				By("Retrieve CA bundle again (after validity has expired)")
				fakeClock.Step(30 * 24 * time.Hour)

				Eventually(func(g Gomega) []byte {
					g.Expect(getShootWebhookConfig(codec, shootWebhookConfig, shootNamespace.Name)).To(Succeed())
					return shootWebhookConfig.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle
				}).Should(Not(BeEmpty()))

				By("Verify managed resource in incomplete shoot namespace was not created", func() {
					Expect(getShootWebhookConfig(codec, shootWebhookConfig, shootNamespace2.Name)).To(BeNotFoundError())
				})

				By("Read re-generated server certificate from disk")
				Eventually(func(g Gomega) []byte {
					serverCert2, err := os.ReadFile(filepath.Join(defaultServer.Options.CertDir, "tls.crt"))
					g.Expect(err).NotTo(HaveOccurred())
					return serverCert2
				}).Should(And(
					Not(BeEmpty()),
					Not(Equal(serverCert1)),
				))
			})
		})
	})

	Context("run with seed webhook", func() {
		BeforeEach(func() {
			seedWebhook = admissionregistrationv1.MutatingWebhook{
				Name: fmt.Sprintf("%s.%s.extensions.gardener.cloud", seedWebhookName, strings.TrimPrefix(extensionName, "provider-")),
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name:      "gardener-extension-" + extensionName,
						Namespace: extensionNamespace.Name,
						Path:      ptr.To("/" + seedWebhookPath),
						Port:      ptr.To[int32](443),
					},
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}, Scope: &scope},
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					},
				},
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				// here variable failurePolicyFail can't be used as it can be overwritten to
				// `Ignore` by previous tests
				FailurePolicy:      (*admissionregistrationv1.FailurePolicyType)(ptr.To("Fail")),
				MatchPolicy:        &matchPolicyExact,
				SideEffects:        &sideEffectsNone,
				TimeoutSeconds:     &timeoutSeconds,
				ReinvocationPolicy: &reinvocationPolicy,
				NamespaceSelector:  &metav1.LabelSelector{},
				ObjectSelector:     &metav1.LabelSelector{},
			}

			seedWebhookConfig = &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener-extension-" + extensionName},
				Webhooks:   []admissionregistrationv1.MutatingWebhook{seedWebhook},
			}
		})

		JustBeforeEach(func() {
			By("Setup manager")
			mgr, err = manager.New(restConfig, manager.Options{
				Scheme:  kubernetes.SeedScheme,
				Metrics: metricsserver.Options{BindAddress: "0"},
				Controller: controllerconfig.Controller{
					SkipNameValidation: ptr.To(true),
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Register webhooks")
			var (
				serverOptions = &extensionscmdwebhook.ServerOptions{
					Mode:        extensionswebhook.ModeService,
					ServicePort: servicePort,
					Namespace:   extensionNamespace.Name,
				}
				switchOptions = extensionscmdwebhook.NewSwitchOptions(
					extensionscmdwebhook.Switch(seedWebhookName, newSeedWebhook),
					extensionscmdwebhook.Switch(shootMutatingWebhookName, newShootMutatingWebhook),
					extensionscmdwebhook.Switch(shootValidatingWebhookName, newShootValidatingWebhook),
				)
				webhookOptions = extensionscmdwebhook.NewAddToManagerOptions(extensionName, shootWebhookManagedResourceName, shootNamespaceSelector, serverOptions, switchOptions)
			)

			Expect(webhookOptions.Complete()).To(Succeed())
			webhookConfig := webhookOptions.Completed()
			webhookConfig.Clock = fakeClock
			atomicShootWebhookConfig, err = webhookConfig.AddToManager(ctx, mgr, nil, false)
			Expect(err).NotTo(HaveOccurred())

			defaultServer, ok = mgr.GetWebhookServer().(*webhook.DefaultServer)
			Expect(ok).To(BeTrue())

			By("Verify certificates exist on disk")
			Eventually(func(g Gomega) {
				serverCert, err := os.ReadFile(filepath.Join(defaultServer.Options.CertDir, "tls.crt"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(serverCert).NotTo(BeEmpty())

				serverKey, err := os.ReadFile(filepath.Join(defaultServer.Options.CertDir, "tls.key"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(serverKey).NotTo(BeEmpty())
			}).Should(Succeed())

			By("Start manager")
			mgrContext, mgrCancel := context.WithCancel(ctx)

			go func() {
				defer GinkgoRecover()
				Expect(mgr.Start(mgrContext)).To(Succeed())
			}()

			// Wait for the webhook server to start
			Eventually(func() error {
				checker := defaultServer.StartedChecker()
				return checker(&http.Request{})
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Stop manager")
				mgrCancel()
			})

			By("Verify CA bundle was written in atomic shoot webhook configs")
			Eventually(func(g Gomega) {
				val, ok := atomicShootWebhookConfig.Load().(*extensionswebhook.Configs)
				g.Expect(ok).To(BeTrue())
				g.Expect(val.MutatingWebhookConfig).NotTo(BeNil())
				g.Expect(val.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).NotTo(BeEmpty())
				g.Expect(val.ValidatingWebhookConfig).NotTo(BeNil())
				g.Expect(val.ValidatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).NotTo(BeEmpty())
			}).Should(Succeed())
		})

		AfterEach(func() {
			By("Delete webhook config")
			Expect(testClient.Delete(ctx, seedWebhookConfig)).To(Or(Succeed(), BeNotFoundError()))
		})

		Context("seed webhook does not yet exist", func() {
			It("should create the webhook and inject the CA bundle", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seedWebhookConfig), seedWebhookConfig)).To(Succeed())
					g.Expect(extensionswebhook.InjectCABundleIntoWebhookConfig(seedWebhookConfig, nil)).To(Succeed())
					g.Expect(seedWebhookConfig.Webhooks).To(ConsistOf(seedWebhook))
				}).Should(Succeed())

				Eventually(func(g Gomega) []byte {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seedWebhookConfig), seedWebhookConfig)).To(Succeed())
					return seedWebhookConfig.Webhooks[0].ClientConfig.CABundle
				}).Should(Not(BeEmpty()))
			})
		})

		Context("certificate rotation", func() {
			BeforeEach(func() {
				By("Prepare existing shoot webhook resources")
				Expect(testClient.Create(ctx, cluster)).To(Succeed())
				Expect(extensionsshootwebhook.ReconcileWebhookConfig(ctx, testClient, shootNamespace.Name, shootWebhookManagedResourceName, *shootWebhookConfig, &extensions.Cluster{Shoot: &gardencorev1beta1.Shoot{}}, true)).To(Succeed())

				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, cluster)).To(Or(Succeed(), BeNotFoundError()))
				})

				DeferCleanup(test.WithVars(
					&certificates.DefaultSyncPeriod, 100*time.Millisecond,
					&secretsutils.GenerateKey, secretsutils.FakeGenerateKey,
					&secretsutils.Clock, fakeClock,
				))
			})

			It("should rotate the certificates and update the webhook configs", func() {
				var caBundle1, caBundle2, caBundle3, serverCert1 []byte

				By("Retrieve CA bundle (before first reconciliation)")
				Eventually(func(g Gomega) []byte {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seedWebhookConfig), seedWebhookConfig)).To(Succeed())
					caBundle1 = seedWebhookConfig.Webhooks[0].ClientConfig.CABundle
					return caBundle1
				}).Should(Not(BeEmpty()))

				Eventually(func(g Gomega) {
					g.Expect(getShootWebhookConfig(codec, shootWebhookConfig, shootNamespace.Name)).To(Succeed())
					g.Expect(shootWebhookConfig.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle1))
					g.Expect(shootWebhookConfig.ValidatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle1))
				}).Should(Succeed())

				By("Read generated server certificate from disk")
				Eventually(func(g Gomega) []byte {
					serverCert1, err = os.ReadFile(filepath.Join(defaultServer.Options.CertDir, "tls.crt"))
					g.Expect(err).NotTo(HaveOccurred())
					return serverCert1
				}).Should(Not(BeEmpty()))

				Eventually(func(g Gomega) []byte {
					serverKey1, err := os.ReadFile(filepath.Join(defaultServer.Options.CertDir, "tls.key"))
					g.Expect(err).NotTo(HaveOccurred())
					return serverKey1
				}).Should(Not(BeEmpty()))

				By("Retrieve CA bundle again (after validity has expired)")
				fakeClock.Step(30 * 24 * time.Hour)

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seedWebhookConfig), seedWebhookConfig)).To(Succeed())
					caBundle2 = seedWebhookConfig.Webhooks[0].ClientConfig.CABundle
					return string(caBundle2)
				}).Should(And(
					Not(BeEmpty()),
					Not(BeEquivalentTo(caBundle1)),
					Not(ContainSubstring(string(caBundle1))),
				))

				Eventually(func(g Gomega) {
					g.Expect(getShootWebhookConfig(codec, shootWebhookConfig, shootNamespace.Name)).To(Succeed())
					g.Expect(shootWebhookConfig.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle2))
					g.Expect(shootWebhookConfig.ValidatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle2))
				}).Should(Succeed())

				caCert2 := strings.TrimPrefix(string(caBundle2), string(caBundle1))

				By("Read re-generated server certificate from disk")
				Eventually(func(g Gomega) []byte {
					serverCert2, err := os.ReadFile(filepath.Join(defaultServer.Options.CertDir, "tls.crt"))
					g.Expect(err).NotTo(HaveOccurred())
					return serverCert2
				}).Should(And(
					Not(BeEmpty()),
					Not(Equal(serverCert1)),
				))

				// we don't assert that the server key changed since we have overwritten the 'GenerateKey' function with
				// a fake implementation above (hence, it cannot change)

				By("Retrieve CA bundle again (after old secrets are ignored)")
				fakeClock.Step(24 * time.Hour)

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seedWebhookConfig), seedWebhookConfig)).To(Succeed())
					caBundle3 = seedWebhookConfig.Webhooks[0].ClientConfig.CABundle
					return string(caBundle3)
				}).Should(And(
					Not(BeEmpty()),
					Equal(caCert2),
				))

				Eventually(func(g Gomega) {
					g.Expect(getShootWebhookConfig(codec, shootWebhookConfig, shootNamespace.Name)).To(Succeed())
					g.Expect(shootWebhookConfig.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle3))
					g.Expect(shootWebhookConfig.ValidatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle3))
				}).Should(Succeed())
			})
		})
	})
})

func newSeedWebhook(_ manager.Manager) (*extensionswebhook.Webhook, error) {
	return &extensionswebhook.Webhook{
		Name:     seedWebhookName,
		Path:     seedWebhookPath,
		Provider: extensionType,
		Types:    []extensionswebhook.Type{{Obj: &corev1.Service{}}},
		Target:   extensionswebhook.TargetSeed,
	}, nil
}

func newShootMutatingWebhook(_ manager.Manager) (*extensionswebhook.Webhook, error) {
	return &extensionswebhook.Webhook{
		Name:     shootMutatingWebhookName,
		Path:     shootMutatingWebhookPath,
		Provider: extensionType,
		Types:    []extensionswebhook.Type{{Obj: &corev1.ServiceAccount{}}},
		Target:   extensionswebhook.TargetShoot,
	}, nil
}

func newShootValidatingWebhook(_ manager.Manager) (*extensionswebhook.Webhook, error) {
	return &extensionswebhook.Webhook{
		Action:   "validating",
		Name:     shootValidatingWebhookName,
		Path:     shootValidatingWebhookPath,
		Provider: extensionType,
		Types:    []extensionswebhook.Type{{Obj: &corev1.ServiceAccount{}}},
		Target:   extensionswebhook.TargetShoot,
	}, nil
}

func newCodec(scheme *runtime.Scheme, codec serializer.CodecFactory, serializer *json.Serializer) runtime.Codec {
	var groupVersions []schema.GroupVersion
	for k := range scheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}

	return codec.CodecForVersions(serializer, serializer, schema.GroupVersions(groupVersions), schema.GroupVersions(groupVersions))
}

func getShootWebhookConfig(codec runtime.Codec, shootWebhookConfig *extensionswebhook.Configs, namespace string) error {
	managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{
		Name:      shootWebhookManagedResourceName,
		Namespace: namespace,
	}}

	if err := testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource); err != nil {
		return err
	}

	managedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResource.Spec.SecretRefs[0].Name, Namespace: namespace}}
	if err := testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret); err != nil {
		return err
	}

	manifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
	if err != nil {
		return err
	}

	for _, manifest := range manifests {
		if shootWebhookConfig.MutatingWebhookConfig != nil && strings.Contains(manifest, "kind: MutatingWebhookConfiguration") && strings.Contains(manifest, "name: "+shootWebhookConfig.MutatingWebhookConfig.Name) {
			if _, _, err := codec.Decode([]byte(manifest), nil, shootWebhookConfig.MutatingWebhookConfig); err != nil {
				return err
			}
		}
		if shootWebhookConfig.ValidatingWebhookConfig != nil && strings.Contains(manifest, "kind: ValidatingWebhookConfiguration") && strings.Contains(manifest, "name: "+shootWebhookConfig.ValidatingWebhookConfig.Name) {
			if _, _, err := codec.Decode([]byte(manifest), nil, shootWebhookConfig.ValidatingWebhookConfig); err != nil {
				return err
			}
		}
	}

	return nil
}
