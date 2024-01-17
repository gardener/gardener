// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedresource

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
)

var _ = Describe("cleaner", func() {

	Describe("#cleanupStatefulSet", func() {
		var (
			s    *runtime.Scheme
			ctx  context.Context
			ctrl *gomock.Controller
			c    *mockclient.MockClient
			sts  *appsv1.StatefulSet
		)

		BeforeEach(func() {
			s = runtime.NewScheme()
			Expect(appsv1.AddToScheme(s)).ToNot(HaveOccurred(), "schema add should succeed")

			ctx = context.TODO()
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			sts = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo-ns",
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Replicas: ptr.To(int32(1)),
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								VolumeName:       "foo-pvc",
								StorageClassName: ptr.To("ultra-fast"),
							},
						},
					},
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should do nothing if deletePVCs is false", func() {
			err := cleanupStatefulSet(ctx, c, s, sts, false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should do nothing if conversion to appsv1.StatefulSet fails", func() {
			s = runtime.NewScheme()

			err := cleanupStatefulSet(ctx, c, s, sts, true)
			Expect(err).To(MatchError(ContainSubstring("failed cleaning up PersistentVolumeClaims of StatefulSet: could not convert object to StatefulSet")))
		})

		It("should do nothing if .spec.volumeClaimTemplate is not set", func() {
			sts.Spec.VolumeClaimTemplates = nil

			err := cleanupStatefulSet(ctx, c, s, sts, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should do nothing if list PVCs fails", func() {
			fakeErr := fmt.Errorf("fake")

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaimList{}), client.InNamespace(sts.Namespace), client.MatchingLabels(sts.Spec.Selector.MatchLabels)).
				DoAndReturn(func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
					return fakeErr
				})

			err := cleanupStatefulSet(ctx, c, s, sts, true)
			Expect(err).To(MatchError(ContainSubstring(fakeErr.Error())))
		})

		It("should do nothing if all PVCs of the StatefulSet have already been deleted", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaimList{}), client.InNamespace(sts.Namespace), client.MatchingLabels(sts.Spec.Selector.MatchLabels)).
				DoAndReturn(func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
					return nil
				})

			err := cleanupStatefulSet(ctx, c, s, sts, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete all PVCs of the StatefulSet", func() {
			fakeErr := fmt.Errorf("fake")

			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaimList{}), client.InNamespace(sts.Namespace), client.MatchingLabels(sts.Spec.Selector.MatchLabels)).
					DoAndReturn(func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
						list.(*corev1.PersistentVolumeClaimList).Items = []corev1.PersistentVolumeClaim{
							{
								Spec: corev1.PersistentVolumeClaimSpec{
									AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
									VolumeName:       "foo-pvc-foo-0",
									StorageClassName: ptr.To("ultra-fast"),
								},
							},
						}
						return nil
					}),
				c.EXPECT().DeleteAllOf(ctx, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{}), client.InNamespace(sts.Namespace), client.MatchingLabels(sts.Spec.Selector.MatchLabels)).
					DoAndReturn(func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
						return fakeErr
					}),
			)

			err := cleanupStatefulSet(ctx, c, s, sts, true)
			Expect(err).To(MatchError(ContainSubstring(fakeErr.Error())))
		})

		It("should delete all PVCs of the StatefulSet", func() {
			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaimList{}), client.InNamespace(sts.Namespace), client.MatchingLabels(sts.Spec.Selector.MatchLabels)).
					DoAndReturn(func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
						list.(*corev1.PersistentVolumeClaimList).Items = []corev1.PersistentVolumeClaim{
							{
								Spec: corev1.PersistentVolumeClaimSpec{
									AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
									VolumeName:       "foo-pvc-foo-0",
									StorageClassName: ptr.To("ultra-fast"),
								},
							},
						}
						return nil
					}),
				c.EXPECT().DeleteAllOf(ctx, gomock.AssignableToTypeOf(&corev1.PersistentVolumeClaim{}), client.InNamespace(sts.Namespace), client.MatchingLabels(sts.Spec.Selector.MatchLabels)).
					DoAndReturn(func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
						return nil
					}),
			)

			err := cleanupStatefulSet(ctx, c, s, sts, true)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
