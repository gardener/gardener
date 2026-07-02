// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// ControlPlaneEndpointsFromNodes builds the list of ControlPlaneEndpoints for a SelfHostedShootExposure resource from
// the given control plane nodes. The extension controller may choose which addresses to use, so all are included.
// Returns an error if there are no healthy nodes or if a node has no addresses.
func ControlPlaneEndpointsFromNodes(nodes []corev1.Node) ([]extensionsv1alpha1.ControlPlaneEndpoint, error) {
	healthy := health.FilterHealthyNodes(nodes)
	if len(healthy) == 0 {
		return nil, fmt.Errorf("no healthy control plane nodes found")
	}

	endpoints := make([]extensionsv1alpha1.ControlPlaneEndpoint, 0, len(healthy))
	for i := range healthy {
		node := &healthy[i]
		if len(node.Status.Addresses) == 0 {
			return nil, fmt.Errorf("node %q has no addresses", node.Name)
		}
		nodeCopy := node.DeepCopy()
		endpoints = append(endpoints, extensionsv1alpha1.ControlPlaneEndpoint{
			NodeName:  nodeCopy.Name,
			Addresses: nodeCopy.Status.Addresses,
		})
	}

	return endpoints, nil
}
