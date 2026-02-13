// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1alpha1/samplers.go.

package v1alpha1

type (
	// SamplerType represents sampler type.
	// +kubebuilder:validation:Enum=always_on;always_off;traceidratio;parentbased_always_on;parentbased_always_off;parentbased_traceidratio;jaeger_remote;xray
	SamplerType string
)

const (
	// AlwaysOn represents AlwaysOnSampler.
	AlwaysOn SamplerType = "always_on"
	// AlwaysOff represents AlwaysOffSampler.
	AlwaysOff SamplerType = "always_off"
	// TraceIDRatio represents TraceIdRatioBased.
	TraceIDRatio SamplerType = "traceidratio"
	// ParentBasedAlwaysOn represents ParentBased(root=AlwaysOnSampler).
	ParentBasedAlwaysOn SamplerType = "parentbased_always_on"
	// ParentBasedAlwaysOff represents ParentBased(root=AlwaysOffSampler).
	ParentBasedAlwaysOff SamplerType = "parentbased_always_off"
	// ParentBasedTraceIDRatio represents ParentBased(root=TraceIdRatioBased).
	ParentBasedTraceIDRatio SamplerType = "parentbased_traceidratio"
	// JaegerRemote represents JaegerRemoteSampler.
	JaegerRemote SamplerType = "jaeger_remote"
	// ParentBasedJaegerRemote represents ParentBased(root=JaegerRemoteSampler).
	ParentBasedJaegerRemote SamplerType = "parentbased_jaeger_remote"
	// XRay represents AWS X-Ray Centralized Sampling.
	XRaySampler SamplerType = "xray"
)
