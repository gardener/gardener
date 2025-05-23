// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionclusterrole_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/extensionclusterrole"
)

var _ = Describe("Add", func() {
	var (
		reconciler     *Reconciler
		serviceAccount *corev1.ServiceAccount
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "seed-foo",
				Name:      "baz",
				Labels:    map[string]string{"foo": "bar"},
			},
		}
	})

	Describe("ServiceAccountPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ServiceAccountPredicate()
		})

		tests := func(f func(obj client.Object) bool) {
			It("should return false because object is no ServiceAccount", func() {
				Expect(f(&corev1.ConfigMap{})).To(BeFalse())
			})

			It("should return false because namespace is not prefixed with 'seed-'", func() {
				serviceAccount.Namespace = "foo"
				Expect(f(serviceAccount)).To(BeFalse())
			})

			It("should return true because object matches all conditions", func() {
				Expect(f(serviceAccount)).To(BeTrue())
			})
		}

		Describe("#Create", func() {
			tests(func(obj client.Object) bool { return p.Create(event.CreateEvent{Object: obj}) })
		})

		Describe("#Update", func() {
			tests(func(obj client.Object) bool { return p.Update(event.UpdateEvent{ObjectNew: obj}) })
		})

		Describe("#Delete", func() {
			tests(func(obj client.Object) bool { return p.Delete(event.DeleteEvent{Object: obj}) })
		})

		Describe("#Generic", func() {
			tests(func(obj client.Object) bool { return p.Generic(event.GenericEvent{Object: obj}) })
		})
	})

	Describe("#MapToMatchingClusterRoles", func() {
		var (
			ctx                                      = context.Background()
			log                                      logr.Logger
			fakeClient                               client.Client
			clusterRole1, clusterRole2, clusterRole3 *rbacv1.ClusterRole
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			reconciler.Client = fakeClient

			clusterRole1 = &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
				Name:        "clusterRole1",
				Labels:      map[string]string{"authorization.gardener.cloud/custom-extensions-permissions": "true"},
				Annotations: map[string]string{"authorization.gardener.cloud/extensions-serviceaccount-selector": `{"matchLabels":{"foo":"bar"}}`},
			}}
			clusterRole2 = &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
				Name:        "clusterRole2",
				Labels:      map[string]string{"authorization.gardener.cloud/custom-extensions-permissions": "true"},
				Annotations: map[string]string{"authorization.gardener.cloud/extensions-serviceaccount-selector": `{"matchLabels":{"bar":"baz"}}`},
			}}
			clusterRole3 = &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "clusterRole3"}}

			Expect(fakeClient.Create(ctx, clusterRole1)).To(Succeed())
			Expect(fakeClient.Create(ctx, clusterRole2)).To(Succeed())
			Expect(fakeClient.Create(ctx, clusterRole3)).To(Succeed())
		})

		It("should map to all matching cluster roles", func() {
			Expect(reconciler.MapToMatchingClusterRoles(log)(ctx, serviceAccount)).To(HaveExactElements(reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterRole1.Name}}))
		})

		It("should map to fail when a selector cannot be parsed", func() {
			Expect(fakeClient.Create(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
				Name:        "clusterRole4",
				Labels:      map[string]string{"authorization.gardener.cloud/custom-extensions-permissions": "true"},
				Annotations: map[string]string{"authorization.gardener.cloud/extensions-serviceaccount-selector": `{cannot-parse-this`},
			}})).To(Succeed())

			Expect(reconciler.MapToMatchingClusterRoles(log)(ctx, serviceAccount)).To(HaveExactElements(reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterRole1.Name}}))
		})
	})
})
