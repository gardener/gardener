// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Garden Reference controller tests", func() {
	var (
		secret1 *corev1.Secret
		secret2 *corev1.Secret
		secret3 *corev1.Secret
		secret4 *corev1.Secret
		secret5 *corev1.Secret
		secret6 *corev1.Secret
		secret7 *corev1.Secret
		secret8 *corev1.Secret
		secret9 *corev1.Secret

		configMap1 *corev1.ConfigMap
		configMap2 *corev1.ConfigMap
		configMap3 *corev1.ConfigMap
		configMap4 *corev1.ConfigMap

		allReferencedObjects []client.Object
		garden               *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		secret1 = initializeObject("secret").(*corev1.Secret)
		secret2 = initializeObject("secret").(*corev1.Secret)
		secret3 = initializeObject("secret").(*corev1.Secret)
		secret4 = initializeObject("secret").(*corev1.Secret)
		secret5 = initializeObject("secret").(*corev1.Secret)
		secret6 = initializeObject("secret").(*corev1.Secret)
		secret7 = initializeObject("secret").(*corev1.Secret)
		secret8 = initializeObject("secret").(*corev1.Secret)
		secret9 = initializeObject("secret").(*corev1.Secret)

		configMap1 = initializeObject("configMap").(*corev1.ConfigMap)
		configMap2 = initializeObject("configMap").(*corev1.ConfigMap)
		configMap3 = initializeObject("configMap").(*corev1.ConfigMap)
		configMap4 = initializeObject("configMap").(*corev1.ConfigMap)

		allReferencedObjects = append([]client.Object{}, secret1, secret2, secret3, secret4, secret5, secret6, secret7, secret8, secret9)
		allReferencedObjects = append(allReferencedObjects, configMap1, configMap2, configMap3, configMap4)

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "garden-" + testRunID,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.GardenSpec{
				DNS: &operatorv1alpha1.DNSManagement{
					Providers: []operatorv1alpha1.DNSProvider{
						{
							Name:      "primary",
							Type:      "test",
							SecretRef: corev1.LocalObjectReference{Name: secret9.Name},
						},
					},
				},
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     []string{"10.1.0.0/16"},
						Services: []string{"10.2.0.0/16"},
					},
					Ingress: operatorv1alpha1.Ingress{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.runtime-garden.local.gardener.cloud"}},
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
					Provider: operatorv1alpha1.Provider{
						Zones: []string{"a", "b", "c"},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "virtual-garden.local.gardener.cloud"}},
					},
					ETCD: &operatorv1alpha1.ETCD{
						Main: &operatorv1alpha1.ETCDMain{
							Backup: &operatorv1alpha1.Backup{
								SecretRef: corev1.LocalObjectReference{
									Name: secret1.Name,
								},
							},
						},
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.31.1",
						KubeAPIServer: &operatorv1alpha1.KubeAPIServerConfig{
							KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{
								AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{{Name: "ValidatingWebhook", KubeconfigSecretName: &secret2.Name}},
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
										SecretName:     secret8.Name,
									}},
								},
							},
							Authentication: &operatorv1alpha1.Authentication{
								Webhook: &operatorv1alpha1.AuthenticationWebhook{
									KubeconfigSecretName: secret3.Name,
								},
							},
							AuditWebhook: &operatorv1alpha1.AuditWebhook{
								KubeconfigSecretName: secret4.Name,
							},
							SNI: &operatorv1alpha1.SNI{
								SecretName: &secret5.Name,
							},
						},
					},
					Gardener: operatorv1alpha1.Gardener{
						ClusterIdentity: "test",
						APIServer: &operatorv1alpha1.GardenerAPIServerConfig{
							AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{{Name: "ValidatingWebhook", KubeconfigSecretName: &secret6.Name}},
							AuditConfig: &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: configMap2.Name,
									},
								},
							},
							AuditWebhook: &operatorv1alpha1.AuditWebhook{
								KubeconfigSecretName: secret7.Name,
							},
						},
					},
					Maintenance: operatorv1alpha1.Maintenance{
						TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
					Networking: operatorv1alpha1.Networking{
						Services: []string{"100.64.0.0/13"},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create Garden")
		Expect(testClient.Create(ctx, garden)).To(Succeed())
		log.Info("Created Garden for test", "garden", client.ObjectKeyFromObject(garden))

		DeferCleanup(func() {
			By("Delete Garden")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, garden))).To(Succeed())

			By("Ensure Garden is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
			}).Should(BeNotFoundError())
		})
	})

	Context("no references", func() {
		BeforeEach(func() {
			garden.Spec.VirtualCluster.ETCD = nil
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = nil
			garden.Spec.VirtualCluster.Gardener.APIServer = nil
			garden.Spec.DNS = nil
		})

		It("should not add the finalizer to the garden", func() {
			Consistently(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Finalizers
			}).ShouldNot(ContainElement("gardener.cloud/reference-protection"))
		})
	})

	Context("w/ references", func() {
		JustBeforeEach(func() {
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Finalizers
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

		It("should remove finalizers from the garden and the referenced secrets and configmaps", func() {
			patch := client.MergeFrom(garden.DeepCopy())
			garden.Spec.VirtualCluster.ETCD = nil
			garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = nil
			garden.Spec.VirtualCluster.Gardener.APIServer = nil
			garden.Spec.DNS = nil
			Expect(testClient.Patch(ctx, garden, patch)).To(Succeed())

			for _, obj := range append([]client.Object{garden}, allReferencedObjects...) {
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					return obj.GetFinalizers()
				}).ShouldNot(ContainElement("gardener.cloud/reference-protection"), obj.GetName()+" should not have the finalizer")
			}
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
