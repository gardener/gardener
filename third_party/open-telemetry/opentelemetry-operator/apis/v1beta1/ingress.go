// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1beta1/ingress.go.

package v1beta1

import networkingv1 "k8s.io/api/networking/v1"

type (
	// IngressType represents how a collector should be exposed (ingress vs route).
	// +kubebuilder:validation:Enum=ingress;route
	IngressType string
)

const (
	// IngressTypeIngress specifies that an ingress should be created.
	IngressTypeIngress IngressType = "ingress"
	// IngressTypeRoute IngressTypeOpenshiftRoute specifies that a route should be created.
	IngressTypeRoute IngressType = "route"
)

type (
	// TLSRouteTerminationType is used to indicate which tls settings should be used.
	// +kubebuilder:validation:Enum=insecure;edge;passthrough;reencrypt
	TLSRouteTerminationType string
)

const (
	// TLSRouteTerminationTypeInsecure indicates that insecure connections are allowed.
	TLSRouteTerminationTypeInsecure TLSRouteTerminationType = "insecure"
	// TLSRouteTerminationTypeEdge indicates that encryption should be terminated
	// at the edge router.
	TLSRouteTerminationTypeEdge TLSRouteTerminationType = "edge"
	// TLSRouteTerminationTypePassthrough indicates that the destination service is
	// responsible for decrypting traffic.
	TLSRouteTerminationTypePassthrough TLSRouteTerminationType = "passthrough"
	// TLSRouteTerminationTypeReencrypt indicates that traffic will be decrypted on the edge
	// and re-encrypt using a new certificate.
	TLSRouteTerminationTypeReencrypt TLSRouteTerminationType = "reencrypt"
)

// IngressRuleType defines how the collector receivers will be exposed in the Ingress.
//
// +kubebuilder:validation:Enum=path;subdomain
type IngressRuleType string

const (
	// IngressRuleTypePath configures Ingress to use single host with multiple paths.
	// This configuration might require additional ingress setting to rewrite paths.
	IngressRuleTypePath IngressRuleType = "path"

	// IngressRuleTypeSubdomain configures Ingress to use multiple hosts - one for each exposed
	// receiver port. The port name is used as a subdomain for the host defined in the Ingress e.g. otlp-http.example.com.
	IngressRuleTypeSubdomain IngressRuleType = "subdomain"
)

// Ingress is used to specify how OpenTelemetry Collector is exposed. This
// functionality is only available if one of the valid modes is set.
// Valid modes are: deployment, daemonset and statefulset.
// NOTE: If this feature is activated, all specified receivers are exposed.
// Currently, this has a few limitations. Depending on the ingress controller
// there are problems with TLS and gRPC.
// SEE: https://github.com/open-telemetry/opentelemetry-operator/issues/1306.
// NOTE: As a workaround, port name and appProtocol could be specified directly
// in the CR.
// SEE: OpenTelemetryCollector.spec.ports[index].
type Ingress struct {
	// Type default value is: ""
	// Supported types are: ingress, route
	Type IngressType `json:"type,omitempty"`

	// RuleType defines how Ingress exposes collector receivers.
	// IngressRuleTypePath ("path") exposes each receiver port on a unique path on single domain defined in Hostname.
	// IngressRuleTypeSubdomain ("subdomain") exposes each receiver port on a unique subdomain of Hostname.
	// Default is IngressRuleTypePath ("path").
	RuleType IngressRuleType `json:"ruleType,omitempty"`

	// Hostname by which the ingress proxy can be reached.
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// Annotations to add to ingress.
	// e.g. 'cert-manager.io/cluster-issuer: "letsencrypt"'
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// TLS configuration.
	// +optional
	TLS []networkingv1.IngressTLS `json:"tls,omitempty"`

	// IngressClassName is the name of an IngressClass cluster resource. Ingress
	// controller implementations use this field to know whether they should be
	// serving this Ingress resource.
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Route is an OpenShift specific section that is only considered when
	// type "route" is used.
	// +optional
	Route OpenShiftRoute `json:"route,omitempty"`
}

// OpenShiftRoute defines openshift route specific settings.
type OpenShiftRoute struct {
	// Termination indicates termination type. By default "edge" is used.
	Termination TLSRouteTerminationType `json:"termination,omitempty"`
}
