// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/extensions/pkg/webhook"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Registration", func() {
	Describe("#PrefixedName", func() {
		It("should return an empty string", func() {
			Expect(PrefixedName("gardener-foo")).To(Equal("gardener-foo"))
		})

		It("should return 'gardener-extension-'", func() {
			Expect(PrefixedName("provider-bar")).To(Equal("gardener-extension-provider-bar"))
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
			webhooks     = []*Webhook{
				{
					Name:     "webhook1",
					Provider: "provider1",
					Types:    []Type{{Obj: &corev1.ConfigMap{}}, {Obj: &corev1.Secret{}}},
					Target:   TargetSeed,
					Path:     "path1",
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
				{
					Name:     "webhook2",
					Provider: "provider2",
					Types:    []Type{{Obj: &corev1.Pod{}}},
					Target:   TargetSeed,
					Path:     "path2",
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"bar": "foo"}},
				},
				{
					Name:           "webhook3",
					Provider:       "provider3",
					Types:          []Type{{Obj: &corev1.ServiceAccount{}, Subresource: pointer.String("token")}},
					Target:         TargetShoot,
					Path:           "path3",
					Selector:       &metav1.LabelSelector{MatchLabels: map[string]string{"baz": "foo"}},
					ObjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "baz"}},
					TimeoutSeconds: pointer.Int32(1337),
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

			fakeClient client.Client
		)

		BeforeEach(func() {
			restMapper := meta.NewDefaultRESTMapper(nil)
			for _, kind := range []string{"ConfigMap", "Secret", "Pod", "Service", "ServiceAccount"} {
				restMapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: kind}, meta.RESTScopeNamespace)
			}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithRESTMapper(restMapper).Build()
		})

		DescribeTable("it should return the expected configs",
			func(mode, url string) {
				seedWebhookConfig, shootWebhookConfig, err := BuildWebhookConfigs(webhooks, fakeClient, namespace, providerName, servicePort, mode, url, nil)
				Expect(err).NotTo(HaveOccurred())

				var (
					buildSeedClientConfig = func(path string) admissionregistrationv1.WebhookClientConfig {
						out := admissionregistrationv1.WebhookClientConfig{}

						if mode == ModeService {
							out.Service = &admissionregistrationv1.ServiceReference{
								Name:      "gardener-extension-" + providerName,
								Namespace: namespace,
								Path:      pointer.String("/" + path),
							}
						}

						if mode == ModeURL {
							out.URL = pointer.String("https://" + url + "/" + path)
						}

						if mode == ModeURLWithServiceName {
							out.URL = pointer.String(fmt.Sprintf("https://gardener-extension-%s.%s:%d/%s", providerName, namespace, servicePort, path))
						}

						return out
					}

					buildShootClientConfig = func(path string) admissionregistrationv1.WebhookClientConfig {
						out := admissionregistrationv1.WebhookClientConfig{
							URL: pointer.String(fmt.Sprintf("https://gardener-extension-%s.%s:%d/%s", providerName, namespace, servicePort, path)),
						}

						if url != "" {
							out.URL = pointer.String("https://" + url + "/" + path)
						}

						return out
					}
				)

				Expect(seedWebhookConfig).To(Equal(&admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gardener-extension-" + providerName,
						Labels: map[string]string{"remediation.webhook.shoot.gardener.cloud/exclude": "true"},
					},
					Webhooks: []admissionregistrationv1.MutatingWebhook{
						{
							Name:         webhooks[0].Name + ".foo.extensions.gardener.cloud",
							ClientConfig: buildSeedClientConfig(webhooks[0].Path),
							Rules: []admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"secrets"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							AdmissionReviewVersions: []string{"v1", "v1beta1"},
							NamespaceSelector:       webhooks[0].Selector,
							FailurePolicy:           &failurePolicyFail,
							MatchPolicy:             &matchPolicyExact,
							SideEffects:             &sideEffectsNone,
							TimeoutSeconds:          &defaultTimeoutSeconds,
						},
						{
							Name:         webhooks[1].Name + ".foo.extensions.gardener.cloud",
							ClientConfig: buildSeedClientConfig(webhooks[1].Path),
							Rules: []admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							AdmissionReviewVersions: []string{"v1", "v1beta1"},
							NamespaceSelector:       webhooks[1].Selector,
							FailurePolicy:           &failurePolicyFail,
							MatchPolicy:             &matchPolicyExact,
							SideEffects:             &sideEffectsNone,
							TimeoutSeconds:          &defaultTimeoutSeconds,
						},
					},
				}))

				Expect(shootWebhookConfig).To(Equal(&admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gardener-extension-" + providerName + "-shoot",
						Labels: map[string]string{"remediation.webhook.shoot.gardener.cloud/exclude": "true"},
					},
					Webhooks: []admissionregistrationv1.MutatingWebhook{
						{
							Name:         webhooks[2].Name + ".foo.extensions.gardener.cloud",
							ClientConfig: buildShootClientConfig(webhooks[2].Path),
							Rules: []admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"serviceaccounts/token"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							AdmissionReviewVersions: []string{"v1", "v1beta1"},
							NamespaceSelector:       webhooks[2].Selector,
							ObjectSelector:          webhooks[2].ObjectSelector,
							FailurePolicy:           &failurePolicyIgnore,
							MatchPolicy:             &matchPolicyExact,
							SideEffects:             &sideEffectsNone,
							TimeoutSeconds:          webhooks[2].TimeoutSeconds,
						},
						{
							Name:         webhooks[3].Name + ".foo.extensions.gardener.cloud",
							ClientConfig: buildShootClientConfig(webhooks[3].Path),
							Rules: []admissionregistrationv1.RuleWithOperations{
								{
									Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}},
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
								},
							},
							AdmissionReviewVersions: []string{"v1", "v1beta1"},
							NamespaceSelector:       webhooks[3].Selector,
							FailurePolicy:           webhooks[3].FailurePolicy,
							MatchPolicy:             &matchPolicyExact,
							SideEffects:             &sideEffectsNone,
							TimeoutSeconds:          &defaultTimeoutSeconds,
						},
					},
				}))
			},

			Entry("service mode", ModeService, ""),
			Entry("url with service name mode", ModeURLWithServiceName, ""),
			Entry("url mode", ModeURL, "my-custom-url:4337"),
		)
	})

	Describe("#ReconcileSeedWebhookConfig", func() {
		var (
			ctx        = context.TODO()
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
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(false),
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
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(false),
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
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(false),
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
