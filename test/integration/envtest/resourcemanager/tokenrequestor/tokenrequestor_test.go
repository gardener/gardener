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

package tokenrequestor_test

import (
	"context"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("TokenRequestor tests", func() {
	var (
		ctx = context.Background()

		namespace *corev1.Namespace

		secretName         = "kube-scheduler"
		serviceAccountName = "kube-scheduler-serviceaccount"

		secret         *corev1.Secret
		serviceAccount *corev1.ServiceAccount
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-namespace",
			},
		}
		Expect(testClient.Create(ctx, namespace)).To(Or(Succeed(), BeAlreadyExistsError()))

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace.Name,
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      serviceAccountName,
					"serviceaccount.resources.gardener.cloud/namespace": namespace.Name,
				},
			},
		}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: namespace.Name,
			},
		}
	})

	It("should behave correctly when: create w/o label, update w/ label, delete w/ label", func() {
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		Consistently(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(BeNotFoundError())

		secret.Labels = map[string]string{"resources.gardener.cloud/purpose": "tokenrequestor"}
		Expect(testClient.Update(ctx, secret)).To(Succeed())

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(Succeed())

		Expect(testClient.Delete(ctx, secret)).To(Succeed())

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(BeNotFoundError())
	})

	It("should behave correctly when: create w/ label, update w/o label, delete w/o label", func() {
		secret.Labels = map[string]string{"resources.gardener.cloud/purpose": "tokenrequestor"}
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(Succeed())

		Expect(testClient.Delete(ctx, serviceAccount)).To(Succeed())

		patch := secret.DeepCopy()
		secret.Labels = nil
		Expect(testClient.Patch(ctx, secret, client.MergeFrom(patch))).To(Succeed())

		Consistently(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(BeNotFoundError())

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: namespace.Name,
			},
		}
		Expect(testClient.Create(ctx, serviceAccount)).To(Succeed())
		Expect(testClient.Delete(ctx, secret)).To(Succeed())

		Consistently(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(Succeed())

		// Remove finalizers since the TokenRequestor will not act on this secret anymore
		patch = secret.DeepCopy()
		secret.Finalizers = nil
		Expect(testClient.Patch(ctx, secret, client.MergeFrom(patch))).To(Succeed())
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, secret)).To(Or(Succeed(), BeNotFoundError()))
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
		}).Should(BeNotFoundError())

		Expect(testClient.Delete(ctx, serviceAccount)).To(Or(Succeed(), BeNotFoundError()))
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(BeNotFoundError())
	})
})
