// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// DestinationRuleWithLocalityPreference returns a function setting the given attributes to a destination rule object.
func DestinationRuleWithLocalityPreference(destinationRule *istionetworkingv1beta1.DestinationRule, labels map[string]string, destinationHost string) func() error {
	return DestinationRuleWithLocalityPreferenceAndTLS(destinationRule, labels, destinationHost, &istioapinetworkingv1beta1.ClientTLSSettings{Mode: istioapinetworkingv1beta1.ClientTLSSettings_DISABLE})
}

// DestinationRuleWithTLSTermination returns a function setting the given attributes to a destination rule object.
func DestinationRuleWithTLSTermination(destinationRule *istionetworkingv1beta1.DestinationRule, labels map[string]string, destinationHost, sniHost, caSecret string, mode istioapinetworkingv1beta1.ClientTLSSettings_TLSmode) func() error {
	return destinationRuleWithTrafficPolicy(
		destinationRule,
		labels,
		destinationHost,
		&istioapinetworkingv1beta1.LoadBalancerSettings{
			LbPolicy: &istioapinetworkingv1beta1.LoadBalancerSettings_Simple{
				Simple: istioapinetworkingv1beta1.LoadBalancerSettings_LEAST_REQUEST,
			},
		},
		// OutlierDetection must be nil that simple load balancing policy takes effect.
		nil,
		&istioapinetworkingv1beta1.ClientTLSSettings{
			Mode:           mode,
			CredentialName: caSecret,
			Sni:            sniHost,
		},
	)
}

// DestinationRuleWithLocalityPreferenceAndTLS returns a function setting the given attributes to a destination rule object.
func DestinationRuleWithLocalityPreferenceAndTLS(destinationRule *istionetworkingv1beta1.DestinationRule, labels map[string]string, destinationHost string, tls *istioapinetworkingv1beta1.ClientTLSSettings) func() error {
	return destinationRuleWithTrafficPolicy(
		destinationRule,
		labels,
		destinationHost,
		&istioapinetworkingv1beta1.LoadBalancerSettings{
			LocalityLbSetting: &istioapinetworkingv1beta1.LocalityLoadBalancerSetting{
				Enabled:          &wrapperspb.BoolValue{Value: true},
				FailoverPriority: []string{corev1.LabelTopologyZone},
			},
		},
		// OutlierDetection is required for locality settings to take effect.
		&istioapinetworkingv1beta1.OutlierDetection{
			MinHealthPercent: 0,
		},
		tls,
	)
}

func destinationRuleWithTrafficPolicy(
	destinationRule *istionetworkingv1beta1.DestinationRule,
	labels map[string]string,
	destinationHost string,
	loadbalancer *istioapinetworkingv1beta1.LoadBalancerSettings,
	outlierDetection *istioapinetworkingv1beta1.OutlierDetection,
	tls *istioapinetworkingv1beta1.ClientTLSSettings,
) func() error {
	return func() error {
		destinationRule.Labels = labels
		destinationRule.Spec = istioapinetworkingv1beta1.DestinationRule{
			ExportTo: []string{"*"},
			Host:     destinationHost,
			TrafficPolicy: &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Tcp: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
						MaxConnections: 5000,
						TcpKeepalive: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
							Time:     &durationpb.Duration{Seconds: 7200},
							Interval: &durationpb.Duration{Seconds: 75},
						},
					},
				},
				LoadBalancer:     loadbalancer,
				OutlierDetection: outlierDetection,
				Tls:              tls,
			},
		}
		return nil
	}
}
