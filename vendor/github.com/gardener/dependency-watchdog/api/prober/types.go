// Copyright 2022 SAP SE or an SAP affiliate company
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

package prober

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Config provides typed access to prober configuration
type Config struct {
	// InternalKubeConfigSecretName is the name of the kubernetes secret which has the kubeconfig to connect to the shoot control plane API server via internal domain
	InternalKubeConfigSecretName string `json:"internalKubeConfigSecretName"`
	// ExternalKubeConfigSecretName is the name of the kubernetes secret which has the kubeconfig to connect to the shoot control plane API server via external domain
	ExternalKubeConfigSecretName string `json:"externalKubeConfigSecretName"`
	// ProbeInterval is the interval with which the probe will be run
	ProbeInterval *metav1.Duration `json:"probeInterval,omitempty"`
	// InitialDelay is the initial delay in running a probe for the first time
	InitialDelay *metav1.Duration `json:"initialDelay,omitempty"`
	// ProbeTimeout is the timeout that is set on the client which is used to reach the shoot control plane API server
	ProbeTimeout *metav1.Duration `json:"probeTimeout,omitempty"`
	// SuccessThreshold is the number of consecutive times a probe is successful to ascertain that the probe is healthy
	SuccessThreshold *int `json:"successThreshold,omitempty"`
	// FailureThreshold is the number of consecutive times a probe is unsuccessful to ascertain that the probe is unhealthy
	FailureThreshold *int `json:"failureThreshold,omitempty"`
	// InternalProbeFailureBackoffDuration is the backoff duration if the internal probe is unhealthy, before reattempting the internal probe
	InternalProbeFailureBackoffDuration *metav1.Duration `json:"internalProbeFailureBackoffDuration,omitempty"`
	// BackoffJitterFactor is the jitter with which a probe is run
	BackoffJitterFactor *float64 `json:"backoffJitterFactor,omitempty"`
	// DependentResourceInfos are the dependent resources that should be considered for scaling in case the shoot control API server cannot be reached via external domain
	DependentResourceInfos []DependentResourceInfo `json:"dependentResourceInfos"`
}

// DependentResourceInfo captures a dependent resource which should be scaled
type DependentResourceInfo struct {
	// Ref identifies a resource
	Ref *autoscalingv1.CrossVersionObjectReference `json:"ref"`
	// Optional should be false if this resource should be present. If the resource is optional then it should be true
	// If this field is not specified, then its zero value (false for boolean) will be assumed.
	Optional bool `json:"optional"`
	// ScaleUpInfo captures the configuration to scale up the resource identified by Ref
	ScaleUpInfo *ScaleInfo `json:"scaleUp,omitempty"`
	// ScaleDownInfo captures the configuration to scale down the resource identified by Ref
	ScaleDownInfo *ScaleInfo `json:"scaleDown,omitempty"`
}

// ScaleInfo captures the configuration required to scale a dependent resource
type ScaleInfo struct {
	// Level is used to order the dependent resources. Highest level or the first level starts at 0 and increments. Each dependent resource on a level will have to wait for
	// all resource in a previous level to finish their scaling operation. If there are more than one resource defined with the same level then they will be scaled concurrently.
	Level int `json:"level"`
	// InitialDelay is the time to delay (duration) the scale down/up of this resource. If not specified its default value will be 30s.
	InitialDelay *metav1.Duration `json:"initialDelay,omitempty"`
	// ScaleTimeout is the time timeout duration to wait for when attempting to update the scaling sub-resource.
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}
