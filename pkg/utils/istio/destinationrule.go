// SPDX-FileCopyrightText: Contributors to the Gardener project
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

// MaxConnectionDuration is the maximum duration of a connection in seconds. It is set to 24 hours (86400 seconds) to prevent issues with expiring client certificates.
const MaxConnectionDuration = 86400

// TCPKeepaliveTime is the duration in seconds a connection needs to be idle before TCP keepalive probes are sent.
// TCPKeepaliveInterval is the duration in seconds between individual TCP keepalive probes.
// TCPKeepaliveProbes is the number of TCP keepalive probes to send before considering the connection dead.
// => After 60s + 30s * 5 = 210s (3.5 minutes) of idle time without answers to the keepalive probes, the connection will be considered dead and closed.
// Values are choosen between the kubernetes client and server defaults, which are between 30s and 120s for the TCPKeepaliveTime and unspecified 
// TCPKeepaliveInterval and TCPKeepaliveProbes.
// client: https://github.com/kubernetes/kubernetes/blob/b264d0913501e614a75eb021a0e2578ee12d0281/staging/src/k8s.io/client-go/transport/cache.go#L123
// server: https://github.com/kubernetes/kubernetes/blob/b264d0913501e614a75eb021a0e2578ee12d0281/staging/src/k8s.io/apiserver/pkg/server/secure_serving.go#L288
const TCPKeepaliveTime = 60
const TCPKeepaliveInterval = 30
const TCPKeepaliveProbes = 5

// DestinationRuleWithLocalityPreference returns a function setting the given attributes to a destination rule object.
func DestinationRuleWithLocalityPreference(destinationRule *istionetworkingv1beta1.DestinationRule, labels map[string]string, exportTo []string, destinationHost string) func() error {
	return DestinationRuleWithLocalityPreferenceAndTLS(destinationRule, labels, exportTo, destinationHost, &istioapinetworkingv1beta1.ClientTLSSettings{Mode: istioapinetworkingv1beta1.ClientTLSSettings_DISABLE})
}

// DestinationRuleWithTLSTermination returns a function setting the given attributes to a destination rule object.
func DestinationRuleWithTLSTermination(destinationRule *istionetworkingv1beta1.DestinationRule, labels map[string]string, exportTo []string, destinationHost, sniHost, caSecret string, mode istioapinetworkingv1beta1.ClientTLSSettings_TLSmode) func() error {
	return destinationRuleWithTrafficPolicy(
		destinationRule,
		labels,
		exportTo,
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
		&istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
			UseClientProtocol: true,
		},
	)
}

// DestinationRuleWithLocalityPreferenceAndTLS returns a function setting the given attributes to a destination rule object.
func DestinationRuleWithLocalityPreferenceAndTLS(destinationRule *istionetworkingv1beta1.DestinationRule, labels map[string]string, exportTo []string, destinationHost string, tls *istioapinetworkingv1beta1.ClientTLSSettings) func() error {
	return destinationRuleWithTrafficPolicy(
		destinationRule,
		labels,
		exportTo,
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
		nil,
	)
}

func destinationRuleWithTrafficPolicy(
	destinationRule *istionetworkingv1beta1.DestinationRule,
	labels map[string]string,
	exportTo []string,
	destinationHost string,
	loadbalancer *istioapinetworkingv1beta1.LoadBalancerSettings,
	outlierDetection *istioapinetworkingv1beta1.OutlierDetection,
	tls *istioapinetworkingv1beta1.ClientTLSSettings,
	httpConnectionPool *istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings,
) func() error {
	return func() error {
		destinationRule.Labels = labels
		destinationRule.Spec = istioapinetworkingv1beta1.DestinationRule{
			ExportTo: exportTo,
			Host:     destinationHost,
			TrafficPolicy: &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Tcp: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
						MaxConnectionDuration: &durationpb.Duration{Seconds: MaxConnectionDuration},
						TcpKeepalive: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
							Time:     &durationpb.Duration{Seconds: TCPKeepaliveTime},
							Interval: &durationpb.Duration{Seconds: TCPKeepaliveInterval},
							Probes:   TCPKeepaliveProbes,
						},
					},
					Http: httpConnectionPool,
				},
				LoadBalancer:     loadbalancer,
				OutlierDetection: outlierDetection,
				Tls:              tls,
			},
		}
		return nil
	}
}
