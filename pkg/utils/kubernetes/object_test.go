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
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Object", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx     = context.TODO()
		fakeErr = errors.New("fake err")

		obj1 = &corev1.Secret{}
		obj2 = &appsv1.Deployment{}
		objs = []client.Object{obj1, obj2}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeleteObjects", func() {
		It("should fail because an object fails to delete", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, obj1).Return(fakeErr),
			)

			Expect(DeleteObjects(ctx, c, objs...)).To(MatchError(fakeErr))
		})

		It("should fail because an object fails to delete", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, obj1),
				c.EXPECT().Delete(ctx, obj2).Return(fakeErr),
			)

			Expect(DeleteObjects(ctx, c, objs...)).To(MatchError(fakeErr))
		})

		It("should successfully delete all objects", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, obj1),
				c.EXPECT().Delete(ctx, obj2),
			)

			Expect(DeleteObjects(ctx, c, objs...)).To(Succeed())
		})
	})

	Describe("#DeleteObject", func() {
		It("should fail to delete the object", func() {
			c.EXPECT().Delete(ctx, obj1).Return(fakeErr)

			Expect(DeleteObject(ctx, c, obj1)).To(MatchError(fakeErr))
		})

		It("should not fail to delete the object (not found error)", func() {
			c.EXPECT().Delete(ctx, obj1).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			Expect(DeleteObject(ctx, c, obj1)).To(Succeed())
		})

		It("should not fail to delete the object (no match error)", func() {
			c.EXPECT().Delete(ctx, obj1).Return(&meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}})

			Expect(DeleteObject(ctx, c, obj1)).To(Succeed())
		})

		It("should successfully delete the object", func() {
			c.EXPECT().Delete(ctx, obj1)

			Expect(DeleteObject(ctx, c, obj1)).To(Succeed())
		})
	})

	Describe("#DeleteObjectsFromListConditionally", func() {
		var (
			obj1       = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "1"}}
			obj2       = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "2"}}
			obj3       = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "3"}}
			listObject = &corev1.NamespaceList{Items: []corev1.Namespace{*obj1, *obj2, *obj3}}

			predicateFn = func(obj runtime.Object) bool {
				acc, _ := meta.Accessor(obj)
				return acc.GetName() != "2"
			}
		)

		It("should return an error if deleting an object failed", func() {
			c.EXPECT().Delete(ctx, obj1)
			c.EXPECT().Delete(ctx, obj3).Return(fakeErr)

			err := DeleteObjectsFromListConditionally(ctx, c, listObject, predicateFn)
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
		})

		It("should successfully delete the relevant objects", func() {
			c.EXPECT().Delete(ctx, obj1)
			c.EXPECT().Delete(ctx, obj3)

			Expect(DeleteObjectsFromListConditionally(ctx, c, listObject, predicateFn)).To(Succeed())
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
			c.EXPECT().List(ctx, gomock.Any(), client.InNamespace(namespace), client.Limit(1)).Return(fakeErr)

			inUse, err := ResourcesExist(ctx, c, objList, scheme, listOpts...)
			Expect(err).To(MatchError(fakeErr))
			Expect(inUse).To(BeTrue())
		})

		Context("with partialObjectMetadataList", func() {
			var partialObjectMetadataList *metav1.PartialObjectMetadataList

			BeforeEach(func() {
				partialObjectMetadataList = &metav1.PartialObjectMetadataList{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "SecretList"}}
				listOpts = append(listOpts, client.MatchingFields{"metadata.name": "foo"})
			})

			It("should return true because objects found", func() {
				c.EXPECT().List(ctx, partialObjectMetadataList, client.InNamespace(namespace), client.MatchingFields{"metadata.name": "foo"}, client.Limit(1)).DoAndReturn(func(_ context.Context, list *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					(&metav1.PartialObjectMetadataList{Items: []metav1.PartialObjectMetadata{{}}}).DeepCopyInto(list)
					return nil
				})

				inUse, err := ResourcesExist(ctx, c, objList, scheme, listOpts...)
				Expect(err).NotTo(HaveOccurred())
				Expect(inUse).To(BeTrue())
			})

			It("should return false because no objects found", func() {
				c.EXPECT().List(ctx, partialObjectMetadataList, client.InNamespace(namespace), client.MatchingFields{"metadata.name": "foo"}, client.Limit(1))

				inUse, err := ResourcesExist(ctx, c, objList, scheme, listOpts...)
				Expect(err).NotTo(HaveOccurred())
				Expect(inUse).To(BeFalse())
			})
		})

		Context("with objectList", func() {
			BeforeEach(func() {
				listOpts = append(listOpts, client.MatchingFields{"data.foo": "bar"})
			})

			It("should return true because objects found", func() {
				c.EXPECT().List(ctx, objList, client.InNamespace(namespace), client.MatchingFields{"data.foo": "bar"}, client.Limit(1)).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{{}}
					return nil
				})

				inUse, err := ResourcesExist(ctx, c, objList, scheme, listOpts...)
				Expect(err).NotTo(HaveOccurred())
				Expect(inUse).To(BeTrue())
			})

			It("should return false because no objects found", func() {
				c.EXPECT().List(ctx, objList, client.InNamespace(namespace), client.MatchingFields{"data.foo": "bar"}, client.Limit(1))

				inUse, err := ResourcesExist(ctx, c, objList, scheme, listOpts...)
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
