// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("Utils", func() {
	var (
		fakeClient client.Client
		ctx        context.Context
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		ctx = context.TODO()
	})

	Describe("UnsafeGuessKind", func() {
		It("should guess the kind correctly", func() {
			Expect(UnsafeGuessKind(&extensionsv1alpha1.Infrastructure{})).To(Equal("Infrastructure"))
		})
	})

	Describe("#RemoveAnnotation", func() {
		It("should delete specific annotation", func() {
			annotation := "test-delete-annotation-key"

			annotations := make(map[string]string)
			annotations[annotation] = "test-delete-annotation-value"
			annotations["test-no-delete-annotation-key"] = "test-no-delete-annotation-value"

			worker := &extensionsv1alpha1.Worker{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Worker",
					APIVersion: "extensions.gardener.cloud/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-worker",
					Namespace:   "test-namespace",
					Annotations: annotations,
				},
			}
			Expect(fakeClient.Create(ctx, worker.DeepCopy())).To(Succeed())

			Expect(RemoveAnnotation(ctx, fakeClient, worker, annotation)).To(Succeed())
			Expect(worker.Annotations).To(HaveLen(1))
			notdeletedAnnotation, ok := worker.Annotations["test-no-delete-annotation-key"]
			Expect(ok).To(BeTrue())
			Expect(notdeletedAnnotation).To(BeEquivalentTo("test-no-delete-annotation-value"))

			// Verify final state in the fake client
			result := &extensionsv1alpha1.Worker{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(worker), result)).To(Succeed())
			Expect(result.Annotations).NotTo(HaveKey(annotation))
			Expect(result.Annotations).To(HaveKeyWithValue("test-no-delete-annotation-key", "test-no-delete-annotation-value"))
		})
	})

	Describe("#ShouldSkipOperation", func() {
		var (
			worker *extensionsv1alpha1.Worker
		)

		BeforeEach(func() {
			worker = &extensionsv1alpha1.Worker{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Worker",
					APIVersion: "TestApi",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-worker",
					Namespace: "test-namespace",
				},
			}
		})

		Context("reconcile operation", func() {
			var (
				operationType = gardencorev1beta1.LastOperationTypeReconcile
			)

			It("should return false when lastOperation is missing", func() {
				Expect(ShouldSkipOperation(operationType, worker)).To(BeFalse())
			})

			It("should return true when lastOperation is migrate and succeeded", func() {
				worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(ShouldSkipOperation(operationType, worker)).To(BeTrue())
			})

			It("should return false when lastOperation is not migrate", func() {
				worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeRestore,
				}
				Expect(ShouldSkipOperation(operationType, worker)).To(BeFalse())
			})
		})

		Context("restore operation", func() {
			var (
				operationType = gardencorev1beta1.LastOperationTypeRestore
			)

			It("should return false when lastOperation is missing", func() {
				Expect(ShouldSkipOperation(operationType, worker)).To(BeFalse())
			})

			It("should return false when lastOperation is migrate and succeeded", func() {
				worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(ShouldSkipOperation(operationType, worker)).To(BeFalse())
			})

			It("should return false when lastOperation is not migrate", func() {
				worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeReconcile,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(ShouldSkipOperation(operationType, worker)).To(BeFalse())
			})
		})

		Context("delete operation", func() {
			var (
				operationType = gardencorev1beta1.LastOperationTypeDelete
			)

			It("should return false when lastOperation is missing", func() {
				Expect(ShouldSkipOperation(operationType, worker)).To(BeFalse())
			})

			It("should return true when lastOperation is migrate and succeeded", func() {
				worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(ShouldSkipOperation(operationType, worker)).To(BeTrue())
			})

			It("should return false when lastOperation is not migrate", func() {
				worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeReconcile,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(ShouldSkipOperation(operationType, worker)).To(BeFalse())
			})
		})

		Context("migrate operation", func() {
			var (
				operationType = gardencorev1beta1.LastOperationTypeRestore
			)

			It("should return false when lastOperation is missing", func() {
				Expect(ShouldSkipOperation(operationType, worker)).To(BeFalse())
			})

			It("should return false when lastOperation is migrate and succeeded", func() {
				worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(ShouldSkipOperation(operationType, worker)).To(BeFalse())
			})

			It("should return false when lastOperation is not migrate", func() {
				worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeReconcile,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(ShouldSkipOperation(operationType, worker)).To(BeFalse())
			})
		})
	})

	Describe("#IsMigrated", func() {
		var (
			worker *extensionsv1alpha1.Worker
		)

		JustBeforeEach(func() {
			worker = &extensionsv1alpha1.Worker{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Worker",
					APIVersion: "TestApi",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-worker",
					Namespace: "test-namespace",
				},
			}
		})
		It("should return false when lastOperation is missing", func() {
			Expect(IsMigrated(worker)).To(BeFalse())
		})
		It("should return false when lastOperation type is not Migrated", func() {
			worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type: gardencorev1beta1.LastOperationTypeReconcile,
			}
			Expect(IsMigrated(worker)).To(BeFalse())
		})
		It("should return false when lastOperation type is Migrated but state is not succeeded", func() {
			worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
				State: gardencorev1beta1.LastOperationStateProcessing,
			}
			Expect(IsMigrated(worker)).To(BeFalse())
		})
		It("should return true when lastOperation type is Migrated and state is succeeded", func() {
			worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(IsMigrated(worker)).To(BeTrue())
		})
	})

	Describe("#GetObjectByReference", func() {
		var (
			secretRef = &autoscalingv1.CrossVersionObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "foo",
			}
			workloadIdentityRef = &autoscalingv1.CrossVersionObjectReference{
				APIVersion: "security.gardener.cloud/v1alpha1",
				Kind:       "WorkloadIdentity",
				Name:       "bar",
			}
			namespace = "shoot--test--foo"
			refSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.ReferencedResourcesPrefix + "foo",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"foo": []byte("bar"),
				},
			}
			refWorkloadIdentity = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "workload-identity-ref-bar",
					Namespace: namespace,
				},
			}
		)

		It("should call client.Get and return the result for a secret reference", func() {
			Expect(fakeClient.Create(ctx, refSecret.DeepCopy())).To(Succeed())

			secret := &corev1.Secret{}
			Expect(GetObjectByReference(ctx, fakeClient, secretRef, namespace, secret)).To(Succeed())
			Expect(secret.Name).To(Equal(refSecret.Name))
			Expect(secret.Namespace).To(Equal(refSecret.Namespace))
			Expect(secret.Data).To(Equal(refSecret.Data))
		})

		It("should call client.Get and return the result for a workload identity reference", func() {
			Expect(fakeClient.Create(ctx, refWorkloadIdentity.DeepCopy())).To(Succeed())

			secret := &corev1.Secret{}
			Expect(GetObjectByReference(ctx, fakeClient, workloadIdentityRef, namespace, secret)).To(Succeed())
			Expect(secret.Name).To(Equal(refWorkloadIdentity.Name))
			Expect(secret.Namespace).To(Equal(refWorkloadIdentity.Namespace))
		})
	})
})
