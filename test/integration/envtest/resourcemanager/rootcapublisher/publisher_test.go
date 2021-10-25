// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package rootcapublisher_test

import (
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Root CA Controller tests", func() {
	var (
		namespace *corev1.Namespace
		configMap *corev1.ConfigMap
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"},
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-root-ca.crt",
				Namespace: namespace.Name,
			},
		}

		Expect(testClient.Create(ctx, namespace)).To(Or(Succeed(), BeAlreadyExistsError()))

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
		}).Should(Succeed())
	})

	Context("kube-root-ca.crt config map", func() {
		AfterEach(func() {
			Eventually(func() map[string]string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())

				return configMap.Data
			}).Should(SatisfyAll(Not(BeNil()), HaveKeyWithValue("ca.crt", string(certFile))))
		})

		It("should successfully create a config map on creating a namespace", func() {})

		It("should successfully update the config map if manual update occur", func() {
			configMap.Data = nil
			Expect(testClient.Update(ctx, configMap)).To(Succeed())
		})

		It("should recreate the config map if it gets deleted", func() {
			Expect(testClient.Delete(ctx, configMap)).To(Succeed())

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
			}).Should(BeNotFoundError())
		})
	})

	Context("Other config maps", func() {
		It("should ignore config maps with different name", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: namespace.Name,
				},
				Data: map[string]string{"foo": "bar"},
			}
			Expect(testClient.Create(ctx, cm)).To(Succeed())

			baseCM := cm.DeepCopy()
			cm.Data["foo"] = "newbar"

			Consistently(func() map[string]string {
				Expect(testClient.Patch(ctx, cm, client.MergeFrom(baseCM))).To(Succeed())

				return cm.Data
			}).Should(SatisfyAll(HaveLen(1), HaveKeyWithValue("foo", "newbar")))
		})

		It("should ignore config maps that are updated by the k8s publisher", func() {
			configMap.Data = nil
			configMap.Annotations = map[string]string{"kubernetes.io/description": "test description"}
			Expect(testClient.Update(ctx, configMap)).To(Succeed())

			Consistently(func() map[string]string {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())

				return configMap.Data
			}).Should(BeNil())
		})
	})
})
