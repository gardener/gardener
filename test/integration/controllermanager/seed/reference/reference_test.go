// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Seed Reference controller tests", func() {
	var (
		secret1    *corev1.Secret
		configMap1 *corev1.ConfigMap

		allReferencedObjects []client.Object
		seed                 *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		secret1 = initializeObject("secret").(*corev1.Secret)
		configMap1 = initializeObject("configMap").(*corev1.ConfigMap)

		allReferencedObjects = []client.Object{secret1, configMap1}

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Labels: map[string]string{
					testID:                               testRunID,
					"provider.extensions.gardener.cloud": "local",
				},
			},
			Spec: gardencorev1beta1.SeedSpec{
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{
						Type: "local",
						SecretRef: corev1.SecretReference{
							Name:      "dns-secret",
							Namespace: testNamespace.Name,
						},
					},
				},
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "example.com",
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
				},
				Provider: gardencorev1beta1.SeedProvider{
					Type:   "local",
					Region: "region",
				},
				Resources: []gardencorev1beta1.NamedResourceReference{
					{
						Name: "foo",
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       secret1.Name,
						},
					},
					{
						Name: "bar",
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       configMap1.Name,
						},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create Seed")
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created Seed for test", "seed", client.ObjectKeyFromObject(seed))

		DeferCleanup(func() {
			By("Delete Seed")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())
		})
	})

	Context("no references", func() {
		BeforeEach(func() {
			seed.Spec.Resources = nil
		})

		It("should not add the finalizer to the seed", func() {
			Consistently(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				return seed.Finalizers
			}).ShouldNot(ContainElement("gardener.cloud/reference-protection"))
		})
	})

	Context("w/ references", func() {
		JustBeforeEach(func() {
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				return seed.Finalizers
			}).Should(ContainElement("gardener.cloud/reference-protection"))
		})

		It("should add finalizers to the referenced secrets and configmaps", func() {
			for _, obj := range allReferencedObjects {
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					return obj.GetFinalizers()
				}).Should(ContainElement("gardener.cloud/reference-protection"), obj.GetName()+" should have the finalizer")
			}
		})

		It("should remove finalizers from the seed and the referenced secrets and configmaps", func() {
			patch := client.MergeFrom(seed.DeepCopy())
			seed.Spec.Resources = nil
			Expect(testClient.Patch(ctx, seed, patch)).To(Succeed())

			for _, obj := range append([]client.Object{seed}, allReferencedObjects...) {
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					return obj.GetFinalizers()
				}).ShouldNot(ContainElement("gardener.cloud/reference-protection"), obj.GetName()+" should not have the finalizer")
			}
		})

		Context("multiple seeds", func() {
			var seed2 *gardencorev1beta1.Seed

			BeforeEach(func() {
				seed2 = seed.DeepCopy()
			})

			JustBeforeEach(func() {
				By("Create second Seed")
				Expect(testClient.Create(ctx, seed2)).To(Succeed())
				log.Info("Created second Seed for test", "seed", client.ObjectKeyFromObject(seed2))

				DeferCleanup(func() {
					By("Delete second Seed")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed2))).To(Succeed())
				})

				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed2), seed2)).To(Succeed())
					return seed2.Finalizers
				}).Should(ContainElement("gardener.cloud/reference-protection"))
			})

			It("should not remove finalizers from the referenced secrets and configmaps because another seed still references them", func() {
				patch := client.MergeFrom(seed.DeepCopy())
				seed.Spec.Resources = nil
				Expect(testClient.Patch(ctx, seed, patch)).To(Succeed())

				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					return seed.GetFinalizers()
				}).ShouldNot(ContainElement("gardener.cloud/reference-protection"), seed.GetName()+" should not have the finalizer")

				for _, obj := range append([]client.Object{seed2}, allReferencedObjects...) {
					Consistently(func(g Gomega) []string {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
						return obj.GetFinalizers()
					}).Should(ContainElement("gardener.cloud/reference-protection"), obj.GetName()+" should have the finalizer")
				}
			})
		})
	})
})

func initializeObject(kind string) client.Object {
	var (
		obj  client.Object
		meta = metav1.ObjectMeta{
			GenerateName: strings.ToLower(kind) + "-",
			Namespace:    testNamespace.Name,
			Labels:       map[string]string{testID: testRunID},
		}
	)

	if kind == "secret" {
		obj = &corev1.Secret{ObjectMeta: meta}
	} else if kind == "configMap" {
		obj = &corev1.ConfigMap{ObjectMeta: meta}
	}

	By("Create " + strings.ToTitle(kind))
	ExpectWithOffset(1, testClient.Create(ctx, obj)).To(Succeed())
	log.Info("Created object for test", kind, client.ObjectKeyFromObject(obj)) //nolint:logcheck

	DeferCleanup(func() {
		By("Delete  " + strings.ToTitle(kind))
		ExpectWithOffset(1, client.IgnoreNotFound(testClient.Delete(ctx, obj))).To(Succeed())
		log.Info("Deleted object for test", kind, client.ObjectKeyFromObject(obj)) //nolint:logcheck
	})

	return obj
}
