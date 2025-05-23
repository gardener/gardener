// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Utils", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
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
					APIVersion: "TestApi",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-worker",
					Namespace:   "test-namespace",
					Annotations: annotations,
				},
			}
			workerWithAnnotation := worker.DeepCopy()
			expectedWorker := worker.DeepCopy()
			delete(expectedWorker.Annotations, annotation)

			ctx := context.TODO()
			test.EXPECTPatch(ctx, c, expectedWorker, workerWithAnnotation, types.MergePatchType)

			Expect(RemoveAnnotation(ctx, c, worker, annotation)).To(Succeed())
			Expect(worker.Annotations).To(HaveLen(1))
			notdeletedAnnotation, ok := worker.Annotations["test-no-delete-annotation-key"]
			Expect(ok).To(BeTrue())
			Expect(notdeletedAnnotation).To(BeEquivalentTo("test-no-delete-annotation-value"))
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
			ctx = context.TODO()
			ref = &autoscalingv1.CrossVersionObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "foo",
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
		)

		It("should call client.Get and return the result", func() {
			secret := &corev1.Secret{}
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: v1beta1constants.ReferencedResourcesPrefix + ref.Name}, secret).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret, _ ...client.GetOption) error {
					refSecret.DeepCopyInto(secret)
					return nil
				})
			Expect(GetObjectByReference(ctx, c, ref, namespace, secret)).To(Succeed())
			Expect(secret).To(Equal(refSecret))
		})
	})
})
