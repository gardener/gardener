// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/shared"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

type istioTestValues struct {
	client                            client.Client
	chartRenderer                     chartrenderer.Interface
	istiodImageName, ingressImageName string
	prefix                            string
	ingressNamespace                  string
	priorityClassName                 string
	istiodEnabled                     bool
	labels                            map[string]string
	lbAnnotations                     map[string]string
	externalTrafficPolicy             *corev1.ServiceExternalTrafficPolicyType
	serviceExternalIP                 *string
	servicePorts                      []corev1.ServicePort
	proxyProtocolEnabled              bool
	vpnEnabled                        bool
	zones                             []string
}

func createIstio(testValues istioTestValues) istio.Interface {
	istio, err := NewIstio(
		testValues.client,
		imagevector.ImageVector{
			{Name: "istio-istiod", Repository: testValues.istiodImageName},
			{Name: "istio-proxy", Repository: testValues.ingressImageName},
		},
		testValues.chartRenderer,
		testValues.prefix,
		testValues.ingressNamespace,
		testValues.priorityClassName,
		testValues.istiodEnabled,
		testValues.labels,
		testValues.lbAnnotations,
		testValues.externalTrafficPolicy,
		testValues.serviceExternalIP,
		testValues.servicePorts,
		testValues.proxyProtocolEnabled,
		testValues.vpnEnabled,
		testValues.zones)

	Expect(err).To(Not(HaveOccurred()))
	return istio
}

func checkIstio(istioDeploy istio.Interface, testValues istioTestValues) {
	var minReplicas, maxReplicas *int

	if zoneSize := len(testValues.zones); zoneSize > 1 {
		minReplicas = pointer.Int(zoneSize * 2)
		maxReplicas = pointer.Int(zoneSize * 4)
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
				TrustDomain:           "cluster.local",
				Image:                 testValues.ingressImageName,
				IstiodNamespace:       "istio-system",
				Annotations:           testValues.lbAnnotations,
				ExternalTrafficPolicy: testValues.externalTrafficPolicy,
				MinReplicas:           minReplicas,
				MaxReplicas:           maxReplicas,
				Ports:                 testValues.servicePorts,
				LoadBalancerIP:        testValues.serviceExternalIP,
				Labels:                testValues.labels,
				Namespace:             "shared-istio-test-some-istio-ingress",
				PriorityClassName:     testValues.priorityClassName,
				ProxyProtocolEnabled:  testValues.proxyProtocolEnabled,
				VPNEnabled:            testValues.vpnEnabled,
			},
		},
		NamePrefix: testValues.prefix,
	}))
}

func checkAdditionalIstioGateway(istioDeploy istio.Interface,
	namespace string,
	annotations map[string]string,
	labels map[string]string,
	externalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType,
	serviceExternalIP *string,
	zone *string) {

	var (
		zones       []string
		minReplicas *int
		maxReplicas *int

		ingressValues = istioDeploy.GetValues().IngressGateway
	)

	if zone == nil {
		minReplicas = ingressValues[0].MinReplicas
		maxReplicas = ingressValues[0].MaxReplicas
	} else {
		zones = []string{*zone}
	}

	Expect(ingressValues[len(ingressValues)-1]).To(Equal(istio.IngressGatewayValues{
		TrustDomain:           "cluster.local",
		Image:                 ingressValues[0].Image,
		IstiodNamespace:       "istio-system",
		Annotations:           annotations,
		ExternalTrafficPolicy: externalTrafficPolicy,
		MinReplicas:           minReplicas,
		MaxReplicas:           maxReplicas,
		Ports:                 ingressValues[0].Ports,
		LoadBalancerIP:        serviceExternalIP,
		Labels:                labels,
		Namespace:             namespace,
		PriorityClassName:     ingressValues[0].PriorityClassName,
		ProxyProtocolEnabled:  ingressValues[0].ProxyProtocolEnabled,
		VPNEnabled:            true,
		Zones:                 zones,
	}))
}

var _ = Describe("Istio", func() {
	var (
		testValues  istioTestValues
		zones       []string
		istioDeploy istio.Interface
	)

	BeforeEach(func() {
		zones = nil
		istioDeploy = nil
	})

	JustBeforeEach(func() {
		trafficPolicy := corev1.ServiceExternalTrafficPolicyTypeLocal
		testValues = istioTestValues{
			istiodImageName:       "istiod",
			ingressImageName:      "istio-ingress",
			prefix:                "shared-istio-test-",
			ingressNamespace:      "some-istio-ingress",
			priorityClassName:     "some-high-priority-class",
			istiodEnabled:         true,
			labels:                map[string]string{"some": "labelValue"},
			lbAnnotations:         map[string]string{"some": "annotationValue"},
			externalTrafficPolicy: &trafficPolicy,
			serviceExternalIP:     pointer.String("1.2.3.4"),
			servicePorts:          []corev1.ServicePort{{Port: 443}},
			proxyProtocolEnabled:  false,
			vpnEnabled:            true,
			zones:                 zones,
		}

		istioDeploy = createIstio(testValues)
	})

	Describe("#NewIstio", func() {
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
		})

		Context("with multiple zones", func() {
			BeforeEach(func() {
				zones = []string{"1", "2", "3"}
			})

			It("should successfully create a new Istio deployer", func() {
				checkIstio(istioDeploy, testValues)
			})
		})
	})

	Describe("#AddIstioIngressGateway", func() {
		var (
			namespace             string
			annotations           map[string]string
			labels                map[string]string
			externalTrafficPolicy corev1.ServiceExternalTrafficPolicyType
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
			externalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
			serviceExternalIP = pointer.String("1.1.1.1")
		})

		It("should fail because initial ingress gateway is missing", func() {
			istioDeploy = istio.NewIstio(nil, nil, istio.Values{})

			Expect(AddIstioIngressGateway(
				istioDeploy,
				namespace,
				annotations,
				labels,
				&externalTrafficPolicy,
				serviceExternalIP,
				zone)).To(MatchError("at least one ingress gateway must be present before adding further ones"))
		})

		Context("without zone", func() {
			BeforeEach(func() {
				zone = nil
			})

			It("should successfully add an additional ingress gateway", func() {
				Expect(AddIstioIngressGateway(
					istioDeploy,
					namespace,
					annotations,
					labels,
					&externalTrafficPolicy,
					serviceExternalIP,
					zone)).To(Succeed())

				checkAdditionalIstioGateway(
					istioDeploy,
					namespace,
					annotations,
					labels,
					&externalTrafficPolicy,
					serviceExternalIP,
					zone,
				)
			})
		})

		Context("with zone", func() {
			BeforeEach(func() {
				zone = pointer.String("1")
			})

			It("should successfully add an additional ingress gateway", func() {
				Expect(AddIstioIngressGateway(
					istioDeploy,
					namespace,
					annotations,
					labels,
					&externalTrafficPolicy,
					serviceExternalIP,
					zone)).To(Succeed())

				checkAdditionalIstioGateway(
					istioDeploy,
					namespace,
					annotations,
					labels,
					&externalTrafficPolicy,
					serviceExternalIP,
					zone,
				)
			})
		})
	})
})
