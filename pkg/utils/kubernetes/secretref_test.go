// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("secretref", func() {
	var (
		ctx        context.Context
		fakeClient client.Client

		secretRef               *corev1.SecretReference
		secret                  *corev1.Secret
		internalSecret          *gardencorev1beta1.InternalSecret
		workloadIdentity        *securityv1alpha1.WorkloadIdentity
		objectRef               *corev1.ObjectReference
		secretPartialObjectMeta *metav1.PartialObjectMetadata
		crossVersionRef         *autoscalingv1.CrossVersionObjectReference
	)

	BeforeEach(func() {
		ctx = context.TODO()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

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
		internalSecret = &gardencorev1beta1.InternalSecret{
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
				"bar": []byte("foo"),
			},
			Type: corev1.SecretTypeOpaque,
		}
		workloadIdentity = &securityv1alpha1.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: securityv1alpha1.WorkloadIdentitySpec{
				Audiences: []string{"aud"},
				TargetSystem: securityv1alpha1.TargetSystem{
					Type: "local",
				},
			},
		}
		secretPartialObjectMeta = &metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: *secret.ObjectMeta.DeepCopy(),
		}
		crossVersionRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Name:       name,
		}
	})

	Describe("#GetSecretByReference", func() {
		It("should get the secret", func() {
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := kubernetesutils.GetSecretByReference(ctx, fakeClient, secretRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Name).To(Equal(secret.Name))
			Expect(result.Data).To(Equal(secret.Data))
		})

		It("should fail if getting the secret failed", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("error")
					},
				}).
				Build()

			result, err := kubernetesutils.GetSecretByReference(ctx, fakeClient, secretRef)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("#GetSecretMetadataByReference", func() {
		It("should get the secret", func() {
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := kubernetesutils.GetSecretMetadataByReference(ctx, fakeClient, secretRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Name).To(Equal(secretPartialObjectMeta.Name))
			Expect(result.Namespace).To(Equal(secretPartialObjectMeta.Namespace))
			Expect(result.OwnerReferences).To(Equal(secretPartialObjectMeta.OwnerReferences))
			Expect(result.TypeMeta).To(Equal(secretPartialObjectMeta.TypeMeta))
		})

		It("should fail if getting the secret failed", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("error")
					},
				}).
				Build()

			result, err := kubernetesutils.GetSecretMetadataByReference(ctx, fakeClient, secretRef)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("#GetSecretByObjectReference", func() {
		It("should get the secret", func() {
			Expect(fakeClient.Create(ctx, secret.DeepCopy())).To(Succeed())

			result, err := kubernetesutils.GetSecretByObjectReference(ctx, fakeClient, objectRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Name).To(Equal(secret.Name))
			Expect(result.Namespace).To(Equal(secret.Namespace))
			Expect(result.Data).To(Equal(secret.Data))
		})

		It("should fail if getting the secret failed", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("error")
					},
				}).
				Build()

			result, err := kubernetesutils.GetSecretByObjectReference(ctx, fakeClient, objectRef)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if object reference is nil", func() {
			_, err := kubernetesutils.GetSecretByObjectReference(ctx, fakeClient, nil)
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

			_, err := kubernetesutils.GetSecretByObjectReference(ctx, fakeClient, ref)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("objectRef does not refer to secret"))
		})
	})

	Describe("#GetCredentialsByObjectReference", func() {
		It("should get referenced Secret", func() {
			Expect(fakeClient.Create(ctx, secret.DeepCopy())).To(Succeed())

			result, err := kubernetesutils.GetCredentialsByObjectReference(ctx, fakeClient, *objectRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.GetName()).To(Equal(secret.Name))
			Expect(result.GetNamespace()).To(Equal(secret.Namespace))
		})

		It("should fail to get referenced Secret if reading it fails", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("error")
					},
				}).
				Build()

			result, err := kubernetesutils.GetCredentialsByObjectReference(ctx, fakeClient, *objectRef)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should get referenced InternalSecret", func() {
			Expect(fakeClient.Create(ctx, internalSecret.DeepCopy())).To(Succeed())

			objectRef = &corev1.ObjectReference{
				APIVersion: "core.gardener.cloud/v1beta1",
				Kind:       "InternalSecret",
				Namespace:  namespace,
				Name:       name,
			}

			result, err := kubernetesutils.GetCredentialsByObjectReference(ctx, fakeClient, *objectRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.GetName()).To(Equal(internalSecret.Name))
			Expect(result.GetNamespace()).To(Equal(internalSecret.Namespace))
		})

		It("should fail to get referenced InternalSecret if reading it fails", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("error")
					},
				}).
				Build()

			objectRef = &corev1.ObjectReference{
				APIVersion: "core.gardener.cloud/v1beta1",
				Kind:       "InternalSecret",
				Namespace:  namespace,
				Name:       name,
			}

			result, err := kubernetesutils.GetCredentialsByObjectReference(ctx, fakeClient, *objectRef)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should get referenced WorkloadIdentity", func() {
			Expect(fakeClient.Create(ctx, workloadIdentity.DeepCopy())).To(Succeed())

			objectRef = &corev1.ObjectReference{
				APIVersion: "security.gardener.cloud/v1alpha1",
				Kind:       "WorkloadIdentity",
				Namespace:  namespace,
				Name:       name,
			}

			result, err := kubernetesutils.GetCredentialsByObjectReference(ctx, fakeClient, *objectRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.GetName()).To(Equal(workloadIdentity.Name))
			Expect(result.GetNamespace()).To(Equal(workloadIdentity.Namespace))
		})

		It("should fail to get referenced WorkloadIdentity if reading it fails", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("error")
					},
				}).
				Build()

			objectRef = &corev1.ObjectReference{
				APIVersion: "security.gardener.cloud/v1alpha1",
				Kind:       "WorkloadIdentity",
				Namespace:  namespace,
				Name:       name,
			}

			result, err := kubernetesutils.GetCredentialsByObjectReference(ctx, fakeClient, *objectRef)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if object reference does not refer to a supported object", func() {
			ref := &corev1.ObjectReference{
				APIVersion: "foo.bar/v1alpha1",
				Kind:       "Baz",
				Name:       name,
				Namespace:  namespace,
			}

			_, err := kubernetesutils.GetCredentialsByObjectReference(ctx, fakeClient, *ref)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unsupported credentials reference: garden/foo, foo.bar/v1alpha1, Kind=Baz"))
		})
	})

	Describe("#DeleteSecretByReference", func() {
		It("should delete the secret if it exists", func() {
			secretToDelete := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}

			Expect(fakeClient.Create(ctx, secretToDelete)).To(Succeed())

			err := kubernetesutils.DeleteSecretByReference(ctx, fakeClient, secretRef)
			Expect(err).NotTo(HaveOccurred())

			// Verify the secret was deleted
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretToDelete), &corev1.Secret{})).To(BeNotFoundError())
		})

		It("should succeed if the secret doesn't exist", func() {
			err := kubernetesutils.DeleteSecretByReference(ctx, fakeClient, secretRef)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if deleting the secret failed", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
						return fmt.Errorf("error")
					},
				}).
				Build()

			err := kubernetesutils.DeleteSecretByReference(ctx, fakeClient, secretRef)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeleteSecretByObjectReference", func() {
		It("should delete the secret if it exists", func() {
			secretToDelete := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}

			Expect(fakeClient.Create(ctx, secretToDelete)).To(Succeed())

			err := kubernetesutils.DeleteSecretByObjectReference(ctx, fakeClient, objectRef)
			Expect(err).NotTo(HaveOccurred())

			// Verify the secret was deleted
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretToDelete), &corev1.Secret{})).To(BeNotFoundError())
		})

		It("should succeed if the secret doesn't exist", func() {
			err := kubernetesutils.DeleteSecretByObjectReference(ctx, fakeClient, objectRef)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if deleting the secret failed", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
						return fmt.Errorf("error")
					},
				}).
				Build()

			err := kubernetesutils.DeleteSecretByObjectReference(ctx, fakeClient, objectRef)
			Expect(err).To(HaveOccurred())
		})

		It("should fail if object reference is nil", func() {
			err := kubernetesutils.DeleteSecretByObjectReference(ctx, fakeClient, nil)
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

			err := kubernetesutils.DeleteSecretByObjectReference(ctx, fakeClient, ref)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("objectRef does not refer to secret"))
		})
	})

	Describe("#GetCredentialsByCrossVersionObjectReference", func() {
		It("should get referenced Secret", func() {
			Expect(fakeClient.Create(ctx, secret.DeepCopy())).To(Succeed())

			result, err := kubernetesutils.GetCredentialsByCrossVersionObjectReference(ctx, fakeClient, *crossVersionRef, namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.GetName()).To(Equal(secret.Name))
			Expect(result.GetNamespace()).To(Equal(secret.Namespace))
		})

		It("should fail to get referenced Secret if reading it fails", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("error")
					},
				}).
				Build()

			result, err := kubernetesutils.GetCredentialsByCrossVersionObjectReference(ctx, fakeClient, *crossVersionRef, namespace)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should get referenced WorkloadIdentity", func() {
			ref := autoscalingv1.CrossVersionObjectReference{
				APIVersion: "security.gardener.cloud/v1alpha1",
				Kind:       "WorkloadIdentity",
				Name:       name,
			}

			Expect(fakeClient.Create(ctx, workloadIdentity.DeepCopy())).To(Succeed())

			result, err := kubernetesutils.GetCredentialsByCrossVersionObjectReference(ctx, fakeClient, ref, namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.GetName()).To(Equal(workloadIdentity.Name))
			Expect(result.GetNamespace()).To(Equal(workloadIdentity.Namespace))
		})

		It("should fail to get referenced WorkloadIdentity if reading it fails", func() {
			ref := autoscalingv1.CrossVersionObjectReference{
				APIVersion: "security.gardener.cloud/v1alpha1",
				Kind:       "WorkloadIdentity",
				Name:       name,
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fmt.Errorf("error")
					},
				}).
				Build()

			result, err := kubernetesutils.GetCredentialsByCrossVersionObjectReference(ctx, fakeClient, ref, namespace)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if cross version reference does not refer to supported credentials", func() {
			ref := autoscalingv1.CrossVersionObjectReference{
				APIVersion: "foo.bar/v1alpha1",
				Kind:       "Baz",
				Name:       name,
			}

			_, err := kubernetesutils.GetCredentialsByCrossVersionObjectReference(ctx, fakeClient, ref, namespace)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("unsupported credentials reference: garden/foo, foo.bar/v1alpha1, Kind=Baz"))
		})

		It("should use the provided namespace parameter", func() {
			differentNamespace := "different-namespace"
			secretInDifferentNs := secret.DeepCopy()
			secretInDifferentNs.Namespace = differentNamespace

			Expect(fakeClient.Create(ctx, secretInDifferentNs)).To(Succeed())

			result, err := kubernetesutils.GetCredentialsByCrossVersionObjectReference(ctx, fakeClient, *crossVersionRef, differentNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.GetNamespace()).To(Equal(differentNamespace))
		})
	})
})
