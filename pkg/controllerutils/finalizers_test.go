// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	. "github.com/gardener/gardener/pkg/controllerutils"
)

const resourceVersion = "42"

var _ = Describe("Finalizers", func() {
	var (
		ctx context.Context

		scheme *runtime.Scheme

		obj client.Object
	)

	BeforeEach(func() {
		ctx = context.Background()

		scheme = runtime.NewScheme()
		Expect(kubernetesscheme.AddToScheme(scheme)).To(Succeed())

		obj = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "some-ns", Name: "some-name"}}
		obj.SetResourceVersion(resourceVersion)
	})

	Describe("RemoveFinalizers", func() {
		test := func(expectedPatchFinalizers string, existingFinalizers []string, finalizer string) {
			It(fmt.Sprintf("should succeed %v, %v", existingFinalizers, finalizer), func() {
				obj.SetFinalizers(append(existingFinalizers[:0:0], existingFinalizers...))

				patchCalled := false
				fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
						patchCalled = true
						Expect(patch.Type()).To(Equal(types.MergePatchType))
						Expect(patch.Data(o)).To(BeEquivalentTo(expectedMergePatchWithOptimisticLocking(expectedPatchFinalizers)))
						return nil
					},
				}).Build()

				Expect(RemoveFinalizers(ctx, fakeClient, obj, finalizer)).To(Succeed())
				Expect(patchCalled).To(BeTrue())
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

				patchCalled := false
				fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
						patchCalled = true
						Expect(patch.Type()).To(Equal(types.MergePatchType))
						Expect(patch.Data(o)).To(BeEquivalentTo(expectedPatchFinalizers))
						return nil
					},
				}).Build()

				Expect(RemoveAllFinalizers(ctx, fakeClient, obj)).To(Succeed())
				Expect(patchCalled).To(BeTrue())
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
