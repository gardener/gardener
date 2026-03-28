// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProviderConfig contains the configuration API for cloud-controller-manager-local (used by the
// pkg/provider-local/cloud-provider package).
type CloudProviderConfig struct {
	metav1.TypeMeta

	// RuntimeCluster configures how cloud-controller-manager-local connects to the runtime cluster (seed) of the shoot
	// cluster, i.e., the kind cluster where the shoot machine pods run.
	// This is only required if the cloud-controller-manager-local is running for a shoot cluster, not for the kind
	// cluster itself.
	RuntimeCluster *RuntimeCluster
	// LoadBalancer contains the configuration for the service controller of cloud-controller-manager-local.
	LoadBalancer LoadBalancer
}

// RuntimeCluster configures how cloud-controller-manager-local connects to the runtime cluster (seed) of the shoot
// cluster, i.e., the kind cluster where the shoot machine pods run.
type RuntimeCluster struct {
	// Namespace configures the namespace of the runtime cluster where the shoot machine pods run.
	// If RuntimeCluster is set, this field is required.
	Namespace string
	// Kubeconfig configures the path to the kubeconfig file for connecting to the runtime cluster. If not set,
	// cloud-controller-manager-local uses the in-cluster credentials (ServiceAccount).
	Kubeconfig *string
}

// LoadBalancer contains the configuration for the service controller of cloud-controller-manager-local.
type LoadBalancer struct {
	// Image is the envoy container image used for starting load balancer containers.
	Image string
}
