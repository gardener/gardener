// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Shoot Reference controller tests", func() {
	var (
		secret1 *corev1.Secret
		secret2 *corev1.Secret
		secret3 *corev1.Secret
		secret4 *corev1.Secret
		secret5 *corev1.Secret

		configMap1 *corev1.ConfigMap
		configMap2 *corev1.ConfigMap
		configMap3 *corev1.ConfigMap
		configMap4 *corev1.ConfigMap

		allReferencedObjects []client.Object
		shoot                *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		secret1 = initializeObject("secret").(*corev1.Secret)
		secret2 = initializeObject("secret").(*corev1.Secret)
		secret3 = initializeObject("secret").(*corev1.Secret)
		secret4 = initializeObject("secret").(*corev1.Secret)
		secret5 = initializeObject("secret").(*corev1.Secret)

		configMap1 = initializeObject("configMap").(*corev1.ConfigMap)
		configMap2 = initializeObject("configMap").(*corev1.ConfigMap)
		configMap3 = initializeObject("configMap").(*corev1.ConfigMap)
		configMap4 = initializeObject("configMap").(*corev1.ConfigMap)

		allReferencedObjects = append([]client.Object{}, secret1, secret2, secret3, secret4, secret5)
		allReferencedObjects = append(allReferencedObjects, configMap1, configMap2, configMap3, configMap4)

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
					Version: "1.31.1",
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{{
							Name:                 "ValidatingAdmissionWebhook",
							KubeconfigSecretName: &secret5.Name,
						}},
						AuditConfig: &gardencorev1beta1.AuditConfig{
							AuditPolicy: &gardencorev1beta1.AuditPolicy{
								ConfigMapRef: &corev1.ObjectReference{
									Name: configMap1.Name,
								},
							},
						},
						StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{
							ConfigMapName: configMap3.Name,
						},
						StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
							ConfigMapName: configMap4.Name,
							Kubeconfigs: []gardencorev1beta1.AuthorizerKubeconfigReference{{
								AuthorizerName: "foo",
								SecretName:     secret4.Name,
							}},
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
			for _, obj := range allReferencedObjects {
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

			for _, obj := range append([]client.Object{shoot}, allReferencedObjects...) {
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

				for _, obj := range append([]client.Object{shoot2}, allReferencedObjects...) {
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

	switch kind {
	case "secret":
		obj = &corev1.Secret{ObjectMeta: meta}
	case "configMap":
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
