// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/managedresource"
	"github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

var _ = Describe("#MapSecretToManagedResources", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		m          handler.MapFunc
		secret     *corev1.Secret
		filter     *predicate.ClassFilter
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mr-secret",
				Namespace: "mr-namespace",
			},
		}

		filter = predicate.NewClassFilter("seed")

		m = (&Reconciler{SourceClient: fakeClient}).MapSecretToManagedResources(filter)
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
		fakeErr := errors.New("fake")
		listFakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
			List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
				return fakeErr
			},
		}).Build()
		m = (&Reconciler{SourceClient: listFakeClient}).MapSecretToManagedResources(filter)

		requests := m(ctx, secret)
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if there are no ManagedResources", func() {
		requests := m(ctx, secret)
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if there are no ManagedResources we are responsible for", func() {
		mr := &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-mr",
				Namespace: secret.Namespace,
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{Class: ptr.To("other")},
		}
		Expect(fakeClient.Create(ctx, mr)).To(Succeed())

		requests := m(ctx, secret)
		Expect(requests).To(BeEmpty())
	})

	It("should correctly map to ManagedResources that reference the secret", func() {
		mr := &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mr",
				Namespace: secret.Namespace,
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class:      ptr.To(filter.ResourceClass()),
				SecretRefs: []corev1.LocalObjectReference{{Name: secret.Name}},
			},
		}
		Expect(fakeClient.Create(ctx, mr)).To(Succeed())

		requests := m(ctx, secret)
		Expect(requests).To(ConsistOf(
			reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      mr.Name,
				Namespace: mr.Namespace,
			}},
		))
	})
})
