// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istioclusterconfiguration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioapinetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("IstioClusterConfiguration controller tests", Serial, func() {
	var (
		sourceNamespace       *corev1.Namespace
		istioIngressNamespace *corev1.Namespace
		service               *corev1.Service
		destinationRule       *istionetworkingv1beta1.DestinationRule
	)

	BeforeEach(func() {
		sourceNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-source-",
			},
		}
		Expect(testClient.Create(ctx, sourceNamespace)).To(Succeed())
		log.Info("Created source namespace", "name", sourceNamespace.Name)

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, sourceNamespace)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(sourceNamespace), sourceNamespace)
			}).Should(BeNotFoundError())
		})

		istioIngressNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-istio-ingress-",
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress,
				},
			},
		}
		Expect(testClient.Create(ctx, istioIngressNamespace)).To(Succeed())
		log.Info("Created istio-ingress namespace", "name", istioIngressNamespace.Name)

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, istioIngressNamespace)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(istioIngressNamespace), istioIngressNamespace)
			}).Should(BeNotFoundError())
		})

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: sourceNamespace.Name,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{Name: "https-main", Port: 443},
				},
			},
		}
		Expect(testClient.Create(ctx, service)).To(Succeed())

		destinationRule = &istionetworkingv1beta1.DestinationRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: sourceNamespace.Name,
			},
			Spec: istioapinetworkingv1beta1.DestinationRule{
				Host:     service.Name + "." + sourceNamespace.Name + ".svc.cluster.local",
				ExportTo: []string{"*"},
			},
		}
		Expect(testClient.Create(ctx, destinationRule)).To(Succeed())
	})

	envoyFilterName := func(sourceNamespaceName string) string {
		return sourceNamespaceName + "-cluster-configuration"
	}

	It("should create an EnvoyFilter in the istio-ingress namespace", func() {
		Eventually(func(g Gomega) {
			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			g.Expect(testClient.Get(ctx, types.NamespacedName{
				Name:      envoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace.Name,
			}, envoyFilter)).To(Succeed())

			g.Expect(envoyFilter.Labels).To(HaveKeyWithValue("resources.gardener.cloud/managed-by", "istio-cluster-configuration"))
			g.Expect(envoyFilter.OwnerReferences).To(HaveLen(1))
			g.Expect(envoyFilter.OwnerReferences[0].Name).To(Equal(sourceNamespace.Name))
			g.Expect(envoyFilter.OwnerReferences[0].Kind).To(Equal("Namespace"))

			g.Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))
			patch := envoyFilter.Spec.ConfigPatches[0]
			g.Expect(patch.ApplyTo).To(Equal(istioapinetworkingv1alpha3.EnvoyFilter_CLUSTER))
			g.Expect(patch.Match.Context).To(Equal(istioapinetworkingv1alpha3.EnvoyFilter_GATEWAY))
			g.Expect(patch.Match.GetCluster().Name).To(Equal("outbound|443||" + service.Name + "." + sourceNamespace.Name + ".svc.cluster.local"))
			g.Expect(patch.Patch.Operation).To(Equal(istioapinetworkingv1alpha3.EnvoyFilter_Patch_MERGE))
			g.Expect(patch.Patch.Value.Fields["per_connection_buffer_limit_bytes"].GetNumberValue()).To(Equal(float64(32768)))
			g.Expect(patch.Patch.Value.Fields).NotTo(HaveKey("typed_extension_protocol_options"))
		}).Should(Succeed())
	})

	It("should update EnvoyFilter when service port name changes to gRPC", func() {
		Eventually(func(g Gomega) {
			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			g.Expect(testClient.Get(ctx, types.NamespacedName{
				Name:      envoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace.Name,
			}, envoyFilter)).To(Succeed())
		}).Should(Succeed())

		By("Update port name to grpc")
		patch := client.MergeFrom(service.DeepCopy())
		service.Spec.Ports[0].Name = "grpc-main"
		Expect(testClient.Patch(ctx, service, patch)).To(Succeed())

		Eventually(func(g Gomega) {
			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			g.Expect(testClient.Get(ctx, types.NamespacedName{
				Name:      envoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace.Name,
			}, envoyFilter)).To(Succeed())

			g.Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))
			g.Expect(envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields).To(HaveKey("typed_extension_protocol_options"))
		}).Should(Succeed())
	})

	It("should delete EnvoyFilter when DestinationRule is removed", func() {
		Eventually(func(g Gomega) {
			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			g.Expect(testClient.Get(ctx, types.NamespacedName{
				Name:      envoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace.Name,
			}, envoyFilter)).To(Succeed())
		}).Should(Succeed())

		By("Delete DestinationRule")
		Expect(testClient.Delete(ctx, destinationRule)).To(Succeed())

		Eventually(func(g Gomega) {
			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			err := testClient.Get(ctx, types.NamespacedName{
				Name:      envoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace.Name,
			}, envoyFilter)
			g.Expect(err).To(BeNotFoundError())
		}).Should(Succeed())
	})

	It("should create EnvoyFilters in multiple istio-ingress namespaces", func() {
		istioIngressNamespace2 := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-istio-ingress2-",
				Labels: map[string]string{
					v1beta1constants.LabelExposureClassHandlerName: "test-handler",
				},
			},
		}
		Expect(testClient.Create(ctx, istioIngressNamespace2)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, istioIngressNamespace2)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(istioIngressNamespace2), istioIngressNamespace2)
			}).Should(BeNotFoundError())
		})

		Eventually(func(g Gomega) {
			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			g.Expect(testClient.Get(ctx, types.NamespacedName{
				Name:      envoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace.Name,
			}, envoyFilter)).To(Succeed())
			g.Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			g.Expect(testClient.Get(ctx, types.NamespacedName{
				Name:      envoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace2.Name,
			}, envoyFilter)).To(Succeed())
			g.Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))
		}).Should(Succeed())
	})

	It("should respect exportTo restriction", func() {
		istioIngressNamespace2 := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-istio-ingress2-",
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress,
				},
			},
		}
		Expect(testClient.Create(ctx, istioIngressNamespace2)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, istioIngressNamespace2)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(istioIngressNamespace2), istioIngressNamespace2)
			}).Should(BeNotFoundError())
		})

		By("Update exportTo to target only the first istio-ingress namespace")
		patch := client.MergeFrom(destinationRule.DeepCopy())
		destinationRule.Spec.ExportTo = []string{istioIngressNamespace.Name}
		Expect(testClient.Patch(ctx, destinationRule, patch)).To(Succeed())

		Eventually(func(g Gomega) {
			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			g.Expect(testClient.Get(ctx, types.NamespacedName{
				Name:      envoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace.Name,
			}, envoyFilter)).To(Succeed())
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			envoyFilterList := &istionetworkingv1alpha3.EnvoyFilterList{}
			g.Expect(testClient.List(ctx, envoyFilterList, client.InNamespace(istioIngressNamespace2.Name))).To(Succeed())
			g.Expect(envoyFilterList.Items).To(BeEmpty())
		}).Should(Succeed())
	})

	It("should react when a new istio-ingress namespace is created", func() {
		Eventually(func(g Gomega) {
			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			g.Expect(testClient.Get(ctx, types.NamespacedName{
				Name:      envoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace.Name,
			}, envoyFilter)).To(Succeed())
		}).Should(Succeed())

		By("Create a new istio-ingress namespace")
		istioIngressNamespace2 := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-istio-ingress-new-",
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress,
				},
			},
		}
		Expect(testClient.Create(ctx, istioIngressNamespace2)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, istioIngressNamespace2)).To(Or(Succeed(), BeNotFoundError()))
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(istioIngressNamespace2), istioIngressNamespace2)
			}).Should(BeNotFoundError())
		})

		Eventually(func(g Gomega) {
			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			g.Expect(testClient.Get(ctx, types.NamespacedName{
				Name:      envoyFilterName(sourceNamespace.Name),
				Namespace: istioIngressNamespace2.Name,
			}, envoyFilter)).To(Succeed())
			g.Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))
		}).Should(Succeed())
	})
})
