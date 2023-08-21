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

package secret_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	secretcontroller "github.com/gardener/gardener/pkg/resourcemanager/controller/secret"
	"github.com/gardener/gardener/pkg/resourcemanager/predicate"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("SecretReconciler", func() {
	var (
		ctx = context.Background()

		ctrl *gomock.Controller
		c    *mockclient.MockClient

		r           *secretcontroller.Reconciler
		classFilter *predicate.ClassFilter
		secret      *corev1.Secret
		secretReq   reconcile.Request
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		classFilter = predicate.NewClassFilter("seed")
		r = &secretcontroller.Reconciler{SourceClient: c, ClassFilter: classFilter}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "mr-ns",
				Name:            "mr-secret",
				ResourceVersion: "42",
			},
		}
		secretReq = reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: secret.Namespace,
			Name:      secret.Name,
		}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		It("should do nothing if the secret has been deleted", func() {
			c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
				Return(apierrors.NewNotFound(corev1.Resource("secrets"), secret.Name))

			res, err := r.Reconcile(ctx, secretReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{
				Requeue: false,
			}))
		})

		It("should do nothing if secret get fails", func() {
			fakeErr := fmt.Errorf("fake")

			c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
				Return(fakeErr)

			_, err := r.Reconcile(ctx, secretReq)
			Expect(err).To(MatchError(ContainSubstring("fake")))
		})

		It("should do nothing if MR list fails", func() {
			fakeErr := fmt.Errorf("fake")

			gomock.InOrder(
				c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
						secret.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					}),
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
					Return(fakeErr),
			)

			_, err := r.Reconcile(ctx, secretReq)
			Expect(err).To(MatchError(ContainSubstring("fake")))
		})

		It("should do nothing if there is no MR in namespace", func() {
			gomock.InOrder(
				c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
						secret.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					}),
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
					Return(nil),
			)

			res, err := r.Reconcile(ctx, secretReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{
				Requeue: false,
			}))
		})

		It("should do nothing if there is no MR which we are responsible for", func() {
			mrs := []resourcesv1alpha1.ManagedResource{{
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String("other"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: "foo",
					}},
				},
			}}

			gomock.InOrder(
				c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
						secret.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					}),
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
					DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
						list.(*resourcesv1alpha1.ManagedResourceList).Items = mrs
						return nil
					}),
			)

			res, err := r.Reconcile(ctx, secretReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{
				Requeue: false,
			}))
		})

		It("should do nothing if there is no MR referencing this secret", func() {
			mrs := []resourcesv1alpha1.ManagedResource{{
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String(classFilter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: "foo",
					}},
				},
			}}

			gomock.InOrder(
				c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
						secret.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					}),
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
					DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
						list.(*resourcesv1alpha1.ManagedResourceList).Items = mrs
						return nil
					}),
			)

			res, err := r.Reconcile(ctx, secretReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{
				Requeue: false,
			}))
		})

		It("should do nothing if finalizer was already added", func() {
			secret.Finalizers = []string{classFilter.FinalizerName()}

			mrs := []resourcesv1alpha1.ManagedResource{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "MR1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String(classFilter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: secret.Name,
					}},
				},
			}}

			c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
					secret.DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
				DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					list.(*resourcesv1alpha1.ManagedResourceList).Items = mrs
					return nil
				})

			secretAfter := secret.DeepCopy()
			secretAfter.SetLabels(map[string]string{v1alpha1.ReferencedBy: "MR1"})
			test.EXPECTPatchWithOptimisticLock(gomock.Any(), c, secretAfter, secret, types.MergePatchType)

			res, err := r.Reconcile(ctx, secretReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{
				Requeue: false,
			}))
		})

		It("should add finalizer to secret if referenced by MR", func() {
			mrs := []resourcesv1alpha1.ManagedResource{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "MR1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String(classFilter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: secret.Name,
					}},
				},
			}}

			c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
					secret.DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
				DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					list.(*resourcesv1alpha1.ManagedResourceList).Items = mrs
					return nil
				})

			secretAfter := secret.DeepCopy()
			secretAfter.SetLabels(map[string]string{v1alpha1.ReferencedBy: "MR1"})
			test.EXPECTPatchWithOptimisticLock(gomock.Any(), c, secretAfter, secret, types.MergePatchType)

			secretAfter2 := secretAfter.DeepCopy()
			secretAfter2.SetFinalizers([]string{classFilter.FinalizerName()})
			test.EXPECTPatchWithOptimisticLock(gomock.Any(), c, secretAfter2, secretAfter, types.MergePatchType)

			res, err := r.Reconcile(ctx, secretReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{
				Requeue: false,
			}))
		})

		It("should do nothing if finalizer was already removed", func() {
			mrs := []resourcesv1alpha1.ManagedResource{{
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String(classFilter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{},
				},
			}}

			c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
					secret.DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
				DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					list.(*resourcesv1alpha1.ManagedResourceList).Items = mrs
					return nil
				})

			res, err := r.Reconcile(ctx, secretReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{
				Requeue: false,
			}))
		})

		It("should remove finalizer from secret if reference was removed", func() {
			secret.Finalizers = []string{classFilter.FinalizerName()}

			mrs := []resourcesv1alpha1.ManagedResource{{
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      pointer.String(classFilter.ResourceClass()),
					SecretRefs: []corev1.LocalObjectReference{},
				},
			}}

			c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
					secret.DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
				DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					list.(*resourcesv1alpha1.ManagedResourceList).Items = mrs
					return nil
				})

			secretAfter := secret.DeepCopy()
			secretAfter.SetFinalizers(nil)
			test.EXPECTPatchWithOptimisticLock(gomock.Any(), c, secretAfter, secret, types.MergePatchType)

			res, err := r.Reconcile(ctx, secretReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{
				Requeue: false,
			}))
		})

		It("should remove finalizer from secret if class changed", func() {
			secret.Finalizers = []string{classFilter.FinalizerName()}

			mrs := []resourcesv1alpha1.ManagedResource{{
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String("other"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: secret.Name,
					}},
				},
			}}

			c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
					secret.DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
				DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					list.(*resourcesv1alpha1.ManagedResourceList).Items = mrs
					return nil
				})

			secretAfter := secret.DeepCopy()
			secretAfter.SetFinalizers(nil)
			test.EXPECTPatchWithOptimisticLock(gomock.Any(), c, secretAfter, secret, types.MergePatchType)

			res, err := r.Reconcile(ctx, secretReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{
				Requeue: false,
			}))
		})

		It("should requeue if secret update fails", func() {
			secret.Finalizers = []string{classFilter.FinalizerName()}

			mrs := []resourcesv1alpha1.ManagedResource{{
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String("other"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: secret.Name,
					}},
				},
			}}

			c.EXPECT().Get(gomock.Any(), secretReq.NamespacedName, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
					secret.DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})
			c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
				DoAndReturn(func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					list.(*resourcesv1alpha1.ManagedResourceList).Items = mrs
					return nil
				})

			secretAfter := secret.DeepCopy()
			secretAfter.SetFinalizers(nil)
			test.EXPECTPatchWithOptimisticLock(gomock.Any(), c, secretAfter, secret, types.MergePatchType, fmt.Errorf("fake"))

			res, err := r.Reconcile(ctx, secretReq)
			Expect(err).To(MatchError(ContainSubstring("fake")))
			Expect(res).To(Equal(reconcile.Result{}))
		})
	})
})
