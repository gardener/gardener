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

package cloudprovider_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var _ = Describe("CloudProvider tests", func() {
	var (
		secret *corev1.Secret

		originalData = map[string][]byte{
			"clientID": []byte("test"),
		}
	)

	BeforeEach(func() {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: testNamespace.Name,
			},
			Data: originalData,
		}

		DeferCleanup(func() {
			By("Delete Secret")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, secret))).To(Succeed())
		})
	})

	JustBeforeEach(func() {
		By("Create Secret")
		Expect(testClient.Create(ctx, secret)).To(Succeed())
	})

	Context("secret name is not cloudprovider", func() {
		BeforeEach(func() {
			secret.Name = "test-secret"
		})

		It("should not mutate the secret", func() {
			By("Patch Secret to invoke webhook")
			Consistently(func(g Gomega) map[string][]byte {
				g.Expect(testClient.Patch(ctx, secret, client.RawPatch(types.MergePatchType, []byte("{}")))).To(Succeed())
				return secret.Data
			}).Should(Equal(originalData))
		})
	})

	Context("secret name is cloudprovider", func() {
		It("should not mutate the secret because matching labels are not present", func() {
			By("Patch Secret to invoke webhook")
			Consistently(func(g Gomega) map[string][]byte {
				g.Expect(testClient.Patch(ctx, secret, client.RawPatch(types.MergePatchType, []byte("{}")))).To(Succeed())
				return secret.Data
			}).Should(Equal(originalData))
		})

		Context("purpose label present", func() {
			BeforeEach(func() {
				secret.ObjectMeta.Labels = map[string]string{
					v1beta1constants.GardenerPurpose: v1beta1constants.SecretNameCloudProvider,
				}
			})

			It("should mutate the secret because matching labels are present", func() {
				By("Patch Secret to invoke webhook")
				Consistently(func(g Gomega) map[string][]byte {
					g.Expect(testClient.Patch(ctx, secret, client.RawPatch(types.MergePatchType, []byte("{}")))).To(Succeed())
					return secret.Data
				}).Should(And(
					HaveKeyWithValue("clientID", BeEquivalentTo("foo")),
					HaveKeyWithValue("clientSecret", BeEquivalentTo("bar")),
				))
			})
		})
	})
})
