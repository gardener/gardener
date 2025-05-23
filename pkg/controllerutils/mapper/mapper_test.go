// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mapper_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllerutils/mapper"
	mockcache "github.com/gardener/gardener/third_party/mock/controller-runtime/cache"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
)

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handler Suite")
}

var _ = Describe("Controller Mapper", func() {
	var (
		ctx = context.TODO()

		fakeClient client.Client
		ctrl       *gomock.Controller
		cache      *mockcache.MockCache
		mgr        *mockmanager.MockManager

		namespace = "some-namespace"
		cluster   *extensionsv1alpha1.Cluster

		newObjListFunc func() client.ObjectList
		infra          *extensionsv1alpha1.Infrastructure
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		ctrl = gomock.NewController(GinkgoT())
		cache = mockcache.NewMockCache(ctrl)
		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetCache().Return(cache).AnyTimes()

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		newObjListFunc = func() client.ObjectList { return &extensionsv1alpha1.InfrastructureList{} }
		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "infra",
				Namespace: namespace,
			},
		}
	})

	Describe("#ClusterToObjectMapper", func() {
		var mapper handler.MapFunc

		BeforeEach(func() {
			mapper = ClusterToObjectMapper(fakeClient, newObjListFunc, nil)
		})

		It("should find all objects for the passed cluster", func() {
			Expect(fakeClient.Create(ctx, infra)).To(Succeed())

			Expect(mapper(ctx, cluster)).To(ConsistOf(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      infra.Name,
					Namespace: infra.Namespace,
				},
			}))
		})

		It("should find no objects for the passed cluster because predicates do not match", func() {
			mapper = ClusterToObjectMapper(fakeClient, newObjListFunc, []predicate.Predicate{
				predicate.Funcs{GenericFunc: func(_ event.GenericEvent) bool {
					return false
				}},
			})

			Expect(fakeClient.Create(ctx, infra)).To(Succeed())

			Expect(mapper(ctx, cluster)).To(BeEmpty())
		})

		It("should find no objects because list is empty", func() {
			Expect(mapper(ctx, cluster)).To(BeEmpty())
		})

		It("should find no objects because the passed object is no cluster", func() {
			Expect(mapper(ctx, infra)).To(BeEmpty())
		})
	})

	Describe("#ObjectListToRequests", func() {
		list := &corev1.SecretList{
			Items: []corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: "namespace2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace3"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace4"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace5"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "namespace6"}},
			},
		}

		It("should return the correct requests w/p predicates", func() {
			Expect(ObjectListToRequests(list)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace1"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace2"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret2", Namespace: "namespace2"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace3"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace4"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace5"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace6"}},
			))
		})

		It("should return the correct requests w/ predicates", func() {
			var (
				predicate1 = func(o client.Object) bool { return o.GetNamespace() != "namespace3" }
				predicate2 = func(o client.Object) bool { return o.GetNamespace() != "namespace5" }
			)

			Expect(ObjectListToRequests(list, predicate1, predicate2)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace1"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace2"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret2", Namespace: "namespace2"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace4"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace6"}},
			))
		})
	})
})
