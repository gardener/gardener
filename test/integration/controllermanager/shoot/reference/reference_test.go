// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Shoot Reference controller tests", func() {
	var (
		secret1    *corev1.Secret
		secret2    *corev1.Secret
		secret3    *corev1.Secret
		configMap1 *corev1.ConfigMap
		configMap2 *corev1.ConfigMap
		shoot      *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		secret1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "secret-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}

		secret2 = secret1.DeepCopy()
		secret3 = secret1.DeepCopy()

		configMap1 = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "configmap-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}
		configMap2 = configMap1.DeepCopy()

		By("Create Secret1")
		Expect(testClient.Create(ctx, secret1)).To(Succeed())
		log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(secret1))

		By("Create Secret2")
		Expect(testClient.Create(ctx, secret2)).To(Succeed())
		log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(secret2))

		By("Create Secret3")
		Expect(testClient.Create(ctx, secret3)).To(Succeed())
		log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(secret3))

		By("Create ConfigMap1")
		Expect(testClient.Create(ctx, configMap1)).To(Succeed())
		log.Info("Created ConfigMap for test", "configMap", client.ObjectKeyFromObject(configMap1))

		By("Create ConfigMap2")
		Expect(testClient.Create(ctx, configMap2)).To(Succeed())
		log.Info("Created ConfigMap for test", "configMap", client.ObjectKeyFromObject(configMap2))

		DeferCleanup(func() {
			By("Delete Secret1")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, secret1))).To(Succeed())
			By("Delete Secret2")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, secret2))).To(Succeed())
			By("Delete Secret3")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, secret3))).To(Succeed())

			By("Delete ConfigMap1")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, configMap1))).To(Succeed())
			By("Delete ConfigMap2")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, configMap2))).To(Succeed())
		})

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To("secretbinding"),
				CloudProfileName:  ptr.To("cloudprofile1"),
				Region:            "europe-central-1",
				Provider: gardencorev1beta1.Provider{
					Type: "foo-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 3,
							Maximum: 3,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
							},
						},
					},
				},
				DNS: &gardencorev1beta1.DNS{
					Domain: ptr.To("some-domain.example.com"),
					Providers: []gardencorev1beta1.DNSProvider{
						{Type: ptr.To("type"), SecretName: ptr.To(secret1.Name)},
						{Type: ptr.To("type"), SecretName: ptr.To(secret2.Name)},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.25.1",
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						AuditConfig: &gardencorev1beta1.AuditConfig{
							AuditPolicy: &gardencorev1beta1.AuditPolicy{
								ConfigMapRef: &corev1.ObjectReference{
									Name: configMap1.Name,
								},
							},
						},
					},
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To("foo-networking"),
				},
				Resources: []gardencorev1beta1.NamedResourceReference{
					{
						Name: "foo",
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       secret3.Name,
						},
					},
					{
						Name: "bar",
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       configMap2.Name,
						},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create Shoot")
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
		})
	})

	Context("no references", func() {
		BeforeEach(func() {
			shoot.Spec.DNS.Providers = nil
			shoot.Spec.Kubernetes.KubeAPIServer = nil
			shoot.Spec.Resources = nil
		})

		It("should not add the finalizer to the shoot", func() {
			Consistently(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Finalizers
			}).ShouldNot(ContainElement("gardener.cloud/reference-protection"))
		})
	})

	Context("w/ references", func() {
		JustBeforeEach(func() {
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Finalizers
			}).Should(ContainElement("gardener.cloud/reference-protection"))
		})

		It("should add finalizers to the referenced secrets and configmaps", func() {
			for _, obj := range []client.Object{secret1, secret2, secret3, configMap1, configMap2} {
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					return obj.GetFinalizers()
				}).Should(ContainElement("gardener.cloud/reference-protection"), obj.GetName()+" should have the finalizer")
			}
		})

		It("should remove finalizers from the shoot and the referenced secrets and configmaps", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.DNS.Providers = nil
			shoot.Spec.Kubernetes.KubeAPIServer = nil
			shoot.Spec.Resources = nil
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			for _, obj := range []client.Object{shoot, secret1, secret2, secret3, configMap1, configMap2} {
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					return obj.GetFinalizers()
				}).ShouldNot(ContainElement("gardener.cloud/reference-protection"), obj.GetName()+" should not have the finalizer")
			}
		})

		Context("multiple shoots", func() {
			var shoot2 *gardencorev1beta1.Shoot

			BeforeEach(func() {
				shoot2 = shoot.DeepCopy()
			})

			JustBeforeEach(func() {
				By("Create second Shoot")
				Expect(testClient.Create(ctx, shoot2)).To(Succeed())
				log.Info("Created second Shoot for test", "shoot", client.ObjectKeyFromObject(shoot2))

				DeferCleanup(func() {
					By("Delete second Shoot")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot2))).To(Succeed())
				})

				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot2), shoot2)).To(Succeed())
					return shoot2.Finalizers
				}).Should(ContainElement("gardener.cloud/reference-protection"))
			})

			It("should not remove finalizers from the referenced secrets and configmaps because another shoot still references them", func() {
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.DNS.Providers = nil
				shoot.Spec.Kubernetes.KubeAPIServer = nil
				shoot.Spec.Resources = nil
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.GetFinalizers()
				}).ShouldNot(ContainElement("gardener.cloud/reference-protection"), shoot.GetName()+" should not have the finalizer")

				for _, obj := range []client.Object{shoot2, secret1, secret2, secret3, configMap1, configMap2} {
					Consistently(func(g Gomega) []string {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
						return obj.GetFinalizers()
					}).Should(ContainElement("gardener.cloud/reference-protection"), obj.GetName()+" should have the finalizer")
				}
			})
		})
	})
})
