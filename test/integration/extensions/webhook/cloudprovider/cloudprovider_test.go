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

package cloudprovider_test

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("", func() {
	var (
		cluster *extensionsv1alpha1.Cluster
		secret  *corev1.Secret

		originalData = map[string][]byte{
			"clientID": []byte("test"),
		}
	)
	BeforeEach(func() {
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace.Name,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
				Seed:         runtime.RawExtension{Raw: []byte("{}")},
				Shoot:        runtime.RawExtension{Raw: []byte("{}")},
			},
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: testNamespace.Name,
			},
			Data: originalData,
		}
	})

	JustBeforeEach(func() {
		By("Create Cluster")
		Expect(testClient.Create(ctx, cluster)).To(Succeed())
		log.Info("Created Cluster for test", "cluster", client.ObjectKeyFromObject(cluster))

		DeferCleanup(func() {
			By("deleting Cluster")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, cluster))).To(Succeed())
		})
	})

	Context("secret name is not cloudprovider", func() {
		BeforeEach(func() {
			secret.Name = "test-secret"
			Expect(testClient.Create(ctx, secret)).To(Succeed())

			DeferCleanup(func() {
				By("deleting Secret")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, secret))).To(Succeed())
			})
		})

		It("should not mutate the secret", func() {
			Expect(secret.Data).To(Equal(originalData))
		})
	})

	Context("secretname is cloudprofile", func() {
		BeforeEach(func() {
			DeferCleanup(func() {
				By("delete Secret")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, secret))).To(Succeed())
			})
		})

		It("should mutate secret", func() {
			By("create Secret")
			Eventually(func(g Gomega) {
				Expect(testClient.Create(ctx, secret)).To(Succeed())

				Expect(secret.Data).To(Equal(map[string][]byte{
					"clientID":     []byte("foo"),
					"clientSecret": []byte("bar"),
				}))
			}).Should(Succeed())
		})
	})
})
