// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProviderConfig contains the configuration API for cloud-controller-manager-local (used by the
// pkg/provider-local/cloud-provider package).
type CloudProviderConfig struct {
	metav1.TypeMeta `json:",inline"`

	// RuntimeCluster configures how cloud-controller-manager-local connects to the runtime cluster (seed) of the shoot
	// cluster, i.e., the kind cluster where the shoot machine pods run.
	// This is only required if the cloud-controller-manager-local is running for a shoot cluster, not for the kind
	// cluster itself.
	// +optional
	RuntimeCluster *RuntimeCluster `json:"runtimeCluster,omitempty"`
	// LoadBalancer contains the configuration for the service controller of cloud-controller-manager-local.
	LoadBalancer LoadBalancer `json:"loadBalancer"`
}

// RuntimeCluster configures how cloud-controller-manager-local connects to the runtime cluster (seed) of the shoot
// cluster, i.e., the kind cluster where the shoot machine pods run.
type RuntimeCluster struct {
	// Namespace configures the namespace of the runtime cluster where the shoot machine pods run.
	// If RuntimeCluster is set, this field is required.
	Namespace string `json:"namespace"`
	// Kubeconfig configures the path to the kubeconfig file for connecting to the runtime cluster. If not set,
	// cloud-controller-manager-local uses the in-cluster credentials (ServiceAccount).
	// +optional
	Kubeconfig *string `json:"kubeconfig,omitempty"`
}

// LoadBalancer contains the configuration for the service controller of cloud-controller-manager-local.
type LoadBalancer struct {
	// Image is the envoy container image used for starting load balancer containers.
	Image string `json:"image"`
}
