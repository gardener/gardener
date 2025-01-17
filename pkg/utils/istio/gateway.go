// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
)

// GatewayWithTLSPassthrough returns a function setting the given attributes to a gateway object.
func GatewayWithTLSPassthrough(gateway *istionetworkingv1beta1.Gateway, labels map[string]string, istioLabels map[string]string, hosts []string, port uint32) func() error {
	return func() error {
		gateway.Labels = labels
		gateway.Spec = istioapinetworkingv1beta1.Gateway{
			Selector: istioLabels,
			Servers: []*istioapinetworkingv1beta1.Server{{
				Hosts: hosts,
				Port: &istioapinetworkingv1beta1.Port{
					Number:   port,
					Name:     "tls",
					Protocol: "TLS",
				},
				Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode: istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH,
				},
			}},
		}
		return nil
	}
}

// GatewayWithTLSTermination returns a function setting the given attributes to a gateway object.
func GatewayWithTLSTermination(gateway *istionetworkingv1beta1.Gateway, labels map[string]string, istioLabels map[string]string, hosts []string, port uint32, tlsSecret string) func() error {
	return func() error {
		gateway.Labels = labels
		gateway.Spec = istioapinetworkingv1beta1.Gateway{
			Selector: istioLabels,
			Servers: []*istioapinetworkingv1beta1.Server{{
				Hosts: hosts,
				Port: &istioapinetworkingv1beta1.Port{
					Number:   port,
					Name:     "tls",
					Protocol: "HTTPS",
				},
				Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode:           istioapinetworkingv1beta1.ServerTLSSettings_SIMPLE,
					CredentialName: tlsSecret,
				},
			}},
		}
		return nil
	}
}

// GatewayWithMutualTLS returns a function setting the given attributes to a gateway object.
func GatewayWithMutualTLS(gateway *istionetworkingv1beta1.Gateway, labels map[string]string, istioLabels map[string]string, hosts []string, port uint32, tlsSecret string) func() error {
	return func() error {
		gateway.Labels = labels
		gateway.Spec = istioapinetworkingv1beta1.Gateway{
			Selector: istioLabels,
			Servers: []*istioapinetworkingv1beta1.Server{{
				Hosts: hosts,
				Port: &istioapinetworkingv1beta1.Port{
					Number:   port,
					Name:     "tls",
					Protocol: "HTTPS",
				},
				Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode:           istioapinetworkingv1beta1.ServerTLSSettings_OPTIONAL_MUTUAL,
					CredentialName: tlsSecret,
				},
			}},
		}
		return nil
	}
}
