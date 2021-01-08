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

package etcd_test

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane/etcd"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Waiter", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake error")
		namespace = "shoot--foo--bar"

		interval        = time.Second / 20
		severeThreshold = time.Second / 10
		timeout         = time.Second / 5
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#WaitUntilEtcdsReady", func() {
		It("should return an error when the listing fail", func() {
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{}), gomock.AssignableToTypeOf(client.InNamespace(namespace)), client.MatchingLabels{"gardener.cloud/role": "controlplane"}).Return(fakeErr)

			Expect(WaitUntilEtcdsReady(ctx, c, logger.NewFieldLogger(logger.NewNopLogger(), "", ""), namespace, 0, interval, severeThreshold, timeout)).To(MatchError(fakeErr))
		})

		It("should return an error when not all required etcds are created", func() {
			etcdList := &druidv1alpha1.EtcdList{}

			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{}), gomock.AssignableToTypeOf(client.InNamespace(namespace)), client.MatchingLabels{"gardener.cloud/role": "controlplane"}).DoAndReturn(func(ctx context.Context, obj runtime.Object, opts ...client.ListOption) error {
				etcdList.DeepCopyInto(obj.(*druidv1alpha1.EtcdList))
				return nil
			}).AnyTimes()

			Expect(WaitUntilEtcdsReady(ctx, c, logger.NewFieldLogger(logger.NewNopLogger(), "", ""), namespace, 1, interval, severeThreshold, timeout)).To(MatchError(ContainSubstring("etcd resources found")))
		})

		It("should wait until all etcds are ready", func() {
			etcdList := &druidv1alpha1.EtcdList{
				Items: []druidv1alpha1.Etcd{
					{
						Status: druidv1alpha1.EtcdStatus{
							ObservedGeneration: pointer.Int64Ptr(0),
							Ready:              pointer.BoolPtr(true),
						},
					},
					{
						Status: druidv1alpha1.EtcdStatus{
							ObservedGeneration: pointer.Int64Ptr(0),
							Ready:              pointer.BoolPtr(true),
						},
					},
				},
			}

			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{}), gomock.AssignableToTypeOf(client.InNamespace(namespace)), client.MatchingLabels{"gardener.cloud/role": "controlplane"}).DoAndReturn(func(ctx context.Context, obj runtime.Object, opts ...client.ListOption) error {
				etcdList.DeepCopyInto(obj.(*druidv1alpha1.EtcdList))
				return nil
			}).AnyTimes()

			Expect(WaitUntilEtcdsReady(ctx, c, logger.NewFieldLogger(logger.NewNopLogger(), "", ""), namespace, len(etcdList.Items), interval, severeThreshold, timeout)).To(Succeed())
		})

		It("should fail when an etcd has a last error", func() {
			var (
				errorMsg = "foo"
				etcdList = &druidv1alpha1.EtcdList{
					Items: []druidv1alpha1.Etcd{
						{
							Status: druidv1alpha1.EtcdStatus{
								LastError: pointer.StringPtr(errorMsg),
							},
						},
					},
				}
			)

			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{}), gomock.AssignableToTypeOf(client.InNamespace(namespace)), client.MatchingLabels{"gardener.cloud/role": "controlplane"}).DoAndReturn(func(ctx context.Context, obj runtime.Object, opts ...client.ListOption) error {
				etcdList.DeepCopyInto(obj.(*druidv1alpha1.EtcdList))
				return nil
			}).AnyTimes()

			Expect(WaitUntilEtcdsReady(ctx, c, logger.NewFieldLogger(logger.NewNopLogger(), "", ""), namespace, len(etcdList.Items), interval, severeThreshold, timeout)).To(MatchError(ContainSubstring("reconciliation errored: " + errorMsg)))
		})

		It("should fail when etcds have unexpected properties", func() {
			etcdList := &druidv1alpha1.EtcdList{
				Items: []druidv1alpha1.Etcd{
					{
						ObjectMeta: metav1.ObjectMeta{
							DeletionTimestamp: &metav1.Time{},
						},
					},
					{},
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"gardener.cloud/operation": "reconcile",
							},
						},
						Status: druidv1alpha1.EtcdStatus{
							ObservedGeneration: pointer.Int64Ptr(0),
						},
					},
					{
						Status: druidv1alpha1.EtcdStatus{
							ObservedGeneration: pointer.Int64Ptr(0),
							Ready:              pointer.BoolPtr(false),
						},
					},
				},
			}

			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{}), gomock.AssignableToTypeOf(client.InNamespace(namespace)), client.MatchingLabels{"gardener.cloud/role": "controlplane"}).DoAndReturn(func(ctx context.Context, obj runtime.Object, opts ...client.ListOption) error {
				etcdList.DeepCopyInto(obj.(*druidv1alpha1.EtcdList))
				return nil
			}).AnyTimes()

			Expect(WaitUntilEtcdsReady(ctx, c, logger.NewFieldLogger(logger.NewNopLogger(), "", ""), namespace, len(etcdList.Items), interval, severeThreshold, timeout)).To(MatchError(SatisfyAll(
				ContainSubstring("unexpectedly has a deletion timestamp"),
				ContainSubstring("reconciliation pending"),
				ContainSubstring("reconciliation in process"),
				ContainSubstring("not ready yet"),
			)))
		})
	})
})
