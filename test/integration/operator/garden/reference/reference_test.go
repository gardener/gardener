// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package reference_test

import (
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
		secret1    *corev1.Secret
		secret2    *corev1.Secret
		secret3    *corev1.Secret
		secret4    *corev1.Secret
		secret5    *corev1.Secret
		secret6    *corev1.Secret
		secret7    *corev1.Secret
		configMap1 *corev1.ConfigMap
		configMap2 *corev1.ConfigMap
		garden     *operatorv1alpha1.Garden
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
		secret4 = secret1.DeepCopy()
		secret5 = secret1.DeepCopy()
		secret6 = secret1.DeepCopy()
		secret7 = secret1.DeepCopy()

		configMap1 = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "configmap-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}
		configMap2 = configMap1.DeepCopy()

		for _, secret := range []*corev1.Secret{secret1, secret2, secret3, secret4, secret5, secret6, secret7} {
			By("Create Secret")
			Expect(testClient.Create(ctx, secret)).To(Succeed())
			log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(secret))

			DeferCleanup(func() {
				By("Delete Secret")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, secret))).To(Succeed())
			})
		}

		for _, configMap := range []*corev1.ConfigMap{configMap1, configMap2} {
			By("Create ConfigMap")
			Expect(testClient.Create(ctx, configMap)).To(Succeed())
			log.Info("Created ConfigMap for test", "configMap", client.ObjectKeyFromObject(configMap))

			DeferCleanup(func() {
				By("Delete ConfigMap")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, configMap))).To(Succeed())
			})
		}

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "garden-" + testRunID,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.GardenSpec{
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     "10.1.0.0/16",
						Services: "10.2.0.0/16",
					},
					Ingress: gardencorev1beta1.Ingress{
						Domain: "ingress.runtime-garden.local.gardener.cloud",
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
						Domains: []string{"virtual-garden.local.gardener.cloud"},
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
						Version: "1.26.3",
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
								SecretName: secret5.Name,
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
						Services: "100.64.0.0/13",
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
			for _, obj := range []client.Object{secret1, secret2, secret3, secret4, secret5, secret6, secret7, configMap1, configMap2} {
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
			Expect(testClient.Patch(ctx, garden, patch)).To(Succeed())

			for _, obj := range []client.Object{garden, secret1, secret2, secret3, secret4, secret5, secret6, secret7, configMap1, configMap2} {
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					return obj.GetFinalizers()
				}).ShouldNot(ContainElement("gardener.cloud/reference-protection"), obj.GetName()+" should not have the finalizer")
			}
		})
	})
})
