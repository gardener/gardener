// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1alpha1/propagators.go.

package v1alpha1

type (
	// Propagator represents the propagation type.
	// +kubebuilder:validation:Enum=tracecontext;baggage;b3;b3multi;jaeger;xray;ottrace;none
	Propagator string
)

const (
	// TraceContext represents W3C Trace Context.
	TraceContext Propagator = "tracecontext"
	// Baggage represents W3C Baggage.
	Baggage Propagator = "baggage"
	// B3 represents B3 Single.
	B3 Propagator = "b3"
	// B3Multi represents B3 Multi.
	B3Multi Propagator = "b3multi"
	// Jaeger represents Jaeger.
	Jaeger Propagator = "jaeger"
	// XRay represents AWS X-Ray.
	XRay Propagator = "xray"
	// OTTrace represents OT Trace.
	OTTrace Propagator = "ottrace"
	// None represents automatically configured propagator.
	None Propagator = "none"
)
