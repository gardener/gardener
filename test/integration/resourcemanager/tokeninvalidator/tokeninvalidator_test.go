// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("TokenInvalidator tests", func() {
	var (
		resourceName string

		validToken = []byte("some-valid-token")

		serviceAccount *corev1.ServiceAccount
		secret         *corev1.Secret

		verifyNotInvalidated = func(g Gomega) bool {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			return secret.Labels["token-invalidator.resources.gardener.cloud/consider"] == "" &&
				(secret.Data["token"] == nil || bytes.Equal(secret.Data["token"], validToken))
		}

		verifyInvalidated = func(g Gomega) bool {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			return secret.Labels["token-invalidator.resources.gardener.cloud/consider"] == "true" &&
				!bytes.Equal(secret.Data["token"], validToken)
		}
	)

	BeforeEach(func() {
		resourceName = "test-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
			},
			Secrets: []corev1.ObjectReference{{Name: resourceName}},
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        resourceName,
				Namespace:   testNamespace.Name,
				Annotations: map[string]string{"kubernetes.io/service-account.name": resourceName},
			},
			Data: map[string][]byte{"token": validToken},
		}
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
		By("Create Secret and ServiceAccount with automountServiceAccountToken=nil")
		serviceAccount.AutomountServiceAccountToken = nil
		Expect(testClient.Create(ctx, serviceAccount)).To(Succeed())
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		By("Ensure token is not getting invalidated")
		Consistently(verifyNotInvalidated).Should(BeTrue())

		By("Update ServiceAccount with automountServiceAccountToken=true")
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(true)
		Expect(testClient.Update(ctx, serviceAccount)).To(Succeed())

		By("Ensure token is still not getting invalidated")
		Consistently(verifyNotInvalidated).Should(BeTrue())
	})

	It("should invalidate the token", func() {
		By("Create Secret and ServiceAccount with automountServiceAccountToken=false")
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		Expect(testClient.Create(ctx, serviceAccount)).To(Succeed())
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		By("Ensure token is getting invalidated")
		Eventually(verifyInvalidated).Should(BeTrue())

		By("Delete token key from secret data to trigger regeneration")
		delete(secret.Data, "token")
		Expect(testClient.Update(ctx, secret)).To(Succeed())

		By("Ensure token is again getting invalidated")
		Eventually(verifyInvalidated).Should(BeTrue())
	})

	It("should invalidate the token and then regenerate it", func() {
		By("Create Secret and ServiceAccount with automountServiceAccountToken=false")
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		Expect(testClient.Create(ctx, serviceAccount)).To(Succeed())
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		By("Ensure token is getting invalidated")
		Eventually(verifyInvalidated).Should(BeTrue())

		By("Label ServiceAccount with skip=true")
		metav1.SetMetaDataLabel(&serviceAccount.ObjectMeta, "token-invalidator.resources.gardener.cloud/skip", "true")
		Expect(testClient.Update(ctx, serviceAccount)).To(Succeed())

		By("Ensure token is not getting invalidated")
		Eventually(verifyNotInvalidated).Should(BeTrue())
	})

	It("should wait with invalidation until the pods using the static token are deleted", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: secret.Namespace,
				Labels:    map[string]string{resourcesv1alpha1.ProjectedTokenSkip: "true"},
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

		By("Create Pod")
		Expect(testClient.Create(ctx, pod)).To(Succeed())

		By("Create Secret and ServiceAccount with automountServiceAccountToken=false")
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		Expect(testClient.Create(ctx, serviceAccount)).To(Succeed())
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		By("Ensure token is not getting invalidated yet")
		Consistently(verifyNotInvalidated).Should(BeTrue())

		By("Delete Pod")
		Expect(testClient.Delete(ctx, pod)).To(Succeed())

		By("Ensure token is now getting invalidated")
		Eventually(verifyInvalidated).Should(BeTrue())
	})
})
