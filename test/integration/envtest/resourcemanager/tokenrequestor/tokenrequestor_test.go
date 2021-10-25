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
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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

		secret.Labels = map[string]string{"resources.gardener.cloud/purpose": "token-requestor"}
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
		secret.Labels = map[string]string{"resources.gardener.cloud/purpose": "token-requestor"}
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(Succeed())

		patch := client.MergeFrom(secret.DeepCopy())
		secret.Labels = nil
		Expect(testClient.Patch(ctx, secret, patch)).To(Succeed())

		Eventually(func() error {
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
	})

	It("should be able to authenticate with the created token", func() {
		secret.Labels = map[string]string{"resources.gardener.cloud/purpose": "token-requestor"}
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())

		var (
			newClient     client.Client
			newRestConfig = &rest.Config{
				Host:            restConfig.Host,
				BearerToken:     string(secret.Data["token"]),
				TLSClientConfig: rest.TLSClientConfig{CAData: restConfig.TLSClientConfig.CAData},
			}
		)
		newClient, err = client.New(newRestConfig, client.Options{Scheme: kubernetesscheme.Scheme})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() error {
			return newClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(BeForbiddenError())
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
