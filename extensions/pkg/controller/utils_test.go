// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller_test

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			Expect(controller.UnsafeGuessKind(&extensionsv1alpha1.Infrastructure{})).To(Equal("Infrastructure"))
		})
	})

	Describe("#GetSecretByRef", func() {
		var (
			ctx = context.TODO()

			name      = "foo"
			namespace = "bar"
		)

		It("should get the secret", func() {
			var (
				objectMeta = metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				}
				data = map[string][]byte{
					"foo": []byte("bar"),
				}
			)

			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
				secret.ObjectMeta = objectMeta
				secret.Data = data
				return nil
			})

			secret, err := controller.GetSecretByReference(ctx, c, &corev1.SecretReference{
				Name:      name,
				Namespace: namespace,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(secret).To(Equal(&corev1.Secret{
				ObjectMeta: objectMeta,
				Data:       data,
			}))
		})

		It("should return the error", func() {
			ctx := context.TODO()

			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fmt.Errorf("error"))

			secret, err := controller.GetSecretByReference(ctx, c, &corev1.SecretReference{
				Name:      name,
				Namespace: namespace,
			})

			Expect(err).To(HaveOccurred())
			Expect(secret).To(BeNil())
		})
	})

	Describe("#DeleteAllFinalizers", func() {
		It("should delete all finalizers", func() {
			creationTimestamp := v1.Now()
			deletionTimestamp := v1.Now()
			labels := make(map[string]string)
			labels["test-label-key"] = "test-label-value"
			annotation := make(map[string]string)
			annotation["test-annotation-key"] = "test-annotation-value"
			owner := []metav1.OwnerReference{
				{
					APIVersion:         "test-api",
					Kind:               "test-owner-kind",
					Name:               "test-owner",
					UID:                types.UID("test-owner-UID"),
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(true),
				},
			}

			testFinalizer1 := "test-finalizer1"
			testFinalizer2 := "test-finalizer2"
			testFinalizer3 := "test-finalizer4"

			finalizers := []string{
				testFinalizer1,
				testFinalizer2,
				testFinalizer3,
			}

			secretRef := corev1.SecretReference{
				Name:      "test-secret",
				Namespace: "test-namespace",
			}

			worker := &extensionsv1alpha1.Worker{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Worker",
					APIVersion: "TestApi",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       "test-worker",
					Namespace:                  "test-namespace",
					GenerateName:               "test-generate-name",
					SelfLink:                   "test-self-link",
					UID:                        types.UID("test-UID"),
					ResourceVersion:            "test-resource-version",
					Generation:                 int64(1),
					CreationTimestamp:          creationTimestamp,
					DeletionTimestamp:          &deletionTimestamp,
					DeletionGracePeriodSeconds: pointer.Int64Ptr(10),
					Labels:                     labels,
					Annotations:                annotation,
					OwnerReferences:            owner,
					Finalizers:                 finalizers,
					ClusterName:                "test-cluster-name",
				},
				Spec: extensionsv1alpha1.WorkerSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: "",
					},
					Region:    "test-region",
					SecretRef: secretRef,
				},
			}
			ctx := context.TODO()
			key, err := client.ObjectKeyFromObject(worker)
			Expect(err).NotTo(HaveOccurred())

			gomock.InOrder(
				c.EXPECT().
					Get(ctx, key, worker).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, worker *extensionsv1alpha1.Worker) error {
						if len(worker.Finalizers) < 1 {
							return fmt.Errorf("Worker %s has no finalizers", worker.Name)
						}
						for _, finalizer := range worker.Finalizers {
							if finalizer != testFinalizer1 && finalizer != testFinalizer2 && finalizer != testFinalizer3 {
								return fmt.Errorf("Finalizer %s not found for worker %s", finalizer, worker.Name)
							}
						}
						return nil
					}),

				c.EXPECT().Update(ctx, worker),
			)

			Expect(controller.DeleteAllFinalizers(ctx, c, worker)).To(Succeed())
			Expect(len(worker.Finalizers)).To(Equal(0))
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
			workerWithAnnotation := worker.DeepCopyObject()
			ctx := context.TODO()

			c.EXPECT().Patch(ctx, worker, client.MergeFrom(workerWithAnnotation))

			Expect(controller.RemoveAnnotation(ctx, c, worker, annotation)).To(Succeed())
			Expect(len(worker.Annotations)).To(Equal(1))
			notdeletedAnnotation, ok := worker.Annotations["test-no-delete-annotation-key"]
			Expect(ok).To(BeTrue())
			Expect(notdeletedAnnotation).To(BeEquivalentTo("test-no-delete-annotation-value"))
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
			Expect(controller.IsMigrated(worker)).To(BeFalse())
		})
		It("should return false when lastOperation type is not Migrated", func() {
			worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type: gardencorev1beta1.LastOperationTypeReconcile,
			}
			Expect(controller.IsMigrated(worker)).To(BeFalse())
		})
		It("should return false when lastOperation type is Migrated but state is not succeeded", func() {
			worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
				State: gardencorev1beta1.LastOperationStateProcessing,
			}
			Expect(controller.IsMigrated(worker)).To(BeFalse())
		})
		It("should return true when lastOperation type is Migrated and state is succeeded", func() {
			worker.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(controller.IsMigrated(worker)).To(BeTrue())
		})
	})
})
