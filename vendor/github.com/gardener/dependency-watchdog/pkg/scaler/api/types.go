// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
)

// ProbeDependantsList holds a list of probes (internal and external) and their corresponding
// dependant Scales. If the external probe fails and the internal probe still succeeds, then the
// corresponding dependant Scales are scaled down to `zero`. They are scaled back to their
// original scale when the external probe succeeds again.
type ProbeDependantsList struct {
	Probes    []ProbeDependants `json:"probes"`
	Namespace string            `json:"namespace"`
}

// ProbeDependants struct captures the details about a probe and its dependant scale sub-resources.
type ProbeDependants struct {
	Name            string                   `json:"name"`
	Probe           *ProbeConfig             `json:"probe"`
	DependantScales []*DependantScaleDetails `json:"dependantScales"`
}

// ProbeConfig struct captures the details for probing a Kubernetes apiserver.
type ProbeConfig struct {
	External            *ProbeDetails `json:"external,omitempty"`
	Internal            *ProbeDetails `json:"internal,omitempty"`
	InitialDelaySeconds *int32        `json:"initialDelaySeconds,omitempty"`
	TimeoutSeconds      *int32        `json:"timeoutSeconds,omitempty"`
	PeriodSeconds       *int32        `json:"periodSeconds,omitempty"`
	SuccessThreshold    *int32        `json:"successThreshold,omitempty"`
	FailureThreshold    *int32        `json:"failureThreshold,omitempty"`
}

// ProbeDetails captures the kubeconfig secret details to probe a Kubernetes apiserver.
type ProbeDetails struct {
	KubeconfigSecretName string `json:"kubeconfigSecretName"`
}

// DependantScaleDetails has the details about the dependant scale sub-resource.
type DependantScaleDetails struct {
	ScaleRef              autoscalingv1.CrossVersionObjectReference   `json:"scaleRef"`
	Replicas              *int32                                      `json:"replicas"`
	ScaleUpDelaySeconds   *int32                                      `json:"scaleUpDelaySeconds,omitempty"`
	ScaleDownDelaySeconds *int32                                      `json:"scaleDownDelaySeconds,omitempty"`
	ScaleRefDependsOn     []autoscalingv1.CrossVersionObjectReference `json:"scaleRefDependsOn,omitempty"`
}
