// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/extensions/pkg/webhook"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Registration", func() {
	Describe("#PrefixedName", func() {
		It("should not prefix because name starts with 'gardener'", func() {
			Expect(PrefixedName("gardener-foo", false)).To(Equal("gardener-foo"))
		})

		It("should prefix with 'gardener-extension-'", func() {
			Expect(PrefixedName("provider-bar", false)).To(Equal("gardener-extension-provider-bar"))
		})

		It("should not prefix with 'gardener-extension-' because explicitly skipped", func() {
			Expect(PrefixedName("provider-bar", true)).To(Equal("provider-bar"))
		})
	})

	Describe("Configs", func() {
		var configs Configs

		BeforeEach(func() {
			configs = Configs{}
		})

		Describe("#GetWebhookConfigs", func() {
			It("should return no config", func() {
				Expect(configs.GetWebhookConfigs()).To(BeEmpty())
			})

			It("should return all webhook configs", func() {
				configs.MutatingWebhookConfig = &admissionregistrationv1.MutatingWebhookConfiguration{}
				Expect(configs.GetWebhookConfigs()).To(ConsistOf(configs.MutatingWebhookConfig))
			})
		})

		Describe("#DeepCopy", func() {
			It("should succeed with given webhook configs", func() {
				configs.MutatingWebhookConfig = &admissionregistrationv1.MutatingWebhookConfiguration{}
				configs.ValidatingWebhookConfig = &admissionregistrationv1.ValidatingWebhookConfiguration{}

				copy := configs.DeepCopy()
				Expect(copy.MutatingWebhookConfig).To(Not(ShareSameReferenceAs(configs.MutatingWebhookConfig)))
				Expect(copy.ValidatingWebhookConfig).To(Not(ShareSameReferenceAs(configs.ValidatingWebhookConfig)))
			})
		})

		Describe("#HasWebhookConfigs", func() {
			It("should return 'true' if at least one webhook config is given", func() {
				configs.ValidatingWebhookConfig = &admissionregistrationv1.ValidatingWebhookConfiguration{}
				Expect(configs.HasWebhookConfig()).To(BeTrue())
			})

			It("should return 'false' if no webhook config is given", func() {
				Expect(configs.HasWebhookConfig()).To(BeFalse())
			})
		})
	})

	Describe("#BuildWebhookConfigs", func() {
		var (
			failurePolicyIgnore         = admissionregistrationv1.Ignore
			failurePolicyFail           = admissionregistrationv1.Fail
			matchPolicyExact            = admissionregistrationv1.Exact
			sideEffectsNone             = admissionregistrationv1.SideEffectClassNone
			defaultTimeoutSeconds int32 = 10

			providerName = "provider-foo"
			namespace    = "extension-" + providerName
			servicePort  = 12345

			mutatingWebhooks = []*Webhook{
				{
					Action:            "mutating",
					Name:              "webhook1",
					Provider:          "provider1",
					Types:             []Type{{Obj: &corev1.ConfigMap{}}, {Obj: &corev1.Secret{}}},
					Target:            TargetSeed,
					Path:              "path1",
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
				{
					Name:              "webhook2",
					Provider:          "provider2",
					Types:             []Type{{Obj: &corev1.Pod{}}},
					Target:            TargetSeed,
					Path:              "path2",
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"bar": "foo"}},
				},
				{
					Action:            "mutating",
					Name:              "webhook3",
					Provider:          "provider3",
					Types:             []Type{{Obj: &corev1.ServiceAccount{}, Subresource: ptr.To("token")}},
					Target:            TargetShoot,
					Path:              "path3",
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"baz": "foo"}},
					ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "baz"}},
					TimeoutSeconds:    ptr.To[int32](1337),
				},
				{
					Name:          "webhook4",
					Provider:      "provider4",
					Types:         []Type{{Obj: &corev1.Service{}}},
					Target:        TargetShoot,
					Path:          "path4",
					FailurePolicy: &failurePolicyFail,
				},
			}

			validatingWebhooks = []*Webhook{
				{
					Action:            "validating",
					Name:              "webhook1",
					Provider:          "provider1",
					Types:             []Type{{Obj: &corev1.ConfigMap{}}, {Obj: &corev1.Secret{}}},
					Target:            TargetSeed,
					Path:              "path1",
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
				{
					Action:            "validating",
					Name:              "webhook2",
					Provider:          "provider2",
					Types:             []Type{{Obj: &corev1.Pod{}}},
					Target:            TargetSeed,
					Path:              "path2",
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"bar": "foo"}},
				},
				{
					Action:            "validating",
					Name:              "webhook3",
					Provider:          "provider3",
					Types:             []Type{{Obj: &corev1.ServiceAccount{}, Subresource: ptr.To("token")}},
					Target:            TargetShoot,
					Path:              "path3",
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"baz": "foo"}},
					ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "baz"}},
					TimeoutSeconds:    ptr.To[int32](1337),
				},
				{
					Action:        "validating",
					Name:          "webhook4",
					Provider:      "provider4",
					Types:         []Type{{Obj: &corev1.Service{}}},
					Target:        TargetShoot,
					Path:          "path4",
					FailurePolicy: &failurePolicyFail,
				},
			}

			webhooks = append(mutatingWebhooks, validatingWebhooks...)

			fakeClient client.Client
		)

		BeforeEach(func() {
			restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "", Version: "v1"}})
			for _, kind := range []string{"ConfigMap", "Secret", "Pod", "Service", "ServiceAccount"} {
				restMapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: kind}, meta.RESTScopeNamespace)
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithRESTMapper(restMapper).Build()
		})

		DescribeTable("it should return the expected configs",
			func(mode, url string, mergeShootWithSeedWebhooks bool) {
				seedWebhookConfig, shootWebhookConfig, err := BuildWebhookConfigs(webhooks, fakeClient, namespace, providerName, false, servicePort, mode, url, nil, mergeShootWithSeedWebhooks)
				Expect(err).NotTo(HaveOccurred())

				var (
					buildSeedClientConfig = func(path string) admissionregistrationv1.WebhookClientConfig {
						out := admissionregistrationv1.WebhookClientConfig{}

						if mode == ModeService {
							out.Service = &admissionregistrationv1.ServiceReference{
								Name:      "gardener-extension-" + providerName,
								Namespace: namespace,
								Path:      ptr.To("/" + path),
							}
						}

						if mode == ModeURL {
							out.URL = ptr.To("https://" + url + "/" + path)
						}

						if mode == ModeURLWithServiceName {
							out.URL = ptr.To(fmt.Sprintf("https://gardener-extension-%s.%s:%d/%s", providerName, namespace, servicePort, path))
						}

						return out
					}

					buildShootClientConfig = func(path string) admissionregistrationv1.WebhookClientConfig {
						out := admissionregistrationv1.WebhookClientConfig{
							URL: ptr.To(fmt.Sprintf("https://gardener-extension-%s.%s:%d/%s", providerName, namespace, servicePort, path)),
						}

						if url != "" {
							out.URL = ptr.To("https://" + url + "/" + path)
						}

						return out
					}

					buildMutatingWebhook = func(
						webhook *Webhook,
						clientConfigFn func(string) admissionregistrationv1.WebhookClientConfig,
						rules []admissionregistrationv1.RuleWithOperations,
						failurePolicy admissionregistrationv1.FailurePolicyType,
						matchPolicy admissionregistrationv1.MatchPolicyType,
						sideEffects admissionregistrationv1.SideEffectClass,
						timeoutSeconds int32,
					) admissionregistrationv1.MutatingWebhook {
						return admissionregistrationv1.MutatingWebhook{
							Name:                    webhook.Name + ".foo.extensions.gardener.cloud",
							ClientConfig:            clientConfigFn(webhook.Path),
							Rules:                   rules,
							AdmissionReviewVersions: []string{"v1", "v1beta1"},
							NamespaceSelector:       webhook.NamespaceSelector,
							ObjectSelector:          webhook.ObjectSelector,
							FailurePolicy:           &failurePolicy,
							MatchPolicy:             &matchPolicy,
							SideEffects:             &sideEffects,
							TimeoutSeconds:          &timeoutSeconds,
						}
					}

					buildValidatingWebhook = func(
						webhook *Webhook,
						clientConfigFn func(string) admissionregistrationv1.WebhookClientConfig,
						rules []admissionregistrationv1.RuleWithOperations,
						failurePolicy admissionregistrationv1.FailurePolicyType,
						matchPolicy admissionregistrationv1.MatchPolicyType,
						sideEffects admissionregistrationv1.SideEffectClass,
						timeoutSeconds int32,
					) admissionregistrationv1.ValidatingWebhook {
						return admissionregistrationv1.ValidatingWebhook{
							Name:                    webhook.Name + ".foo.extensions.gardener.cloud",
							ClientConfig:            clientConfigFn(webhook.Path),
							Rules:                   rules,
							AdmissionReviewVersions: []string{"v1", "v1beta1"},
							NamespaceSelector:       webhook.NamespaceSelector,
							ObjectSelector:          webhook.ObjectSelector,
							FailurePolicy:           &failurePolicy,
							MatchPolicy:             &matchPolicy,
							SideEffects:             &sideEffects,
							TimeoutSeconds:          &timeoutSeconds,
						}
					}
				)

				// Check seed mutating webhooks
				expectedSeedMutatingWebhookConfig := &admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gardener-extension-" + providerName,
						Labels: map[string]string{"remediation.webhook.shoot.gardener.cloud/exclude": "true"},
					},
					Webhooks: []admissionregistrationv1.MutatingWebhook{
						buildMutatingWebhook(
							mutatingWebhooks[0],
							buildSeedClientConfig,
							[]admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"secrets"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							failurePolicyFail,
							matchPolicyExact,
							sideEffectsNone,
							defaultTimeoutSeconds,
						),
						buildMutatingWebhook(
							mutatingWebhooks[1],
							buildSeedClientConfig,
							[]admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							failurePolicyFail,
							matchPolicyExact,
							sideEffectsNone,
							defaultTimeoutSeconds,
						),
					},
				}

				if mergeShootWithSeedWebhooks {
					expectedSeedMutatingWebhookConfig.Webhooks = append(expectedSeedMutatingWebhookConfig.Webhooks,
						buildMutatingWebhook(
							mutatingWebhooks[2],
							buildSeedClientConfig,
							[]admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"serviceaccounts/token"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							failurePolicyIgnore,
							matchPolicyExact,
							sideEffectsNone,
							*mutatingWebhooks[2].TimeoutSeconds,
						),
						buildMutatingWebhook(
							mutatingWebhooks[3],
							buildSeedClientConfig,
							[]admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							*webhooks[3].FailurePolicy,
							matchPolicyExact,
							sideEffectsNone,
							defaultTimeoutSeconds,
						),
					)
				}
				Expect(seedWebhookConfig.MutatingWebhookConfig).To(Equal(expectedSeedMutatingWebhookConfig))

				// Check shoot mutating webhooks
				if !mergeShootWithSeedWebhooks {
					Expect(shootWebhookConfig.MutatingWebhookConfig).To(Equal(&admissionregistrationv1.MutatingWebhookConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "gardener-extension-" + providerName + "-shoot",
							Labels: map[string]string{"remediation.webhook.shoot.gardener.cloud/exclude": "true"},
						},
						Webhooks: []admissionregistrationv1.MutatingWebhook{
							buildMutatingWebhook(
								mutatingWebhooks[2],
								buildShootClientConfig,
								[]admissionregistrationv1.RuleWithOperations{
									{
										Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"serviceaccounts/token"}},
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
									},
								},
								failurePolicyIgnore,
								matchPolicyExact,
								sideEffectsNone,
								*mutatingWebhooks[2].TimeoutSeconds,
							),
							buildMutatingWebhook(
								mutatingWebhooks[3],
								buildShootClientConfig,
								[]admissionregistrationv1.RuleWithOperations{
									{
										Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}},
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
									},
								},
								*mutatingWebhooks[3].FailurePolicy,
								matchPolicyExact,
								sideEffectsNone,
								defaultTimeoutSeconds,
							),
						},
					}))
				} else {
					Expect(shootWebhookConfig.MutatingWebhookConfig).To(BeNil())
				}

				// Check seed validating webhooks
				expectedSeedValidatingWebhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gardener-extension-" + providerName,
						Labels: map[string]string{"remediation.webhook.shoot.gardener.cloud/exclude": "true"},
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{
						buildValidatingWebhook(
							validatingWebhooks[0],
							buildSeedClientConfig,
							[]admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"secrets"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							failurePolicyFail,
							matchPolicyExact,
							sideEffectsNone,
							defaultTimeoutSeconds,
						),
						buildValidatingWebhook(
							validatingWebhooks[1],
							buildSeedClientConfig,
							[]admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							failurePolicyFail,
							matchPolicyExact,
							sideEffectsNone,
							defaultTimeoutSeconds,
						),
					},
				}

				if mergeShootWithSeedWebhooks {
					expectedSeedValidatingWebhookConfig.Webhooks = append(expectedSeedValidatingWebhookConfig.Webhooks,
						buildValidatingWebhook(
							validatingWebhooks[2],
							buildSeedClientConfig,
							[]admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"serviceaccounts/token"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							failurePolicyIgnore,
							matchPolicyExact,
							sideEffectsNone,
							*validatingWebhooks[2].TimeoutSeconds,
						),
						buildValidatingWebhook(
							validatingWebhooks[3],
							buildSeedClientConfig,
							[]admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							*validatingWebhooks[3].FailurePolicy,
							matchPolicyExact,
							sideEffectsNone,
							defaultTimeoutSeconds,
						),
					)

					Expect(shootWebhookConfig.ValidatingWebhookConfig).To(BeNil())
				}
				Expect(seedWebhookConfig.ValidatingWebhookConfig).To(Equal(expectedSeedValidatingWebhookConfig))

				// Check shoot validating webhooks
				if !mergeShootWithSeedWebhooks {
					Expect(shootWebhookConfig.ValidatingWebhookConfig).To(Equal(&admissionregistrationv1.ValidatingWebhookConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "gardener-extension-" + providerName + "-shoot",
							Labels: map[string]string{"remediation.webhook.shoot.gardener.cloud/exclude": "true"},
						},
						Webhooks: []admissionregistrationv1.ValidatingWebhook{
							buildValidatingWebhook(
								validatingWebhooks[2],
								buildShootClientConfig,
								[]admissionregistrationv1.RuleWithOperations{
									{
										Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"serviceaccounts/token"}},
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
									},
								},
								failurePolicyIgnore,
								matchPolicyExact,
								sideEffectsNone,
								*validatingWebhooks[2].TimeoutSeconds,
							),
							buildValidatingWebhook(
								validatingWebhooks[3],
								buildShootClientConfig,
								[]admissionregistrationv1.RuleWithOperations{
									{
										Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}},
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
									},
								},
								*validatingWebhooks[3].FailurePolicy,
								matchPolicyExact,
								sideEffectsNone,
								defaultTimeoutSeconds,
							),
						},
					}))
				} else {
					Expect(shootWebhookConfig.ValidatingWebhookConfig).To(BeNil())
				}
			},

			Entry("service mode", ModeService, "", false),
			Entry("url with service name mode", ModeURLWithServiceName, "", false),
			Entry("url mode", ModeURL, "my-custom-url:4337", false),
			Entry("merge shoot into seed webooks", ModeService, "", true),
		)
	})

	Describe("#ReconcileSeedWebhookConfig", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client

			ownerNamespaceName = "extension-provider-foo"
			caBundle           = []byte("ca-bundle")

			ownerNamespace *corev1.Namespace
			webhookConfig  *admissionregistrationv1.MutatingWebhookConfiguration
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

			ownerNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ownerNamespaceName}}
			webhookConfig = &admissionregistrationv1.MutatingWebhookConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "admissionregistration.k8s.io/v1",
					Kind:       "MutatingWebhookConfiguration",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener-extension-provider-foo",
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{{}},
			}
		})

		It("should create the webhook config w/o owner namespace", func() {
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(webhookConfig), &admissionregistrationv1.MutatingWebhookConfiguration{})).To(BeNotFoundError())

			Expect(ReconcileSeedWebhookConfig(ctx, fakeClient, webhookConfig, "", caBundle)).To(Succeed())

			obj := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(webhookConfig), obj)).To(Succeed())
			Expect(obj).To(Equal(webhookConfig))

			Expect(webhookConfig.OwnerReferences).To(BeEmpty())
			Expect(webhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))
		})

		It("should create the webhook config w/ owner namespace", func() {
			Expect(fakeClient.Create(ctx, ownerNamespace)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(webhookConfig), &admissionregistrationv1.MutatingWebhookConfiguration{})).To(BeNotFoundError())

			Expect(ReconcileSeedWebhookConfig(ctx, fakeClient, webhookConfig, ownerNamespaceName, caBundle)).To(Succeed())

			obj := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(webhookConfig), obj)).To(Succeed())
			Expect(obj).To(Equal(webhookConfig))

			Expect(webhookConfig.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				APIVersion:         "v1",
				Kind:               "Namespace",
				Name:               ownerNamespaceName,
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(false),
			}))
			Expect(webhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))
		})

		It("should update the webhook config w/o owner namespace, w/o existing CA bundle", func() {
			Expect(fakeClient.Create(ctx, webhookConfig)).To(Succeed())

			Expect(ReconcileSeedWebhookConfig(ctx, fakeClient, webhookConfig, "", caBundle)).To(Succeed())

			obj := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(webhookConfig), obj)).To(Succeed())
			Expect(obj).To(Equal(webhookConfig))

			Expect(webhookConfig.OwnerReferences).To(BeEmpty())
			Expect(webhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))
		})

		It("should update the webhook config w/ owner namespace, w/o existing CA bundle", func() {
			Expect(fakeClient.Create(ctx, ownerNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, webhookConfig)).To(Succeed())

			Expect(ReconcileSeedWebhookConfig(ctx, fakeClient, webhookConfig, ownerNamespaceName, caBundle)).To(Succeed())

			obj := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(webhookConfig), obj)).To(Succeed())
			Expect(obj).To(Equal(webhookConfig))

			Expect(webhookConfig.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				APIVersion:         "v1",
				Kind:               "Namespace",
				Name:               ownerNamespaceName,
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(false),
			}))
			Expect(webhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))
		})

		It("should update the webhook config w/ owner namespace, w/ existing CA bundle", func() {
			webhookConfig.Webhooks[0].ClientConfig.CABundle = []byte("some-existing-ca-bundle-to-be-overwritten")

			Expect(fakeClient.Create(ctx, ownerNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, webhookConfig)).To(Succeed())

			Expect(ReconcileSeedWebhookConfig(ctx, fakeClient, webhookConfig, ownerNamespaceName, caBundle)).To(Succeed())

			obj := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(webhookConfig), obj)).To(Succeed())
			Expect(obj).To(Equal(webhookConfig))

			Expect(webhookConfig.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				APIVersion:         "v1",
				Kind:               "Namespace",
				Name:               ownerNamespaceName,
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(false),
			}))
			Expect(webhookConfig.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))
		})
	})

	Describe("#OverwriteWebhooks", func() {
		It("should work for admissionregistrationv1.MutatingWebhookConfiguration", func() {
			current := &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{Name: "wh1"}}}
			desired := &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{Name: "wh2"}}}

			Expect(OverwriteWebhooks(current, desired)).To(Succeed())
			Expect(current.Webhooks).To(Equal(desired.Webhooks))
		})

		It("should work for admissionregistrationv1beta1.MutatingWebhookConfiguration", func() {
			current := &admissionregistrationv1beta1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1beta1.MutatingWebhook{{Name: "wh1"}}}
			desired := &admissionregistrationv1beta1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1beta1.MutatingWebhook{{Name: "wh2"}}}

			Expect(OverwriteWebhooks(current, desired)).To(Succeed())
			Expect(current.Webhooks).To(Equal(desired.Webhooks))
		})

		It("should work for admissionregistrationv1.ValidatingWebhookConfiguration", func() {
			current := &admissionregistrationv1.ValidatingWebhookConfiguration{Webhooks: []admissionregistrationv1.ValidatingWebhook{{Name: "wh1"}}}
			desired := &admissionregistrationv1.ValidatingWebhookConfiguration{Webhooks: []admissionregistrationv1.ValidatingWebhook{{Name: "wh2"}}}

			Expect(OverwriteWebhooks(current, desired)).To(Succeed())
			Expect(current.Webhooks).To(Equal(desired.Webhooks))
		})

		It("should work for admissionregistrationv1beta1.ValidatingWebhookConfiguration", func() {
			current := &admissionregistrationv1beta1.ValidatingWebhookConfiguration{Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{{Name: "wh1"}}}
			desired := &admissionregistrationv1beta1.ValidatingWebhookConfiguration{Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{{Name: "wh2"}}}

			Expect(OverwriteWebhooks(current, desired)).To(Succeed())
			Expect(current.Webhooks).To(Equal(desired.Webhooks))
		})

		It("should return an error since current's type is not handled", func() {
			Expect(OverwriteWebhooks(&corev1.Pod{}, nil)).To(MatchError(ContainSubstring("unexpected webhook config type")))
		})
	})

	Describe("#GetCABundleFromWebhookConfig", func() {
		caBundle := []byte("ca-bundle")

		It("admissionregistrationv1.MutatingWebhookConfiguration", func() {
			By("Return the CA bundle for the first webhook")
			result, err := GetCABundleFromWebhookConfig(&admissionregistrationv1.MutatingWebhookConfiguration{
				Webhooks: []admissionregistrationv1.MutatingWebhook{
					{ClientConfig: admissionregistrationv1.WebhookClientConfig{CABundle: caBundle}},
					{ClientConfig: admissionregistrationv1.WebhookClientConfig{CABundle: []byte("something-else")}},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(caBundle))

			By("Return nil since there is no CA bundle")
			result, err = GetCABundleFromWebhookConfig(&admissionregistrationv1.MutatingWebhookConfiguration{
				Webhooks: []admissionregistrationv1.MutatingWebhook{{}},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("admissionregistrationv1beta1.MutatingWebhookConfiguration", func() {
			By("Return the CA bundle for the first webhook")
			result, err := GetCABundleFromWebhookConfig(&admissionregistrationv1beta1.MutatingWebhookConfiguration{
				Webhooks: []admissionregistrationv1beta1.MutatingWebhook{
					{ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{CABundle: caBundle}},
					{ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{CABundle: []byte("something-else")}},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(caBundle))

			By("Return nil since there is no CA bundle")
			result, err = GetCABundleFromWebhookConfig(&admissionregistrationv1beta1.MutatingWebhookConfiguration{
				Webhooks: []admissionregistrationv1beta1.MutatingWebhook{{}},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("admissionregistrationv1.ValidatingWebhookConfiguration", func() {
			By("Return the CA bundle for the first webhook")
			result, err := GetCABundleFromWebhookConfig(&admissionregistrationv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregistrationv1.ValidatingWebhook{
					{ClientConfig: admissionregistrationv1.WebhookClientConfig{CABundle: caBundle}},
					{ClientConfig: admissionregistrationv1.WebhookClientConfig{CABundle: []byte("something-else")}},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(caBundle))

			By("Return nil since there is no CA bundle")
			result, err = GetCABundleFromWebhookConfig(&admissionregistrationv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregistrationv1.ValidatingWebhook{{}},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("admissionregistrationv1beta1.ValidatingWebhookConfiguration", func() {
			By("Return the CA bundle for the first webhook")
			result, err := GetCABundleFromWebhookConfig(&admissionregistrationv1beta1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{
					{ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{CABundle: caBundle}},
					{ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{CABundle: []byte("something-else")}},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(caBundle))

			By("Return nil since there is no CA bundle")
			result, err = GetCABundleFromWebhookConfig(&admissionregistrationv1beta1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{{}},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return an error since current's type is not handled", func() {
			result, err := GetCABundleFromWebhookConfig(&corev1.Pod{})
			Expect(err).To(MatchError(ContainSubstring("unexpected webhook config type")))
			Expect(result).To(BeNil())
		})
	})

	Describe("#InjectCABundleIntoWebhookConfig", func() {
		caBundle := []byte("ca-bundle")

		It("admissionregistrationv1.MutatingWebhookConfiguration", func() {
			obj := &admissionregistrationv1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1.MutatingWebhook{{}, {}}}
			Expect(InjectCABundleIntoWebhookConfig(obj, caBundle)).To(Succeed())
			Expect(obj.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))
			Expect(obj.Webhooks[1].ClientConfig.CABundle).To(Equal(caBundle))
		})

		It("admissionregistrationv1beta1.MutatingWebhookConfiguration", func() {
			obj := &admissionregistrationv1beta1.MutatingWebhookConfiguration{Webhooks: []admissionregistrationv1beta1.MutatingWebhook{{}, {}}}
			Expect(InjectCABundleIntoWebhookConfig(obj, caBundle)).To(Succeed())
			Expect(obj.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))
			Expect(obj.Webhooks[1].ClientConfig.CABundle).To(Equal(caBundle))
		})

		It("admissionregistrationv1.ValidatingWebhookConfiguration", func() {
			obj := &admissionregistrationv1.ValidatingWebhookConfiguration{Webhooks: []admissionregistrationv1.ValidatingWebhook{{}, {}}}
			Expect(InjectCABundleIntoWebhookConfig(obj, caBundle)).To(Succeed())
			Expect(obj.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))
			Expect(obj.Webhooks[1].ClientConfig.CABundle).To(Equal(caBundle))
		})

		It("admissionregistrationv1beta1.ValidatingWebhookConfiguration", func() {
			obj := &admissionregistrationv1beta1.ValidatingWebhookConfiguration{Webhooks: []admissionregistrationv1beta1.ValidatingWebhook{{}, {}}}
			Expect(InjectCABundleIntoWebhookConfig(obj, caBundle)).To(Succeed())
			Expect(obj.Webhooks[0].ClientConfig.CABundle).To(Equal(caBundle))
			Expect(obj.Webhooks[1].ClientConfig.CABundle).To(Equal(caBundle))
		})

		It("should return an error since current's type is not handled", func() {
			Expect(InjectCABundleIntoWebhookConfig(&corev1.Pod{}, caBundle)).To(MatchError(ContainSubstring("unexpected webhook config type")))
		})
	})
})
