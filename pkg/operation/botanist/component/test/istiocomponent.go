// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package test

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/durationpb"
	istioapimetav1alpha1 "istio.io/api/meta/v1alpha1"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
)

// CmpOptsForDestinationRule returns a compare option to ignore unexported fields in types related to destination rules
func CmpOptsForDestinationRule() cmp.Option {
	return cmpopts.IgnoreUnexported(
		istioapinetworkingv1beta1.DestinationRule{},
		istioapinetworkingv1beta1.TrafficPolicy{},
		istioapinetworkingv1beta1.ConnectionPoolSettings{},
		istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings{},
		istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{},
		durationpb.Duration{},
		istioapinetworkingv1beta1.ClientTLSSettings{},
		istioapimetav1alpha1.IstioStatus{},
	)
}

// CmpOptsForGateway returns a compare option to ignore unexported fields in types related to gateways
func CmpOptsForGateway() cmp.Option {
	return cmpopts.IgnoreUnexported(
		istioapinetworkingv1beta1.Gateway{},
		istioapinetworkingv1beta1.Server{},
		istioapinetworkingv1beta1.Port{},
		istioapinetworkingv1beta1.ServerTLSSettings{},
		istioapimetav1alpha1.IstioStatus{},
	)
}

// CmpOptsForVirtualService returns a compare option to ignore unexported fields in types related to virtual services
func CmpOptsForVirtualService() cmp.Option {
	return cmpopts.IgnoreUnexported(
		istioapinetworkingv1beta1.VirtualService{},
		istioapinetworkingv1beta1.HTTPRoute{},
		istioapinetworkingv1beta1.HTTPRouteDestination{},
		istioapinetworkingv1beta1.Destination{},
		istioapinetworkingv1beta1.PortSelector{},
		istioapinetworkingv1beta1.TLSRoute{},
		istioapinetworkingv1beta1.TLSMatchAttributes{},
		istioapinetworkingv1beta1.RouteDestination{},
		istioapimetav1alpha1.IstioStatus{},
	)
}
