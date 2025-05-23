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
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/networkpolicy"
)

var _ = Describe("Add", func() {
	var (
		ctx        = context.TODO()
		log        logr.Logger
		fakeClient client.Client
		reconciler *Reconciler
	)

	BeforeEach(func() {
		log = logr.Discard()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(resourcemanagerclient.TargetScheme).Build()
		reconciler = &Reconciler{
			TargetClient: fakeClient,
		}
	})

	Describe("#ServicePredicate", func() {
		var (
			p       predicate.Predicate
			service *corev1.Service
		)

		BeforeEach(func() {
			p = reconciler.ServicePredicate()
			service = &corev1.Service{}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no service", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no service", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: service})).To(BeFalse())
			})

			It("should return false because nothing changed", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: service, ObjectNew: service})).To(BeFalse())
			})

			It("should return true because the deletion timestamp was set", func() {
				oldService := service.DeepCopy()
				service.DeletionTimestamp = &metav1.Time{}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldService, ObjectNew: service})).To(BeTrue())
			})

			It("should return true because the selector was changed", func() {
				oldService := service.DeepCopy()
				service.Spec.Selector = map[string]string{"foo": "bar"}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldService, ObjectNew: service})).To(BeTrue())
			})

			It("should return true because the ports were changed", func() {
				oldService := service.DeepCopy()
				service.Spec.Ports = []corev1.ServicePort{{}}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldService, ObjectNew: service})).To(BeTrue())
			})

			It("should return true because the namespace-selectors annotation was changed", func() {
				oldService := service.DeepCopy()
				service.Annotations = map[string]string{"networking.resources.gardener.cloud/namespace-selectors": "foo"}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldService, ObjectNew: service})).To(BeTrue())
			})

			It("should return true because the pod-label-selector-namespace-alias annotation was changed", func() {
				oldService := service.DeepCopy()
				service.Annotations = map[string]string{"networking.resources.gardener.cloud/pod-label-selector-namespace-alias": "foo"}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldService, ObjectNew: service})).To(BeTrue())
			})

			It("should return true because the from-world-to-ports annotation was changed", func() {
				oldService := service.DeepCopy()
				service.Annotations = map[string]string{"networking.resources.gardener.cloud/from-world-to-ports": "foo"}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldService, ObjectNew: service})).To(BeTrue())
			})

			It("should return true because a custom pod label selector was added", func() {
				oldService := service.DeepCopy()
				service.Annotations = map[string]string{"networking.resources.gardener.cloud/from-foo-allowed-ports": "foo"}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldService, ObjectNew: service})).To(BeTrue())
			})

			It("should return true because a custom pod label selector was removed", func() {
				oldService := service.DeepCopy()
				oldService.Annotations = map[string]string{"networking.resources.gardener.cloud/from-foo-allowed-ports": "foo"}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldService, ObjectNew: service})).To(BeTrue())
			})

			It("should return true because a custom pod label selector was changed", func() {
				oldService := service.DeepCopy()
				oldService.Annotations = map[string]string{"networking.resources.gardener.cloud/from-foo-allowed-ports": "foo"}
				service.Annotations = map[string]string{"networking.resources.gardener.cloud/from-foo-allowed-ports": "bar"}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldService, ObjectNew: service})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})

	Describe("#IngressPredicate", func() {
		var (
			p       predicate.Predicate
			ingress *networkingv1.Ingress
		)

		BeforeEach(func() {
			p = reconciler.IngressPredicate()
			ingress = &networkingv1.Ingress{}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no ingress", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no ingress", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: ingress})).To(BeFalse())
			})

			It("should return false because nothing changed", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: ingress, ObjectNew: ingress})).To(BeFalse())
			})

			It("should return true because the rules were changed", func() {
				oldIngress := ingress.DeepCopy()
				ingress.Spec.Rules = append(ingress.Spec.Rules, networkingv1.IngressRule{})

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldIngress, ObjectNew: ingress})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})

	Describe("#MapNetworkPolicyToService", func() {
		var (
			serviceName      = "svc"
			serviceNamespace = "svcns"
			networkPolicy    *networkingv1.NetworkPolicy
		)

		BeforeEach(func() {
			networkPolicy = &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				"networking.resources.gardener.cloud/service-name":      serviceName,
				"networking.resources.gardener.cloud/service-namespace": serviceNamespace,
			}}}
		})

		It("should map to the referenced service", func() {
			Expect(reconciler.MapNetworkPolicyToService(ctx, networkPolicy)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: serviceNamespace, Name: serviceName}},
			))
		})

		It("should return nil if the object is has no reference", func() {
			networkPolicy.Labels = nil
			Expect(reconciler.MapNetworkPolicyToService(ctx, networkPolicy)).To(BeNil())
		})

		It("should return nil if the object is nil", func() {
			Expect(reconciler.MapNetworkPolicyToService(ctx, nil)).To(BeNil())
		})
	})

	Describe("#MapToAllServices", func() {
		var (
			service1 *corev1.Service
			service2 *corev1.Service
		)

		BeforeEach(func() {
			service1 = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "service1", Namespace: "namespace1"}}
			service2 = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "service2", Namespace: "namespace2"}}
		})

		It("should map to all services", func() {
			Expect(fakeClient.Create(ctx, service1)).To(Succeed())
			Expect(fakeClient.Create(ctx, service2)).To(Succeed())

			Expect(reconciler.MapToAllServices(log)(ctx, nil)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: service1.Namespace, Name: service1.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: service2.Namespace, Name: service2.Name}},
			))
		})

		It("should return nil if there are no services", func() {
			Expect(reconciler.MapToAllServices(log)(ctx, nil)).To(BeNil())
		})
	})

	Describe("#MapIngressToServices", func() {
		It("should map to all referenced services", func() {
			var (
				namespace = "some-namespace"
				service1  = "svc1"
				service2  = "svc2"
				service3  = "svc3"

				ingress = &networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Backend: networkingv1.IngressBackend{
													Resource: &corev1.TypedLocalObjectReference{
														Name: "foo",
													},
												},
											},
										},
									},
								},
							},
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: service1,
													},
												},
											},
										},
									},
								},
							},
							{
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: service2,
													},
												},
											},
											{
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: service3,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}
			)

			Expect(reconciler.MapIngressToServices(ctx, ingress)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: service1}},
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: service2}},
				reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: service3}},
			))
		})

		It("should return nil if there are no referenced services", func() {
			Expect(reconciler.MapToAllServices(log)(ctx, &networkingv1.Ingress{Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "foo"}}}})).To(BeNil())
		})

		It("should return nil if the passed object is nil", func() {
			Expect(reconciler.MapToAllServices(log)(ctx, nil)).To(BeNil())
		})
	})
})
