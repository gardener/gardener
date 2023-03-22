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

package secret_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const finalizerName = "resources.gardener.cloud/gardener-resource-manager"

var _ = Describe("Secret controller tests", func() {
	var (
		managedResource *resourcesv1alpha1.ManagedResource
		secretFoo       *corev1.Secret
		secretBar       *corev1.Secret
	)

	BeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    testNamespace.Name,
				GenerateName: "test-",
			},
		}

		secretFoo = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "foo-",
				Namespace:    testNamespace.Name,
			},
		}

		secretBar = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "bar-",
				Namespace:    testNamespace.Name,
			},
		}
	})

	JustBeforeEach(func() {
		By("Create secret for test")
		Expect(testClient.Create(ctx, secretFoo)).To(Succeed())
		log.Info("Created Secret for test", "secretName", secretFoo.Name)
		Expect(testClient.Create(ctx, secretBar)).To(Succeed())
		By("Create ManagedResource for test")
		log.Info("Created Secret for test", "secretName", secretBar.Name)

		managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{
			{Name: secretFoo.Name},
			{Name: secretBar.Name},
		}

		Expect(testClient.Create(ctx, managedResource)).To(Succeed())
		log.Info("Created ManagedResource for test", "managedResource", client.ObjectKeyFromObject(managedResource))
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
		Expect(testClient.Delete(ctx, secretFoo)).To(Or(Succeed(), BeNotFoundError()))
		Expect(testClient.Delete(ctx, secretBar)).To(Or(Succeed(), BeNotFoundError()))
		// Wait for clean up of the secret
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(secretFoo), secretFoo)
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(secretBar), secretBar)
		}).Should(BeNotFoundError())
	})

	Context("Secret finalizer", func() {
		It("should successfully add finalizer to all the secrets which are referenced by ManagedResources", func() {
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretFoo), secretFoo)).To(Succeed())
				return secretFoo.ObjectMeta.Finalizers
			}).Should(
				ContainElement(finalizerName),
			)
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretBar), secretBar)).To(Succeed())
				return secretBar.ObjectMeta.Finalizers
			}).Should(
				ContainElement(finalizerName),
			)
		})

		It("should remove finalizer from secrets which are no longer referenced by any ManagedResource", func() {
			By("Update ManagedResource to reference some other secret")
			patch := client.MergeFrom(managedResource.DeepCopy())
			managedResource.Spec.SecretRefs[0].Name = "test"
			Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretFoo), secretFoo)).To(Succeed())
				return secretFoo.ObjectMeta.Finalizers
			}).ShouldNot(
				ContainElement(finalizerName),
			)
		})

		It("should do nothing if there is no ManagedResource referencing the secret", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: testNamespace.Name,
				},
			}
			Expect(testClient.Create(ctx, secret)).To(Succeed())

			Consistently(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				return secret.ObjectMeta.Finalizers
			}).ShouldNot(
				ContainElement(finalizerName),
			)
		})
	})
})
