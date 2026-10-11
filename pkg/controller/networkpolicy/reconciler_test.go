// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/controller/networkpolicy"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		reconciler *Reconciler
		namespace  *corev1.Namespace
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		reconciler = &Reconciler{RuntimeClient: fakeClient}

		// A namespace which does not match any of the configured network policy selectors.
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "user-namespace"},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
		}
		Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
	})

	It("keeps a NetworkPolicy that is not managed by Gardener", func() {
		userPolicy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "deny-all", Namespace: namespace.Name},
		}
		Expect(fakeClient.Create(ctx, userPolicy)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.Name}})
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeClient.Get(ctx, types.NamespacedName{Name: "deny-all", Namespace: namespace.Name}, &networkingv1.NetworkPolicy{})).To(Succeed())
	})

	It("deletes a NetworkPolicy that is managed by Gardener", func() {
		gardenerPolicy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "deny-all",
				Namespace:   namespace.Name,
				Annotations: map[string]string{v1beta1constants.GardenerDescription: "Disables all ingress and egress traffic."},
			},
		}
		Expect(fakeClient.Create(ctx, gardenerPolicy)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.Name}})
		Expect(err).NotTo(HaveOccurred())

		err = fakeClient.Get(ctx, types.NamespacedName{Name: "deny-all", Namespace: namespace.Name}, &networkingv1.NetworkPolicy{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected NetworkPolicy to be deleted")
	})
})
