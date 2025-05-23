// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
		istioapinetworkingv1beta1.LoadBalancerSettings{},
		istioapinetworkingv1beta1.LocalityLoadBalancerSetting{},
		istioapinetworkingv1beta1.OutlierDetection{},
		durationpb.Duration{},
		wrapperspb.BoolValue{},
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
		istioapinetworkingv1beta1.HTTPMatchRequest{},
		istioapinetworkingv1beta1.HTTPRoute{},
		istioapinetworkingv1beta1.HTTPRouteDestination{},
		istioapinetworkingv1beta1.Destination{},
		istioapinetworkingv1beta1.PortSelector{},
		istioapinetworkingv1beta1.TLSRoute{},
		istioapinetworkingv1beta1.TLSMatchAttributes{},
		istioapinetworkingv1beta1.RouteDestination{},
		istioapinetworkingv1beta1.StringMatch{},
		istioapimetav1alpha1.IstioStatus{},
	)
}
