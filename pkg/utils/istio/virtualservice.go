// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
)

// ConnectionUpgradeRegex matches the HTTP Connection header value for connection upgrade requests.
// RFC 9110 Section 7.6.1 defines the Connection header as a comma-separated list of case-insensitive tokens (#connection-option).
// To adhere to the specification, this regex matches 'upgrade' as a separate token inside a RFC conform Connection header.
// This supports single values as well as multi-value headers (e.g. 'keep-alive, upgrade') while
// preventing false positives from non-comma-separated words (e.g. 'no-upgrade' or words separated by spaces).
const ConnectionUpgradeRegex = `(?i)(^|.*,)\s*upgrade\s*(,.*|$)`

// VirtualServiceWithSNIMatch returns a function setting the given attributes to a virtual service object.
func VirtualServiceWithSNIMatch(virtualService *istionetworkingv1beta1.VirtualService, labels map[string]string, exportTo []string, hosts []string, gatewayName string, port uint32, destinationHost string) func() error {
	return func() error {
		virtualService.Labels = labels
		virtualService.Spec = istioapinetworkingv1beta1.VirtualService{
			ExportTo: exportTo,
			Hosts:    hosts,
			Gateways: []string{gatewayName},
			Tls: []*istioapinetworkingv1beta1.TLSRoute{{
				Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
					Port:     httpsPort,
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
func VirtualServiceForTLSTermination(virtualService *istionetworkingv1beta1.VirtualService, labels map[string]string, exportTo []string, hosts []string, gatewayName string, port uint32, destinationHost, destinationUpgradeHost, connectionUpgradeRouteName string) func() error {
	return func() error {
		virtualService.Labels = labels
		virtualService.Spec = istioapinetworkingv1beta1.VirtualService{
			ExportTo: exportTo,
			Hosts:    hosts,
			Gateways: []string{gatewayName},
			Http: []*istioapinetworkingv1beta1.HTTPRoute{
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

		if destinationUpgradeHost != "" && connectionUpgradeRouteName != "" {
			virtualService.Spec.Http = append([]*istioapinetworkingv1beta1.HTTPRoute{{
				Name: connectionUpgradeRouteName,
				Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{
					{
						Headers: map[string]*istioapinetworkingv1beta1.StringMatch{
							"Connection": {MatchType: &istioapinetworkingv1beta1.StringMatch_Regex{Regex: ConnectionUpgradeRegex}},
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
			}}, virtualService.Spec.Http...)
		}

		return nil
	}
}
