// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerutils_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/controllerutils"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
)

const resourceVersion = "42"

var _ = Describe("Finalizers", func() {
	var (
		ctx context.Context

		ctrl       *gomock.Controller
		mockReader *mockclient.MockReader
		mockWriter *mockclient.MockWriter

		obj client.Object
	)

	BeforeEach(func() {
		ctx = context.Background()

		ctrl = gomock.NewController(GinkgoT())
		mockReader = mockclient.NewMockReader(ctrl)
		mockWriter = mockclient.NewMockWriter(ctrl)

		_ = mockReader

		obj = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "some-ns", Name: "some-name"}}
		obj.SetResourceVersion(resourceVersion)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("no retry on conflict", func() {
		Describe("PatchAddFinalizers", func() {
			test := func(description string, expectedPatchFinalizers string, existingFinalizers, finalizers []string) {
				It(description+fmt.Sprintf(" %v, %v", existingFinalizers, finalizers), func() {
					obj.SetFinalizers(existingFinalizers)
					mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						Expect(patch.Type()).To(Equal(types.MergePatchType))
						Expect(patch.Data(o)).To(BeEquivalentTo(expectedMergePatch(expectedPatchFinalizers)))
						return nil
					})

					Expect(PatchAddFinalizers(ctx, mockWriter, obj, finalizers...)).To(Succeed())
				})
			}
			test("should add given finalizers via patch", `["foo"]`, nil, []string{"foo"})
			test("should add given finalizers via patch", `["foo","bar"]`, nil, []string{"foo", "bar"})
			test("should add given finalizers via patch", `["bar","foo"]`, []string{"bar"}, []string{"foo"})
			test("should add given finalizers via patch", `["bar","foo","baz"]`, []string{"bar"}, []string{"foo", "baz"})
			test("should not add finalizers if already present", ``, []string{"foo"}, []string{"foo"})
			test("should not add finalizers if already present", ``, []string{"foo", "bar"}, []string{"bar"})
			test("should do nothing if no finalizers are given", ``, nil, nil)
			test("should do nothing if no finalizers are given", ``, []string{"foo"}, nil)
			test("should do nothing if no finalizers are given", ``, []string{"foo", "bar"}, nil)

			It("should fail on conflict", func() {
				mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).Return(apierrors.NewConflict(schema.GroupResource{}, obj.GetName(), fmt.Errorf("conflict")))
				Expect(PatchAddFinalizers(ctx, mockWriter, obj, "foo")).To(MatchError(ContainSubstring("conflict")))
			})
		})

		Describe("PatchRemoveFinalizers", func() {
			test := func(description string, expectedPatchFinalizers string, existingFinalizers, finalizers []string) {
				It(description+fmt.Sprintf(" %v, %v", existingFinalizers, finalizers), func() {
					obj.SetFinalizers(existingFinalizers)
					mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						Expect(patch.Type()).To(Equal(types.MergePatchType))
						Expect(patch.Data(o)).To(BeEquivalentTo(expectedMergePatch(expectedPatchFinalizers)))
						return nil
					})

					Expect(PatchRemoveFinalizers(ctx, mockWriter, obj, finalizers...)).To(Succeed())
				})
			}
			test("should remove given finalizers via patch", `null`, []string{"foo"}, []string{"foo"})
			test("should remove given finalizers via patch", `["bar"]`, []string{"foo", "bar"}, []string{"foo"})
			test("should remove given finalizers via patch", `null`, []string{"foo", "bar"}, []string{"foo", "bar"})
			test("should not remove finalizers if not present", ``, nil, []string{"foo"})
			test("should not remove finalizers if not present", ``, []string{"foo"}, []string{"bar"})
			test("should not remove finalizers if not present", ``, []string{"foo", "bar"}, []string{"baz"})
			test("should do nothing if no finalizers are given", ``, nil, nil)
			test("should do nothing if no finalizers are given", ``, []string{"foo"}, nil)
			test("should do nothing if no finalizers are given", ``, []string{"foo", "bar"}, nil)

			It("should fail on conflict", func() {
				mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).Return(apierrors.NewConflict(schema.GroupResource{}, obj.GetName(), fmt.Errorf("conflict")))
				Expect(PatchRemoveFinalizers(ctx, mockWriter, obj, "foo")).To(MatchError(ContainSubstring("conflict")))
			})
		})

		Describe("StrategicMergePatchAddFinalizers", func() {
			test := func(description string, expectedPatch string, existingFinalizers, finalizers []string) {
				It(description+fmt.Sprintf(" %v, %v", existingFinalizers, finalizers), func() {
					obj.SetFinalizers(existingFinalizers)
					mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						Expect(patch.Type()).To(Equal(types.StrategicMergePatchType))
						Expect(patch.Data(o)).To(BeEquivalentTo(expectedPatch))
						return nil
					})

					Expect(StrategicMergePatchAddFinalizers(ctx, mockWriter, obj, finalizers...)).To(Succeed())
				})
			}
			test("should add given finalizers via patch", `{"metadata":{"finalizers":["foo"]}}`, nil, []string{"foo"})
			test("should add given finalizers via patch", `{"metadata":{"finalizers":["foo","bar"]}}`, nil, []string{"foo", "bar"})
			test("should add given finalizers via patch", `{"metadata":{"$setElementOrder/finalizers":["bar","foo"],"finalizers":["foo"]}}`, []string{"bar"}, []string{"foo"})
			test("should add given finalizers via patch", `{"metadata":{"$setElementOrder/finalizers":["bar","foo"],"finalizers":["foo"]}}`, []string{"bar"}, []string{"foo", "bar"})
			test("should not add finalizers if already present", `{}`, []string{"foo"}, []string{"foo"})
			test("should not add finalizers if already present", `{"metadata":{"$setElementOrder/finalizers":["foo","bar"]}}`, []string{"foo", "bar"}, []string{"bar"})
			test("should do nothing if no finalizers are given", `{}`, nil, nil)
			test("should do nothing if no finalizers are given", `{}`, []string{"foo"}, nil)
			test("should do nothing if no finalizers are given", `{"metadata":{"$setElementOrder/finalizers":["foo","bar"]}}`, []string{"foo", "bar"}, nil)
		})
	})

	Context("with retry on conflict", func() {
		Describe("EnsureFinalizer", func() {
			Context("no conflict", func() {
				test := func(description string, expectedPatchFinalizers string, existingFinalizers []string, finalizer string) {
					It(description+fmt.Sprintf(" %v, %v", existingFinalizers, finalizer), func() {
						gomock.InOrder(
							mockReader.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o client.Object) error {
								o.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))
								return nil
							}),
							mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
								Expect(patch.Type()).To(Equal(types.MergePatchType))
								Expect(patch.Data(o)).To(BeEquivalentTo(expectedMergePatch(expectedPatchFinalizers)))
								return nil
							}),
						)

						Expect(EnsureFinalizer(ctx, mockReader, mockWriter, obj, finalizer)).To(Succeed())
					})
				}

				test("should succeed if no conflict occurs", `["foo"]`, nil, "foo")
				test("should succeed if no conflict occurs", `["foo","bar"]`, []string{"foo"}, "bar")
				test("should succeed if no conflict occurs", ``, []string{"foo"}, "foo")
			})

			Context("conflict", func() {
				test := func(description string, expectedPatchFinalizers string, existingFinalizers []string, finalizer string) {
					It(description+fmt.Sprintf(" %v, %v", existingFinalizers, finalizer), func() {
						gomock.InOrder(
							mockReader.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o client.Object) error {
								o.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))
								return nil
							}),
							mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).Return(apierrors.NewConflict(schema.GroupResource{}, obj.GetName(), fmt.Errorf("conflict"))),
							mockReader.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o client.Object) error {
								o.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))
								return nil
							}),
							mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
								Expect(patch.Type()).To(Equal(types.MergePatchType))
								Expect(patch.Data(o)).To(BeEquivalentTo(expectedMergePatch(expectedPatchFinalizers)))
								return nil
							}),
						)

						Expect(EnsureFinalizer(ctx, mockReader, mockWriter, obj, finalizer)).To(Succeed())
					})
				}

				test("should succeed if a conflict occurs", `["foo"]`, nil, "foo")
				test("should succeed if a conflict occurs", `["foo","bar"]`, []string{"foo"}, "bar")
				test("should succeed if a conflict occurs", ``, []string{"foo"}, "foo")
			})
			It("should fail on not found", func() {
				mockReader.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).Return(apierrors.NewNotFound(schema.GroupResource{}, obj.GetName()))
				Expect(EnsureFinalizer(ctx, mockReader, mockWriter, obj, "foo")).To(MatchError(ContainSubstring("not found")))
			})
		})

		Describe("RemoveFinalizer", func() {
			Context("no conflict", func() {
				test := func(description string, expectedPatchFinalizers string, existingFinalizers []string, finalizer string) {
					It(description+fmt.Sprintf(" %v, %v", existingFinalizers, finalizer), func() {
						gomock.InOrder(
							mockReader.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o client.Object) error {
								o.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))
								return nil
							}),
							mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
								Expect(patch.Type()).To(Equal(types.MergePatchType))
								Expect(patch.Data(o)).To(BeEquivalentTo(expectedMergePatch(expectedPatchFinalizers)))
								return nil
							}),
						)

						Expect(RemoveFinalizer(ctx, mockReader, mockWriter, obj, finalizer)).To(Succeed())
					})
				}

				test("should succeed if no conflict occurs", ``, nil, "foo")
				test("should succeed if no conflict occurs", ``, []string{"foo"}, "bar")
				test("should succeed if no conflict occurs", `null`, []string{"foo"}, "foo")
				test("should succeed if no conflict occurs", `["bar"]`, []string{"foo", "bar"}, "foo")
			})

			Context("conflict", func() {
				test := func(description string, expectedPatchFinalizers string, existingFinalizers []string, finalizer string) {
					It(description+fmt.Sprintf(" %v, %v", existingFinalizers, finalizer), func() {
						gomock.InOrder(
							mockReader.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o client.Object) error {
								o.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))
								return nil
							}),
							mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).Return(apierrors.NewConflict(schema.GroupResource{}, obj.GetName(), fmt.Errorf("conflict"))),
							mockReader.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o client.Object) error {
								o.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))
								return nil
							}),
							mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
								Expect(patch.Type()).To(Equal(types.MergePatchType))
								Expect(patch.Data(o)).To(BeEquivalentTo(expectedMergePatch(expectedPatchFinalizers)))
								return nil
							}),
						)

						Expect(RemoveFinalizer(ctx, mockReader, mockWriter, obj, finalizer)).To(Succeed())
					})
				}

				test("should succeed if a conflict occurs", ``, nil, "foo")
				test("should succeed if a conflict occurs", ``, []string{"foo"}, "bar")
				test("should succeed if a conflict occurs", `null`, []string{"foo"}, "foo")
				test("should succeed if a conflict occurs", `["bar"]`, []string{"foo", "bar"}, "foo")
			})
		})

		Describe("RemoveAllFinalizers", func() {
			Context("no conflict", func() {
				test := func(description string, expectedPatchFinalizers string, existingFinalizers []string) {
					It(description+fmt.Sprintf(" %v", existingFinalizers), func() {
						gomock.InOrder(
							mockReader.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o client.Object) error {
								o.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))
								return nil
							}),
							mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
								Expect(patch.Type()).To(Equal(types.MergePatchType))
								Expect(patch.Data(o)).To(BeEquivalentTo(expectedMergePatch(expectedPatchFinalizers)))
								return nil
							}),
						)

						Expect(RemoveAllFinalizers(ctx, mockReader, mockWriter, obj)).To(Succeed())
					})
				}

				test("should succeed if no conflict occurs", ``, nil)
				test("should succeed if no conflict occurs", `null`, []string{"foo"})
				test("should succeed if no conflict occurs", `null`, []string{"foo", "bar"})
			})

			Context("conflict", func() {
				test := func(description string, expectedPatchFinalizers string, existingFinalizers []string) {
					It(description+fmt.Sprintf(" %v", existingFinalizers), func() {
						gomock.InOrder(
							mockReader.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o client.Object) error {
								o.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))
								return nil
							}),
							mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).Return(apierrors.NewConflict(schema.GroupResource{}, obj.GetName(), fmt.Errorf("conflict"))),
							mockReader.EXPECT().Get(ctx, client.ObjectKeyFromObject(obj), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o client.Object) error {
								o.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))
								return nil
							}),
							mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
								Expect(patch.Type()).To(Equal(types.MergePatchType))
								Expect(patch.Data(o)).To(BeEquivalentTo(expectedMergePatch(expectedPatchFinalizers)))
								return nil
							}),
						)

						Expect(RemoveAllFinalizers(ctx, mockReader, mockWriter, obj)).To(Succeed())
					})
				}

				test("should succeed if a conflict occurs", ``, nil)
				test("should succeed if a conflict occurs", `null`, []string{"foo"})
				test("should succeed if a conflict occurs", `null`, []string{"foo", "bar"})
			})
		})
	})
})

func expectedMergePatch(expectedPatchFinalizers string) string {
	finalizersJSONString := ""
	if expectedPatchFinalizers != "" {
		finalizersJSONString = fmt.Sprintf(`"finalizers":%s,`, expectedPatchFinalizers)
	}
	return fmt.Sprintf(`{"metadata":{%s"resourceVersion":"%s"}}`, finalizersJSONString, resourceVersion)
}
