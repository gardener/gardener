// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istioclusterconfiguration_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/istioclusterconfiguration"
)

var _ = Describe("Add", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		reconciler *Reconciler
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(resourcemanagerclient.TargetScheme).
			Build()
		reconciler = &Reconciler{
			TargetClient: fakeClient,
		}
	})

	Describe("#DestinationRulePredicate", func() {
		var (
			pred            predicate.Predicate
			destinationRule *istionetworkingv1beta1.DestinationRule
		)

		BeforeEach(func() {
			pred = reconciler.DestinationRulePredicate()
			destinationRule = &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc.ns.svc.cluster.local",
					ExportTo: []string{"*"},
				},
			}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(pred.Create(event.CreateEvent{Object: destinationRule})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is not a DestinationRule", func() {
				Expect(pred.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is not a DestinationRule", func() {
				Expect(pred.Update(event.UpdateEvent{ObjectNew: destinationRule})).To(BeFalse())
			})

			It("should return false because nothing changed", func() {
				Expect(pred.Update(event.UpdateEvent{ObjectOld: destinationRule, ObjectNew: destinationRule})).To(BeFalse())
			})

			It("should return true because host changed", func() {
				oldDestinationRule := destinationRule.DeepCopy()
				destinationRule.Spec.Host = "other.ns.svc.cluster.local"
				Expect(pred.Update(event.UpdateEvent{ObjectOld: oldDestinationRule, ObjectNew: destinationRule})).To(BeTrue())
			})

			It("should return true because exportTo changed", func() {
				oldDestinationRule := destinationRule.DeepCopy()
				destinationRule.Spec.ExportTo = []string{"."}
				Expect(pred.Update(event.UpdateEvent{ObjectOld: oldDestinationRule, ObjectNew: destinationRule})).To(BeTrue())
			})

			It("should return true because trafficPolicy changed", func() {
				oldDestinationRule := destinationRule.DeepCopy()
				destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
					ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
						Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
							UseClientProtocol: true,
						},
					},
				}
				Expect(pred.Update(event.UpdateEvent{ObjectOld: oldDestinationRule, ObjectNew: destinationRule})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(pred.Delete(event.DeleteEvent{Object: destinationRule})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(pred.Generic(event.GenericEvent{Object: destinationRule})).To(BeTrue())
			})
		})
	})

	Describe("#ServicePredicate", func() {
		var (
			pred    predicate.Predicate
			service *corev1.Service
		)

		BeforeEach(func() {
			pred = reconciler.ServicePredicate()
			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "ns"},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
				},
			}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(pred.Create(event.CreateEvent{Object: service})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is not a Service", func() {
				Expect(pred.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is not a Service", func() {
				Expect(pred.Update(event.UpdateEvent{ObjectNew: service})).To(BeFalse())
			})

			It("should return false because nothing changed", func() {
				Expect(pred.Update(event.UpdateEvent{ObjectOld: service, ObjectNew: service})).To(BeFalse())
			})

			It("should return true because ports changed", func() {
				oldService := service.DeepCopy()
				service.Spec.Ports = []corev1.ServicePort{{Name: "grpc", Port: 9090}}
				Expect(pred.Update(event.UpdateEvent{ObjectOld: oldService, ObjectNew: service})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(pred.Delete(event.DeleteEvent{Object: service})).To(BeTrue())
			})
		})
	})

	Describe("#NamespacePredicate", func() {
		var (
			pred      predicate.Predicate
			namespace *corev1.Namespace
		)

		BeforeEach(func() {
			pred = reconciler.NamespacePredicate()
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "istio-ingress",
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress,
					},
				},
			}
		})

		Describe("#Create", func() {
			It("should return true for istio-ingress namespace", func() {
				Expect(pred.Create(event.CreateEvent{Object: namespace})).To(BeTrue())
			})

			It("should return false for non-istio-ingress namespace", func() {
				namespace.Labels = nil
				Expect(pred.Create(event.CreateEvent{Object: namespace})).To(BeFalse())
			})
		})

		Describe("#Update", func() {
			It("should return false if label did not change", func() {
				Expect(pred.Update(event.UpdateEvent{ObjectOld: namespace, ObjectNew: namespace})).To(BeFalse())
			})

			It("should return true if label was added", func() {
				oldNamespace := namespace.DeepCopy()
				oldNamespace.Labels = nil
				Expect(pred.Update(event.UpdateEvent{ObjectOld: oldNamespace, ObjectNew: namespace})).To(BeTrue())
			})

			It("should return true if label was removed", func() {
				newNamespace := namespace.DeepCopy()
				newNamespace.Labels = nil
				Expect(pred.Update(event.UpdateEvent{ObjectOld: namespace, ObjectNew: newNamespace})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return true for istio-ingress namespace", func() {
				Expect(pred.Delete(event.DeleteEvent{Object: namespace})).To(BeTrue())
			})

			It("should return false for non-istio-ingress namespace", func() {
				namespace.Labels = nil
				Expect(pred.Delete(event.DeleteEvent{Object: namespace})).To(BeFalse())
			})
		})
	})

	Describe("#MapDestinationRuleToNamespace", func() {
		It("should return nil for nil object", func() {
			Expect(reconciler.MapDestinationRuleToNamespace(ctx, nil)).To(BeNil())
		})

		It("should map DestinationRule to its namespace", func() {
			destinationRule := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "shoot-ns"},
			}
			Expect(reconciler.MapDestinationRuleToNamespace(ctx, destinationRule)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-ns"}},
			))
		})
	})

	Describe("#MapServiceToNamespaces", func() {
		It("should return nil for non-Service object", func() {
			namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns"}}
			Expect(reconciler.MapServiceToNamespaces(ctx, namespace)).To(BeNil())
		})

		It("should map service to namespaces of DRs that reference it by FQDN", func() {
			destinationRule := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "shoot-ns-1"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host: "my-svc.shoot-ns-1.svc.cluster.local",
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule)).To(Succeed())

			destinationRule2 := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr2", Namespace: "shoot-ns-2"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host: "other-svc.shoot-ns-2.svc.cluster.local",
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule2)).To(Succeed())

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "shoot-ns-1"},
			}

			requests := reconciler.MapServiceToNamespaces(ctx, service)
			Expect(requests).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-ns-1"}},
			))
		})

		It("should map service to namespaces of DRs that reference it by short name", func() {
			destinationRule := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "shoot-ns"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host: "my-svc",
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule)).To(Succeed())

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "shoot-ns"},
			}

			requests := reconciler.MapServiceToNamespaces(ctx, service)
			Expect(requests).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-ns"}},
			))
		})

		It("should not match service when DR uses short name but is in a different namespace", func() {
			destinationRule := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "other-ns"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host: "my-svc",
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule)).To(Succeed())

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "shoot-ns"},
			}

			requests := reconciler.MapServiceToNamespaces(ctx, service)
			Expect(requests).To(BeEmpty())
		})

		It("should not return duplicates", func() {
			destinationRule1 := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "shoot-ns"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host: "my-svc.shoot-ns.svc.cluster.local",
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule1)).To(Succeed())

			destinationRule2 := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr2", Namespace: "shoot-ns"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host: "my-svc.shoot-ns.svc.cluster.local",
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule2)).To(Succeed())

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "shoot-ns"},
			}

			requests := reconciler.MapServiceToNamespaces(ctx, service)
			Expect(requests).To(HaveLen(1))
		})
	})

	Describe("#MapNamespaceToSourceNamespaces", func() {
		var istioIngressNamespace *corev1.Namespace

		BeforeEach(func() {
			istioIngressNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "istio-ingress",
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress,
					},
				},
			}
		})

		It("should return nil for nil object", func() {
			Expect(reconciler.MapNamespaceToSourceNamespaces(ctx, nil)).To(BeNil())
		})

		It("should return empty when no DestinationRules exist", func() {
			requests := reconciler.MapNamespaceToSourceNamespaces(ctx, istioIngressNamespace)
			Expect(requests).To(BeEmpty())
		})

		It("should return namespaces of DRs with empty exportTo (defaults to all)", func() {
			destinationRule := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "shoot-ns-1"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc1.shoot-ns-1.svc.cluster.local",
					ExportTo: nil,
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule)).To(Succeed())

			requests := reconciler.MapNamespaceToSourceNamespaces(ctx, istioIngressNamespace)
			Expect(requests).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-ns-1"}},
			))
		})

		It("should return namespaces of DRs with exportTo '*'", func() {
			destinationRule := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "shoot-ns-1"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc1.shoot-ns-1.svc.cluster.local",
					ExportTo: []string{"*"},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule)).To(Succeed())

			requests := reconciler.MapNamespaceToSourceNamespaces(ctx, istioIngressNamespace)
			Expect(requests).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-ns-1"}},
			))
		})

		It("should return namespaces of DRs that explicitly export to the namespace", func() {
			destinationRule := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "shoot-ns-1"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc1.shoot-ns-1.svc.cluster.local",
					ExportTo: []string{"istio-ingress"},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule)).To(Succeed())

			requests := reconciler.MapNamespaceToSourceNamespaces(ctx, istioIngressNamespace)
			Expect(requests).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-ns-1"}},
			))
		})

		It("should not return namespaces of DRs that export to a different namespace", func() {
			destinationRule := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "shoot-ns-1"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc1.shoot-ns-1.svc.cluster.local",
					ExportTo: []string{"other-istio-ingress"},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule)).To(Succeed())

			requests := reconciler.MapNamespaceToSourceNamespaces(ctx, istioIngressNamespace)
			Expect(requests).To(BeEmpty())
		})

		It("should return namespaces of DRs with exportTo '.' when DR is in the same namespace", func() {
			destinationRule := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "istio-ingress"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc1.istio-ingress.svc.cluster.local",
					ExportTo: []string{"."},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule)).To(Succeed())

			requests := reconciler.MapNamespaceToSourceNamespaces(ctx, istioIngressNamespace)
			Expect(requests).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "istio-ingress"}},
			))
		})

		It("should not return namespaces of DRs with exportTo '.' when DR is in a different namespace", func() {
			destinationRule := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "shoot-ns-1"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc1.shoot-ns-1.svc.cluster.local",
					ExportTo: []string{"."},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule)).To(Succeed())

			requests := reconciler.MapNamespaceToSourceNamespaces(ctx, istioIngressNamespace)
			Expect(requests).To(BeEmpty())
		})

		It("should not return duplicates when multiple DRs in same namespace match", func() {
			destinationRule1 := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "shoot-ns-1"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc1.shoot-ns-1.svc.cluster.local",
					ExportTo: []string{"*"},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule1)).To(Succeed())

			destinationRule2 := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr2", Namespace: "shoot-ns-1"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc2.shoot-ns-1.svc.cluster.local",
					ExportTo: []string{"istio-ingress"},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule2)).To(Succeed())

			requests := reconciler.MapNamespaceToSourceNamespaces(ctx, istioIngressNamespace)
			Expect(requests).To(HaveLen(1))
			Expect(requests).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-ns-1"}},
			))
		})

		It("should return multiple namespaces when DRs from different namespaces match", func() {
			destinationRule1 := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr1", Namespace: "shoot-ns-1"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc1.shoot-ns-1.svc.cluster.local",
					ExportTo: []string{"*"},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule1)).To(Succeed())

			destinationRule2 := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr2", Namespace: "shoot-ns-2"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc2.shoot-ns-2.svc.cluster.local",
					ExportTo: []string{"istio-ingress"},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule2)).To(Succeed())

			destinationRule3 := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "dr3", Namespace: "shoot-ns-3"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "svc3.shoot-ns-3.svc.cluster.local",
					ExportTo: []string{"other-ns"},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule3)).To(Succeed())

			requests := reconciler.MapNamespaceToSourceNamespaces(ctx, istioIngressNamespace)
			Expect(requests).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-ns-1"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "shoot-ns-2"}},
			))
		})
	})
})
