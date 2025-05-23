// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/controllerutils"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

const resourceVersion = "42"

var _ = Describe("Finalizers", func() {
	var (
		ctx context.Context

		ctrl       *gomock.Controller
		mockWriter *mockclient.MockWriter

		obj client.Object
	)

	BeforeEach(func() {
		ctx = context.Background()

		ctrl = gomock.NewController(GinkgoT())
		mockWriter = mockclient.NewMockWriter(ctrl)

		obj = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "some-ns", Name: "some-name"}}
		obj.SetResourceVersion(resourceVersion)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("RemoveFinalizers", func() {
		test := func(expectedPatchFinalizers string, existingFinalizers []string, finalizer string) {
			It(fmt.Sprintf("should succeed %v, %v", existingFinalizers, finalizer), func() {
				obj.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))

				mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Type()).To(Equal(types.MergePatchType))
					Expect(patch.Data(o)).To(BeEquivalentTo(expectedMergePatchWithOptimisticLocking(expectedPatchFinalizers)))
					return nil
				})

				Expect(RemoveFinalizers(ctx, mockWriter, obj, finalizer)).To(Succeed())
			})
		}

		test(``, nil, "foo")
		test(``, []string{"foo"}, "bar")
		test(`null`, []string{"foo"}, "foo")
		test(`["bar"]`, []string{"foo", "bar"}, "foo")
	})

	Describe("RemoveAllFinalizers", func() {
		test := func(expectedPatchFinalizers string, existingFinalizers []string) {
			It(fmt.Sprintf("should succeed %v", existingFinalizers), func() {
				obj.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))

				mockWriter.EXPECT().Patch(ctx, obj, gomock.Any()).DoAndReturn(func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					Expect(patch.Type()).To(Equal(types.MergePatchType))
					Expect(patch.Data(o)).To(BeEquivalentTo(expectedPatchFinalizers))
					return nil
				})

				Expect(RemoveAllFinalizers(ctx, mockWriter, obj)).To(Succeed())
			})
		}

		test(`{}`, nil)
		test(`{"metadata":{"finalizers":null}}`, []string{"foo"})
		test(`{"metadata":{"finalizers":null}}`, []string{"foo", "bar"})
	})
})

func expectedMergePatchWithOptimisticLocking(expectedPatchFinalizers string) string {
	finalizersJSONString := ""
	if expectedPatchFinalizers != "" {
		finalizersJSONString = fmt.Sprintf(`"finalizers":%s,`, expectedPatchFinalizers)
	}
	return fmt.Sprintf(`{"metadata":{%s"resourceVersion":"%s"}}`, finalizersJSONString, resourceVersion)
}
