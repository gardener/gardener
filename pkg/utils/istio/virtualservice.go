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

package istio

import (
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
)

// VirtualServiceWithSNIMatch returns a function setting the given attributes to a virtual service object.
func VirtualServiceWithSNIMatch(virtualService *istionetworkingv1beta1.VirtualService, labels map[string]string, hosts []string, gatewayName string, externalPort uint32, destinationHost string, destinationPort uint32) func() error {
	return func() error {
		virtualService.Labels = labels
		virtualService.Spec = istioapinetworkingv1beta1.VirtualService{
			ExportTo: []string{"*"},
			Hosts:    hosts,
			Gateways: []string{gatewayName},
			Tls: []*istioapinetworkingv1beta1.TLSRoute{{
				Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
					Port:     externalPort,
					SniHosts: hosts,
				}},
				Route: []*istioapinetworkingv1beta1.RouteDestination{{
					Destination: &istioapinetworkingv1beta1.Destination{
						Host: destinationHost,
						Port: &istioapinetworkingv1beta1.PortSelector{Number: destinationPort},
					},
				}},
			}},
		}
		return nil
	}
}
