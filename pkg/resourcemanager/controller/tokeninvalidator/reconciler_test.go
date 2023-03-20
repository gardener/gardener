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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/tokeninvalidator"
)

var _ = Describe("TokenInvalidator", func() {
	var (
		ctx = context.TODO()

		fakeClient client.Client
		ctrl       reconcile.Reconciler
		request    reconcile.Request

		secretPartialObjectMeta *metav1.PartialObjectMetadata
		secret                  *corev1.Secret
		serviceAccount          *corev1.ServiceAccount

		serviceAccountName = "serviceaccount-name"
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		ctrl = &Reconciler{
			TargetReader: fakeClient,
			TargetClient: fakeClient,
		}

		secretPartialObjectMeta = &metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        "secret-name",
				Namespace:   "secret-namespace",
				Annotations: map[string]string{"kubernetes.io/service-account.name": serviceAccountName},
			},
		}
		secret = &corev1.Secret{
			ObjectMeta: secretPartialObjectMeta.ObjectMeta,
		}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: secret.Namespace,
			},
		}

		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			},
		}
	})

	Describe("#Reconcile", func() {
		It("should not return an error when secret could not be read", func() {
			result, err := ctrl.Reconcile(ctx, request)
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should add the purpose label", func() {
			Expect(fakeClient.Create(ctx, secretPartialObjectMeta)).To(Succeed())
			Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.Labels).To(HaveKeyWithValue("resources.gardener.cloud/purpose", "token-invalidator"))
		})

		Context("remove consider label", func() {
			BeforeEach(func() {
				secretPartialObjectMeta.Labels = map[string]string{"token-invalidator.resources.gardener.cloud/consider": "true"}
				Expect(fakeClient.Create(ctx, secretPartialObjectMeta)).To(Succeed())
			})

			AfterEach(func() {
				result, err := ctrl.Reconcile(ctx, request)
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				Expect(secret.Labels).NotTo(HaveKeyWithValue("token-invalidator.resources.gardener.cloud/consider", "true"))
			})

			It("AutomountServiceAccountToken=nil", func() {
				serviceAccount.AutomountServiceAccountToken = nil
				Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())
			})

			It("AutomountServiceAccountToken=nil and skip label", func() {
				serviceAccount.AutomountServiceAccountToken = nil
				serviceAccount.Labels = map[string]string{"token-invalidator.resources.gardener.cloud/skip": "true"}
				Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())
			})

			It("AutomountServiceAccountToken=true", func() {
				serviceAccount.AutomountServiceAccountToken = pointer.Bool(true)
				Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())
			})

			It("AutomountServiceAccountToken=true and skip label", func() {
				serviceAccount.AutomountServiceAccountToken = pointer.Bool(true)
				serviceAccount.Labels = map[string]string{"token-invalidator.resources.gardener.cloud/skip": "true"}
				Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())
			})

			It("AutomountServiceAccountToken=false but skip label", func() {
				serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
				serviceAccount.Labels = map[string]string{"token-invalidator.resources.gardener.cloud/skip": "true"}
				Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())
			})
		})

		It("should do nothing because secret already has the 'consider' label", func() {
			secretPartialObjectMeta.Labels = map[string]string{"token-invalidator.resources.gardener.cloud/consider": "true"}
			Expect(fakeClient.Create(ctx, secretPartialObjectMeta)).To(Succeed())

			serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
			Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())

			result, err := ctrl.Reconcile(ctx, request)
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("add consider label", func() {
			BeforeEach(func() {
				secretPartialObjectMeta.Labels = nil
				Expect(fakeClient.Create(ctx, secretPartialObjectMeta)).To(Succeed())
			})

			Context("no requeue", func() {
				It("AutomountServiceAccountToken=false", func() {
					serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
					Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())

					result, err := ctrl.Reconcile(ctx, request)
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
					Expect(secret.Labels).To(HaveKeyWithValue("token-invalidator.resources.gardener.cloud/consider", "true"))
				})
			})

			Context("requeue", func() {
				It("pod with mounted service account secret still exists", func() {
					pod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foo",
							Namespace: secretPartialObjectMeta.Namespace,
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{{
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretPartialObjectMeta.Name,
									},
								},
							}},
						},
					}
					Expect(fakeClient.Create(ctx, pod)).To(Succeed())

					serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
					Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())

					result, err := ctrl.Reconcile(ctx, request)
					Expect(result).To(Equal(reconcile.Result{Requeue: true}))
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
					Expect(secret.Labels).NotTo(HaveKeyWithValue("token-invalidator.resources.gardener.cloud/consider", "true"))
				})
			})
		})
	})
})
