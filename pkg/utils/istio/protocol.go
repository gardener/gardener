// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"strings"

	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// HTTPProtocolPolicy represents how the Envoy cluster should handle HTTP protocol options.
type HTTPProtocolPolicy int

const (
	// HTTPProtocolPolicyDefault indicates no explicit HTTP protocol configuration is needed.
	HTTPProtocolPolicyDefault HTTPProtocolPolicy = iota
	// HTTPProtocolPolicyExplicitHTTP2 configures the cluster to always use HTTP/2.
	HTTPProtocolPolicyExplicitHTTP2
	// HTTPProtocolPolicyUseClientProtocol configures the cluster to use the downstream client's protocol.
	HTTPProtocolPolicyUseClientProtocol
)

// DetermineProtocolMode determines the protocol mode for a given service port based on the
// DestinationRule's traffic policy settings and the port's protocol indicators.
func DetermineProtocolMode(destinationRule *istionetworkingv1beta1.DestinationRule, port corev1.ServicePort) HTTPProtocolPolicy {
	if destinationRule.Spec.TrafficPolicy != nil {
		for _, portLevel := range destinationRule.Spec.TrafficPolicy.PortLevelSettings {
			if portLevel != nil && portLevel.Port != nil && portLevel.Port.Number == uint32(port.Port) { // #nosec G115 -- service port validated at admission time
				if mode := protocolModeFromConnectionPoolSettings(portLevel.ConnectionPool); mode != HTTPProtocolPolicyDefault {
					return mode
				}
			}
		}
	}

	if destinationRule.Spec.TrafficPolicy != nil && destinationRule.Spec.TrafficPolicy.ConnectionPool != nil {
		if mode := protocolModeFromConnectionPoolSettings(destinationRule.Spec.TrafficPolicy.ConnectionPool); mode != HTTPProtocolPolicyDefault {
			return mode
		}
	}

	if IsHTTP2Port(port) {
		return HTTPProtocolPolicyExplicitHTTP2
	}

	return HTTPProtocolPolicyDefault
}

func protocolModeFromConnectionPoolSettings(connectionPoolSettings *istioapinetworkingv1beta1.ConnectionPoolSettings) HTTPProtocolPolicy {
	if connectionPoolSettings == nil || connectionPoolSettings.Http == nil {
		return HTTPProtocolPolicyDefault
	}

	if connectionPoolSettings.Http.UseClientProtocol {
		return HTTPProtocolPolicyUseClientProtocol
	}

	if connectionPoolSettings.Http.H2UpgradePolicy == istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings_UPGRADE {
		return HTTPProtocolPolicyExplicitHTTP2
	}

	return HTTPProtocolPolicyDefault
}

// IsHTTP2Port determines whether a service port uses HTTP/2 by replicating Istio's protocol
// detection logic. We duplicate this instead of importing istio.io/istio because that module
// is very large and only istio.io/api and istio.io/client-go are direct dependencies.
// See:
//   - https://github.com/istio/istio/blob/2e4387a/pkg/config/protocol/instance.go#L67-L109
//   - https://github.com/istio/istio/blob/2e4387a/pkg/config/kube/conversion.go#L76-L84
func IsHTTP2Port(port corev1.ServicePort) bool {
	if port.AppProtocol != nil {
		return parseProtocol(*port.AppProtocol).isHTTP2()
	}
	return parseProtocol(extractProtocolFromPortName(port.Name)).isHTTP2()
}

type serviceProtocol string

const (
	http2   serviceProtocol = "http2"
	grpc    serviceProtocol = "grpc"
	grpcWeb serviceProtocol = "grpc-web"
	other   serviceProtocol = ""
)

func (p serviceProtocol) isHTTP2() bool {
	switch p {
	case http2, grpc, grpcWeb:
		return true
	default:
		return false
	}
}

func parseProtocol(s string) serviceProtocol {
	switch strings.ToLower(s) {
	case "http2", "kubernetes.io/h2c":
		return http2
	case "grpc":
		return grpc
	case "grpc-web":
		return grpcWeb
	default:
		return other
	}
}

func extractProtocolFromPortName(name string) string {
	if len(name) >= len("grpc-web") && strings.EqualFold(name[:len("grpc-web")], "grpc-web") {
		return "grpc-web"
	}
	if prefix, _, ok := strings.Cut(name, "-"); ok {
		return prefix
	}
	return name
}
