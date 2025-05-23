// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensions_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
)

var _ = Describe("extensions", func() {
	var (
		ctx       context.Context
		log       logr.Logger
		ctrl      *gomock.Controller
		mockNow   *mocktime.MockNow
		now       time.Time
		fakeOps   *retryfake.Ops
		resetVars func()

		c      client.Client
		scheme *runtime.Scheme

		defaultInterval  time.Duration
		defaultTimeout   time.Duration
		defaultThreshold time.Duration

		namespace string
		name      string

		expected *extensionsv1alpha1.Worker
	)

	BeforeEach(func() {
		ctx = context.TODO()
		log = logr.Discard()
		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)

		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())

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

		c = fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&extensionsv1alpha1.Worker{}).Build()

		fakeOps = &retryfake.Ops{MaxAttempts: 1}
		resetVars = test.WithVars(
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		)
	})

	AfterEach(func() {
		resetVars()
		ctrl.Finish()
	})

	Describe("#WaitUntilExtensionObjectReady", func() {
		AfterEach(func() {
			Expect(client.ObjectKeyFromObject(expected)).To(Equal(client.ObjectKey{Namespace: namespace, Name: name}), "should not reset object's key")
		})

		It("should return error if extension object does not exist", func() {
			err := WaitUntilExtensionObjectReady(
				ctx, c, log,
				expected, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultTimeout, defaultTimeout, nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("should return error if extension object is not ready", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilExtensionObjectReady(
				ctx, c, log,
				expected, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultThreshold, defaultTimeout, nil,
			)
			Expect(err).To(HaveOccurred(), "worker readiness error")
		})

		It("should return success if extension object got ready the first time", func() {
			passedObj := expected.DeepCopy()
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Now(),
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilExtensionObjectReady(
				ctx, c, log,
				passedObj, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultThreshold, defaultTimeout, nil,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call postReadyFunc if extension object is ready", func() {
			passedObj := expected.DeepCopy()
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Now(),
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")

			val := 0
			err := WaitUntilExtensionObjectReady(
				ctx, c, log,
				passedObj, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultThreshold, defaultTimeout, func() error {
					val++
					return nil
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal(1))
		})

		It("should set passed object to latest state once ready", func() {
			passedObj := expected.DeepCopy()
			passedObj.SetAnnotations(map[string]string{"gardener.cloud/operation": "reconcile"})
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")

			err := WaitUntilExtensionObjectReady(
				ctx, c, log,
				passedObj, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultThreshold, defaultTimeout, nil,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(passedObj).To(Equal(expected))
		})
	})

	Describe("#WaitUntilObjectReadyWithHealthFunction", func() {
		AfterEach(func() {
			Expect(client.ObjectKeyFromObject(expected)).To(Equal(client.ObjectKey{Namespace: namespace, Name: name}), "should not reset object's key")
		})

		It("should return error if object does not exist", func() {
			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, c, log,
				func(_ client.Object) error {
					return nil
				},
				expected, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultThreshold, defaultTimeout,
				nil,
			)
			Expect(err).To(HaveOccurred())
		})

		It("should retry getting object if it does not exist in the cache yet", func() {
			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Scheme().Return(scheme).AnyTimes()

			gomock.InOrder(
				mc.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(expected)).
					Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("workers"), expected.Name)),
				mc.EXPECT().Get(gomock.Any(), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(expected)).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Worker, _ ...client.GetOption) error {
						expected.DeepCopyInto(obj)
						return nil
					}),
			)

			fakeOps.MaxAttempts = 2

			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, mc, log,
				func(_ client.Object) error {
					return nil
				},
				expected, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultThreshold, 5*defaultTimeout,
				nil,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if ready func returns error", func() {
			fakeError := &specialWrappingError{
				error: v1beta1helper.NewErrorWithCodes(errors.New("foo"), gardencorev1beta1.ErrorInfraUnauthorized),
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, c, log,
				func(_ client.Object) error {
					return fakeError
				},
				expected, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultThreshold, defaultTimeout,
				nil,
			)
			Expect(err).To(HaveOccurred())

			// ensure, that errors are properly wrapped
			var specialError interface {
				Special()
			}
			Expect(errors.As(err, &specialError)).To(BeTrue(), "should properly wrap the error returned by the health func")
			Expect(v1beta1helper.ExtractErrorCodes(err)).To(ConsistOf(gardencorev1beta1.ErrorInfraUnauthorized), "should be able to extract error codes from wrapped error")
		})

		It("should return error if client has not observed latest timestamp annotation", func() {
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Now(),
			}
			now = time.Now()
			metav1.SetMetaDataAnnotation(&expected.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Format(time.RFC3339Nano))
			passedObj := expected.DeepCopy()
			metav1.SetMetaDataAnnotation(&passedObj.ObjectMeta, v1beta1constants.GardenerTimestamp, now.Add(time.Millisecond).UTC().Format(time.RFC3339Nano))

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, c, log,
				func(_ client.Object) error {
					return nil
				},
				passedObj, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultThreshold, defaultTimeout, nil,
			)
			Expect(err).To(MatchError(ContainSubstring("annotation is not")), "worker readiness error")
		})

		It("should return success if health func does not return error and we observed latest timestamp annotation", func() {
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Now(),
			}
			now = time.Now()
			metav1.SetMetaDataAnnotation(&expected.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Format(time.RFC3339Nano))
			passedObj := expected.DeepCopy()

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, c, log,
				func(_ client.Object) error {
					return nil
				},
				passedObj, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultThreshold, defaultTimeout, nil,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass correct object to health func", func() {
			passedObj := expected.DeepCopy()
			metav1.SetMetaDataAnnotation(&passedObj.ObjectMeta, "foo", "bar")
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating worker succeeds")
			err := WaitUntilObjectReadyWithHealthFunction(
				ctx, c, log,
				func(obj client.Object) error {
					Expect(obj).To(Equal(expected))
					return nil
				},
				passedObj, extensionsv1alpha1.WorkerResource,
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
				func(_ client.Object) error {
					return nil
				},
				expected, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultThreshold, defaultTimeout, func() error {
					val++
					return nil
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal(1))
		})
	})

	Describe("#DeleteExtensionObject", func() {
		It("should not return error if extension object does not exist", func() {
			Expect(DeleteExtensionObject(ctx, c, expected)).To(Succeed())
		})

		It("should not return error if deleted successfully", func() {
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
			Expect(DeleteExtensionObject(ctx, c, expected)).To(Succeed())
		})

		It("should delete extension object", func() {
			defer test.WithVars(
				&TimeNow, mockNow.Do,
			)()

			expected.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			mc := mockclient.NewMockClient(ctrl)
			// add deletion confirmation and Timestamp annotation
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{}), gomock.Any()).SetArg(1, *expected).Return(nil)
			mc.EXPECT().Delete(ctx, expected).Times(1).Return(fmt.Errorf("some random error"))

			Expect(DeleteExtensionObject(ctx, mc, expected)).To(HaveOccurred())
		})
	})

	Describe("#DeleteExtensionObjects", func() {
		It("should delete all extension objects", func() {
			deletionTimestamp := metav1.Now()
			expected.DeletionTimestamp = &deletionTimestamp

			expected2 := expected.DeepCopy()
			expected2.Name = "worker2"
			list := &extensionsv1alpha1.WorkerList{
				Items: []extensionsv1alpha1.Worker{*expected, *expected2},
			}
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
			Expect(c.Create(ctx, expected2)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")

			err := DeleteExtensionObjects(
				ctx,
				c,
				list,
				namespace,
				func(_ extensionsv1alpha1.Object) bool { return true },
			)

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#WaitUntilExtensionObjectsDeleted", func() {
		Context("no predicate given", func() {
			It("should return error if at least one extension object is not deleted", func() {
				list := &extensionsv1alpha1.WorkerList{}

				Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")

				err := WaitUntilExtensionObjectsDeleted(
					ctx,
					c,
					log,
					list,
					extensionsv1alpha1.WorkerResource,
					namespace, defaultInterval, defaultTimeout,
					nil,
				)

				Expect(err).To(HaveOccurred())
			})

			It("should return success if all extensions CRs are deleted", func() {
				list := &extensionsv1alpha1.WorkerList{}
				err := WaitUntilExtensionObjectsDeleted(
					ctx,
					c,
					log,
					list,
					extensionsv1alpha1.WorkerResource,
					namespace, defaultInterval, defaultTimeout,
					nil,
				)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("predicate given", func() {
			It("should return error if at least one extension object is not deleted", func() {
				list := &extensionsv1alpha1.WorkerList{}

				Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")

				err := WaitUntilExtensionObjectsDeleted(
					ctx,
					c,
					log,
					list,
					extensionsv1alpha1.WorkerResource,
					namespace, defaultInterval, defaultTimeout,
					func(obj extensionsv1alpha1.Object) bool {
						return obj.GetName() == expected.GetName()
					},
				)

				Expect(err).To(HaveOccurred())
			})

			It("should return success if all extensions CRs matching the predicate are deleted", func() {
				list := &extensionsv1alpha1.WorkerList{}

				Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")

				err := WaitUntilExtensionObjectsDeleted(
					ctx,
					c,
					log,
					list,
					extensionsv1alpha1.WorkerResource,
					namespace, defaultInterval, defaultTimeout,
					func(obj extensionsv1alpha1.Object) bool {
						return obj.GetName() != expected.GetName()
					},
				)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success if all extensions CRs are deleted", func() {
				list := &extensionsv1alpha1.WorkerList{}
				err := WaitUntilExtensionObjectsDeleted(
					ctx,
					c,
					log,
					list,
					extensionsv1alpha1.WorkerResource,
					namespace, defaultInterval, defaultTimeout,
					nil,
				)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("#WaitUntilExtensionObjectDeleted", func() {
		AfterEach(func() {
			Expect(client.ObjectKeyFromObject(expected)).To(Equal(client.ObjectKey{Namespace: namespace, Name: name}), "should not reset object's key")
		})

		It("should return error if extension object is not deleted", func() {
			deletionTimestamp := metav1.Now()
			expected.DeletionTimestamp = &deletionTimestamp

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
			err := WaitUntilExtensionObjectDeleted(ctx, c, log,
				expected, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultTimeout)

			Expect(err).To(HaveOccurred())
		})

		It("should return error with codes if extension object has status.lastError.codes", func() {
			deletionTimestamp := metav1.Now()
			expected.DeletionTimestamp = &deletionTimestamp
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "invalid credentials",
				Codes:       []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraUnauthorized},
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
			err := WaitUntilExtensionObjectDeleted(ctx, c, log,
				expected, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultTimeout)

			Expect(err).To(HaveOccurred())

			// ensure, that errors are properly wrapped
			Expect(v1beta1helper.ExtractErrorCodes(err)).To(ConsistOf(gardencorev1beta1.ErrorInfraUnauthorized), "should be able to extract error codes from wrapped error")
		})

		It("should return success if extensions CRs gets deleted", func() {
			err := WaitUntilExtensionObjectDeleted(ctx, c, log,
				expected, extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultTimeout)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("restoring extension object state", func() {
		var (
			expectedState *runtime.RawExtension
			shootState    *gardencorev1beta1.ShootState
		)

		BeforeEach(func() {
			expectedState = &runtime.RawExtension{Raw: []byte(`{"data":"value"}`)}

			shootState = &gardencorev1beta1.ShootState{
				Spec: gardencorev1beta1.ShootStateSpec{
					Extensions: []gardencorev1beta1.ExtensionResourceState{
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
			It("should restore the extension object state with the provided deploy function and annotate it for restoration", func() {
				defer test.WithVars(
					&TimeNow, mockNow.Do,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				err := RestoreExtensionWithDeployFunction(
					ctx,
					c,
					shootState,
					extensionsv1alpha1.WorkerResource,
					func(ctx context.Context, _ string) (extensionsv1alpha1.Object, error) {
						Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
						return expected, nil
					},
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(expected.Annotations).To(Equal(map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRestore,
					v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
				}))
				Expect(expected.Status.State).To(Equal(expectedState))
			})

			It("should only annotate the resource with restore operation annotation if a corresponding state does not exist in the ShootState", func() {
				defer test.WithVars(
					&TimeNow, mockNow.Do,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				expected.Name = "worker2"
				err := RestoreExtensionWithDeployFunction(
					ctx,
					c,
					shootState,
					extensionsv1alpha1.WorkerResource,
					func(ctx context.Context, _ string) (extensionsv1alpha1.Object, error) {
						Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
						return expected, nil
					},
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(expected.Annotations).To(Equal(map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRestore,
					v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
				}))
				Expect(expected.Status.State).To(BeNil())
			})
		})

		Describe("#RestoreExtensionObjectState", func() {
			It("should return error if the extension object does not exist", func() {
				err := RestoreExtensionObjectState(
					ctx,
					c,
					shootState,
					expected,
					extensionsv1alpha1.WorkerResource,
				)
				Expect(err).To(HaveOccurred())
			})

			It("should update the state if the extension object exists", func() {
				defer test.WithVars(
					&TimeNow, mockNow.Do,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
				err := RestoreExtensionObjectState(
					ctx,
					c,
					shootState,
					expected,
					extensionsv1alpha1.WorkerResource,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(expected.Status.State).To(Equal(expectedState))
			})
		})
	})

	Describe("#MigrateExtensionObject", func() {
		It("should not return error if extension object does not exist", func() {
			Expect(MigrateExtensionObject(ctx, c, expected)).To(Succeed())
		})

		It("should properly annotate extension object for migration", func() {
			defer test.WithVars(
				&TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			expectedWithAnnotations := expected.DeepCopy()
			expectedWithAnnotations.Annotations = map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationMigrate,
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Patch(ctx, expectedWithAnnotations, gomock.AssignableToTypeOf(client.MergeFrom(expected))).Return(nil)

			err := MigrateExtensionObject(ctx, mc, expected)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#MigrateExtensionObjects", func() {
		It("should not return error if there are no extension resources", func() {
			Expect(
				MigrateExtensionObjects(ctx, c, &extensionsv1alpha1.BackupBucketList{}, namespace, nil),
			).To(Succeed())
		})

		It("should properly annotate all extension objects for migration", func() {
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
				MigrateExtensionObjects(ctx, c, &extensionsv1alpha1.ContainerRuntimeList{}, namespace, nil),
			).To(Succeed())

			containerRuntimeList := &extensionsv1alpha1.ContainerRuntimeList{}
			Expect(c.List(ctx, containerRuntimeList, client.InNamespace(namespace))).To(Succeed())
			Expect(containerRuntimeList.Items).To(HaveLen(4))
			for _, item := range containerRuntimeList.Items {
				Expect(item.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))
			}
		})

		It("should properly annotate only the desired extension objects for migration", func() {
			for i := 0; i < 4; i++ {
				containerRuntimeExtension := &extensionsv1alpha1.ContainerRuntime{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      fmt.Sprintf("containerruntime-%d", i),
					},
				}
				Expect(c.Create(ctx, containerRuntimeExtension)).To(Succeed(), containerRuntimeExtension.Name+" should get created")
			}

			Expect(
				MigrateExtensionObjects(ctx, c, &extensionsv1alpha1.ContainerRuntimeList{}, namespace, func(obj extensionsv1alpha1.Object) bool {
					return obj.GetName() == "containerruntime-2"
				}),
			).To(Succeed())

			containerRuntimeList := &extensionsv1alpha1.ContainerRuntimeList{}
			Expect(c.List(ctx, containerRuntimeList, client.InNamespace(namespace))).To(Succeed())
			Expect(containerRuntimeList.Items).To(HaveLen(4))
			for _, item := range containerRuntimeList.Items {
				if item.Name == "containerruntime-2" {
					Expect(item.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate), item.Name+" should have gardener.cloud/operation annotation")
				} else {
					_, ok := item.Annotations[v1beta1constants.GardenerOperation]
					Expect(ok).To(BeFalse(), item.Name+" should not have gardener.cloud/operation annotation")
				}
			}
		})
	})

	Describe("#WaitUntilExtensionObjectMigrated", func() {
		AfterEach(func() {
			Expect(client.ObjectKeyFromObject(expected)).To(Equal(client.ObjectKey{Namespace: namespace, Name: name}), "should not reset object's key")
		})

		It("should not return error if resource does not exist", func() {
			err := WaitUntilExtensionObjectMigrated(
				ctx,
				c,
				expected,
				extensionsv1alpha1.WorkerResource,
				defaultInterval, defaultTimeout,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("should return error if migration times out",
			func(lastOperation *gardencorev1beta1.LastOperation, match func() gomegatypes.GomegaMatcher) {
				expected.Status.LastOperation = lastOperation
				Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
				err := WaitUntilExtensionObjectMigrated(
					ctx,
					c,
					expected,
					extensionsv1alpha1.WorkerResource,
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

	Describe("#WaitUntilExtensionObjectsMigrated", func() {
		It("should not return error if there are no extension objects", func() {
			Expect(WaitUntilExtensionObjectsMigrated(
				ctx,
				c,
				&extensionsv1alpha1.WorkerList{},
				extensionsv1alpha1.WorkerResource,
				namespace,
				defaultInterval,
				defaultTimeout,
				nil)).To(Succeed())
		})

		DescribeTable("should return error if migration times out",
			func(lastOperations []*gardencorev1beta1.LastOperation, match func() gomegatypes.GomegaMatcher) {
				for i, lastOp := range lastOperations {
					existing := expected.DeepCopy()
					existing.Status.LastOperation = lastOp
					existing.Name = fmt.Sprintf("worker-%d", i)
					Expect(c.Create(ctx, existing)).ToNot(HaveOccurred(), "adding pre-existing worker succeeds")
				}
				err := WaitUntilExtensionObjectsMigrated(
					ctx,
					c,
					&extensionsv1alpha1.WorkerList{},
					extensionsv1alpha1.WorkerResource,
					namespace,
					defaultInterval,
					defaultTimeout,
					nil)
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

	Describe("#AnnotateObjectWithOperation", func() {
		It("should return error if object does not exist", func() {
			Expect(AnnotateObjectWithOperation(ctx, c, expected, v1beta1constants.GardenerOperationMigrate)).NotTo(Succeed())
		})

		It("should annotate extension object with operation", func() {
			defer test.WithVars(
				&TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			expectedWithAnnotations := expected.DeepCopy()
			expectedWithAnnotations.Annotations = map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationMigrate,
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Patch(ctx, expectedWithAnnotations, gomock.AssignableToTypeOf(client.MergeFrom(expected))).Return(nil)

			err := AnnotateObjectWithOperation(ctx, mc, expected, v1beta1constants.GardenerOperationMigrate)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

type specialWrappingError struct {
	error
}

func (s *specialWrappingError) Unwrap() error {
	return s.error
}

func (s *specialWrappingError) Special() {}
