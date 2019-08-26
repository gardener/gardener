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

package client_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	mockutilclient "github.com/gardener/gardener/pkg/mock/gardener/utils/kubernetes/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	mockcorev1 "github.com/gardener/gardener/pkg/mock/client-go/core/v1"
	mocktime "github.com/gardener/gardener/pkg/mock/gardener/utils/time"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/kubernetes/client"

	"sigs.k8s.io/controller-runtime/pkg/client"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}

type clientDeleteOptionFuncMatcher struct {
	options client.DeleteOptions
}

func (d *clientDeleteOptionFuncMatcher) String() string {
	return fmt.Sprintf("changes delete options to %#v", d.options)
}

func (d *clientDeleteOptionFuncMatcher) Matches(x interface{}) bool {
	f, ok := x.(client.DeleteOptionFunc)
	if !ok {
		return false
	}

	actual := client.DeleteOptions{}
	f(&actual)

	return reflect.DeepEqual(d.options, actual)
}

func ClientDeleteOptionFuncProduces(o client.DeleteOptions) gomock.Matcher {
	return &clientDeleteOptionFuncMatcher{o}
}

type clientListOptionFuncMatcher struct {
	options client.ListOptions
}

func (d *clientListOptionFuncMatcher) String() string {
	return fmt.Sprintf("changes list options to %#v", d.options)
}

func (d *clientListOptionFuncMatcher) Matches(x interface{}) bool {
	f, ok := x.(client.ListOptionFunc)
	if !ok {
		return false
	}

	actual := client.ListOptions{}
	f(&actual)

	return reflect.DeepEqual(d.options, actual)
}

func ClientListOptionFuncProduces(o client.ListOptions) gomock.Matcher {
	return &clientListOptionFuncMatcher{o}
}

type cleanOptionFuncMatcher struct {
	options CleanOptions
}

func (d *cleanOptionFuncMatcher) String() string {
	return fmt.Sprintf("changes list options to %#v", d.options)
}

func (d *cleanOptionFuncMatcher) Matches(x interface{}) bool {
	f, ok := x.(CleanOptionFunc)
	if !ok {
		return false
	}

	actual := CleanOptions{}
	f(&actual)

	return reflect.DeepEqual(d.options, actual)
}

func CleanOptionFuncProduces(o CleanOptions) gomock.Matcher {
	return &cleanOptionFuncMatcher{o}
}

var _ = Describe("Cleaner", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		cm1Key client.ObjectKey
		cm2Key client.ObjectKey
		nsKey  client.ObjectKey

		cm1    corev1.ConfigMap
		cm2    corev1.ConfigMap
		cmList corev1.ConfigMapList
		ns     corev1.Namespace
		//cmObjects []runtime.Object

		cm2WithFinalizer corev1.ConfigMap
		nsWithFinalizer  corev1.Namespace
		//cmListWithFinalizer corev1.ConfigMapList
	)
	BeforeEach(func() {
		var err error
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		cm1Key = kutil.Key("n", "foo")
		cm2Key = kutil.Key("n", "bar")
		nsKey = kutil.Key("baz")

		cm1 = corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
		cm2 = corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar"}}
		cmList = corev1.ConfigMapList{Items: []corev1.ConfigMap{cm1, cm2}}
		ns = corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "baz"}}
		//cmObjects, err = meta.ExtractList(&cmList)
		Expect(err).NotTo(HaveOccurred())

		cm2.DeepCopyInto(&cm2WithFinalizer)
		cm2WithFinalizer.Finalizers = []string{"finalize.me"}
		ns.DeepCopyInto(&nsWithFinalizer)
		nsWithFinalizer.Spec.Finalizers = []corev1.FinalizerName{"kubernetes"}
		//cmListWithFinalizer = corev1.ConfigMapList{Items: []corev1.ConfigMap{cm1, cm2WithFinalizer}}

	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Delete", func() {
		It("should delete the target object", func() {
			ctx := context.TODO()

			c.EXPECT().Delete(ctx, &cm1, ClientDeleteOptionFuncProduces(client.DeleteOptions{}))

			Expect(Delete(ctx, c, &cm1)).To(Succeed())
		})

		It("should delete all objects matching the selector", func() {
			var (
				ctx  = context.TODO()
				list = &corev1.ConfigMapList{}
			)

			gomock.InOrder(
				c.EXPECT().List(ctx, list, ClientListOptionFuncProduces(client.ListOptions{})).SetArg(1, cmList),
				c.EXPECT().Delete(ctx, &cm1, ClientDeleteOptionFuncProduces(client.DeleteOptions{})),
				c.EXPECT().Delete(ctx, &cm2, ClientDeleteOptionFuncProduces(client.DeleteOptions{})),
			)

			Expect(Delete(ctx, c, list)).To(Succeed())
		})
	})

	Context("Cleaner", func() {
		var (
			timeOps *mocktime.MockOps
		)
		BeforeEach(func() {
			timeOps = mocktime.NewMockOps(ctrl)
		})

		Describe("#Clean", func() {
			It("should delete the target object", func() {
				var (
					ctx     = context.TODO()
					cleaner = NewCleaner(timeOps, NewFinalizer())
				)

				gomock.InOrder(
					c.EXPECT().Get(ctx, cm1Key, &cm1),
					c.EXPECT().Delete(ctx, &cm1, ClientDeleteOptionFuncProduces(client.DeleteOptions{})),
				)

				Expect(cleaner.Clean(ctx, c, &cm1)).To(Succeed())
			})

			It("should delete all objects matching the selector", func() {
				var (
					ctx     = context.TODO()
					list    = &corev1.ConfigMapList{}
					cleaner = NewCleaner(timeOps, NewFinalizer())
				)

				listCall := c.EXPECT().List(ctx, list, ClientListOptionFuncProduces(client.ListOptions{})).SetArg(1, cmList)
				c.EXPECT().Delete(ctx, &cm1, ClientDeleteOptionFuncProduces(client.DeleteOptions{})).After(listCall)
				c.EXPECT().Delete(ctx, &cm2, ClientDeleteOptionFuncProduces(client.DeleteOptions{})).After(listCall)

				Expect(cleaner.Clean(ctx, c, list)).To(Succeed())
			})

			It("should finalize the object if its deletion timestamp is over the finalize grace period", func() {
				var (
					ctx               = context.TODO()
					deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
					now               = time.Unix(60, 0)
					cleaner           = NewCleaner(timeOps, NewFinalizer())
				)

				cm2WithFinalizer.DeletionTimestamp = &deletionTimestamp
				cm2.DeletionTimestamp = &deletionTimestamp

				gomock.InOrder(
					c.EXPECT().Get(ctx, cm2Key, &cm2).SetArg(2, cm2WithFinalizer),
					timeOps.EXPECT().Now().Return(now),
					c.EXPECT().Patch(ctx, &cm2, client.MergeFrom(&cm2WithFinalizer)),
				)

				Expect(cleaner.Clean(ctx, c, &cm2, FinalizeGracePeriodSeconds(20))).To(Succeed())
			})

			It("should finalize the namespace if its deletion timestamp is over the finalize grace period", func() {
				var (
					ctx               = context.TODO()
					deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
					now               = time.Unix(60, 0)
					nsInterface       = mockcorev1.NewMockNamespaceInterface(ctrl)
					finalizer         = NewNamespaceFinalizer(nsInterface)
					cleaner           = NewCleaner(timeOps, finalizer)
				)

				nsWithFinalizer.DeletionTimestamp = &deletionTimestamp
				ns.DeletionTimestamp = &deletionTimestamp

				gomock.InOrder(
					c.EXPECT().Get(ctx, nsKey, &nsWithFinalizer),
					timeOps.EXPECT().Now().Return(now),
					nsInterface.EXPECT().Finalize(&ns).Return(&ns, nil),
				)

				Expect(cleaner.Clean(ctx, c, &nsWithFinalizer, FinalizeGracePeriodSeconds(20))).To(Succeed())
			})

			It("should delete the object if its deletion timestamp is not over the finalize grace period", func() {
				var (
					ctx               = context.TODO()
					deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
					now               = time.Unix(50, 0)
					cleaner           = NewCleaner(timeOps, NewFinalizer())
				)

				cm2WithFinalizer.DeletionTimestamp = &deletionTimestamp
				cm2.DeletionTimestamp = &deletionTimestamp

				gomock.InOrder(
					c.EXPECT().Get(ctx, cm2Key, &cm2).SetArg(2, cm2WithFinalizer),
					timeOps.EXPECT().Now().Return(now),
					c.EXPECT().Delete(ctx, &cm2, ClientDeleteOptionFuncProduces(client.DeleteOptions{})),
				)

				Expect(cleaner.Clean(ctx, c, &cm2, FinalizeGracePeriodSeconds(20))).To(Succeed())
			})

			It("should delete the object if its deletion timestamp is over the finalize grace period and no finalizer is left", func() {
				var (
					ctx               = context.TODO()
					deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
					now               = time.Unix(50, 0)
					cleaner           = NewCleaner(timeOps, NewFinalizer())
				)

				cm2WithFinalizer.DeletionTimestamp = &deletionTimestamp
				cm2.DeletionTimestamp = &deletionTimestamp

				gomock.InOrder(
					c.EXPECT().Get(ctx, cm2Key, &cm2),
					timeOps.EXPECT().Now().Return(now),
					c.EXPECT().Delete(ctx, &cm2, ClientDeleteOptionFuncProduces(client.DeleteOptions{})),
				)

				Expect(cleaner.Clean(ctx, c, &cm2, FinalizeGracePeriodSeconds(10))).To(Succeed())
			})

			It("should finalize the list if the object's deletion timestamps are over the finalize grace period", func() {
				var (
					ctx               = context.TODO()
					deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
					now               = time.Unix(60, 0)
					list              = &corev1.ConfigMapList{}
					cleaner           = NewCleaner(timeOps, NewFinalizer())
				)

				cm2WithFinalizer.DeletionTimestamp = &deletionTimestamp
				cm2.DeletionTimestamp = &deletionTimestamp

				gomock.InOrder(
					c.EXPECT().List(ctx, list, ClientListOptionFuncProduces(client.ListOptions{})).SetArg(1, corev1.ConfigMapList{Items: []corev1.ConfigMap{cm2WithFinalizer}}),
					timeOps.EXPECT().Now().Return(now),
					c.EXPECT().Patch(ctx, &cm2, client.MergeFrom(&cm2WithFinalizer)),
				)

				Expect(cleaner.Clean(ctx, c, list, FinalizeGracePeriodSeconds(20))).To(Succeed())
			})

			It("should delete the list if the object's deletion timestamps are not over the finalize grace period", func() {
				var (
					ctx               = context.TODO()
					deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
					now               = time.Unix(50, 0)
					list              = &corev1.ConfigMapList{}
					cleaner           = NewCleaner(timeOps, NewFinalizer())
				)

				cm2WithFinalizer.DeletionTimestamp = &deletionTimestamp
				cm2.DeletionTimestamp = &deletionTimestamp

				gomock.InOrder(
					c.EXPECT().List(ctx, list, ClientListOptionFuncProduces(client.ListOptions{})).SetArg(1, corev1.ConfigMapList{Items: []corev1.ConfigMap{cm2WithFinalizer}}),
					timeOps.EXPECT().Now().Return(now),
					c.EXPECT().Delete(ctx, &cm2WithFinalizer, ClientDeleteOptionFuncProduces(client.DeleteOptions{})),
				)

				Expect(cleaner.Clean(ctx, c, list, FinalizeGracePeriodSeconds(20))).To(Succeed())
			})

			It("should delete the list if the object's deletion timestamps is over the finalize grace period and no finalizers are left", func() {
				var (
					ctx               = context.TODO()
					deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
					now               = time.Unix(50, 0)
					list              = &corev1.ConfigMapList{}
					cleaner           = NewCleaner(timeOps, NewFinalizer())
				)

				cm2WithFinalizer.DeletionTimestamp = &deletionTimestamp
				cm2.DeletionTimestamp = &deletionTimestamp

				gomock.InOrder(
					c.EXPECT().List(ctx, list, ClientListOptionFuncProduces(client.ListOptions{})).SetArg(1, corev1.ConfigMapList{Items: []corev1.ConfigMap{cm2}}),
					timeOps.EXPECT().Now().Return(now),
					c.EXPECT().Delete(ctx, &cm2, ClientDeleteOptionFuncProduces(client.DeleteOptions{})),
				)

				Expect(cleaner.Clean(ctx, c, list, FinalizeGracePeriodSeconds(10))).To(Succeed())
			})
		})
	})

	Describe("#EnsureGone", func() {
		It("should ensure that the object is gone", func() {
			ctx := context.TODO()

			c.EXPECT().Get(ctx, cm1Key, &cm1).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			Expect(EnsureGone(ctx, c, &cm1)).To(Succeed())
		})

		It("should ensure that the list is gone", func() {
			var (
				ctx  = context.TODO()
				list = corev1.ConfigMapList{}
			)

			c.EXPECT().List(ctx, &list)

			Expect(EnsureGone(ctx, c, &list)).To(Succeed())
		})

		It("should error that the object is still present", func() {
			ctx := context.TODO()

			c.EXPECT().Get(ctx, cm1Key, &cm1)

			Expect(EnsureGone(ctx, c, &cm1)).To(Equal(NewObjectsRemaining(&cm1)))
		})

		It("should error that the list is non-empty", func() {
			var (
				ctx  = context.TODO()
				list = corev1.ConfigMapList{}
			)

			c.EXPECT().List(ctx, &list).SetArg(1, cmList)

			Expect(EnsureGone(ctx, c, &list)).To(Equal(NewObjectsRemaining(&cmList)))
		})
	})

	Context("#CleanOps", func() {
		var (
			cleaner *mockutilclient.MockCleaner
			ensurer *mockutilclient.MockGoneEnsurer
			o       CleanOps
		)
		BeforeEach(func() {
			cleaner = mockutilclient.NewMockCleaner(ctrl)
			ensurer = mockutilclient.NewMockGoneEnsurer(ctrl)
			o = NewCleanOps(cleaner, ensurer)
		})

		Describe("CleanAndEnsureGone", func() {
			It("should clean and ensure that the object is gone", func() {
				ctx := context.TODO()

				gomock.InOrder(
					cleaner.EXPECT().Clean(ctx, c, &cm1, CleanOptionFuncProduces(CleanOptions{})),
					ensurer.EXPECT().EnsureGone(ctx, c, &cm1, ClientListOptionFuncProduces(client.ListOptions{})),
				)

				Expect(o.CleanAndEnsureGone(ctx, c, &cm1))
			})
		})
	})
})
