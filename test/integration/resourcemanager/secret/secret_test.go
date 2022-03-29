// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

const (
	finalizerName = "resources.gardener.cloud/gardener-resource-manager"
)

var _ = Describe("Secret controller tests", func() {
	var (
		managedResource *resourcesv1alpha1.ManagedResource
		secret          *corev1.Secret
	)

	BeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    testNamespace.Name,
				GenerateName: "test-",
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{{
					Name: "foo",
				}},
			},
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: testNamespace.Name,
			},
		}
	})

	Context("Secret finalizer", func() {
		JustBeforeEach(func() {
			By("creating secret for test")
			Expect(testClient.Create(ctx, secret)).To(Succeed())
			By("creating ManagedResource for test")
			Expect(testClient.Create(ctx, managedResource)).To(Succeed())
			log.Info("Created ManagedResource for test", "managedResource", client.ObjectKeyFromObject(managedResource))
		})

		AfterEach(func() {
			Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
			Expect(testClient.Delete(ctx, secret)).To(Or(Succeed(), BeNotFoundError()))
			// Wait for clean up of the secret
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
			}).Should(BeNotFoundError())
		})

		It("should successfully add finalizer to all the secrets reference by managed resource", func() {
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				return secret.ObjectMeta.Finalizers
			}).Should(
				ContainElement(finalizerName),
			)
		})

		It("should remove finalizer from secret which are no longer referenced by any managed resource", func() {
			By("update ManagedResource to reference some other secret")
			patch := client.MergeFrom(managedResource.DeepCopy())
			managedResource.Spec.SecretRefs[0].Name = "bar"
			Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				return secret.ObjectMeta.Finalizers
			}).ShouldNot(
				ContainElement(finalizerName),
			)
		})

		It("should do nothing if there is no MR refrencing the secret", func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: testNamespace.Name,
				},
			}
			Expect(testClient.Create(ctx, secret)).To(Succeed())

			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				return secret.ObjectMeta.Finalizers
			}).ShouldNot(
				ContainElement(finalizerName),
			)
		})
	})
})
