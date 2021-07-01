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

package kubernetes_test

import (
	"context"
	"fmt"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("secretref", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx context.Context

		secretRef *corev1.SecretReference
		secret    *corev1.Secret
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		ctx = context.TODO()

		secretRef = &corev1.SecretReference{
			Name:      name,
			Namespace: namespace,
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "core.gardener.cloud/v1beta1",
						Kind:               "Shoot",
						Name:               name,
						UID:                "uid",
						Controller:         pointer.Bool(true),
						BlockOwnerDeletion: pointer.Bool(true),
					},
				},
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
			Type: corev1.SecretTypeOpaque,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetSecretByReference", func() {
		It("should get the secret", func() {
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *corev1.Secret) error {
				*s = *secret
				return nil
			})

			result, err := kutil.GetSecretByReference(ctx, c, secretRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(secret))
		})

		It("should fail if getting the secret failed", func() {
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fmt.Errorf("error"))

			result, err := kutil.GetSecretByReference(ctx, c, secretRef)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("#DeleteSecretByReference", func() {
		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		})

		It("should delete the secret if it exists", func() {
			c.EXPECT().Delete(ctx, secret).Return(nil)

			err := kutil.DeleteSecretByReference(ctx, c, secretRef)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if the secret doesn't exist", func() {
			c.EXPECT().Delete(ctx, secret).Return(apierrors.NewNotFound(corev1.Resource("secret"), name))

			err := kutil.DeleteSecretByReference(ctx, c, secretRef)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if deleting the secret failed", func() {
			c.EXPECT().Delete(ctx, secret).Return(fmt.Errorf("error"))

			err := kutil.DeleteSecretByReference(ctx, c, secretRef)
			Expect(err).To(HaveOccurred())
		})
	})
})
