// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
