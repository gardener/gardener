// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/types"
)

var _ = Describe("extensions", func() {
	var (
		ctx     context.Context
		log     logrus.FieldLogger
		ctrl    *gomock.Controller
		mockNow *mocktime.MockNow
		now     time.Time

		c client.Client

		defaultInterval  time.Duration
		defaultTimeout   time.Duration
		defaultThreshold time.Duration

		namespace string
		name      string

		expected *extensionsv1alpha1.Worker
	)

	BeforeEach(func() {
		ctx = context.TODO()
		log = logger.NewNopLogger()
		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		c = fake.NewFakeClientWithScheme(s)

		defaultInterval = 1 * time.Millisecond
		defaultTimeout = 1 * time.Millisecond
		defaultThreshold = 1 * time.Millisecond

		namespace = "test-namespace"
		name = "test-name"

		expected = &extensionsv1alpha1.Worker{
			TypeMeta: metav1.TypeMeta{
				Kind:       extensionsv1alpha1.WorkerResource,
				APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#WaitUntilExtensionCRReady", func() {
		It("should return error if extension CR does not exist", func() {
			err := WaitUntilExtensionCRReady(
				ctx, c, log,
				func() runtime.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultTimeout, defaultTimeout, nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("should return error if extension CR is not ready", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilExtensionCRReady(
				ctx, c, log,
				func() runtime.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultThreshold, defaultTimeout, nil,
			)
			Expect(err).To(HaveOccurred(), "worker readiness error")
		})

		It("should return success if extension CR is ready", func() {
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilExtensionCRReady(
				ctx, c, log,
				func() runtime.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultThreshold, defaultTimeout, nil,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call postReadyFunc if extension CR is ready", func() {
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")

			val := 0
			err := WaitUntilExtensionCRReady(
				ctx, c, log,
				func() runtime.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultThreshold, defaultTimeout, func(runtime.Object) error {
					val++
					return nil
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal(1))
		})
	})

	Describe("#WaitUntilObjectReadyWithHealthFunction", func() {
		It("should return error if object does not exist error", func() {
			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, c, log,
				func(obj runtime.Object) error {
					return nil
				},
				func() runtime.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultThreshold, defaultTimeout,
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("should return error if ready func returns error", func() {
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, c, log,
				func(obj runtime.Object) error {
					return errors.New("error")
				},
				func() runtime.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultThreshold, defaultTimeout,
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("should return success if health func does not return error", func() {
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, c, log,
				func(obj runtime.Object) error {
					return nil
				},
				func() runtime.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultThreshold, defaultTimeout,
				nil,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass correct object to health func", func() {
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, c, log,
				func(obj runtime.Object) error {
					Expect(obj).To(Equal(expected))
					return nil
				},
				func() runtime.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultThreshold, defaultTimeout,
				nil,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call post ready func if health func does not return error", func() {
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")

			val := 0
			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, c, log,
				func(obj runtime.Object) error {
					return nil
				},
				func() runtime.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultThreshold, defaultTimeout, func(runtime.Object) error {
					val++
					return nil
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal(1))
		})
	})

	Describe("#DeleteExtensionCR", func() {
		It("should not return error if extension CR does not exist", func() {
			Expect(DeleteExtensionCR(ctx, c, func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} }, namespace, name)).To(Succeed())
		})

		It("should not return error if deleted successfully", func() {
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
			Expect(DeleteExtensionCR(ctx, c, func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} }, namespace, name)).To(Succeed())
		})

		It("should delete extension CR", func() {
			defer test.WithVars(
				&common.TimeNow, mockNow.Do,
			)()

			expected.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			mc := mockclient.NewMockClient(ctrl)
			// check if the extension CR exists
			mc.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{})).SetArg(2, *expected).Return(nil)
			// add deletion confirmation and Timestamp annotation
			mc.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{})).Return(nil)
			mc.EXPECT().Delete(ctx, expected).Times(1).Return(fmt.Errorf("some random error"))

			Expect(DeleteExtensionCR(ctx, mc, func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} }, namespace, name)).To(HaveOccurred())
		})
	})

	Describe("#DeleteExtensionCRs", func() {
		It("should delete all extension CRs", func() {
			deletionTimestamp := metav1.Now()
			expected.ObjectMeta.DeletionTimestamp = &deletionTimestamp

			expected2 := expected.DeepCopy()
			expected2.Name = "worker2"
			list := &extensionsv1alpha1.WorkerList{
				Items: []extensionsv1alpha1.Worker{*expected, *expected2},
			}
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
			Expect(c.Create(ctx, expected2)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")

			err := DeleteExtensionCRs(
				ctx,
				c,
				list,
				func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
				namespace,
				func(obj extensionsv1alpha1.Object) bool { return true },
			)

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#WaitUntilExtensionCRsDeleted", func() {
		It("should return error if atleast one extension CR is not deleted", func() {
			list := &extensionsv1alpha1.WorkerList{}

			deletionTimestamp := metav1.Now()
			expected.ObjectMeta.DeletionTimestamp = &deletionTimestamp

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")

			err := WaitUntilExtensionCRsDeleted(
				ctx,
				c,
				log,
				list,
				func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, defaultInterval, defaultTimeout,
				func(object extensionsv1alpha1.Object) bool { return true })

			Expect(err).To(HaveOccurred())
		})

		It("should return success if all extensions CRs are deleted", func() {
			list := &extensionsv1alpha1.WorkerList{}
			err := WaitUntilExtensionCRsDeleted(
				ctx,
				c,
				log,
				list,
				func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, defaultInterval, defaultTimeout,
				func(object extensionsv1alpha1.Object) bool { return true })
			Expect(err).NotTo(HaveOccurred())
		})

	})

	Describe("#WaitUntilExtensionCRDeleted", func() {
		It("should return error if extension CR is not deleted", func() {
			deletionTimestamp := metav1.Now()
			expected.ObjectMeta.DeletionTimestamp = &deletionTimestamp

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
			err := WaitUntilExtensionCRDeleted(ctx, c, log,
				func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultTimeout)

			Expect(err).To(HaveOccurred())
		})

		It("should return success if extensions CRs gets deleted", func() {
			err := WaitUntilExtensionCRDeleted(ctx, c, log,
				func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} }, extensionsv1alpha1.WorkerResource,
				namespace, name,
				defaultInterval, defaultTimeout)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("restoring extension CR state", func() {
		var (
			expectedState *runtime.RawExtension
			shootState    *gardencorev1alpha1.ShootState
		)

		BeforeEach(func() {
			expectedState = &runtime.RawExtension{Raw: []byte(`{"data":"value"}`)}

			shootState = &gardencorev1alpha1.ShootState{
				Spec: gardencorev1alpha1.ShootStateSpec{
					Extensions: []gardencorev1alpha1.ExtensionResourceState{
						{
							Name:  &name,
							Kind:  extensionsv1alpha1.WorkerResource,
							State: expectedState,
						},
					},
				},
			}
		})

		Describe("#RestoreExtensionWithDeployFunction", func() {
			It("should restore the extension CR state with the provided deploy fuction and annotate it for restoration", func() {
				defer test.WithVars(
					&common.TimeNow, mockNow.Do,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				err := RestoreExtensionWithDeployFunction(
					ctx,
					shootState,
					c,
					extensionsv1alpha1.WorkerResource,
					namespace,
					func(ctx context.Context, operationAnnotation string) (extensionsv1alpha1.Object, error) {
						Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
						return expected, nil
					},
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(expected.Annotations).To(Equal(map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRestore,
					v1beta1constants.GardenerTimestamp: now.UTC().String(),
				}))
				Expect(expected.Status.State).To(Equal(expectedState))
			})

			It("should only annotate the resource with restore operation annotation if a corresponding state does not exist in the ShootState", func() {
				defer test.WithVars(
					&common.TimeNow, mockNow.Do,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				expected.Name = "worker2"
				err := RestoreExtensionWithDeployFunction(
					ctx,
					shootState,
					c,
					extensionsv1alpha1.WorkerResource,
					namespace,
					func(ctx context.Context, operationAnnotation string) (extensionsv1alpha1.Object, error) {
						Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
						return expected, nil
					},
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(expected.Annotations).To(Equal(map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRestore,
					v1beta1constants.GardenerTimestamp: now.UTC().String(),
				}))
				Expect(expected.Status.State).To(BeNil())
			})
		})

		Describe("#RestoreExtensionObjectState", func() {
			It("should return error if the extension CR does not exist", func() {
				err := RestoreExtensionObjectState(
					ctx,
					c,
					shootState,
					namespace,
					expected,
					extensionsv1alpha1.WorkerResource,
				)
				Expect(err).To(HaveOccurred())
			})

			It("should update the state if the extension CR exists", func() {
				defer test.WithVars(
					&common.TimeNow, mockNow.Do,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
				err := RestoreExtensionObjectState(
					ctx,
					c,
					shootState,
					namespace,
					expected,
					extensionsv1alpha1.WorkerResource,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(expected.Status.State).To(Equal(expectedState))
			})
		})
	})

	Describe("#MigrateExtensionCR", func() {
		It("should not return error if extension CR does not exist", func() {
			Expect(MigrateExtensionCR(ctx, c, func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} }, namespace, name)).To(Succeed())
		})

		It("should properly annotate extension CR for migration", func() {
			defer test.WithVars(
				&common.TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			expectedWithAnnotations := expected.DeepCopy()
			expectedWithAnnotations.Annotations = map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationMigrate,
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{})).SetArg(2, *expected).Return(nil)
			mc.EXPECT().Patch(ctx, expectedWithAnnotations, gomock.AssignableToTypeOf(client.MergeFrom(expected))).Return(nil)

			err := MigrateExtensionCR(ctx, mc, func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} }, namespace, name)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#MigrateExtensionCRs", func() {
		It("should not return error if there are no extension resources", func() {
			Expect(
				MigrateExtensionCRs(ctx, c, &extensionsv1alpha1.BackupBucketList{}, func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupBucket{} }, namespace),
			).To(Succeed())
		})

		It("should properly annotate all extension CRs for migration", func() {
			for i := 0; i < 4; i++ {
				containerRuntimeExtension := &extensionsv1alpha1.ContainerRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      fmt.Sprintf("containerruntime-%d", i),
					},
				}
				Expect(c.Create(ctx, containerRuntimeExtension)).To(Succeed())
			}

			Expect(
				MigrateExtensionCRs(ctx, c, &extensionsv1alpha1.ContainerRuntimeList{}, func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ContainerRuntime{} }, namespace),
			).To(Succeed())

			containerRuntimeList := &extensionsv1alpha1.ContainerRuntimeList{}
			Expect(c.List(ctx, containerRuntimeList, client.InNamespace(namespace))).To(Succeed())
			Expect(len(containerRuntimeList.Items)).To(Equal(4))
			for _, item := range containerRuntimeList.Items {
				Expect(item.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))
			}
		})
	})

	Describe("#WaitUntilExtensionCRMigrated", func() {
		It("should not return error if resource does not exist", func() {
			err := WaitUntilExtensionCRMigrated(
				ctx,
				c,
				func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
				namespace, name,
				defaultInterval, defaultTimeout,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("should return error if migration times out",
			func(lastOperation *gardencorev1beta1.LastOperation, match func() GomegaMatcher) {
				expected.Status.LastOperation = lastOperation
				Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
				err := WaitUntilExtensionCRMigrated(
					ctx,
					c,
					func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
					namespace, name,
					defaultInterval, defaultTimeout,
				)
				Expect(err).To(match())
			},
			Entry("last operation is not Migrate", &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeReconcile,
			}, HaveOccurred),
			Entry("last operation is Migrate but not successful", &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateProcessing,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}, HaveOccurred),
			Entry("last operation is Migrate and successful", &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}, Succeed),
		)
	})

	Describe("#WaitUntilExtensionCRsMigrated", func() {
		It("should not return error if there are no extension CRs", func() {
			Expect(WaitUntilExtensionCRsMigrated(
				ctx,
				c,
				&extensionsv1alpha1.WorkerList{},
				func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
				namespace,
				defaultInterval,
				defaultTimeout)).To(Succeed())
		})

		DescribeTable("should return error if migration times out",
			func(lastOperations []*gardencorev1beta1.LastOperation, match func() GomegaMatcher) {
				for i, lastOp := range lastOperations {
					existing := expected.DeepCopy()
					existing.Status.LastOperation = lastOp
					existing.Name = fmt.Sprintf("worker-%d", i)
					Expect(c.Create(ctx, existing)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
				}
				err := WaitUntilExtensionCRsMigrated(
					ctx,
					c,
					&extensionsv1alpha1.WorkerList{},
					func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
					namespace,
					defaultInterval,
					defaultTimeout)
				Expect(err).To(match())
			},
			Entry("last operation is not Migrate",
				[]*gardencorev1beta1.LastOperation{
					{
						State: gardencorev1beta1.LastOperationStateSucceeded,
						Type:  gardencorev1beta1.LastOperationTypeReconcile,
					},
					{
						State: gardencorev1beta1.LastOperationStateSucceeded,
						Type:  gardencorev1beta1.LastOperationTypeReconcile,
					},
				},
				HaveOccurred),
			Entry("last operation is Migrate but not successful",
				[]*gardencorev1beta1.LastOperation{
					{
						State: gardencorev1beta1.LastOperationStateProcessing,
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
					},
					{
						State: gardencorev1beta1.LastOperationStateSucceeded,
						Type:  gardencorev1beta1.LastOperationTypeReconcile,
					},
				}, HaveOccurred),
			Entry("last operation is Migrate and successful on only one resource",
				[]*gardencorev1beta1.LastOperation{
					{
						State: gardencorev1beta1.LastOperationStateProcessing,
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
					},
					{
						State: gardencorev1beta1.LastOperationStateSucceeded,
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
					},
				}, HaveOccurred),
			Entry("last operation is Migrate and successful on all resources",
				[]*gardencorev1beta1.LastOperation{
					{
						State: gardencorev1beta1.LastOperationStateSucceeded,
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
					},
					{
						State: gardencorev1beta1.LastOperationStateSucceeded,
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
					},
				}, Succeed),
		)
	})

	Describe("#AnnotateExtensionObjectWithOperation", func() {
		It("should return error if object does not exist", func() {
			Expect(AnnotateExtensionObjectWithOperation(ctx, c, expected, v1beta1constants.GardenerOperationMigrate)).NotTo(Succeed())
		})

		It("should annotate extension object with operation", func() {
			defer test.WithVars(
				&common.TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			expectedWithAnnotations := expected.DeepCopy()
			expectedWithAnnotations.Annotations = map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationMigrate,
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Patch(ctx, expectedWithAnnotations, gomock.AssignableToTypeOf(client.MergeFrom(expected))).Return(nil)

			err := AnnotateExtensionObjectWithOperation(ctx, mc, expected, v1beta1constants.GardenerOperationMigrate)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
