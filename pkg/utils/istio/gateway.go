// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
)

const httpsPort = 443

// ServerConfig is a configuration for a server in an Istio Gateway.
type ServerConfig struct {
	Hosts     []string
	PortName  string
	TLSSecret string
}

// GatewayWithTLSPassthrough returns a function setting the given attributes to a gateway object.
func GatewayWithTLSPassthrough(gateway *istionetworkingv1beta1.Gateway, labels map[string]string, istioLabels map[string]string, hosts []string) func() error {
	return func() error {
		gateway.Labels = labels
		gateway.Spec = istioapinetworkingv1beta1.Gateway{
			Selector: istioLabels,
			Servers: []*istioapinetworkingv1beta1.Server{{
				Hosts: hosts,
				Port: &istioapinetworkingv1beta1.Port{
					Number:   httpsPort,
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
func GatewayWithTLSTermination(gateway *istionetworkingv1beta1.Gateway, labels map[string]string, istioLabels map[string]string, hosts []string, tlsSecret string) func() error {
	return func() error {
		gateway.Labels = labels
		gateway.Spec = istioapinetworkingv1beta1.Gateway{
			Selector: istioLabels,
			Servers: []*istioapinetworkingv1beta1.Server{{
				Hosts: hosts,
				Port: &istioapinetworkingv1beta1.Port{
					Number:   httpsPort,
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
func GatewayWithMutualTLS(gateway *istionetworkingv1beta1.Gateway, labels map[string]string, istioLabels map[string]string, serverConfigs []ServerConfig) func() error {
	return func() error {
		gateway.Labels = labels
		gateway.Spec = istioapinetworkingv1beta1.Gateway{
			Selector: istioLabels,
			Servers:  []*istioapinetworkingv1beta1.Server{},
		}

		for _, serverConfig := range serverConfigs {
			gateway.Spec.Servers = append(gateway.Spec.Servers, &istioapinetworkingv1beta1.Server{
				Hosts: serverConfig.Hosts,
				Port: &istioapinetworkingv1beta1.Port{
					Number:   httpsPort,
					Name:     serverConfig.PortName,
					Protocol: "HTTPS",
				},
				Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode:           istioapinetworkingv1beta1.ServerTLSSettings_OPTIONAL_MUTUAL,
					CredentialName: serverConfig.TLSSecret,
				},
			})
		}
		return nil
	}
}
