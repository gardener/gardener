// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	corescheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	. "github.com/gardener/gardener/pkg/controllerutils"
)

var _ = Describe("Patch", func() {
	var (
		ctx     = context.TODO()
		fakeErr = errors.New("fake err")

		fakeClient client.Client
		scheme     *runtime.Scheme
		obj        *corev1.ServiceAccount
	)

	BeforeEach(func() {
		obj = &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		}}

		scheme = runtime.NewScheme()
		Expect(corescheme.AddToScheme(scheme)).NotTo(HaveOccurred())

		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
	})

	Describe("GetAndCreateOr*Patch", func() {
		testSuite := func(f func(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn, opts ...PatchOption) (controllerutil.OperationResult, error), patchType types.PatchType) {
			It("should return an error because reading the object fails", func() {
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fakeErr
					},
				}).Build()

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because the mutate function returned an error (object found)", func() {
				Expect(fakeClient.Create(ctx, obj)).To(Succeed())

				result, err := f(ctx, fakeClient, obj, func() error { return fakeErr })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because the mutate function returned an error (object not found)", func() {
				result, err := f(ctx, fakeClient, obj, func() error { return fakeErr })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because the create failed", func() {
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
					Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
						return fakeErr
					},
				}).Build()

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should successfully create the object", func() {
				result, err := f(ctx, fakeClient, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultCreated))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error because the patch failed", func() {
				// Use WithObjects to pre-populate the client without modifying obj
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, patch client.Patch, _ ...client.PatchOption) error {
						Expect(patch.Type()).To(Equal(patchType))
						return fakeErr
					},
				}).Build()

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should successfully patch the object", func() {
				mutateFn := func() error {
					obj.Labels = map[string]string{"foo": "bar"}
					return nil
				}

				patchCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, cl client.WithWatch, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						Expect(patch.Type()).To(Equal(patchType))
						patchCalled = true
						return cl.Patch(ctx, o, patch, opts...)
					},
				}).Build()

				result, err := f(ctx, c, obj, mutateFn)
				Expect(result).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
				Expect(patchCalled).To(BeTrue())
			})

			It("should successfully patch the object with optimistic locking", func() {
				mutateFn := func() error {
					obj.Labels = map[string]string{"foo": "bar"}
					return nil
				}

				patchCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, cl client.WithWatch, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						Expect(patch.Type()).To(Equal(patchType))
						data, dataErr := patch.Data(o)
						Expect(dataErr).NotTo(HaveOccurred())
						Expect(string(data)).To(ContainSubstring(`"resourceVersion"`))
						patchCalled = true
						return cl.Patch(ctx, o, patch, opts...)
					},
				}).Build()

				result, err := f(ctx, c, obj, mutateFn, MergeFromOption{client.MergeFromWithOptimisticLock{}})
				Expect(result).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
				Expect(patchCalled).To(BeTrue())
			})

			It("should skip sending an empty patch", func() {
				patchCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, cl client.WithWatch, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						patchCalled = true
						return cl.Patch(ctx, o, patch, opts...)
					},
				}).Build()

				result, err := f(ctx, c, obj, func() error { return nil }, SkipEmptyPatch{})
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).NotTo(HaveOccurred())
				Expect(patchCalled).To(BeFalse())
			})

			It("should skip sending an empty patch with optimistic locking", func() {
				patchCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, cl client.WithWatch, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						patchCalled = true
						return cl.Patch(ctx, o, patch, opts...)
					},
				}).Build()

				result, err := f(ctx, c, obj, func() error { return nil }, MergeFromOption{client.MergeFromWithOptimisticLock{}}, SkipEmptyPatch{})
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).NotTo(HaveOccurred())
				Expect(patchCalled).To(BeFalse())
			})
		}

		Describe("#GetAndCreateOrMergePatch", func() { testSuite(GetAndCreateOrMergePatch, types.MergePatchType) })
		Describe("#GetAndCreateOrStrategicMergePatch", func() { testSuite(GetAndCreateOrStrategicMergePatch, types.StrategicMergePatchType) })
	})

	Describe("CreateOrGetAnd*Patch", func() {
		testSuite := func(f func(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn, opts ...PatchOption) (controllerutil.OperationResult, error), patchType types.PatchType) {
			It("should return an error because the mutate function returned an error", func() {
				result, err := f(ctx, fakeClient, obj, func() error { return fakeErr })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because the create failed", func() {
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
					Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
						return fakeErr
					},
				}).Build()

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should successfully create the object", func() {
				result, err := f(ctx, fakeClient, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultCreated))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error because the get failed", func() {
				// Pre-populate with WithObjects so Create gets AlreadyExists, then inject Get error
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fakeErr
					},
				}).Build()

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return an error because the patch failed", func() {
				// Pre-populate so Create returns AlreadyExists, then Get succeeds, then Patch fails
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, patch client.Patch, _ ...client.PatchOption) error {
						Expect(patch.Type()).To(Equal(patchType))
						return fakeErr
					},
				}).Build()

				result, err := f(ctx, c, obj, func() error { return nil })
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).To(MatchError(fakeErr))
			})

			It("should successfully patch the object", func() {
				// Pre-populate so Create returns AlreadyExists, Get finds it, then mutate+patch
				patchCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, cl client.WithWatch, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						Expect(patch.Type()).To(Equal(patchType))
						data, dataErr := patch.Data(o)
						Expect(dataErr).NotTo(HaveOccurred())
						Expect(string(data)).To(ContainSubstring(`{"metadata":{"labels":{"foo":"bar"}}}`))
						patchCalled = true
						return cl.Patch(ctx, o, patch, opts...)
					},
				}).Build()

				mutateFn := func() error {
					obj.Labels = map[string]string{"foo": "bar"}
					return nil
				}

				result, err := f(ctx, c, obj, mutateFn)
				Expect(result).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
				Expect(patchCalled).To(BeTrue())
			})

			It("should successfully patch the object with optimistic locking", func() {
				// Pre-populate so Create returns AlreadyExists, Get finds it, then mutate+patch
				patchCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, cl client.WithWatch, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						Expect(patch.Type()).To(Equal(patchType))
						data, dataErr := patch.Data(o)
						Expect(dataErr).NotTo(HaveOccurred())
						// The patch must contain the resourceVersion for optimistic locking
						Expect(string(data)).To(ContainSubstring(`{"metadata":{"labels":{"foo":"bar"},"resourceVersion":"`))
						patchCalled = true
						return cl.Patch(ctx, o, patch, opts...)
					},
				}).Build()

				mutateFn := func() error {
					obj.Labels = map[string]string{"foo": "bar"}
					return nil
				}

				result, err := f(ctx, c, obj, mutateFn, MergeFromOption{MergeFromOption: client.MergeFromWithOptimisticLock{}})
				Expect(result).To(Equal(controllerutil.OperationResultUpdated))
				Expect(err).NotTo(HaveOccurred())
				Expect(patchCalled).To(BeTrue())
			})

			It("should skip sending an empty patch", func() {
				// Pre-populate so Create returns AlreadyExists, Get finds it, empty mutate → skip patch
				patchCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, cl client.WithWatch, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						patchCalled = true
						Expect(patch.Type()).To(Equal(patchType))
						return cl.Patch(ctx, o, patch, opts...)
					},
				}).Build()

				result, err := f(ctx, c, obj, func() error { return nil }, SkipEmptyPatch{})
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).NotTo(HaveOccurred())
				Expect(patchCalled).To(BeFalse())
			})

			It("should skip sending an empty patch with optimistic locking", func() {
				// Pre-populate so Create returns AlreadyExists, Get finds it, empty mutate → skip patch
				patchCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(obj.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, cl client.WithWatch, o client.Object, patch client.Patch, opts ...client.PatchOption) error {
						patchCalled = true
						Expect(patch.Type()).To(Equal(patchType))
						return cl.Patch(ctx, o, patch, opts...)
					},
				}).Build()

				result, err := f(ctx, c, obj, func() error { return nil }, MergeFromOption{MergeFromOption: client.MergeFromWithOptimisticLock{}}, SkipEmptyPatch{})
				Expect(result).To(Equal(controllerutil.OperationResultNone))
				Expect(err).NotTo(HaveOccurred())
				Expect(patchCalled).To(BeFalse())
			})
		}

		Describe("#CreateOrGetAndMergePatch", func() { testSuite(CreateOrGetAndMergePatch, types.MergePatchType) })
		Describe("#CreateOrGetAndStrategicMergePatch", func() { testSuite(CreateOrGetAndStrategicMergePatch, types.StrategicMergePatchType) })
	})
})
