// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("secretref", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx context.Context

		secretRef               *corev1.SecretReference
		secret                  *corev1.Secret
		objectRef               *corev1.ObjectReference
		secretPartialObjectMeta *metav1.PartialObjectMetadata
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		ctx = context.TODO()

		secretRef = &corev1.SecretReference{
			Name:      name,
			Namespace: namespace,
		}
		objectRef = &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Name:       name,
			Namespace:  namespace,
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
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				},
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
			Type: corev1.SecretTypeOpaque,
		}
		secretPartialObjectMeta = &metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: *secret.ObjectMeta.DeepCopy(),
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetSecretByReference", func() {
		It("should get the secret", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *corev1.Secret, _ ...client.GetOption) error {
				*s = *secret
				return nil
			})

			result, err := kubernetesutils.GetSecretByReference(ctx, c, secretRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(secret))
		})

		It("should fail if getting the secret failed", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fmt.Errorf("error"))

			result, err := kubernetesutils.GetSecretByReference(ctx, c, secretRef)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("#GetSecretMetadataByReference", func() {
		It("should get the secret", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadata{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *metav1.PartialObjectMetadata, _ ...client.GetOption) error {
				*s = *secretPartialObjectMeta
				return nil
			})

			result, err := kubernetesutils.GetSecretMetadataByReference(ctx, c, secretRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(secretPartialObjectMeta))
		})

		It("should fail if getting the secret failed", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadata{})).Return(fmt.Errorf("error"))

			result, err := kubernetesutils.GetSecretMetadataByReference(ctx, c, secretRef)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("#GetSecretByObjectReference", func() {
		It("should get the secret", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *corev1.Secret, _ ...client.GetOption) error {
				*s = *secret
				return nil
			})

			result, err := kubernetesutils.GetSecretByObjectReference(ctx, c, objectRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(secret))
		})

		It("should fail if getting the secret failed", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fmt.Errorf("error"))

			result, err := kubernetesutils.GetSecretByObjectReference(ctx, c, objectRef)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if object reference is nil", func() {
			_, err := kubernetesutils.GetSecretByObjectReference(ctx, c, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("ref is nil"))
		})

		It("should fail if object reference does not refer to a secret", func() {
			ref := &corev1.ObjectReference{
				APIVersion: "foo.bar/v1alpha1",
				Kind:       "Baz",
				Name:       name,
				Namespace:  namespace,
			}

			_, err := kubernetesutils.GetSecretByObjectReference(ctx, c, ref)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("objectRef does not refer to secret"))
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

			err := kubernetesutils.DeleteSecretByReference(ctx, c, secretRef)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if the secret doesn't exist", func() {
			c.EXPECT().Delete(ctx, secret).Return(apierrors.NewNotFound(corev1.Resource("secret"), name))

			err := kubernetesutils.DeleteSecretByReference(ctx, c, secretRef)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if deleting the secret failed", func() {
			c.EXPECT().Delete(ctx, secret).Return(fmt.Errorf("error"))

			err := kubernetesutils.DeleteSecretByReference(ctx, c, secretRef)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeleteSecretByObjectReference", func() {
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

			err := kubernetesutils.DeleteSecretByObjectReference(ctx, c, objectRef)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed if the secret doesn't exist", func() {
			c.EXPECT().Delete(ctx, secret).Return(apierrors.NewNotFound(corev1.Resource("secret"), name))

			err := kubernetesutils.DeleteSecretByObjectReference(ctx, c, objectRef)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if deleting the secret failed", func() {
			c.EXPECT().Delete(ctx, secret).Return(fmt.Errorf("error"))

			err := kubernetesutils.DeleteSecretByObjectReference(ctx, c, objectRef)
			Expect(err).To(HaveOccurred())
		})

		It("should fail if object reference is nil", func() {
			err := kubernetesutils.DeleteSecretByObjectReference(ctx, c, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("ref is nil"))
		})

		It("should fail if object reference does not refer to a secret", func() {
			ref := &corev1.ObjectReference{
				APIVersion: "foo.bar/v1alpha1",
				Kind:       "Baz",
				Name:       name,
				Namespace:  namespace,
			}

			err := kubernetesutils.DeleteSecretByObjectReference(ctx, c, ref)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("objectRef does not refer to secret"))
		})
	})
})
