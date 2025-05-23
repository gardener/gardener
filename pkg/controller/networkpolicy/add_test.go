// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/controller/networkpolicy"
)

var _ = Describe("Add", func() {
	var (
		ctx = context.TODO()

		networkPolicy *networkingv1.NetworkPolicy
		reconciler    *Reconciler
	)

	BeforeEach(func() {
		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		}
		reconciler = &Reconciler{}
	})

	Describe("#NetworkPolicyPredicate", func() {
		var (
			p             predicate.Predicate
			networkPolicy *networkingv1.NetworkPolicy
		)

		BeforeEach(func() {
			p = reconciler.NetworkPolicyPredicate()
			networkPolicy = &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"}}
		})

		It("should return true because the NetworkPolicy has name 'allow-to-runtime-apiserver'", func() {
			networkPolicy.Name = "allow-to-runtime-apiserver"
			Expect(p.Create(event.CreateEvent{Object: networkPolicy})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: networkPolicy})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: networkPolicy})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: networkPolicy})).To(BeTrue())
		})

		It("should return false because the NetworkPolicy is not managed by this reconciler", func() {
			networkPolicy.Name = "not-managed"
			Expect(p.Create(event.CreateEvent{Object: networkPolicy})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: networkPolicy})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: networkPolicy})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: networkPolicy})).To(BeFalse())
		})
	})

	Describe("#MapToNamespaces", func() {
		var (
			ctx        = context.TODO()
			log        = logr.Discard()
			fakeClient client.Client

			gardenNamespace             *corev1.Namespace
			istioSystemNamespace        *corev1.Namespace
			istioIngressNamespace       *corev1.Namespace
			istioExposureClassNamespace *corev1.Namespace
			shootNamespace              *corev1.Namespace
			extensionNamespace          *corev1.Namespace
			fooNamespace                *corev1.Namespace
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			reconciler.RuntimeClient = fakeClient

			gardenNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "garden",
					Labels: map[string]string{"kubernetes.io/metadata.name": "garden"},
				},
			}
			istioSystemNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "istio-system-123a4",
					Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioSystem},
				},
			}
			istioIngressNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "istio-ingress-123a4",
					Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress},
				},
			}
			istioExposureClassNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "istio-ingress-handler-foo-123a4",
					Labels: map[string]string{v1beta1constants.LabelExposureClassHandlerName: ""},
				},
			}
			shootNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "shoot--bar",
					Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot},
				},
			}
			extensionNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "extension-baz",
					Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension},
				},
			}
			fooNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
		})

		It("should return a request with the relevant namespaces' names", func() {
			Expect(fakeClient.Create(ctx, gardenNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, istioSystemNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, istioIngressNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, istioExposureClassNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, shootNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, extensionNamespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, fooNamespace)).To(Succeed())

			Expect(reconciler.MapToNamespaces(log)(ctx, nil)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: gardenNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: istioSystemNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: istioIngressNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: istioExposureClassNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: shootNamespace.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: extensionNamespace.Name}},
			))
		})
	})

	Describe("#MapObjectToName", func() {
		It("should return a request with the networkpolicy's name", func() {
			Expect(reconciler.MapObjectToName(ctx, networkPolicy)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: networkPolicy.Name}},
			))
		})
	})

	Describe("#MapObjectToNamespace", func() {
		It("should return a request with the namespace's name", func() {
			Expect(reconciler.MapObjectToNamespace(ctx, networkPolicy)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: networkPolicy.Namespace}},
			))
		})
	})

	Describe("#IsKubernetesEndpoint", func() {
		var (
			p        predicate.Predicate
			endpoint *corev1.Endpoints
		)

		BeforeEach(func() {
			p = reconciler.IsKubernetesEndpoint()
			endpoint = &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"}}
		})

		It("should return true because the endpoint is the Kubernetes endpoint", func() {
			Expect(p.Create(event.CreateEvent{Object: endpoint})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: endpoint})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: endpoint})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: endpoint})).To(BeTrue())
		})

		It("should return false because the endpoint is not the Kubernetes endpoint", func() {
			endpoint.Name = "foo"

			Expect(p.Create(event.CreateEvent{Object: endpoint})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: endpoint})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: endpoint})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: endpoint})).To(BeFalse())
		})

		It("should return false because the endpoint is a Kubernetes endpoint in a different namespace", func() {
			endpoint.Namespace = "bar"

			Expect(p.Create(event.CreateEvent{Object: endpoint})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: endpoint})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: endpoint})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: endpoint})).To(BeFalse())
		})
	})
})
