// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istioclusterconfiguration_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/structpb"
	istioapinetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/resourcemanager/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/istioclusterconfiguration"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		reconciler *Reconciler

		sourceNamespace       *corev1.Namespace
		istioIngressNamespace *corev1.Namespace
		service               *corev1.Service
		destinationRule       *istionetworkingv1beta1.DestinationRule
		envoyFilterName       string
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(resourcemanagerclient.TargetScheme).
			Build()

		reconciler = &Reconciler{
			TargetClient: fakeClient,
			Config:       resourcemanagerconfigv1alpha1.IstioClusterConfigurationControllerConfig{},
		}

		sourceNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "shoot--project--my-shoot",
				UID:  "source-ns-uid",
			},
		}

		istioIngressNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "istio-ingress",
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress,
				},
			},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: sourceNamespace.Name,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name: "https-main",
						Port: 443,
					},
				},
			},
		}

		destinationRule = &istionetworkingv1beta1.DestinationRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: sourceNamespace.Name,
			},
			Spec: istioapinetworkingv1beta1.DestinationRule{
				Host:     service.Name + "." + service.Namespace + ".svc.cluster.local",
				ExportTo: []string{"*"},
			},
		}

		envoyFilterName = sourceNamespace.Name + "-cluster-configuration"
	})

	JustBeforeEach(func() {
		Expect(fakeClient.Create(ctx, sourceNamespace)).To(Succeed())
		Expect(fakeClient.Create(ctx, istioIngressNamespace)).To(Succeed())
		Expect(fakeClient.Create(ctx, service)).To(Succeed())
		Expect(fakeClient.Create(ctx, destinationRule)).To(Succeed())
	})

	Describe("#Reconcile", func() {
		It("should create an EnvoyFilter with buffer limit only when port is not HTTP/2", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			Expect(envoyFilter.Labels).To(HaveKeyWithValue("resources.gardener.cloud/managed-by", "istio-cluster-configuration"))
			Expect(envoyFilter.OwnerReferences).To(HaveLen(1))
			Expect(envoyFilter.OwnerReferences[0].Name).To(Equal(sourceNamespace.Name))
			Expect(envoyFilter.OwnerReferences[0].Kind).To(Equal("Namespace"))

			Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))
			patch := envoyFilter.Spec.ConfigPatches[0]
			Expect(patch.ApplyTo).To(Equal(istioapinetworkingv1alpha3.EnvoyFilter_CLUSTER))
			Expect(patch.Match.Context).To(Equal(istioapinetworkingv1alpha3.EnvoyFilter_GATEWAY))
			Expect(patch.Match.GetCluster().Name).To(Equal("outbound|443||" + service.Name + "." + sourceNamespace.Name + ".svc.cluster.local"))
			Expect(patch.Patch.Operation).To(Equal(istioapinetworkingv1alpha3.EnvoyFilter_Patch_MERGE))
			Expect(patch.Patch.Value.Fields["per_connection_buffer_limit_bytes"].GetNumberValue()).To(Equal(float64(32768)))
			Expect(patch.Patch.Value.Fields).NotTo(HaveKey("typed_extension_protocol_options"))
		})

		It("should create an EnvoyFilter with explicit HTTP/2 when port name indicates grpc", func() {
			service.Spec.Ports[0].Name = "grpc-main"
			Expect(fakeClient.Update(ctx, service)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))
			patch := envoyFilter.Spec.ConfigPatches[0]
			Expect(patch.Patch.Value.Fields["per_connection_buffer_limit_bytes"].GetNumberValue()).To(Equal(float64(32768)))

			typedOpts := patch.Patch.Value.Fields["typed_extension_protocol_options"].GetStructValue()
			Expect(typedOpts).NotTo(BeNil())
			httpOpts := typedOpts.Fields["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"].GetStructValue()
			Expect(httpOpts).NotTo(BeNil())
			Expect(httpOpts.Fields["@type"].GetStringValue()).To(Equal("type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions"))
			Expect(httpOpts.Fields).To(HaveKey("explicit_http_config"))
			explicitConfig := httpOpts.Fields["explicit_http_config"].GetStructValue()
			h2opts := explicitConfig.Fields["http2_protocol_options"].GetStructValue()
			Expect(h2opts.Fields["initial_stream_window_size"].GetNumberValue()).To(Equal(float64(65536)))
			Expect(h2opts.Fields["initial_connection_window_size"].GetNumberValue()).To(Equal(float64(1048576)))
		})

		It("should create an EnvoyFilter with explicit HTTP/2 when port name is http2", func() {
			service.Spec.Ports[0].Name = "http2-server"
			Expect(fakeClient.Update(ctx, service)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			httpOpts := envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields["typed_extension_protocol_options"].GetStructValue().
				Fields["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"].GetStructValue()
			Expect(httpOpts.Fields).To(HaveKey("explicit_http_config"))
		})

		It("should create an EnvoyFilter with explicit HTTP/2 when appProtocol is kubernetes.io/h2c", func() {
			service.Spec.Ports[0].AppProtocol = ptr.To("kubernetes.io/h2c")
			Expect(fakeClient.Update(ctx, service)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			httpOpts := envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields["typed_extension_protocol_options"].GetStructValue().
				Fields["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"].GetStructValue()
			Expect(httpOpts.Fields).To(HaveKey("explicit_http_config"))
		})

		It("should create an EnvoyFilter with explicit HTTP/2 when h2UpgradePolicy is UPGRADE", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						H2UpgradePolicy: istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings_UPGRADE,
					},
				},
			}
			Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			httpOpts := envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields["typed_extension_protocol_options"].GetStructValue().
				Fields["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"].GetStructValue()
			Expect(httpOpts.Fields).To(HaveKey("explicit_http_config"))
		})

		It("should create an EnvoyFilter with use_downstream_protocol_config when useClientProtocol is true", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						UseClientProtocol: true,
					},
				},
			}
			Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			httpOpts := envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields["typed_extension_protocol_options"].GetStructValue().
				Fields["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"].GetStructValue()
			Expect(httpOpts.Fields).To(HaveKey("use_downstream_protocol_config"))
			downstreamConfig := httpOpts.Fields["use_downstream_protocol_config"].GetStructValue()
			h2opts := downstreamConfig.Fields["http2_protocol_options"].GetStructValue()
			Expect(h2opts.Fields["initial_stream_window_size"].GetNumberValue()).To(Equal(float64(65536)))
			Expect(h2opts.Fields["initial_connection_window_size"].GetNumberValue()).To(Equal(float64(1048576)))
		})

		It("should prefer useClientProtocol over h2UpgradePolicy", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						UseClientProtocol: true,
						H2UpgradePolicy:   istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings_UPGRADE,
					},
				},
			}
			Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			httpOpts := envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields["typed_extension_protocol_options"].GetStructValue().
				Fields["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"].GetStructValue()
			Expect(httpOpts.Fields).To(HaveKey("use_downstream_protocol_config"))
			Expect(httpOpts.Fields).NotTo(HaveKey("explicit_http_config"))
		})

		It("should use port-level settings over top-level traffic policy", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						UseClientProtocol: true,
					},
				},
				PortLevelSettings: []*istioapinetworkingv1beta1.TrafficPolicy_PortTrafficPolicy{
					{
						Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
						ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
							Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
								H2UpgradePolicy: istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings_UPGRADE,
							},
						},
					},
				},
			}
			Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			httpOpts := envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields["typed_extension_protocol_options"].GetStructValue().
				Fields["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"].GetStructValue()
			Expect(httpOpts.Fields).To(HaveKey("explicit_http_config"))
			Expect(httpOpts.Fields).NotTo(HaveKey("use_downstream_protocol_config"))
		})

		It("should handle multiple service ports", func() {
			service.Spec.Ports = []corev1.ServicePort{
				{Name: "https-main", Port: 443},
				{Name: "grpc-metrics", Port: 9090},
			}
			Expect(fakeClient.Update(ctx, service)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(2))

			fqdn := service.Name + "." + sourceNamespace.Name + ".svc.cluster.local"
			Expect(envoyFilter.Spec.ConfigPatches[0].Match.GetCluster().Name).To(Equal("outbound|443||" + fqdn))
			Expect(envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields).NotTo(HaveKey("typed_extension_protocol_options"))

			Expect(envoyFilter.Spec.ConfigPatches[1].Match.GetCluster().Name).To(Equal("outbound|9090||" + fqdn))
			Expect(envoyFilter.Spec.ConfigPatches[1].Patch.Value.Fields).To(HaveKey("typed_extension_protocol_options"))
		})

		It("should handle multiple DestinationRules in one namespace", func() {
			service2 := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpn-seed-server",
					Namespace: sourceNamespace.Name,
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "grpc-tunnel", Port: 9443},
					},
				},
			}
			Expect(fakeClient.Create(ctx, service2)).To(Succeed())

			destinationRule2 := &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpn-seed-server",
					Namespace: sourceNamespace.Name,
				},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host:     "vpn-seed-server." + sourceNamespace.Name + ".svc.cluster.local",
					ExportTo: []string{"*"},
				},
			}
			Expect(fakeClient.Create(ctx, destinationRule2)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(2))

			clusterNames := []string{
				envoyFilter.Spec.ConfigPatches[0].Match.GetCluster().Name,
				envoyFilter.Spec.ConfigPatches[1].Match.GetCluster().Name,
			}
			Expect(clusterNames).To(ConsistOf(
				"outbound|443||"+service.Name+"."+sourceNamespace.Name+".svc.cluster.local",
				"outbound|9443||vpn-seed-server."+sourceNamespace.Name+".svc.cluster.local",
			))

			for _, patch := range envoyFilter.Spec.ConfigPatches {
				if patch.Match.GetCluster().Name == "outbound|443||"+service.Name+"."+sourceNamespace.Name+".svc.cluster.local" {
					Expect(patch.Patch.Value.Fields).NotTo(HaveKey("typed_extension_protocol_options"))
				}
				if patch.Match.GetCluster().Name == "outbound|9443||vpn-seed-server."+sourceNamespace.Name+".svc.cluster.local" {
					Expect(patch.Patch.Value.Fields).To(HaveKey("typed_extension_protocol_options"))
				}
			}
		})

		It("should skip DestinationRules whose host does not match a service", func() {
			destinationRule.Spec.Host = "non-existent.other-ns.svc.cluster.local"
			Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilterList := &istionetworkingv1alpha3.EnvoyFilterList{}
			Expect(fakeClient.List(ctx, envoyFilterList, client.InNamespace(istioIngressNamespace.Name))).To(Succeed())
			Expect(envoyFilterList.Items).To(BeEmpty())
		})

		It("should skip DestinationRules with external hosts", func() {
			destinationRule.Spec.Host = "external.example.com"
			Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilterList := &istionetworkingv1alpha3.EnvoyFilterList{}
			Expect(fakeClient.List(ctx, envoyFilterList, client.InNamespace(istioIngressNamespace.Name))).To(Succeed())
			Expect(envoyFilterList.Items).To(BeEmpty())
		})

		It("should resolve short host names using the DR namespace", func() {
			destinationRule.Spec.Host = service.Name
			Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())

			Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))
			Expect(envoyFilter.Spec.ConfigPatches[0].Match.GetCluster().Name).To(Equal("outbound|443||" + service.Name + "." + sourceNamespace.Name + ".svc.cluster.local"))
		})

		Context("exposure class namespace", func() {
			var exposureClassNamespace *corev1.Namespace

			BeforeEach(func() {
				exposureClassNamespace = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "istio-ingress--exposure-class",
						Labels: map[string]string{
							v1beta1constants.LabelExposureClassHandlerName: "my-exposure-class",
						},
					},
				}
			})

			JustBeforeEach(func() {
				Expect(fakeClient.Create(ctx, exposureClassNamespace)).To(Succeed())
			})

			It("should create EnvoyFilters in exposure class namespaces", func() {
				destinationRule.Spec.ExportTo = []string{"*"}
				Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: exposureClassNamespace.Name}, envoyFilter)).To(Succeed())
				Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))
				Expect(envoyFilter.Spec.ConfigPatches[0].Match.GetCluster().Name).To(Equal("outbound|443||" + service.Name + "." + sourceNamespace.Name + ".svc.cluster.local"))
			})

			It("should create EnvoyFilters in both istio-ingress and exposure class namespaces", func() {
				destinationRule.Spec.ExportTo = []string{"*"}
				Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter1 := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter1)).To(Succeed())
				Expect(envoyFilter1.Spec.ConfigPatches).To(HaveLen(1))

				envoyFilter2 := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: exposureClassNamespace.Name}, envoyFilter2)).To(Succeed())
				Expect(envoyFilter2.Spec.ConfigPatches).To(HaveLen(1))
			})

			It("should export only to the named exposure class namespace", func() {
				destinationRule.Spec.ExportTo = []string{exposureClassNamespace.Name}
				Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: exposureClassNamespace.Name}, envoyFilter)).To(Succeed())
				Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))

				envoyFilterList := &istionetworkingv1alpha3.EnvoyFilterList{}
				Expect(fakeClient.List(ctx, envoyFilterList, client.InNamespace(istioIngressNamespace.Name))).To(Succeed())
				Expect(envoyFilterList.Items).To(BeEmpty())
			})
		})

		Context("exportTo resolution", func() {
			var istioIngressNamespace2 *corev1.Namespace

			BeforeEach(func() {
				istioIngressNamespace2 = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "istio-ingress--extra",
						Labels: map[string]string{
							v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress,
						},
					},
				}
			})

			JustBeforeEach(func() {
				Expect(fakeClient.Create(ctx, istioIngressNamespace2)).To(Succeed())
			})

			It("should export to all istio-ingress namespaces when exportTo is *", func() {
				destinationRule.Spec.ExportTo = []string{"*"}
				Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter1 := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter1)).To(Succeed())
				Expect(envoyFilter1.Spec.ConfigPatches).To(HaveLen(1))

				envoyFilter2 := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: "istio-ingress--extra"}, envoyFilter2)).To(Succeed())
				Expect(envoyFilter2.Spec.ConfigPatches).To(HaveLen(1))
			})

			It("should export to all istio-ingress namespaces when exportTo is empty", func() {
				destinationRule.Spec.ExportTo = nil
				Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter1 := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter1)).To(Succeed())

				envoyFilter2 := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: "istio-ingress--extra"}, envoyFilter2)).To(Succeed())
			})

			It("should export only to the named namespace", func() {
				destinationRule.Spec.ExportTo = []string{istioIngressNamespace.Name}
				Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())
				Expect(envoyFilter.Spec.ConfigPatches).To(HaveLen(1))

				envoyFilterList := &istionetworkingv1alpha3.EnvoyFilterList{}
				Expect(fakeClient.List(ctx, envoyFilterList, client.InNamespace("istio-ingress--extra"))).To(Succeed())
				Expect(envoyFilterList.Items).To(BeEmpty())
			})

			It("should export to own namespace with '.' only if it is an istio-ingress namespace", func() {
				destinationRule.Spec.ExportTo = []string{"."}
				Expect(fakeClient.Update(ctx, destinationRule)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilterList := &istionetworkingv1alpha3.EnvoyFilterList{}
				Expect(fakeClient.List(ctx, envoyFilterList)).To(Succeed())
				for _, envoyFilter := range envoyFilterList.Items {
					Expect(envoyFilter.Namespace).NotTo(Equal(istioIngressNamespace.Name))
					Expect(envoyFilter.Namespace).NotTo(Equal("istio-ingress--extra"))
				}
			})
		})

		Context("cleanup", func() {
			It("should delete EnvoyFilter when no DestinationRules exist in the source namespace", func() {
				existingEnvoyFilter := &istionetworkingv1alpha3.EnvoyFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      envoyFilterName,
						Namespace: istioIngressNamespace.Name,
						Labels: map[string]string{
							"resources.gardener.cloud/managed-by": "istio-cluster-configuration",
						},
					},
					Spec: istioapinetworkingv1alpha3.EnvoyFilter{
						ConfigPatches: []*istioapinetworkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
							{
								ApplyTo: istioapinetworkingv1alpha3.EnvoyFilter_CLUSTER,
								Patch: &istioapinetworkingv1alpha3.EnvoyFilter_Patch{
									Operation: istioapinetworkingv1alpha3.EnvoyFilter_Patch_MERGE,
									Value:     &structpb.Struct{Fields: map[string]*structpb.Value{}},
								},
							},
						},
					},
				}
				Expect(fakeClient.Create(ctx, existingEnvoyFilter)).To(Succeed())

				Expect(fakeClient.Delete(ctx, destinationRule)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilterList := &istionetworkingv1alpha3.EnvoyFilterList{}
				Expect(fakeClient.List(ctx, envoyFilterList, client.InNamespace(istioIngressNamespace.Name))).To(Succeed())
				Expect(envoyFilterList.Items).To(BeEmpty())
			})
		})

		Context("grpc-web port name handling", func() {
			It("should detect grpc-web as HTTP/2", func() {
				service.Spec.Ports[0].Name = "grpc-web"
				Expect(fakeClient.Update(ctx, service)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())
				Expect(envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields).To(HaveKey("typed_extension_protocol_options"))
			})

			It("should detect grpc-web-extra as HTTP/2", func() {
				service.Spec.Ports[0].Name = "grpc-web-extra"
				Expect(fakeClient.Update(ctx, service)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())
				Expect(envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields).To(HaveKey("typed_extension_protocol_options"))
			})

			It("should detect grpc-webinar as HTTP/2 (prefix is grpc)", func() {
				service.Spec.Ports[0].Name = "grpc-webinar"
				Expect(fakeClient.Update(ctx, service)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())
				Expect(envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields).To(HaveKey("typed_extension_protocol_options"))
			})

			It("should not detect tcp-main as HTTP/2", func() {
				service.Spec.Ports[0].Name = "tcp-main"
				Expect(fakeClient.Update(ctx, service)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())
				Expect(envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields).NotTo(HaveKey("typed_extension_protocol_options"))
			})
		})

		Context("appProtocol handling", func() {
			It("should detect grpc appProtocol as HTTP/2", func() {
				service.Spec.Ports[0].AppProtocol = ptr.To("grpc")
				Expect(fakeClient.Update(ctx, service)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())
				Expect(envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields).To(HaveKey("typed_extension_protocol_options"))
			})

			It("should not detect http appProtocol as HTTP/2", func() {
				service.Spec.Ports[0].AppProtocol = ptr.To("http")
				Expect(fakeClient.Update(ctx, service)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())
				Expect(envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields).NotTo(HaveKey("typed_extension_protocol_options"))
			})

			It("should prioritize appProtocol over port name", func() {
				service.Spec.Ports[0].Name = "grpc-main"
				service.Spec.Ports[0].AppProtocol = ptr.To("http")
				Expect(fakeClient.Update(ctx, service)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: sourceNamespace.Name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{Name: envoyFilterName, Namespace: istioIngressNamespace.Name}, envoyFilter)).To(Succeed())
				Expect(envoyFilter.Spec.ConfigPatches[0].Patch.Value.Fields).NotTo(HaveKey("typed_extension_protocol_options"))
			})
		})
	})
})
