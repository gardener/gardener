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

package reference_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Shoot Reference controller tests", func() {
	var (
		secret1    *corev1.Secret
		secret2    *corev1.Secret
		configMap1 *corev1.ConfigMap
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

		configMap1 = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "configmap-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
		}

		By("Create Secret1")
		Expect(testClient.Create(ctx, secret1)).To(Succeed())
		log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(secret1))

		By("Create Secret2")
		Expect(testClient.Create(ctx, secret2)).To(Succeed())
		log.Info("Created Secret for test", "secret", client.ObjectKeyFromObject(secret2))

		By("Create ConfigMap")
		Expect(testClient.Create(ctx, configMap1)).To(Succeed())
		log.Info("Created ConfigMap for test", "configMap", client.ObjectKeyFromObject(configMap1))

		DeferCleanup(func() {
			By("Delete Secret1")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, secret1))).To(Succeed())

			By("Delete Secret2")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, secret2))).To(Succeed())

			By("Delete ConfigMap")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, configMap1))).To(Succeed())
		})

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: pointer.String("secretbinding"),
				CloudProfileName:  "cloudprofile1",
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
					Domain: pointer.String("some-domain.example.com"),
					Providers: []gardencorev1beta1.DNSProvider{
						{Type: pointer.String("type"), SecretName: pointer.String(secret1.Name)},
						{Type: pointer.String("type"), SecretName: pointer.String(secret2.Name)},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.20.1",
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
				Networking: gardencorev1beta1.Networking{
					Type: "foo-networking",
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
			for _, obj := range []client.Object{secret1, secret2, configMap1} {
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					return obj.GetFinalizers()
				}).Should(ContainElement("gardener.cloud/reference-protection"), obj.GetName()+" should have the finalizer")
			}
		})

		It("should remove finalizers from the referenced secrets and configmaps", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.DNS.Providers = nil
			shoot.Spec.Kubernetes.KubeAPIServer = nil
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			for _, obj := range []client.Object{secret1, secret2, configMap1} {
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					return obj.GetFinalizers()
				}).ShouldNot(ContainElement("gardener.cloud/reference-protection"), obj.GetName()+" should not have the finalizer")
			}
		})
	})
})
