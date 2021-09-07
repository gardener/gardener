// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package imports

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CommonDeploymentConfiguration contains common deployment configurations for multiple Gardener components
type CommonDeploymentConfiguration struct {
	// ReplicaCount is the number of replicas.
	// Default: 1.
	ReplicaCount *int32
	// ServiceAccountName is the name of the ServiceAccount to create and mount into the pod.
	ServiceAccountName *string
	// Resources are compute resources required by the container.
	Resources *corev1.ResourceRequirements
	// PodLabels are additional labels on the pods.
	PodLabels map[string]string
	// PodAnnotations are additional annotations on the pods.
	PodAnnotations map[string]string
	// VPA specifies whether to enable VPA for the deployment.
	// Default: false.
	VPA *bool
}

// Configuration is a wrapper around the component configuration
type Configuration struct {
	// ComponentConfiguration is the component configuration for a component of the Gardener control plane
	ComponentConfiguration runtime.Object
}

// TLSServer configures the TLS serving endpoints of a component
type TLSServer struct {
	// Cert is a certificate used by the component to serve TLS endpoints.
	// If specified, the certificate must be signed by the configured CA.
	Crt string
	// Key is the key for the configured TLS certificate.
	Key string
}
