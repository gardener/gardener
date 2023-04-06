// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	mockutilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client/mock"
	"github.com/gardener/gardener/pkg/utils/test"
	mocktime "github.com/gardener/gardener/pkg/utils/time/mock"
)

func TestClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}

var _ = Describe("Cleaner", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		ctx  context.Context

		cm1Key client.ObjectKey
		cm2Key client.ObjectKey
		nsKey  client.ObjectKey

		cm1    corev1.ConfigMap
		cm2    corev1.ConfigMap
		cmList corev1.ConfigMapList
		ns     corev1.Namespace

		cm2WithFinalizer corev1.ConfigMap
		nsWithFinalizer  corev1.Namespace

		timeOps *mocktime.MockOps
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		ctx = context.Background()

		cm1Key = kubernetesutils.Key("n", "foo")
		cm2Key = kubernetesutils.Key("n", "bar")
		nsKey = kubernetesutils.Key("baz")

		cm1 = corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
		cm2 = corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar"}}
		cmList = corev1.ConfigMapList{Items: []corev1.ConfigMap{cm1, cm2}}
		ns = corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "baz"}}

		cm2.DeepCopyInto(&cm2WithFinalizer)
		cm2WithFinalizer.Finalizers = []string{"finalize.me"}
		ns.DeepCopyInto(&nsWithFinalizer)
		nsWithFinalizer.Spec.Finalizers = []corev1.FinalizerName{"kubernetes"}

		timeOps = mocktime.NewMockOps(ctrl)
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Context("Cleaner", func() {
		Describe("#Clean", func() {
			It("should delete the target object", func() {
				var (
					ctx     = context.TODO()
					cleaner = NewCleaner(timeOps, NewFinalizer())
				)

				gomock.InOrder(
					c.EXPECT().Get(ctx, cm1Key, &cm1),
					c.EXPECT().Delete(ctx, &cm1),
				)

				Expect(cleaner.Clean(ctx, c, &cm1)).To(Succeed())
			})

			It("should succeed if not found error occurs for target object", func() {
				var (
					ctx     = context.TODO()
					cleaner = NewCleaner(timeOps, NewFinalizer())
				)

				c.EXPECT().Get(ctx, cm1Key, &cm1).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

				Expect(cleaner.Clean(ctx, c, &cm1)).To(Succeed())
			})

			It("should succeed if no match error occurs for target object", func() {
				var (
					ctx     = context.TODO()
					cleaner = NewCleaner(timeOps, NewFinalizer())
				)

				c.EXPECT().Get(ctx, cm1Key, &cm1).Return(&meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}})

				Expect(cleaner.Clean(ctx, c, &cm1)).To(Succeed())
			})

			It("should delete all objects matching the selector", func() {
				var (
					ctx     = context.TODO()
					list    = &corev1.ConfigMapList{}
					cleaner = NewCleaner(timeOps, NewFinalizer())
				)

				listCall := c.EXPECT().List(ctx, list).SetArg(1, cmList)
				c.EXPECT().Delete(ctx, &cm1).After(listCall)
				c.EXPECT().Delete(ctx, &cm2).After(listCall)

				Expect(cleaner.Clean(ctx, c, list)).To(Succeed())
			})

			It("should succeed if not found error occurs for list type", func() {
				var (
					ctx     = context.TODO()
					list    = &corev1.ConfigMapList{}
					cleaner = NewCleaner(timeOps, NewFinalizer())
				)

				c.EXPECT().List(ctx, list).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

				Expect(cleaner.Clean(ctx, c, list)).To(Succeed())
			})

			It("should succeed if no match error occurs for list type", func() {
				var (
					ctx     = context.TODO()
					list    = &corev1.ConfigMapList{}
					cleaner = NewCleaner(timeOps, NewFinalizer())
				)

				c.EXPECT().List(ctx, list).Return(&meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}})

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
					test.EXPECTPatch(ctx, c, &cm2, &cm2WithFinalizer, types.MergePatchType),
				)

				Expect(cleaner.Clean(ctx, c, &cm2, FinalizeGracePeriodSeconds(20))).To(Succeed())
			})

			It("should finalize the namespace if its deletion timestamp is over the finalize grace period", func() {
				var (
					ctx               = context.TODO()
					deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
					now               = time.Unix(60, 0)
					sw                = mockclient.NewMockSubResourceClient(ctrl)
					finalizer         = NewNamespaceFinalizer()
					cleaner           = NewCleaner(timeOps, finalizer)
				)

				nsWithFinalizer.DeletionTimestamp = &deletionTimestamp
				ns.DeletionTimestamp = &deletionTimestamp

				gomock.InOrder(
					c.EXPECT().Get(ctx, nsKey, &nsWithFinalizer),
					timeOps.EXPECT().Now().Return(now),
					c.EXPECT().SubResource("finalize").Return(sw),
					sw.EXPECT().Update(ctx, &ns).Return(nil),
				)

				Expect(cleaner.Clean(ctx, c, &nsWithFinalizer, FinalizeGracePeriodSeconds(20))).To(Succeed())
			})

			It("should not delete the object if its deletion timestamp is not over the finalize grace period", func() {
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
				)

				Expect(cleaner.Clean(ctx, c, &cm2, FinalizeGracePeriodSeconds(20))).To(Succeed())
			})

			It("should not delete the object if its deletion timestamp is over the finalize grace period and no finalizer is left", func() {
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
					c.EXPECT().List(ctx, list).SetArg(1, corev1.ConfigMapList{Items: []corev1.ConfigMap{cm2WithFinalizer}}),
					timeOps.EXPECT().Now().Return(now),
					test.EXPECTPatch(ctx, c, &cm2, &cm2WithFinalizer, types.MergePatchType),
				)

				Expect(cleaner.Clean(ctx, c, list, FinalizeGracePeriodSeconds(20))).To(Succeed())
			})

			It("should not delete the list if the object's deletion timestamp is not over the finalize grace period", func() {
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
					c.EXPECT().List(ctx, list).SetArg(1, corev1.ConfigMapList{Items: []corev1.ConfigMap{cm2WithFinalizer}}),
					timeOps.EXPECT().Now().Return(now),
				)

				Expect(cleaner.Clean(ctx, c, list, FinalizeGracePeriodSeconds(20))).To(Succeed())
			})

			It("should not delete the list if the object's deletion timestamp is over the finalize grace period and no finalizers are left", func() {
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
					c.EXPECT().List(ctx, list).SetArg(1, corev1.ConfigMapList{Items: []corev1.ConfigMap{cm2}}),
					timeOps.EXPECT().Now().Return(now),
				)

				Expect(cleaner.Clean(ctx, c, list, FinalizeGracePeriodSeconds(10))).To(Succeed())
			})

			It("should ensure that no error occurs because resource is not present in the cluster", func() {
				var (
					ctx     = context.TODO()
					list    = &corev1.ConfigMapList{}
					cleaner = NewCleaner(timeOps, NewFinalizer())
				)

				c.EXPECT().List(ctx, list).DoAndReturn(func(_ context.Context, _ *corev1.ConfigMapList, _ ...client.ListOption) error {
					return &meta.NoResourceMatchError{}
				})

				Expect(cleaner.Clean(ctx, c, list, FinalizeGracePeriodSeconds(10))).To(Succeed())
			})
		})
	})

	Describe("VolumeSnapshotCleaner", func() {
		var (
			cl       client.Client
			cleaner  Cleaner
			labels   map[string]string
			cleanOps []CleanOption

			deletionTimestamp                metav1.Time
			cleanupContent, remainingContent map[string]*volumesnapshotv1.VolumeSnapshotContent
		)

		BeforeEach(func() {
			var (
				deletionTimestampLater = metav1.NewTime(deletionTimestamp.Add(-1 * time.Second))
				now                    = time.Unix(60, 0)
				finalizers             = []string{"foo/bar"}
			)

			deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
			timeOps.EXPECT().Now().Return(now).AnyTimes()

			cleaner = NewVolumeSnapshotContentCleaner(timeOps)
			labels = map[string]string{"action": "cleanup"}
			cleanOps = []CleanOption{
				utilclient.ListWith{
					client.MatchingLabels(labels),
				},
				utilclient.DeleteWith{
					client.GracePeriodSeconds(29),
				},
			}

			cleanupContent = map[string]*volumesnapshotv1.VolumeSnapshotContent{
				"content1": {
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &deletionTimestamp,
						Finalizers:        finalizers,
						Name:              "content1",
						Namespace:         "default",
						Labels:            labels,
					},
				},
				"content2": {
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &deletionTimestamp,
						Finalizers:        finalizers,
						Name:              "content2",
						Namespace:         "default",
						Annotations: map[string]string{
							"snapshot.storage.kubernetes.io/volumesnapshot-being-deleted": "yes",
							"snapshot.storage.kubernetes.io/volumesnapshot-being-created": "yes",
						},
						Labels: labels,
					},
				},
				"content3": {
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &deletionTimestamp,
						Finalizers:        finalizers,
						Name:              "content3",
						Namespace:         "default",
						Annotations: map[string]string{
							"snapshot.storage.kubernetes.io/volumesnapshot-being-created": "yes",
						},
						Labels: labels,
					},
				},
			}

			remainingContent = map[string]*volumesnapshotv1.VolumeSnapshotContent{
				// Object not in deletion.
				"content4": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "content4",
						Namespace: "default",
						Annotations: map[string]string{
							"snapshot.storage.kubernetes.io/volumesnapshot-being-created": "yes",
						},
						Labels: labels,
					},
				},
				// Object w/o matching label.
				"content5": {
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &deletionTimestamp,
						Finalizers:        finalizers,
						Name:              "content5",
						Namespace:         "default",
					},
				},
				// Object w/ deletionTimestamp before grace period passed.
				"content6": {
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &deletionTimestampLater,
						Finalizers:        finalizers,
						Name:              "content6",
						Namespace:         "default",
					},
				},
			}

			fakeClientBuilder := fakeclient.NewClientBuilder()
			for _, content := range cleanupContent {
				obj := content
				fakeClientBuilder.WithObjects(obj)
			}

			for _, content := range remainingContent {
				obj := content
				fakeClientBuilder.WithObjects(obj)
			}

			cl = fakeClientBuilder.WithScheme(kubernetes.ShootScheme).Build()
		})

		It("should maintain the right annotations for all contents in the list to be cleaned up", func() {
			Expect(cleaner.Clean(ctx, cl, &volumesnapshotv1.VolumeSnapshotContentList{}, cleanOps...)).To(Succeed())

			contents := &volumesnapshotv1.VolumeSnapshotContentList{}
			Expect(cl.List(ctx, contents)).To(Succeed())

			for _, content := range contents.Items {
				if _, ok := cleanupContent[content.Name]; ok {
					Expect(content.Annotations).To(HaveKeyWithValue("snapshot.storage.kubernetes.io/volumesnapshot-being-deleted", "yes"))
					Expect(content.Annotations).NotTo(HaveKeyWithValue("snapshot.storage.kubernetes.io/volumesnapshot-being-created", "yes"))
					continue
				}
				expected := remainingContent[content.Name]
				Expect(expected).NotTo(BeNil())
				Expect(expected.Annotations).To(Equal(content.Annotations))
			}
		})

		It("should maintain the right annotations for the content to be cleaned up", func() {
			cleanupContent := cleanupContent["content1"]

			Expect(cleaner.Clean(ctx, cl, cleanupContent, cleanOps...)).To(Succeed())

			content := &volumesnapshotv1.VolumeSnapshotContent{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(cleanupContent), content)).To(Succeed())

			Expect(content.Annotations).To(HaveKeyWithValue("snapshot.storage.kubernetes.io/volumesnapshot-being-deleted", "yes"))
			Expect(content.Annotations).NotTo(HaveKeyWithValue("snapshot.storage.kubernetes.io/volumesnapshot-being-created", "yes"))
		})
	})

	Describe("#EnsureGone", func() {
		It("should ensure that the object is gone when not found error occurs", func() {
			ctx := context.TODO()

			c.EXPECT().Get(ctx, cm1Key, &cm1).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			Expect(EnsureGone(ctx, c, &cm1)).To(Succeed())
		})

		It("should ensure that the object is gone when no match error occurs", func() {
			ctx := context.TODO()

			c.EXPECT().Get(ctx, cm1Key, &cm1).Return(&meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}})

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

		It("should ensure that the list is gone when not found error occurs", func() {
			var (
				ctx  = context.TODO()
				list = corev1.ConfigMapList{}
			)

			c.EXPECT().List(ctx, &list).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			Expect(EnsureGone(ctx, c, &list)).To(Succeed())
		})

		It("should ensure that the list is gone when no match error occurs", func() {
			var (
				ctx  = context.TODO()
				list = corev1.ConfigMapList{}
			)

			c.EXPECT().List(ctx, &list).Return(&meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}})

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

	Describe("#EnsureGoneBefore", func() {
		var (
			before            = time.Now()
			goneBeforeEnsurer = GoneBeforeEnsurer(before)
		)

		It("should ensure that the object is gone because it is not found", func() {
			ctx := context.TODO()

			c.EXPECT().Get(ctx, cm1Key, &cm1).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			Expect(goneBeforeEnsurer.EnsureGone(ctx, c, &cm1)).To(Succeed())
		})

		It("should ensure that the object is gone because it has a greater deletion timestamp", func() {
			ctx := context.TODO()
			cm1.ObjectMeta.CreationTimestamp = metav1.NewTime(before.Add(time.Second))

			c.EXPECT().Get(ctx, cm1Key, &cm1)

			Expect(goneBeforeEnsurer.EnsureGone(ctx, c, &cm1)).To(Succeed())
		})

		It("should ensure that the list is gone because it is empty", func() {
			var (
				ctx  = context.TODO()
				list = corev1.ConfigMapList{}
			)

			c.EXPECT().List(ctx, &list)

			Expect(goneBeforeEnsurer.EnsureGone(ctx, c, &list)).To(Succeed())
		})

		It("should ensure that no error occurs because the element was created after shoot deletion", func() {
			// move timestamp of configmap ahead of time marker
			cm1.ObjectMeta.CreationTimestamp = metav1.NewTime(before.Add(time.Second))

			var (
				ctx  = context.TODO()
				list = corev1.ConfigMapList{
					Items: []corev1.ConfigMap{
						cm1,
					},
				}
			)

			c.EXPECT().List(ctx, &list)

			Expect(goneBeforeEnsurer.EnsureGone(ctx, c, &list)).To(Not(HaveOccurred()))
		})

		It("should ensure that no error occurs because resource is not present in the cluster", func() {
			// move timestamp of configmap ahead of time marker
			cm1.ObjectMeta.CreationTimestamp = metav1.NewTime(before.Add(time.Second))

			var (
				ctx  = context.TODO()
				list = corev1.ConfigMapList{
					Items: []corev1.ConfigMap{
						cm1,
					},
				}
			)

			c.EXPECT().List(ctx, &list).DoAndReturn(func(_ context.Context, _ *corev1.ConfigMapList, _ ...client.ListOptions) error {
				return &meta.NoResourceMatchError{}
			})

			Expect(goneBeforeEnsurer.EnsureGone(ctx, c, &list)).To(Not(HaveOccurred()))
		})

		It("should ensure that an error occurs because one element in the list is not gone", func() {
			// move timestamp of configmap ahead of time marker
			cm1.ObjectMeta.CreationTimestamp = metav1.NewTime(before.Add(time.Second))

			// move timestamp of configmap before of time marker
			cm2.ObjectMeta.CreationTimestamp = metav1.NewTime(before.Add(-time.Second))

			var (
				ctx  = context.TODO()
				list = corev1.ConfigMapList{
					Items: []corev1.ConfigMap{
						cm1,
						cm2,
					},
				}
			)

			c.EXPECT().List(ctx, &list)

			Expect(goneBeforeEnsurer.EnsureGone(ctx, c, &list)).To(
				Equal(NewObjectsRemaining(&cm2)),
			)
		})

		It("should error that the object is still present", func() {
			ctx := context.TODO()

			c.EXPECT().Get(ctx, cm1Key, &cm1)

			Expect(goneBeforeEnsurer.EnsureGone(ctx, c, &cm1)).To(Equal(NewObjectsRemaining(&cm1)))
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
			o = NewCleanOps(ensurer, cleaner)
		})

		Describe("CleanAndEnsureGone", func() {
			It("should clean and ensure that the object is gone", func() {
				ctx := context.TODO()

				gomock.InOrder(
					cleaner.EXPECT().Clean(ctx, c, &cm1),
					ensurer.EXPECT().EnsureGone(ctx, c, &cm1),
				)

				Expect(o.CleanAndEnsureGone(ctx, c, &cm1)).To(Succeed())
			})
		})
	})
})
