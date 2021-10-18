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

package tokeninvalidator_test

import (
	"bytes"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("TokenInvalidator tests", func() {
	var (
		serviceAccountName = "serviceaccount"
		secretName         = "secret"
		validToken         = []byte("some-valid-token")

		namespace      *corev1.Namespace
		serviceAccount *corev1.ServiceAccount
		secret         *corev1.Secret

		verifyNotInvalidated = func() bool {
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			return secret.Labels["token-invalidator.resources.gardener.cloud/consider"] == "" &&
				(secret.Data["token"] == nil || bytes.Equal(secret.Data["token"], validToken))
		}

		verifyInvalidated = func() bool {
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			return secret.Labels["token-invalidator.resources.gardener.cloud/consider"] == "true" &&
				!bytes.Equal(secret.Data["token"], validToken)
		}
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-namespace",
			},
		}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: namespace.Name,
			},
			Secrets: []corev1.ObjectReference{{Name: secretName}},
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        secretName,
				Namespace:   namespace.Name,
				Annotations: map[string]string{"kubernetes.io/service-account.name": serviceAccountName},
			},
			Data: map[string][]byte{"token": validToken},
		}

		Expect(testClient.Create(ctx, namespace)).To(Or(Succeed(), BeAlreadyExistsError()))
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, serviceAccount)).To(Or(Succeed(), BeNotFoundError()))
		Expect(testClient.Delete(ctx, secret)).To(Or(Succeed(), BeNotFoundError()))
	})

	Context("no action", func() {
		AfterEach(func() {
			secretCopy := secret.DeepCopy()
			Expect(testClient.Create(ctx, secret)).To(Succeed())

			Consistently(func() bool {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secretCopy), secretCopy)).To(Succeed())
				return apiequality.Semantic.DeepEqual(secret, secretCopy)
			}).Should(BeTrue())
		})

		It("secret is no service account token secret", func() {
			secret.Annotations = nil
		})

		It("secret data is nil", func() {
			secret.Data = nil
		})
	})

	It("should not invalidate the token", func() {
		serviceAccount.AutomountServiceAccountToken = nil
		Expect(testClient.Create(ctx, serviceAccount)).To(Succeed())
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		Consistently(verifyNotInvalidated).Should(BeTrue())

		serviceAccount.AutomountServiceAccountToken = pointer.Bool(true)
		Expect(testClient.Update(ctx, serviceAccount)).To(Succeed())

		Consistently(verifyNotInvalidated).Should(BeTrue())
	})

	It("should invalidate the token", func() {
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		Expect(testClient.Create(ctx, serviceAccount)).To(Succeed())
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		Eventually(verifyInvalidated).Should(BeTrue())

		delete(secret.Data, "token")
		Expect(testClient.Update(ctx, secret)).To(Succeed())

		Eventually(verifyInvalidated).Should(BeTrue())
	})

	It("should invalidate the token and then regenerate it", func() {
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		Expect(testClient.Create(ctx, serviceAccount)).To(Succeed())
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		Eventually(verifyInvalidated).Should(BeTrue())

		metav1.SetMetaDataLabel(&serviceAccount.ObjectMeta, "token-invalidator.resources.gardener.cloud/skip", "true")
		Expect(testClient.Update(ctx, serviceAccount)).To(Succeed())

		Eventually(verifyNotInvalidated).Should(BeTrue())
	})

	It("should wait with invalidation until the pods using the static token are deleted", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: secret.Namespace,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "some-container",
					Image: "some-image",
				}},
				Volumes: []corev1.Volume{{
					Name: "token",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: secret.Name,
						},
					},
				}},
			},
		}
		Expect(testClient.Create(ctx, pod)).To(Succeed())

		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		Expect(testClient.Create(ctx, serviceAccount)).To(Succeed())
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		Consistently(verifyNotInvalidated()).Should(BeTrue())

		Expect(testClient.Delete(ctx, pod)).To(Succeed())

		Eventually(verifyInvalidated).Should(BeTrue())
	})
})
