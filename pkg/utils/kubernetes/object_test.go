// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"errors"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Object", func() {
	var (
		ctx        = context.TODO()
		fakeErr    = errors.New("fake err")
		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
	})

	Describe("#DeleteObjects", func() {
		It("should fail because an object fails to delete", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
						return fakeErr
					},
				}).
				Build()

			obj1 := &corev1.Secret{}
			obj2 := &appsv1.Deployment{}
			Expect(DeleteObjects(ctx, fakeClient, obj1, obj2)).To(MatchError(fakeErr))
		})

		It("should fail because the second object fails to delete", func() {
			callCount := 0
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						callCount++
						if callCount == 2 {
							return fakeErr
						}
						return cl.Delete(ctx, obj, opts...)
					},
				}).
				Build()

			obj1 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "default"}}
			obj2 := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "d1", Namespace: "default"}}
			Expect(fakeClient.Create(ctx, obj1)).To(Succeed())
			Expect(fakeClient.Create(ctx, obj2)).To(Succeed())

			Expect(DeleteObjects(ctx, fakeClient, obj1, obj2)).To(MatchError(fakeErr))
		})

		It("should successfully delete all objects", func() {
			obj1 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "default"}}
			obj2 := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "d1", Namespace: "default"}}
			Expect(fakeClient.Create(ctx, obj1)).To(Succeed())
			Expect(fakeClient.Create(ctx, obj2)).To(Succeed())

			Expect(DeleteObjects(ctx, fakeClient, obj1, obj2)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(obj1), &corev1.Secret{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(obj2), &appsv1.Deployment{})).To(BeNotFoundError())
		})
	})

	Describe("#DeleteObject", func() {
		It("should fail to delete the object", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
						return fakeErr
					},
				}).
				Build()

			Expect(DeleteObject(ctx, fakeClient, &corev1.Secret{})).To(MatchError(fakeErr))
		})

		It("should not fail to delete the object (not found error)", func() {
			Expect(DeleteObject(ctx, fakeClient, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "nonexistent", Namespace: "default"}})).To(Succeed())
		})

		It("should not fail to delete the object (no match error)", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
						return &meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}}
					},
				}).
				Build()

			Expect(DeleteObject(ctx, fakeClient, &corev1.Secret{})).To(Succeed())
		})

		It("should successfully delete the object", func() {
			obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "default"}}
			Expect(fakeClient.Create(ctx, obj)).To(Succeed())

			Expect(DeleteObject(ctx, fakeClient, obj)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(obj), &corev1.Secret{})).To(BeNotFoundError())
		})
	})

	Describe("#DeleteObjectsFromListConditionally", func() {
		var (
			obj1 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "1"}}
			obj2 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "2"}}
			obj3 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "3"}}

			predicateFn = func(obj runtime.Object) bool {
				acc, _ := meta.Accessor(obj)
				return acc.GetName() != "2"
			}
		)

		It("should return an error if deleting an object failed", func() {
			Expect(fakeClient.Create(ctx, obj1.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, obj3.DeepCopy())).To(Succeed())

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetesscheme.Scheme).
				WithObjects(obj1.DeepCopy(), obj3.DeepCopy()).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						if obj.GetName() == "3" {
							return fakeErr
						}
						return cl.Delete(ctx, obj, opts...)
					},
				}).
				Build()

			listObject := &corev1.NamespaceList{Items: []corev1.Namespace{*obj1, *obj2, *obj3}}

			err := DeleteObjectsFromListConditionally(ctx, fakeClient, listObject, predicateFn)
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
		})

		It("should successfully delete the relevant objects", func() {
			Expect(fakeClient.Create(ctx, obj1.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, obj2.DeepCopy())).To(Succeed())
			Expect(fakeClient.Create(ctx, obj3.DeepCopy())).To(Succeed())

			Expect(DeleteObjectsFromListConditionally(ctx, fakeClient, &corev1.NamespaceList{Items: []corev1.Namespace{*obj1, *obj2, *obj3}}, predicateFn)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(obj1), &corev1.Namespace{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(obj2), &corev1.Namespace{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(obj3), &corev1.Namespace{})).To(BeNotFoundError())
		})
	})

	Describe("#ResourcesExist", func() {
		var (
			scheme   *runtime.Scheme
			objList  client.ObjectList
			listOpts []client.ListOption
		)

		BeforeEach(func() {
			scheme = kubernetesscheme.Scheme
			objList = &corev1.SecretList{}
			listOpts = []client.ListOption{client.InNamespace(namespace)}
		})

		It("should return an error because the listing failed", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return fakeErr
					},
				}).
				Build()

			inUse, err := ResourcesExist(ctx, fakeClient, objList, scheme, listOpts...)
			Expect(err).To(MatchError(fakeErr))
			Expect(inUse).To(BeTrue())
		})

		Context("with partialObjectMetadataList", func() {
			BeforeEach(func() {
				listOpts = append(listOpts, client.MatchingFields{"metadata.name": "foo"})
				fakeClient = fakeclient.NewClientBuilder().
					WithScheme(kubernetesscheme.Scheme).
					WithIndex(&corev1.Secret{}, "metadata.name", func(obj client.Object) []string {
						return []string{obj.GetName()}
					}).
					Build()
			})

			It("should return true because objects found", func() {
				Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: namespace}})).To(Succeed())

				inUse, err := ResourcesExist(ctx, fakeClient, objList, scheme, listOpts...)
				Expect(err).NotTo(HaveOccurred())
				Expect(inUse).To(BeTrue())
			})

			It("should return false because no objects found", func() {
				inUse, err := ResourcesExist(ctx, fakeClient, objList, scheme, listOpts...)
				Expect(err).NotTo(HaveOccurred())
				Expect(inUse).To(BeFalse())
			})
		})

		Context("with objectList", func() {
			BeforeEach(func() {
				listOpts = append(listOpts, client.MatchingFields{"data.foo": "bar"})

				fakeClient = fakeclient.NewClientBuilder().
					WithScheme(kubernetesscheme.Scheme).
					WithIndex(&corev1.Secret{}, "data.foo", func(obj client.Object) []string {
						s, ok := obj.(*corev1.Secret)
						if !ok {
							return nil
						}
						if v, ok := s.Data["foo"]; ok {
							return []string{string(v)}
						}
						return nil
					}).
					Build()
			})

			It("should return true because objects found", func() {
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: namespace,
					},
					Data: map[string][]byte{"foo": []byte("bar")},
				})).To(Succeed())

				inUse, err := ResourcesExist(ctx, fakeClient, objList, scheme, listOpts...)
				Expect(err).NotTo(HaveOccurred())
				Expect(inUse).To(BeTrue())
			})

			It("should return false because no objects found", func() {
				inUse, err := ResourcesExist(ctx, fakeClient, objList, scheme, listOpts...)
				Expect(err).NotTo(HaveOccurred())
				Expect(inUse).To(BeFalse())
			})
		})
	})

	Describe("#MakeUnique", func() {
		var (
			name                 = "some-name"
			nameWithHyphenSuffix = name + "-"
			labels               = map[string]string{"foo": "bar"}
		)

		It("should do nothing for resources not ConfigMap or Secret", func() {
			Expect(MakeUnique(&corev1.Pod{})).To(MatchError(ContainSubstring("unhandled object type")))
		})

		It("should properly make the ConfigMap immutable", func() {
			var (
				configMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: labels,
					},
					Data:       map[string]string{"foo": "bar"},
					BinaryData: map[string][]byte{"bar": []byte("foo")},
				}
				expectedConfigMap = configMap.DeepCopy()
			)

			expectedConfigMap.Name += "-ec321de5"
			expectedConfigMap.Immutable = ptr.To(true)
			expectedConfigMap.Labels["resources.gardener.cloud/garbage-collectable-reference"] = "true"

			Expect(MakeUnique(configMap)).To(Succeed())
			Expect(configMap).To(Equal(expectedConfigMap))
		})

		It("should properly make the Secret immutable", func() {
			var (
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:   nameWithHyphenSuffix,
						Labels: labels,
					},
					Data:       map[string][]byte{"foo": []byte("bar")},
					StringData: map[string]string{"bar": "foo"},
				}
				expectedSecret = secret.DeepCopy()
			)

			expectedSecret.Name += "ec321de5"
			expectedSecret.Immutable = ptr.To(true)
			expectedSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"] = "true"

			Expect(MakeUnique(secret)).To(Succeed())
			Expect(secret).To(Equal(expectedSecret))
		})
	})
})
