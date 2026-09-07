// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package peerexposure_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/etcd/peerexposure"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("PeerExposure", func() {
	var (
		ctx = context.Background()

		c          client.Client
		component_ component.DeployWaiter
		values     Values

		namespace         = "shoot--foo--bar"
		istioNS           = "istio-ingress"
		clientHost        = "src-etcd-main.ingress.seed.example.com"
		clientServiceFQDN = "etcd-main-client." + "shoot--foo--bar" + ".svc.cluster.local"
		istioLabels       = map[string]string{"istio": "ingressgateway"}
		members           = []PeerMember{
			{
				SNIHost:      "src-etcd-main-0.ingress.seed.example.com",
				PodFQDN:      "etcd-main-0.etcd-main-peer." + "shoot--foo--bar" + ".svc.cluster.local",
				ExternalPort: 12380,
			},
		}
		multiMembers = []PeerMember{
			{
				SNIHost:      "src-etcd-main-0.ingress.seed.example.com",
				PodFQDN:      "etcd-main-0.etcd-main-peer." + "shoot--foo--bar" + ".svc.cluster.local",
				ExternalPort: 12380,
			},
			{
				SNIHost:      "src-etcd-main-1.ingress.seed.example.com",
				PodFQDN:      "etcd-main-1.etcd-main-peer." + "shoot--foo--bar" + ".svc.cluster.local",
				ExternalPort: 12381,
			},
			{
				SNIHost:      "src-etcd-main-2.ingress.seed.example.com",
				PodFQDN:      "etcd-main-2.etcd-main-peer." + "shoot--foo--bar" + ".svc.cluster.local",
				ExternalPort: 12382,
			},
		}

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
	)

	// decodeManifests decodes YAML manifests into a map keyed by "<Kind>/<name>".
	decodeManifests := func(manifests []string) map[string]client.Object {
		decoder := serializer.NewCodecFactory(kubernetes.SeedScheme).UniversalDeserializer()
		result := make(map[string]client.Object, len(manifests))
		for _, m := range manifests {
			obj, gvk, err := decoder.Decode([]byte(m), nil, nil)
			Expect(err).NotTo(HaveOccurred(), "decode manifest")
			o := obj.(client.Object)
			result[gvk.Kind+"/"+o.GetName()] = o
		}
		return result
	}

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		values = Values{
			Role:                         "main",
			Members:                      members,
			IstioIngressGatewayNamespace: istioNS,
			IstioIngressGatewayLabels:    istioLabels,
		}
		component_ = New(c, namespace, values)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "etcd-main-peer-exposure",
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		var objs map[string]client.Object

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(component_.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:       new("seed"),
					SecretRefs:  []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}},
					KeepObjects: new(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			manifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())
			objs = decodeManifests(manifests)
		})

		It("should include a gateway with TLS passthrough on the etcd peer port", func() {
			gw, ok := objs["Gateway/etcd-main-peer"].(*istionetworkingv1beta1.Gateway)
			Expect(ok).To(BeTrue(), "Gateway/etcd-main-peer not found in MR")
			Expect(gw.Labels).To(HaveKeyWithValue("app", "etcd-peer-exposure"))
			Expect(gw.Spec.Selector).To(Equal(istioLabels))
			Expect(gw.Spec.Servers).To(HaveLen(1))
			server := gw.Spec.Servers[0]
			Expect(server.Hosts).To(Equal([]string{members[0].SNIHost}))
			Expect(server.Port.Number).To(Equal(uint32(12380)))
			Expect(server.Port.Name).To(Equal("tls-etcd-peer-0"))
			Expect(server.Port.Protocol).To(Equal("TLS"))
			Expect(server.Tls.Mode).To(Equal(istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH))
		})

		It("should include a virtual service with one per-member route so each SNI host reaches its own pod", func() {
			vs, ok := objs["VirtualService/etcd-main-peer"].(*istionetworkingv1beta1.VirtualService)
			Expect(ok).To(BeTrue(), "VirtualService/etcd-main-peer not found in MR")
			Expect(vs.Labels).To(HaveKeyWithValue("app", "etcd-peer-exposure"))
			Expect(vs.Spec.ExportTo).To(Equal([]string{istioNS}))
			Expect(vs.Spec.Hosts).To(Equal([]string{members[0].SNIHost}))
			Expect(vs.Spec.Gateways).To(Equal([]string{"etcd-main-peer"}))
			Expect(vs.Spec.Tls).To(HaveLen(1))
			route := vs.Spec.Tls[0]
			Expect(route.Match[0].Port).To(Equal(uint32(12380)))
			Expect(route.Match[0].SniHosts).To(Equal([]string{members[0].SNIHost}))
			Expect(route.Route[0].Destination.Host).To(Equal(members[0].PodFQDN))
			Expect(route.Route[0].Destination.Port.Number).To(Equal(uint32(2380)))
		})

		It("should include a ServiceEntry per member exporting the pod subdomain to the ingress namespace", func() {
			se, ok := objs["ServiceEntry/etcd-main-peer-0"].(*istionetworkingv1beta1.ServiceEntry)
			Expect(ok).To(BeTrue(), "ServiceEntry/etcd-main-peer-0 not found in MR")
			Expect(se.Labels).To(HaveKeyWithValue("app", "etcd-peer-exposure"))
			Expect(se.Spec.Hosts).To(Equal([]string{members[0].PodFQDN}))
			Expect(se.Spec.ExportTo).To(Equal([]string{istioNS}))
			Expect(se.Spec.Resolution).To(Equal(istioapinetworkingv1beta1.ServiceEntry_DNS))
			Expect(se.Spec.Ports).To(HaveLen(1))
			Expect(se.Spec.Ports[0].Number).To(Equal(uint32(2380)))
			Expect(se.Spec.Ports[0].Name).To(Equal("tls-etcd-peer"))
			Expect(se.Spec.Ports[0].Protocol).To(Equal("TLS"))
		})

		It("should include a Service with both peer and client ports and namespace-selectors pointing to istio-ingress", func() {
			svc, ok := objs["Service/etcd-main-np"].(*corev1.Service)
			Expect(ok).To(BeTrue(), "Service/etcd-main-np not found in MR")
			Expect(svc.Labels).To(HaveKeyWithValue("app", "etcd-peer-exposure"))
			Expect(svc.Spec.Selector).To(HaveKeyWithValue("app", "etcd-statefulset"))
			Expect(svc.Spec.Selector).To(HaveKeyWithValue("role", "main"))
			Expect(svc.Spec.Ports).To(HaveLen(2))
			ports := []int32{svc.Spec.Ports[0].Port, svc.Spec.Ports[1].Port}
			Expect(ports).To(ConsistOf(BeEquivalentTo(2380), BeEquivalentTo(2379)))
			Expect(svc.Annotations).To(HaveKey(resourcesv1alpha1.NetworkingNamespaceSelectors))
			Expect(svc.Annotations).To(HaveKeyWithValue(resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, "all-shoots"))
		})

		Context("with multiple members", func() {
			BeforeEach(func() {
				values.Members = multiMembers
				component_ = New(c, namespace, values)
			})

			It("should include one gateway server per member with distinct ports and SNI hosts", func() {
				gw, ok := objs["Gateway/etcd-main-peer"].(*istionetworkingv1beta1.Gateway)
				Expect(ok).To(BeTrue())
				Expect(gw.Spec.Servers).To(HaveLen(3))
				for i, m := range multiMembers {
					srv := gw.Spec.Servers[i]
					Expect(srv.Hosts).To(Equal([]string{m.SNIHost}))
					Expect(srv.Port.Number).To(Equal(m.ExternalPort))
					Expect(srv.Port.Name).To(Equal(fmt.Sprintf("tls-etcd-peer-%d", i)))
					Expect(srv.Tls.Mode).To(Equal(istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH))
				}
			})

			It("should include one virtual service TLS route per member routing to the correct pod", func() {
				vs, ok := objs["VirtualService/etcd-main-peer"].(*istionetworkingv1beta1.VirtualService)
				Expect(ok).To(BeTrue())
				Expect(vs.Spec.Hosts).To(ConsistOf(
					multiMembers[0].SNIHost, multiMembers[1].SNIHost, multiMembers[2].SNIHost,
				))
				Expect(vs.Spec.Tls).To(HaveLen(3))
				for i, m := range multiMembers {
					route := vs.Spec.Tls[i]
					Expect(route.Match[0].Port).To(Equal(m.ExternalPort))
					Expect(route.Match[0].SniHosts).To(Equal([]string{m.SNIHost}))
					Expect(route.Route[0].Destination.Host).To(Equal(m.PodFQDN))
					Expect(route.Route[0].Destination.Port.Number).To(Equal(uint32(2380)))
				}
			})

			It("should include one indexed ServiceEntry per member", func() {
				for i, m := range multiMembers {
					se, ok := objs[fmt.Sprintf("ServiceEntry/etcd-main-peer-%d", i)].(*istionetworkingv1beta1.ServiceEntry)
					Expect(ok).To(BeTrue(), "ServiceEntry/etcd-main-peer-%d not found", i)
					Expect(se.Spec.Hosts).To(Equal([]string{m.PodFQDN}))
				}
			})
		})

		Context("when ClientHost is set", func() {
			BeforeEach(func() {
				values.ClientHost = clientHost
				component_ = New(c, namespace, values)
			})

			It("should include a client gateway with TLS passthrough on the etcd client port", func() {
				gw, ok := objs["Gateway/etcd-main-client"].(*istionetworkingv1beta1.Gateway)
				Expect(ok).To(BeTrue(), "Gateway/etcd-main-client not found in MR")
				Expect(gw.Spec.Selector).To(Equal(istioLabels))
				Expect(gw.Spec.Servers).To(HaveLen(1))
				srv := gw.Spec.Servers[0]
				Expect(srv.Hosts).To(Equal([]string{clientHost}))
				Expect(srv.Port.Number).To(Equal(uint32(12379)))
				Expect(srv.Port.Name).To(Equal("tls-etcd-client"))
				Expect(srv.Port.Protocol).To(Equal("TLS"))
				Expect(srv.Tls.Mode).To(Equal(istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH))
			})

			It("should include a client virtual service routing the SNI host to the etcd client service", func() {
				vs, ok := objs["VirtualService/etcd-main-client"].(*istionetworkingv1beta1.VirtualService)
				Expect(ok).To(BeTrue(), "VirtualService/etcd-main-client not found in MR")
				Expect(vs.Spec.ExportTo).To(Equal([]string{istioNS}))
				Expect(vs.Spec.Hosts).To(Equal([]string{clientHost}))
				Expect(vs.Spec.Gateways).To(Equal([]string{"etcd-main-client"}))
				Expect(vs.Spec.Tls).To(HaveLen(1))
				route := vs.Spec.Tls[0]
				Expect(route.Match[0].Port).To(Equal(uint32(12379)))
				Expect(route.Match[0].SniHosts).To(Equal([]string{clientHost}))
				Expect(route.Route[0].Destination.Host).To(Equal(clientServiceFQDN))
				Expect(route.Route[0].Destination.Port.Number).To(Equal(uint32(2379)))
			})

			It("should include a client ServiceEntry exporting the etcd client service to the ingress namespace", func() {
				se, ok := objs["ServiceEntry/etcd-main-client"].(*istionetworkingv1beta1.ServiceEntry)
				Expect(ok).To(BeTrue(), "ServiceEntry/etcd-main-client not found in MR")
				Expect(se.Spec.Hosts).To(Equal([]string{clientServiceFQDN}))
				Expect(se.Spec.ExportTo).To(Equal([]string{istioNS}))
				Expect(se.Spec.Resolution).To(Equal(istioapinetworkingv1beta1.ServiceEntry_DNS))
				Expect(se.Spec.Ports).To(HaveLen(1))
				Expect(se.Spec.Ports[0].Number).To(Equal(uint32(2379)))
				Expect(se.Spec.Ports[0].Name).To(Equal("tls-etcd-client"))
				Expect(se.Spec.Ports[0].Protocol).To(Equal("TLS"))
			})
		})

		Context("when ClientHost is not set", func() {
			It("should not include any client resources in the MR", func() {
				Expect(objs).NotTo(HaveKey("Gateway/etcd-main-client"))
				Expect(objs).NotTo(HaveKey("VirtualService/etcd-main-client"))
				Expect(objs).NotTo(HaveKey("ServiceEntry/etcd-main-client"))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should delete the ManagedResource and its secret", func() {
			Expect(component_.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name

			Expect(component_.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})
})
