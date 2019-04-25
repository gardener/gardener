// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"time"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Cleaner", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		cm1       corev1.ConfigMap
		cm2       corev1.ConfigMap
		cmList    corev1.ConfigMapList
		cmObjects []runtime.Object

		cm2WithFinalizer    corev1.ConfigMap
		cmListWithFinalizer corev1.ConfigMapList
	)
	BeforeEach(func() {
		var err error
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		cm1 = corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
		cm2 = corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar"}}
		cmList = corev1.ConfigMapList{Items: []corev1.ConfigMap{cm1, cm2}}
		cmObjects, err = meta.ExtractList(&cmList)
		Expect(err).NotTo(HaveOccurred())

		cm2.DeepCopyInto(&cm2WithFinalizer)
		cm2WithFinalizer.Finalizers = []string{"finalize.me"}
		cmListWithFinalizer = corev1.ConfigMapList{Items: []corev1.ConfigMap{cm1, cm2WithFinalizer}}
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeleteAll", func() {
		It("should delete all objects in the given list and ignore not found errors", func() {
			ctx := context.TODO()

			c.EXPECT().Delete(ctx, &cm1)
			c.EXPECT().Delete(ctx, &cm2).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

			Expect(DeleteAll(ctx, c, &cmList)).To(Succeed())
		})
	})

	Describe("#FinalizeAll", func() {
		It("should update the finalizers of all objects, if necessary", func() {
			ctx := context.TODO()

			c.EXPECT().Update(ctx, &cm2)

			Expect(FinalizeAll(ctx, c, &cmListWithFinalizer)).To(Succeed())
		})
	})

	Describe("#CheckObjectsRemaining", func() {
		It("should error if the list is non-empty", func() {
			Expect(CheckObjectsRemaining(&cmList)).To(HaveOccurred())
		})

		It("should not error if the list is empty", func() {
			Expect(CheckObjectsRemaining(&corev1.ConfigMapList{})).To(Succeed())
		})
	})

	Describe("#CheckObjectsRemainingMatching", func() {
		It("should fetch the items and succeed if there are no items", func() {
			var (
				ctx  = context.TODO()
				opts *client.ListOptions
				list = &corev1.ConfigMapList{}
			)

			c.EXPECT().List(ctx, opts, list)

			Expect(CheckObjectsRemainingMatching(ctx, c, opts, list)).To(Succeed())
		})

		It("should fetch the items and fail if there are items left", func() {
			var (
				ctx  = context.TODO()
				opts *client.ListOptions
				list = &corev1.ConfigMapList{}
			)

			c.EXPECT().List(ctx, opts, list).SetArg(2, cmList)

			err := CheckObjectsRemainingMatching(ctx, c, opts, list)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(NewObjectsRemaining(cmObjects)))
		})
	})

	Describe("#DeleteMatching", func() {
		It("should delete the matching items", func() {
			var (
				ctx  = context.TODO()
				opts *client.ListOptions
				list = &corev1.ConfigMapList{}
			)

			listCall := c.EXPECT().List(ctx, opts, list).SetArg(2, cmList)
			c.EXPECT().Delete(ctx, &cm1).After(listCall)
			c.EXPECT().Delete(ctx, &cm2).After(listCall)

			Expect(DeleteMatching(ctx, c, opts, list, false)).To(Succeed())
		})

		It("should finalize and delete the matching items", func() {
			var (
				ctx  = context.TODO()
				opts *client.ListOptions
				list = &corev1.ConfigMapList{}
			)

			listCall := c.EXPECT().List(ctx, opts, list).SetArg(2, cmListWithFinalizer)
			updateCall := c.EXPECT().Update(ctx, &cm2).After(listCall)
			c.EXPECT().Delete(ctx, &cm1).After(updateCall)
			c.EXPECT().Delete(ctx, &cm2).After(updateCall)

			Expect(DeleteMatching(ctx, c, opts, list, true)).To(Succeed())
		})
	})

	Describe("#CleanMatching", func() {
		It("should delete the matching items and then check whether something is left", func() {
			var (
				ctx  = context.TODO()
				opts *client.ListOptions
				list = &corev1.ConfigMapList{}
			)

			listCall := c.EXPECT().List(ctx, opts, list).SetArg(2, cmList)

			deleteCall1 := c.EXPECT().Delete(ctx, &cm1).After(listCall)
			deleteCall2 := c.EXPECT().Delete(ctx, &cm2).After(listCall)

			c.EXPECT().List(ctx, opts, &cmList).SetArg(2, corev1.ConfigMapList{}).After(deleteCall1).After(deleteCall2)

			Expect(CleanMatching(ctx, c, opts, opts, list, false)).To(Succeed())
		})

		It("should delete the matching items and fail if something is left", func() {
			var (
				ctx  = context.TODO()
				opts *client.ListOptions
				list = &corev1.ConfigMapList{}
			)

			listCall := c.EXPECT().List(ctx, opts, list).SetArg(2, cmList)

			deleteCall1 := c.EXPECT().Delete(ctx, &cm1).After(listCall)
			deleteCall2 := c.EXPECT().Delete(ctx, &cm2).After(listCall)

			c.EXPECT().List(ctx, opts, &cmList).After(deleteCall1).After(deleteCall2)

			err := CleanMatching(ctx, c, opts, opts, list, false)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(NewObjectsRemaining(cmObjects)))
		})
	})

	Describe("#RetryCleanMatchingUntil", func() {
		It("should retry cleaning until there are no resources left", func() {
			var (
				ctx        = context.TODO()
				deleteOpts *client.ListOptions
				checkOpts  *client.ListOptions
				list       = corev1.ConfigMapList{}
			)

			listCall1 := c.EXPECT().List(ctx, deleteOpts, &list).SetArg(2, cmList)

			deleteCall1 := c.EXPECT().Delete(ctx, &cm1).After(listCall1)
			deleteCall2 := c.EXPECT().Delete(ctx, &cm2).After(listCall1)

			listCall2 := c.EXPECT().List(ctx, checkOpts, &cmList).After(deleteCall1).After(deleteCall2)

			listCall3 := c.EXPECT().List(ctx, deleteOpts, &cmList).After(listCall2)

			deleteCall3 := c.EXPECT().Delete(ctx, &cm1).After(listCall3)
			deleteCall4 := c.EXPECT().Delete(ctx, &cm2).After(listCall3)

			c.EXPECT().List(ctx, checkOpts, &list).SetArg(2, list).After(deleteCall3).After(deleteCall4)

			Expect(RetryCleanMatchingUntil(ctx, 0*time.Second, c, deleteOpts, checkOpts, &list, false)).To(Succeed())
		})
	})
})
