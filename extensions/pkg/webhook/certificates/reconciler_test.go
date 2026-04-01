// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificates

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx context.Context

		fakeClient    client.Client
		fakeClock     *testclock.FakeClock
		r             *reconciler
		namespace     = "garden"
		componentName = "provider-test"
		identity      = "gardener-extension-provider-test-webhook"
		caSecretName  = "ca-provider-test-webhook"
		serverSecName = "provider-test-webhook-server"
	)

	BeforeEach(func() {
		ctx = context.Background()

		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(admissionregistrationv1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		fakeClock = testclock.NewFakeClock(time.Now())

		r = &reconciler{
			Clock:                    fakeClock,
			SyncPeriod:               DefaultSyncPeriod,
			CASecretName:             caSecretName,
			ServerSecretName:         serverSecName,
			Namespace:                namespace,
			Identity:                 identity,
			ComponentName:            componentName,
			DoNotPrefixComponentName: false,
			Mode:                     extensionswebhook.ModeService,
			SourceWebhookConfigs: extensionswebhook.Configs{
				MutatingWebhookConfig: &admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gardener-extension-provider-test",
					},
					Webhooks: []admissionregistrationv1.MutatingWebhook{{
						Name: "test-webhook",
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							CABundle: []byte("old-ca-bundle"),
						},
					}},
				},
			},
			client:       fakeClient,
			sourceClient: fakeClient,
		}
	})

	Describe("#Reconcile", func() {
		It("should generate CA and server cert, create webhook configs, and requeue", func() {
			result, err := r.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))

			// Verify CA secret was created
			caSecrets := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, caSecrets, client.InNamespace(namespace), client.MatchingLabels{
				secretsmanager.LabelKeyName:            caSecretName,
				secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
				secretsmanager.LabelKeyManagerIdentity: identity,
			})).To(Succeed())
			Expect(caSecrets.Items).NotTo(BeEmpty())

			// Verify server secret was created
			serverSecrets := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, serverSecrets, client.InNamespace(namespace), client.MatchingLabels{
				secretsmanager.LabelKeyName:            serverSecName,
				secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
				secretsmanager.LabelKeyManagerIdentity: identity,
			})).To(Succeed())
			Expect(serverSecrets.Items).NotTo(BeEmpty())

			// Verify that server secret contains tls.crt and tls.key
			serverSecret := serverSecrets.Items[0]
			Expect(serverSecret.Data).To(HaveKey(secretsutils.DataKeyCertificate))
			Expect(serverSecret.Data).To(HaveKey(secretsutils.DataKeyPrivateKey))

			// Verify the source webhook config was created with CA bundle
			mutatingConfig := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "gardener-extension-provider-test"}, mutatingConfig)).To(Succeed())
			Expect(mutatingConfig.Webhooks).To(HaveLen(1))
			Expect(mutatingConfig.Webhooks[0].ClientConfig.CABundle).NotTo(BeEmpty())
			Expect(mutatingConfig.Webhooks[0].ClientConfig.CABundle).NotTo(Equal([]byte("old-ca-bundle")))
		})

		It("should handle validating webhook configs", func() {
			r.SourceWebhookConfigs = extensionswebhook.Configs{
				ValidatingWebhookConfig: &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gardener-extension-provider-test-validating",
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{{
						Name: "test-validating-webhook",
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							CABundle: []byte("old-ca-bundle"),
						},
					}},
				},
			}

			result, err := r.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))

			validatingConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "gardener-extension-provider-test-validating"}, validatingConfig)).To(Succeed())
			Expect(validatingConfig.Webhooks).To(HaveLen(1))
			Expect(validatingConfig.Webhooks[0].ClientConfig.CABundle).NotTo(BeEmpty())
			Expect(validatingConfig.Webhooks[0].ClientConfig.CABundle).NotTo(Equal([]byte("old-ca-bundle")))
		})

		It("should handle both mutating and validating webhook configs", func() {
			r.SourceWebhookConfigs = extensionswebhook.Configs{
				MutatingWebhookConfig: &admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gardener-extension-provider-test",
					},
					Webhooks: []admissionregistrationv1.MutatingWebhook{{
						Name: "test-webhook",
					}},
				},
				ValidatingWebhookConfig: &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gardener-extension-provider-test-validating",
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{{
						Name: "test-validating-webhook",
					}},
				},
			}

			result, err := r.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))

			mutatingConfig := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "gardener-extension-provider-test"}, mutatingConfig)).To(Succeed())
			Expect(mutatingConfig.Webhooks[0].ClientConfig.CABundle).NotTo(BeEmpty())

			validatingConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "gardener-extension-provider-test-validating"}, validatingConfig)).To(Succeed())
			Expect(validatingConfig.Webhooks[0].ClientConfig.CABundle).NotTo(BeEmpty())
		})

		It("should update existing webhook config via patch on subsequent reconcile", func() {
			// First reconcile: creates the webhook config
			_, err := r.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())

			// Get the existing webhook config with CA bundle
			mutatingConfig := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "gardener-extension-provider-test"}, mutatingConfig)).To(Succeed())
			firstCABundle := mutatingConfig.Webhooks[0].ClientConfig.CABundle
			Expect(firstCABundle).NotTo(BeEmpty())

			// Second reconcile: should patch the webhook config
			result, err := r.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))

			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "gardener-extension-provider-test"}, mutatingConfig)).To(Succeed())
			Expect(mutatingConfig.Webhooks[0].ClientConfig.CABundle).NotTo(BeEmpty())
		})

		Context("with shoot webhook configs", func() {
			var atomicShootWebhookConfigs atomic.Value

			BeforeEach(func() {
				r.ShootWebhookConfigs = &extensionswebhook.Configs{
					MutatingWebhookConfig: &admissionregistrationv1.MutatingWebhookConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name: "gardener-extension-provider-test-shoot",
						},
						Webhooks: []admissionregistrationv1.MutatingWebhook{{
							Name: "test-shoot-webhook",
							ClientConfig: admissionregistrationv1.WebhookClientConfig{
								CABundle: []byte("old-shoot-ca-bundle"),
							},
						}},
					},
				}
				r.AtomicShootWebhookConfigs = &atomicShootWebhookConfigs
				r.ShootWebhookManagedResourceName = "extension-provider-test-shoot-webhooks"
				r.ShootNamespaceSelector = map[string]string{"provider.extensions.gardener.cloud/type": "test"}
			})

			It("should update shoot webhook configs with the new CA bundle", func() {
				result, err := r.Reconcile(ctx, reconcile.Request{})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))

				// Verify the shoot webhook configs were injected with the new CA bundle
				Expect(r.ShootWebhookConfigs.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).NotTo(BeEmpty())
				Expect(r.ShootWebhookConfigs.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).NotTo(Equal([]byte("old-shoot-ca-bundle")))

				// Verify the atomic value was stored
				storedValue := atomicShootWebhookConfigs.Load()
				Expect(storedValue).NotTo(BeNil())
				storedConfigs, ok := storedValue.(*extensionswebhook.Configs)
				Expect(ok).To(BeTrue())
				Expect(storedConfigs.MutatingWebhookConfig).NotTo(BeNil())
				Expect(storedConfigs.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).NotTo(BeEmpty())
			})

			It("should store a deep copy of the shoot webhook configs", func() {
				_, err := r.Reconcile(ctx, reconcile.Request{})
				Expect(err).NotTo(HaveOccurred())

				storedValue := atomicShootWebhookConfigs.Load()
				storedConfigs := storedValue.(*extensionswebhook.Configs)

				// Verify it's a deep copy (modifying the stored one should not affect the original)
				storedConfigs.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle = []byte("modified")
				Expect(r.ShootWebhookConfigs.MutatingWebhookConfig.Webhooks[0].ClientConfig.CABundle).NotTo(Equal([]byte("modified")))
			})
		})

		Context("with no source webhook configs", func() {
			BeforeEach(func() {
				r.SourceWebhookConfigs = extensionswebhook.Configs{}
			})

			It("should still generate certificates and requeue", func() {
				result, err := r.Reconcile(ctx, reconcile.Request{})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))

				// Verify CA secret was created
				caSecrets := &corev1.SecretList{}
				Expect(fakeClient.List(ctx, caSecrets, client.InNamespace(namespace), client.MatchingLabels{
					secretsmanager.LabelKeyName:            caSecretName,
					secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
					secretsmanager.LabelKeyManagerIdentity: identity,
				})).To(Succeed())
				Expect(caSecrets.Items).NotTo(BeEmpty())
			})
		})
	})

	Describe("#reconcileSourceWebhookConfig", func() {
		var caBundleSecret *corev1.Secret

		BeforeEach(func() {
			caBundleSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ca-bundle-secret",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					secretsutils.DataKeyCertificateBundle: []byte("new-ca-bundle"),
				},
			}
		})

		Context("mutating webhook config", func() {
			var sourceWebhookConfig *admissionregistrationv1.MutatingWebhookConfiguration

			BeforeEach(func() {
				sourceWebhookConfig = &admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-mutating-config",
					},
					Webhooks: []admissionregistrationv1.MutatingWebhook{{
						Name: "webhook-1",
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							CABundle: []byte("placeholder"),
						},
					}},
				}
			})

			It("should create the webhook config if it does not exist", func() {
				Expect(r.reconcileSourceWebhookConfig(ctx, sourceWebhookConfig, caBundleSecret)).To(Succeed())

				config := &admissionregistrationv1.MutatingWebhookConfiguration{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "test-mutating-config"}, config)).To(Succeed())
				Expect(config.Webhooks).To(HaveLen(1))
				Expect(config.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte("new-ca-bundle")))
			})

			It("should patch the webhook config if it already exists", func() {
				existing := sourceWebhookConfig.DeepCopy()
				existing.Webhooks[0].ClientConfig.CABundle = []byte("old-ca-bundle")
				Expect(fakeClient.Create(ctx, existing)).To(Succeed())

				Expect(r.reconcileSourceWebhookConfig(ctx, sourceWebhookConfig, caBundleSecret)).To(Succeed())

				config := &admissionregistrationv1.MutatingWebhookConfiguration{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "test-mutating-config"}, config)).To(Succeed())
				Expect(config.Webhooks).To(HaveLen(1))
				Expect(config.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte("new-ca-bundle")))
			})

			It("should overwrite webhooks and inject CA bundle when patching", func() {
				// Create existing config with different webhooks
				existing := &admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-mutating-config",
					},
					Webhooks: []admissionregistrationv1.MutatingWebhook{{
						Name: "old-webhook",
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							CABundle: []byte("very-old-ca"),
						},
					}},
				}
				Expect(fakeClient.Create(ctx, existing)).To(Succeed())

				Expect(r.reconcileSourceWebhookConfig(ctx, sourceWebhookConfig, caBundleSecret)).To(Succeed())

				config := &admissionregistrationv1.MutatingWebhookConfiguration{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "test-mutating-config"}, config)).To(Succeed())
				Expect(config.Webhooks).To(HaveLen(1))
				Expect(config.Webhooks[0].Name).To(Equal("webhook-1"))
				Expect(config.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte("new-ca-bundle")))
			})
		})

		Context("validating webhook config", func() {
			var sourceWebhookConfig *admissionregistrationv1.ValidatingWebhookConfiguration

			BeforeEach(func() {
				sourceWebhookConfig = &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-validating-config",
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{{
						Name: "validating-webhook-1",
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							CABundle: []byte("placeholder"),
						},
					}},
				}
			})

			It("should create the webhook config if it does not exist", func() {
				Expect(r.reconcileSourceWebhookConfig(ctx, sourceWebhookConfig, caBundleSecret)).To(Succeed())

				config := &admissionregistrationv1.ValidatingWebhookConfiguration{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "test-validating-config"}, config)).To(Succeed())
				Expect(config.Webhooks).To(HaveLen(1))
				Expect(config.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte("new-ca-bundle")))
			})

			It("should patch the webhook config if it already exists", func() {
				existing := sourceWebhookConfig.DeepCopy()
				existing.Webhooks[0].ClientConfig.CABundle = []byte("old-ca-bundle")
				Expect(fakeClient.Create(ctx, existing)).To(Succeed())

				Expect(r.reconcileSourceWebhookConfig(ctx, sourceWebhookConfig, caBundleSecret)).To(Succeed())

				config := &admissionregistrationv1.ValidatingWebhookConfiguration{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: "test-validating-config"}, config)).To(Succeed())
				Expect(config.Webhooks).To(HaveLen(1))
				Expect(config.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte("new-ca-bundle")))
			})
		})
	})

	Describe("#isWebhookServerSecretPresent", func() {
		var scheme *runtime.Scheme

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
		})

		It("should return false if no matching secret exists", func() {
			present, err := isWebhookServerSecretPresent(ctx, fakeClient, scheme, serverSecName, namespace, identity)
			Expect(err).NotTo(HaveOccurred())
			Expect(present).To(BeFalse())
		})

		It("should return true if a matching secret exists", func() {
			secret := newServerSecret("some-secret", namespace, serverSecName, identity, []byte("cert"), []byte("key"))
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			present, err := isWebhookServerSecretPresent(ctx, fakeClient, scheme, serverSecName, namespace, identity)
			Expect(err).NotTo(HaveOccurred())
			Expect(present).To(BeTrue())
		})

		It("should return false if secret exists with different identity", func() {
			secret := newServerSecret("some-secret", namespace, serverSecName, "other-identity", []byte("cert"), []byte("key"))
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			present, err := isWebhookServerSecretPresent(ctx, fakeClient, scheme, serverSecName, namespace, identity)
			Expect(err).NotTo(HaveOccurred())
			Expect(present).To(BeFalse())
		})

		It("should return false if secret exists with different name", func() {
			secret := newServerSecret("some-secret", namespace, "other-name", identity, []byte("cert"), []byte("key"))
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			present, err := isWebhookServerSecretPresent(ctx, fakeClient, scheme, serverSecName, namespace, identity)
			Expect(err).NotTo(HaveOccurred())
			Expect(present).To(BeFalse())
		})
	})

	Describe("#generateWebhookCA", func() {
		It("should generate a CA secret", func() {
			sm, err := secretsmanager.New(
				ctx,
				logr.Discard(),
				fakeClock,
				fakeClient,
				identity,
				secretsmanager.WithCASecretAutoRotation(),
				secretsmanager.WithNamespaces(namespace),
			)
			Expect(err).NotTo(HaveOccurred())

			caSecret, err := r.generateWebhookCA(ctx, sm)
			Expect(err).NotTo(HaveOccurred())
			Expect(caSecret).NotTo(BeNil())
			Expect(caSecret.Data).To(HaveKey(secretsutils.DataKeyCertificateCA))
			Expect(caSecret.Data).To(HaveKey(secretsutils.DataKeyPrivateKeyCA))
		})
	})

	Describe("#generateWebhookServerCert", func() {
		It("should generate a server certificate signed by the CA", func() {
			sm, err := secretsmanager.New(
				ctx,
				logr.Discard(),
				fakeClock,
				fakeClient,
				identity,
				secretsmanager.WithCASecretAutoRotation(),
				secretsmanager.WithNamespaces(namespace),
			)
			Expect(err).NotTo(HaveOccurred())

			// Generate CA first
			_, err = r.generateWebhookCA(ctx, sm)
			Expect(err).NotTo(HaveOccurred())

			serverSecret, err := r.generateWebhookServerCert(ctx, sm)
			Expect(err).NotTo(HaveOccurred())
			Expect(serverSecret).NotTo(BeNil())
			Expect(serverSecret.Data).To(HaveKey(secretsutils.DataKeyCertificate))
			Expect(serverSecret.Data).To(HaveKey(secretsutils.DataKeyPrivateKey))
		})
	})

	Describe("#newSecretsManager", func() {
		It("should create a secrets manager successfully", func() {
			sm, err := r.newSecretsManager(ctx, logr.Discard(), fakeClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(sm).NotTo(BeNil())
		})
	})
})
