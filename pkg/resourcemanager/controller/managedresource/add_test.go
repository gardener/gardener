// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/managedresource"
	"github.com/gardener/gardener/pkg/resourcemanager/predicate"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("#MapSecretToManagedResources", func() {
	var (
		ctx    = context.TODO()
		c      *mockclient.MockClient
		ctrl   *gomock.Controller
		m      handler.MapFunc
		secret *corev1.Secret
		filter *predicate.ClassFilter
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mr-secret",
				Namespace: "mr-namespace",
			},
		}

		filter = predicate.NewClassFilter("seed")

		m = (&Reconciler{SourceClient: c}).MapSecretToManagedResources(filter)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should do nothing, if Object is nil", func() {
		requests := m(ctx, nil)
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if Object is not a Secret", func() {
		requests := m(ctx, &corev1.Pod{})
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if list fails", func() {
		c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
			Return(errors.New("fake"))

		requests := m(ctx, secret)
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if there are no ManagedResources", func() {
		c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace))

		requests := m(ctx, secret)
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if there are no ManagedResources we are responsible for", func() {
		mr := resourcesv1alpha1.ManagedResource{
			Spec: resourcesv1alpha1.ManagedResourceSpec{Class: ptr.To("other")},
		}

		c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
			DoAndReturn(func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
				list.(*resourcesv1alpha1.ManagedResourceList).Items = []resourcesv1alpha1.ManagedResource{mr}
				return nil
			})

		requests := m(ctx, secret)
		Expect(requests).To(BeEmpty())
	})

	It("should correctly map to ManagedResources that reference the secret", func() {
		mr := resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mr",
				Namespace: secret.Namespace,
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class:      ptr.To(filter.ResourceClass()),
				SecretRefs: []corev1.LocalObjectReference{{Name: secret.Name}},
			},
		}

		c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResourceList{}), client.InNamespace(secret.Namespace)).
			DoAndReturn(func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
				list.(*resourcesv1alpha1.ManagedResourceList).Items = []resourcesv1alpha1.ManagedResource{mr}
				return nil
			})

		requests := m(ctx, secret)
		Expect(requests).To(ConsistOf(
			reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      mr.Name,
				Namespace: mr.Namespace,
			}},
		))
	})
})
