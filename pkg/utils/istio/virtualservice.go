// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
)

// VirtualServiceWithSNIMatch returns a function setting the given attributes to a virtual service object.
func VirtualServiceWithSNIMatch(virtualService *istionetworkingv1beta1.VirtualService, labels map[string]string, hosts []string, gatewayName string, port uint32, destinationHost string) func() error {
	return func() error {
		virtualService.Labels = labels
		virtualService.Spec = istioapinetworkingv1beta1.VirtualService{
			ExportTo: []string{"*"},
			Hosts:    hosts,
			Gateways: []string{gatewayName},
			Tls: []*istioapinetworkingv1beta1.TLSRoute{{
				Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
					Port:     port,
					SniHosts: hosts,
				}},
				Route: []*istioapinetworkingv1beta1.RouteDestination{{
					Destination: &istioapinetworkingv1beta1.Destination{
						Host: destinationHost,
						Port: &istioapinetworkingv1beta1.PortSelector{Number: port},
					},
				}},
			}},
		}
		return nil
	}
}

// VirtualServiceForTLSTermination returns a function for use with a gateway that performs TLS termination.
func VirtualServiceForTLSTermination(virtualService *istionetworkingv1beta1.VirtualService, labels map[string]string, hosts []string, gatewayName string, port uint32, destinationHost, destinationUpgradeHost, connectionUpgradeRouteName string) func() error {
	return func() error {
		virtualService.Labels = labels
		virtualService.Spec = istioapinetworkingv1beta1.VirtualService{
			ExportTo: []string{"*"},
			Hosts:    hosts,
			Gateways: []string{gatewayName},
			Http: []*istioapinetworkingv1beta1.HTTPRoute{
				{
					Name: connectionUpgradeRouteName,
					Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{
						{
							Headers: map[string]*istioapinetworkingv1beta1.StringMatch{
								"Connection": {MatchType: &istioapinetworkingv1beta1.StringMatch_Exact{Exact: "Upgrade"}},
								"Upgrade":    {},
							},
						},
					},
					Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{
						{
							Destination: &istioapinetworkingv1beta1.Destination{
								Host: destinationUpgradeHost,
								Port: &istioapinetworkingv1beta1.PortSelector{Number: port},
							},
						},
					},
				},
				{
					Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{
						{
							Destination: &istioapinetworkingv1beta1.Destination{
								Host: destinationHost,
								Port: &istioapinetworkingv1beta1.PortSelector{Number: port},
							},
						},
					},
				},
			},
		}
		return nil
	}
}
