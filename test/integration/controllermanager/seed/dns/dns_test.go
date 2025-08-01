// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dns_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Seed DNS controller integration tests", func() {
	var (
		seed   *gardencorev1beta1.Seed
		secret *corev1.Secret
	)

	BeforeEach(func() {
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Labels: map[string]string{
					testID: testRunID,
					"provider.extensions.gardener.cloud/type": "local",
				},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   "providerType",
				},
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "someingress.example.com",
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{
						Type: "providerType",
						SecretRef: corev1.SecretReference{
							Name:      "some-secret",
							Namespace: "some-namespace",
						},
					},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    ptr.To("10.2.0.0/16"),
					ShootDefaults: &gardencorev1beta1.ShootNetworks{
						Pods:     ptr.To("100.128.0.0/11"),
						Services: ptr.To("100.72.0.0/13"),
					},
				},
			},
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "internal-domain",
				Namespace: testNamespace.Name,
				Annotations: map[string]string{
					"dns.gardener.cloud/provider": "providerType",
					"dns.gardener.cloud/domain":   "internal.example.com",
					"dns.gardener.cloud/zone":     "zone-1",
				},
				Labels: map[string]string{
					"gardener.cloud/role": "internal-domain",
				},
			},
			Data: map[string][]byte{},
		}

		Expect(testClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, secret)).To(Or(Succeed(), BeNotFoundError()))
		})
	})

	It("should default internal DNS from internal domain secret if not set", func() {
		By("Create Seed")
		Expect(testClient.Create(ctx, seed)).To(Succeed())

		By("Expect internal DNS to be defaulted")
		Eventually(func(g Gomega) {
			updatedSeed := &gardencorev1beta1.Seed{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
			expected := &gardencorev1beta1.SeedDNSProviderConf{
				Type:   "providerType",
				Domain: "internal.example.com",
				Zone:   ptr.To("zone-1"),
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  testNamespace.Name,
					Name:       "internal-domain",
				},
			}
			g.Expect(updatedSeed.Spec.DNS.Internal).To(Equal(expected))
		}).Should(Succeed())
	})

	It("should not default internal DNS if already set in the seed", func() {
		seed.Spec.DNS.Internal = &gardencorev1beta1.SeedDNSProviderConf{
			Type:   "custom-provider",
			Domain: "custom.example.com",
			Zone:   ptr.To("custom-zone"),
			CredentialsRef: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  testNamespace.Name,
				Name:       "custom-secret",
			},
		}

		By("Create Seed with internal DNS set")
		Expect(testClient.Create(ctx, seed)).To(Succeed())

		By("Expect internal DNS to remain unchanged")
		Consistently(func(g Gomega) {
			updatedSeed := &gardencorev1beta1.Seed{}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
			expected := &gardencorev1beta1.SeedDNSProviderConf{
				Type:   "custom-provider",
				Domain: "custom.example.com",
				Zone:   ptr.To("custom-zone"),
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  testNamespace.Name,
					Name:       "custom-secret",
				},
			}
			g.Expect(updatedSeed.Spec.DNS.Internal).To(Equal(expected))
		}).Should(Succeed())
	})
})
