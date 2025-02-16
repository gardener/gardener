// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	. "github.com/gardener/gardener/pkg/component/shared"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"
)

type istioTestValues struct {
	client                             client.Client
	chartRenderer                      chartrenderer.Interface
	istiodImageName, ingressImageName  string
	prefix                             string
	ingressNamespace                   string
	priorityClassName                  string
	istiodEnabled                      bool
	labels                             map[string]string
	kubeAPIServerPolicyLabel           string
	lbAnnotations                      map[string]string
	externalTrafficPolicy              *corev1.ServiceExternalTrafficPolicy
	serviceExternalIP                  *string
	servicePorts                       []corev1.ServicePort
	proxyProtocolEnabled               bool
	terminateLoadBalancerProxyProtocol bool
	vpnEnabled                         bool
	zones                              []string
	dualStack                          bool
	enforceSpreadAcrossHosts           bool
}

func createIstio(testValues istioTestValues) istio.Interface {
	DeferCleanup(test.WithVars(
		&ImageVector,
		imagevectorutils.ImageVector{
			{Name: "istio-istiod", Repository: &testValues.istiodImageName},
			{Name: "istio-proxy", Repository: &testValues.ingressImageName},
		},
	))

	istio, err := NewIstio(
		context.Background(),
		testValues.client,
		testValues.chartRenderer,
		testValues.prefix,
		testValues.ingressNamespace,
		testValues.priorityClassName,
		testValues.istiodEnabled,
		testValues.labels,
		testValues.kubeAPIServerPolicyLabel,
		testValues.lbAnnotations,
		testValues.externalTrafficPolicy,
		testValues.serviceExternalIP,
		testValues.servicePorts,
		testValues.proxyProtocolEnabled,
		&testValues.terminateLoadBalancerProxyProtocol,
		testValues.vpnEnabled,
		testValues.zones,
		testValues.dualStack,
	)

	Expect(err).To(Not(HaveOccurred()))
	return istio
}

func checkIstio(istioDeploy istio.Interface, testValues istioTestValues) {
	var minReplicas, maxReplicas *int

	if zoneSize := len(testValues.zones); zoneSize > 1 {
		minReplicas = ptr.To(zoneSize * 2)
		maxReplicas = ptr.To(zoneSize * 6)
	}

	networkPolicyLabels := map[string]string{
		"networking.gardener.cloud/to-dns":                                     "allowed",
		"networking.gardener.cloud/to-runtime-apiserver":                       "allowed",
		"networking.resources.gardener.cloud/to-istio-system-istiod-tcp-15012": "allowed",
		testValues.kubeAPIServerPolicyLabel:                                    "allowed",
	}

	if testValues.vpnEnabled {
		networkPolicyLabels["networking.resources.gardener.cloud/to-all-shoots-vpn-seed-server-tcp-1194"] = "allowed"
		networkPolicyLabels["networking.resources.gardener.cloud/to-all-shoots-vpn-seed-server-0-tcp-1194"] = "allowed"
		networkPolicyLabels["networking.resources.gardener.cloud/to-all-shoots-vpn-seed-server-1-tcp-1194"] = "allowed"
		networkPolicyLabels["networking.resources.gardener.cloud/to-garden-nginx-ingress-controller-tcp-443"] = "allowed"
	}

	Expect(istioDeploy.GetValues()).To(Equal(istio.Values{
		Istiod: istio.IstiodValues{
			Enabled:           testValues.istiodEnabled,
			Image:             testValues.istiodImageName,
			Namespace:         "istio-system",
			PriorityClassName: testValues.priorityClassName,
			TrustDomain:       "cluster.local",
			Zones:             testValues.zones,
		},
		IngressGateway: []istio.IngressGatewayValues{
			{
				TrustDomain:                        "cluster.local",
				Image:                              testValues.ingressImageName,
				IstiodNamespace:                    "istio-system",
				Annotations:                        testValues.lbAnnotations,
				ExternalTrafficPolicy:              testValues.externalTrafficPolicy,
				MinReplicas:                        minReplicas,
				MaxReplicas:                        maxReplicas,
				Ports:                              testValues.servicePorts,
				LoadBalancerIP:                     testValues.serviceExternalIP,
				Labels:                             testValues.labels,
				NetworkPolicyLabels:                networkPolicyLabels,
				Namespace:                          "shared-istio-test-some-istio-ingress",
				PriorityClassName:                  testValues.priorityClassName,
				ProxyProtocolEnabled:               testValues.proxyProtocolEnabled,
				TerminateLoadBalancerProxyProtocol: testValues.terminateLoadBalancerProxyProtocol,
				VPNEnabled:                         testValues.vpnEnabled,
				EnforceSpreadAcrossHosts:           testValues.enforceSpreadAcrossHosts,
			},
		},
		NamePrefix: testValues.prefix,
	}))
}

func checkAdditionalIstioGateway(cl client.Client,
	istioDeploy istio.Interface,
	namespace string,
	annotations map[string]string,
	labels map[string]string,
	externalTrafficPolicy *corev1.ServiceExternalTrafficPolicy,
	serviceExternalIP *string,
	zone *string,
	dualstack bool) {
	var (
		zones                    []string
		minReplicas              *int
		maxReplicas              *int
		enforceSpreadAcrossHosts bool
		err                      error

		ingressValues = istioDeploy.GetValues().IngressGateway
	)

	if zone == nil {
		minReplicas = ingressValues[0].MinReplicas
		maxReplicas = ingressValues[0].MaxReplicas
	} else {
		zones = []string{*zone}

		enforceSpreadAcrossHosts, err = ShouldEnforceSpreadAcrossHosts(context.Background(), cl, []string{*zone})
		Expect(err).ToNot(HaveOccurred())
	}

	Expect(ingressValues[len(ingressValues)-1]).To(Equal(istio.IngressGatewayValues{
		TrustDomain:                        "cluster.local",
		Image:                              ingressValues[0].Image,
		IstiodNamespace:                    "istio-system",
		Annotations:                        annotations,
		ExternalTrafficPolicy:              externalTrafficPolicy,
		MinReplicas:                        minReplicas,
		MaxReplicas:                        maxReplicas,
		Ports:                              ingressValues[0].Ports,
		LoadBalancerIP:                     serviceExternalIP,
		Labels:                             labels,
		NetworkPolicyLabels:                ingressValues[0].NetworkPolicyLabels,
		Namespace:                          namespace,
		PriorityClassName:                  ingressValues[0].PriorityClassName,
		ProxyProtocolEnabled:               ingressValues[0].ProxyProtocolEnabled,
		TerminateLoadBalancerProxyProtocol: ingressValues[0].TerminateLoadBalancerProxyProtocol,
		VPNEnabled:                         true,
		Zones:                              zones,
		DualStack:                          dualstack,
		EnforceSpreadAcrossHosts:           enforceSpreadAcrossHosts,
	}))
}

var _ = Describe("Istio", func() {
	var (
		testValues      istioTestValues
		zones           []string
		vpnEnabled      bool
		proxyProtocolLB bool
		istioDeploy     istio.Interface
	)

	BeforeEach(func() {
		zones = nil
		istioDeploy = nil
	})

	JustBeforeEach(func() {
		trafficPolicy := corev1.ServiceExternalTrafficPolicyLocal
		testValues = istioTestValues{
			client:                             fakeclient.NewClientBuilder().Build(),
			istiodImageName:                    "istiod",
			ingressImageName:                   "istio-ingress",
			prefix:                             "shared-istio-test-",
			ingressNamespace:                   "some-istio-ingress",
			priorityClassName:                  "some-high-priority-class",
			istiodEnabled:                      true,
			labels:                             map[string]string{"some": "labelValue"},
			kubeAPIServerPolicyLabel:           "to-all-test-kube-apiserver",
			lbAnnotations:                      map[string]string{"some": "annotationValue"},
			externalTrafficPolicy:              &trafficPolicy,
			serviceExternalIP:                  ptr.To("1.2.3.4"),
			servicePorts:                       []corev1.ServicePort{{Port: 443}},
			proxyProtocolEnabled:               false,
			terminateLoadBalancerProxyProtocol: proxyProtocolLB,
			vpnEnabled:                         vpnEnabled,
			zones:                              zones,
			enforceSpreadAcrossHosts:           false,
		}

		istioDeploy = createIstio(testValues)
	})

	Describe("#NewIstio", func() {
		Context("with VPN enabled", func() {
			BeforeEach(func() {
				vpnEnabled = true
			})

			It("should successfully create a new Istio deployer", func() {
				checkIstio(istioDeploy, testValues)
			})
		})

		Context("with proxy protocol termination", func() {
			BeforeEach(func() {
				proxyProtocolLB = true
			})

			It("should successfully create a new Istio deployer", func() {
				checkIstio(istioDeploy, testValues)
			})
		})

		Context("without zone", func() {
			It("should successfully create a new Istio deployer", func() {
				checkIstio(istioDeploy, testValues)
			})
		})

		Context("with single zone", func() {
			BeforeEach(func() {
				zones = []string{"1"}
			})

			It("should successfully create a new Istio deployer", func() {
				checkIstio(istioDeploy, testValues)
			})

			Context("with nodes in single zone", func() {
				JustBeforeEach(func() {
					testValues.client = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
					testValues.enforceSpreadAcrossHosts = true
					Expect(testValues.client.Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-0", Labels: map[string]string{"topology.kubernetes.io/zone": "1"}}})).To(Succeed())
					Expect(testValues.client.Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"topology.kubernetes.io/zone": "1"}}})).To(Succeed())
					istioDeploy = createIstio(testValues)
				})

				It("should successfully create a new Istio deployer", func() {
					checkIstio(istioDeploy, testValues)
				})
			})
		})

		Context("with multiple zones", func() {
			BeforeEach(func() {
				zones = []string{"1", "2", "3"}
			})

			It("should successfully create a new Istio deployer", func() {
				checkIstio(istioDeploy, testValues)
			})

			Context("with nodes in the zones", func() {
				JustBeforeEach(func() {
					testValues.client = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
					testValues.enforceSpreadAcrossHosts = true
					Expect(testValues.client.Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-0", Labels: map[string]string{"topology.kubernetes.io/zone": "1"}}})).To(Succeed())
					Expect(testValues.client.Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"topology.kubernetes.io/zone": "1"}}})).To(Succeed())
					Expect(testValues.client.Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2", Labels: map[string]string{"topology.kubernetes.io/zone": "2"}}})).To(Succeed())
					Expect(testValues.client.Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-3", Labels: map[string]string{"topology.kubernetes.io/zone": "2"}}})).To(Succeed())
					Expect(testValues.client.Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-4", Labels: map[string]string{"topology.kubernetes.io/zone": "3"}}})).To(Succeed())
					Expect(testValues.client.Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-5", Labels: map[string]string{"topology.kubernetes.io/zone": "3"}}})).To(Succeed())
					istioDeploy = createIstio(testValues)
				})

				It("should successfully create a new Istio deployer", func() {
					checkIstio(istioDeploy, testValues)
				})
			})
		})
	})

	Describe("#AddIstioIngressGateway", func() {
		var (
			namespace             string
			annotations           map[string]string
			labels                map[string]string
			externalTrafficPolicy corev1.ServiceExternalTrafficPolicy
			serviceExternalIP     *string
			zone                  *string
		)

		BeforeEach(func() {
			namespace = "additional-istio-ingress"
			annotations = map[string]string{
				"additional": "istio-ingress-annotation",
			}
			labels = map[string]string{
				"additional": "istio-ingress-label",
			}
			externalTrafficPolicy = corev1.ServiceExternalTrafficPolicyCluster
			serviceExternalIP = ptr.To("1.1.1.1")
		})

		It("should fail because initial ingress gateway is missing", func() {
			istioDeploy = istio.NewIstio(nil, nil, istio.Values{})

			Expect(AddIstioIngressGateway(
				context.Background(),
				testValues.client,
				istioDeploy,
				namespace,
				annotations,
				labels,
				&externalTrafficPolicy,
				serviceExternalIP,
				zone,
				false,
				&proxyProtocolLB)).To(MatchError("at least one ingress gateway must be present before adding further ones"))
		})

		Context("without zone", func() {
			BeforeEach(func() {
				zone = nil
			})

			It("should successfully add an additional ingress gateway", func() {
				Expect(AddIstioIngressGateway(
					context.Background(),
					testValues.client,
					istioDeploy,
					namespace,
					annotations,
					labels,
					&externalTrafficPolicy,
					serviceExternalIP,
					zone,
					false,
					&proxyProtocolLB)).To(Succeed())

				checkAdditionalIstioGateway(
					testValues.client,
					istioDeploy,
					namespace,
					annotations,
					labels,
					&externalTrafficPolicy,
					serviceExternalIP,
					zone,
					false,
				)
			})
		})

		Context("with zone", func() {
			BeforeEach(func() {
				zone = ptr.To("1")
			})

			It("should successfully add an additional ingress gateway", func() {
				Expect(AddIstioIngressGateway(
					context.Background(),
					testValues.client,
					istioDeploy,
					namespace,
					annotations,
					labels,
					&externalTrafficPolicy,
					serviceExternalIP,
					zone,
					false,
					&proxyProtocolLB)).To(Succeed())

				checkAdditionalIstioGateway(
					testValues.client,
					istioDeploy,
					namespace,
					annotations,
					labels,
					&externalTrafficPolicy,
					serviceExternalIP,
					zone,
					false,
				)
			})

			Context("with nodes in zone", func() {
				JustBeforeEach(func() {
					testValues.client = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
					testValues.enforceSpreadAcrossHosts = true
					Expect(testValues.client.Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-0", Labels: map[string]string{"topology.kubernetes.io/zone": "1"}}})).To(Succeed())
					Expect(testValues.client.Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"topology.kubernetes.io/zone": "1"}}})).To(Succeed())
					istioDeploy = createIstio(testValues)
				})

				It("should successfully create a new Istio deployer", func() {
					Expect(AddIstioIngressGateway(
						context.Background(),
						testValues.client,
						istioDeploy,
						namespace,
						annotations,
						labels,
						&externalTrafficPolicy,
						serviceExternalIP,
						zone,
						false,
						&proxyProtocolLB)).To(Succeed())

					checkAdditionalIstioGateway(
						testValues.client,
						istioDeploy,
						namespace,
						annotations,
						labels,
						&externalTrafficPolicy,
						serviceExternalIP,
						zone,
						false,
					)
				})
			})
		})

		Context("without zone, dualstack enabled", func() {
			BeforeEach(func() {
				zone = nil
			})

			It("should successfully add an additional ingress gateway", func() {
				Expect(AddIstioIngressGateway(
					context.Background(),
					testValues.client,
					istioDeploy,
					namespace,
					annotations,
					labels,
					&externalTrafficPolicy,
					serviceExternalIP,
					zone,
					true,
					&proxyProtocolLB)).To(Succeed())

				checkAdditionalIstioGateway(
					testValues.client,
					istioDeploy,
					namespace,
					annotations,
					labels,
					&externalTrafficPolicy,
					serviceExternalIP,
					zone,
					true,
				)
			})
		})
	})

	DescribeTable("#GetIstioNamespaceForZone",
		func(defaultNamespace string, zone string, matcher gomegatypes.GomegaMatcher) {
			Expect(GetIstioNamespaceForZone(defaultNamespace, zone)).To(matcher)
		},

		Entry("short namespace and zone", "default-namespace", "my-zone", Equal("default-namespace--my-zone")),
		Entry("empty namespace and zone", "", "", Equal("--")),
		Entry("empty namespace and valid zone", "", "my-zone", Equal("--my-zone")),
		Entry("valid namespace and empty zone", "default-namespace", "", Equal("default-namespace--")),
		Entry("namespace and zone too long => hashed zone", "extremely-long-default-namespace", "unnecessarily-long-regional-zone-name", Equal("extremely-long-default-namespace--fc5e9")),
	)

	DescribeTable("#GetIstioZoneLabels",
		func(labels map[string]string, zone *string, matcher gomegatypes.GomegaMatcher) {
			Expect(GetIstioZoneLabels(labels, zone)).To(matcher)
		},

		Entry("no zone, but istio label", map[string]string{istio.DefaultZoneKey: "istio-value"}, nil, Equal(map[string]string{istio.DefaultZoneKey: "istio-value"})),
		Entry("no zone, but gardener.cloud/role label", map[string]string{"gardener.cloud/role": "gardener-role"}, nil, Equal(map[string]string{"gardener.cloud/role": "gardener-role"})),
		Entry("no zone, other labels", map[string]string{"key1": "value1", "key2": "value2"}, nil, Equal(map[string]string{"key1": "value1", "key2": "value2", istio.DefaultZoneKey: "ingressgateway"})),
		Entry("zone and istio label", map[string]string{istio.DefaultZoneKey: "istio-value"}, ptr.To("my-zone"), Equal(map[string]string{istio.DefaultZoneKey: "istio-value--zone--my-zone"})),
		Entry("zone and gardener.cloud/role label", map[string]string{"gardener.cloud/role": "gardener-role"}, ptr.To("my-zone"), Equal(map[string]string{"gardener.cloud/role": "gardener-role--zone--my-zone"})),
		Entry("zone and other labels", map[string]string{"key1": "value1", "key2": "value2"}, ptr.To("my-zone"), Equal(map[string]string{"key1": "value1", "key2": "value2", istio.DefaultZoneKey: "ingressgateway--zone--my-zone"})),
	)

	DescribeTable("#IsZonalIstioExtension",
		func(labels map[string]string, matcherBool gomegatypes.GomegaMatcher, matcherZone gomegatypes.GomegaMatcher) {
			isZone, zone := IsZonalIstioExtension(labels)
			Expect(isZone).To(matcherBool)
			Expect(zone).To(matcherZone)
		},

		Entry("no zonal extension", map[string]string{"key1": "value1", "key2": "value2"}, BeFalse(), Equal("")),
		Entry("no zone, but istio label", map[string]string{istio.DefaultZoneKey: "istio-value"}, BeFalse(), Equal("")),
		Entry("no zone, but gardener.cloud/role label without handler", map[string]string{"gardener.cloud/role": "exposureclass-handler-gardener-role"}, BeFalse(), Equal("")),
		Entry("no zone, but gardener.cloud/role label with handler", map[string]string{"gardener.cloud/role": "exposureclass-handler-gardener-role", "handler.exposureclass.gardener.cloud/name": ""}, BeFalse(), Equal("")),
		Entry("zone and istio label", map[string]string{istio.DefaultZoneKey: "istio-value--zone--my-zone"}, BeTrue(), Equal("my-zone")),
		Entry("zone and gardener.cloud/role label without handler", map[string]string{"gardener.cloud/role": "exposureclass-handler-gardener-role--zone--some-zone"}, BeFalse(), Equal("")),
		Entry("zone and gardener.cloud/role label with handler", map[string]string{"gardener.cloud/role": "exposureclass-handler-gardener-role--zone--some-zone", "handler.exposureclass.gardener.cloud/name": ""}, BeTrue(), Equal("some-zone")),
		Entry("zone and incorrect gardener.cloud/role label with handler", map[string]string{"gardener.cloud/role": "gardener-role--zone--some-zone", "handler.exposureclass.gardener.cloud/name": ""}, BeFalse(), Equal("")),
	)

	DescribeTable("#ShouldEnforceSpreadAcrossHosts",
		func(nodes []corev1.Node, zones []string, expectedHostSpreading bool) {
			cl := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			for _, n := range nodes {
				Expect(cl.Create(context.Background(), &n)).To(Succeed())
			}
			hostSpreadingEnabled, err := ShouldEnforceSpreadAcrossHosts(context.Background(), cl, zones)
			Expect(err).ToNot(HaveOccurred())
			Expect(hostSpreadingEnabled).To(Equal(expectedHostSpreading))
		},

		Entry("no nodes", []corev1.Node{}, []string{}, false),
		Entry("single node", []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node-0", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}}}, []string{"z1"}, false),
		Entry("two nodes", []corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node-0", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}},
		}, []string{"z1"}, true),
		Entry("three nodes with different zones targeting one zone", []corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node-0", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-2", Labels: map[string]string{"topology.kubernetes.io/zone": "z2"}}},
		}, []string{"z1"}, true),
		Entry("three nodes with different zones targeting two zones", []corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node-0", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-2", Labels: map[string]string{"topology.kubernetes.io/zone": "z2"}}},
		}, []string{"z1", "z2"}, false),
		Entry("four nodes with different zones targeting two zones", []corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node-0", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-2", Labels: map[string]string{"topology.kubernetes.io/zone": "z2"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-3", Labels: map[string]string{"topology.kubernetes.io/zone": "z2"}}},
		}, []string{"z1", "z2"}, true),
		Entry("four nodes with different zones targeting different zone", []corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node-0", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"topology.kubernetes.io/zone": "z1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-2", Labels: map[string]string{"topology.kubernetes.io/zone": "z2"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node-3", Labels: map[string]string{"topology.kubernetes.io/zone": "z2"}}},
		}, []string{"z3"}, false),
	)
})
