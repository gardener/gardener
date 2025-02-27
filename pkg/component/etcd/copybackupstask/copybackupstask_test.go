// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package copybackupstask_test

import (
	"context"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/component/etcd/copybackupstask"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("CopyBackupsTask", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
		c    *mockclient.MockClient
		log  logr.Logger

		expected            *druidv1alpha1.EtcdCopyBackupsTask
		values              *Values
		etcdCopyBackupsTask Interface

		notFoundErr = apierrors.NewNotFound(schema.GroupResource{}, "etcdcopybackupstask")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()
		c = mockclient.NewMockClient(ctrl)
		log = logr.Discard()

		expected = &druidv1alpha1.EtcdCopyBackupsTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bar",
				Namespace: "shoot--foo--bar",
			},
			Spec: druidv1alpha1.EtcdCopyBackupsTaskSpec{
				SourceStore: druidv1alpha1.StoreSpec{},
				TargetStore: druidv1alpha1.StoreSpec{},
			},
		}

		values = &Values{
			Name:      expected.Name,
			Namespace: expected.Namespace,
		}
		etcdCopyBackupsTask = New(log, c, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should create the EtcdCopyBackupsTask", func() {
			c.EXPECT().Create(ctx, expected)
			Expect(etcdCopyBackupsTask.Deploy(ctx)).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should not return error if EtcdCopyBackupsTask resource doesn't exist", func() {
			c.EXPECT().Delete(ctx, expected).Return(notFoundErr)
			Expect(etcdCopyBackupsTask.Destroy(ctx)).To(Succeed())
		})

		It("should properly delete EtcdCopyBackupsTask resource if it exissts", func() {
			c.EXPECT().Delete(ctx, expected)
			Expect(etcdCopyBackupsTask.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		var (
			timeoutCtx context.Context
			cancelFunc context.CancelFunc
		)

		BeforeEach(func() {
			timeoutCtx, cancelFunc = context.WithTimeout(ctx, time.Millisecond)
		})

		AfterEach(func() {
			cancelFunc()
		})

		It("should return error if EtcdCopyBackupsTask resource is not found", func() {
			c.EXPECT().Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), expected).Return(notFoundErr).AnyTimes()
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(BeNotFoundError())
		})

		It("should return error if observed generation is nil", func() {
			c.EXPECT().Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), expected).AnyTimes()
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(MatchError(ContainSubstring("observed generation not recorded")))
		})

		It("should return error if observed generation is not updated", func() {
			c.EXPECT().
				Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(expected)).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, etcdCopyBackupsTask *druidv1alpha1.EtcdCopyBackupsTask, _ ...client.GetOption) error {
					etcdCopyBackupsTask.Generation = 1
					etcdCopyBackupsTask.Status.ObservedGeneration = ptr.To[int64](0)
					return nil
				}).AnyTimes()
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(MatchError(ContainSubstring("observed generation outdated (0/1)")))
		})

		It("should return error if EtcdCopyBackupsTask reconciliation encountered error", func() {
			errorText := "some error"
			c.EXPECT().
				Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(expected)).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, etcdCopyBackupsTask *druidv1alpha1.EtcdCopyBackupsTask, _ ...client.GetOption) error {
					etcdCopyBackupsTask.Status.ObservedGeneration = &expected.Generation
					etcdCopyBackupsTask.Status.LastError = &errorText
					return nil
				}).AnyTimes()
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(MatchError(ContainSubstring(errorText)))
		})

		It("should return error if expected Successful or Failed conditions are not added yet", func() {
			c.EXPECT().
				Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(expected)).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, etcdCopyBackupsTask *druidv1alpha1.EtcdCopyBackupsTask, _ ...client.GetOption) error {
					etcdCopyBackupsTask.Status.ObservedGeneration = &expected.Generation
					return nil
				}).AnyTimes()
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(MatchError(ContainSubstring("expected condition")))
		})

		It("should return error if Failed condition with status True has been added", func() {
			c.EXPECT().
				Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), expected).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, etcdCopyBackupsTask *druidv1alpha1.EtcdCopyBackupsTask, _ ...client.GetOption) error {
					etcdCopyBackupsTask.Status.ObservedGeneration = &expected.Generation
					etcdCopyBackupsTask.Status.Conditions = []druidv1alpha1.Condition{
						{
							Type:    druidv1alpha1.EtcdCopyBackupsTaskFailed,
							Status:  druidv1alpha1.ConditionTrue,
							Reason:  "reason",
							Message: "message",
						},
					}
					return nil
				}).AnyTimes()
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(HaveOccurred())
		})

		It("should be successful if Successful condition with status True has been added", func() {
			c.EXPECT().
				Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), expected).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, etcdCopyBackupsTask *druidv1alpha1.EtcdCopyBackupsTask, _ ...client.GetOption) error {
					etcdCopyBackupsTask.Status.ObservedGeneration = &expected.Generation
					etcdCopyBackupsTask.Status.Conditions = []druidv1alpha1.Condition{
						{
							Type:    druidv1alpha1.EtcdCopyBackupsTaskSucceeded,
							Status:  druidv1alpha1.ConditionTrue,
							Reason:  "reason",
							Message: "message",
						},
					}
					return nil
				}).AnyTimes()
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(Succeed())
		})

		It("should eventually return success when Successful condition is reported with status True", func() {
			gomock.InOrder(
				c.EXPECT().Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTask{})).Return(notFoundErr),
				c.EXPECT().
					Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTask{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, etcdCopyBackupsTask *druidv1alpha1.EtcdCopyBackupsTask, _ ...client.GetOption) error {
						etcdCopyBackupsTask.Generation = 1
						etcdCopyBackupsTask.Status.ObservedGeneration = ptr.To[int64](0)
						return nil
					}),
				c.EXPECT().
					Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTask{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, etcdCopyBackupsTask *druidv1alpha1.EtcdCopyBackupsTask, _ ...client.GetOption) error {
						etcdCopyBackupsTask.Generation = 1
						etcdCopyBackupsTask.Status.ObservedGeneration = ptr.To[int64](1)
						return nil
					}),
				c.EXPECT().
					Get(gomock.AssignableToTypeOf(timeoutCtx), client.ObjectKeyFromObject(expected), gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTask{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, etcdCopyBackupsTask *druidv1alpha1.EtcdCopyBackupsTask, _ ...client.GetOption) error {
						etcdCopyBackupsTask.Generation = 1
						etcdCopyBackupsTask.Status.ObservedGeneration = ptr.To[int64](1)
						etcdCopyBackupsTask.Status.Conditions = []druidv1alpha1.Condition{
							{
								Type:    druidv1alpha1.EtcdCopyBackupsTaskSucceeded,
								Status:  druidv1alpha1.ConditionTrue,
								Reason:  "reason",
								Message: "message",
							},
						}
						return nil
					}),
			)
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(Succeed())
		})

	})

	Describe("#WaitCleanup", func() {
		It("should be successful if EtcdCopyBackupsTask resource does not exist", func() {
			c.EXPECT().Get(gomock.Any(), gomock.AssignableToTypeOf(client.ObjectKey{}), expected).Return(notFoundErr)
			Expect(etcdCopyBackupsTask.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error if EtcdCopyBackupsTask resource does not get deleted", func() {
			c.EXPECT().Get(gomock.Any(), gomock.AssignableToTypeOf(client.ObjectKey{}), expected).AnyTimes()
			Expect(etcdCopyBackupsTask.WaitCleanup(ctx)).To(HaveOccurred())
		})

		It("should be successful when EtcdCopyBackupsTask gets deleted", func() {
			gomock.InOrder(
				c.EXPECT().Get(gomock.Any(), gomock.AssignableToTypeOf(client.ObjectKey{}), expected).Times(3),
				c.EXPECT().Get(gomock.Any(), gomock.AssignableToTypeOf(client.ObjectKey{}), expected).Return(notFoundErr),
			)
			Expect(etcdCopyBackupsTask.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
