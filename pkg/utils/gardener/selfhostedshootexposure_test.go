// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("SelfHostedShootExposure", func() {
	var (
		// healthyNode returns a node that passes health.CheckNode (Ready, no pressure) with the given addresses.
		healthyNode = func(name string, addresses ...corev1.NodeAddress) corev1.Node {
			return corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
					Addresses:  addresses,
				},
			}
		}
		// unhealthyNode returns a node that fails health.CheckNode (NotReady).
		unhealthyNode = func(name string, addresses ...corev1.NodeAddress) corev1.Node {
			n := healthyNode(name, addresses...)
			n.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}
			return n
		}
		externalIP = func(address string) corev1.NodeAddress {
			return corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: address}
		}
		internalIP = func(address string) corev1.NodeAddress {
			return corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: address}
		}
	)

	Describe("#ControlPlaneEndpointsFromNodes", func() {
		It("should build endpoints from healthy nodes with all their addresses", func() {
			nodes := []corev1.Node{
				healthyNode("cp-1", externalIP("1.2.3.4"), internalIP("10.0.0.1")),
				healthyNode("cp-2", externalIP("1.2.3.5")),
			}

			endpoints, err := ControlPlaneEndpointsFromNodes(nodes)

			Expect(err).NotTo(HaveOccurred())
			Expect(endpoints).To(ConsistOf(
				extensionsv1alpha1.ControlPlaneEndpoint{NodeName: "cp-1", Addresses: []corev1.NodeAddress{externalIP("1.2.3.4"), internalIP("10.0.0.1")}},
				extensionsv1alpha1.ControlPlaneEndpoint{NodeName: "cp-2", Addresses: []corev1.NodeAddress{externalIP("1.2.3.5")}},
			))
		})

		It("should drop unhealthy nodes when at least one node is healthy", func() {
			nodes := []corev1.Node{
				unhealthyNode("cp-1", internalIP("10.0.0.1")),
				healthyNode("cp-2", internalIP("10.0.0.2")),
			}

			endpoints, err := ControlPlaneEndpointsFromNodes(nodes)

			Expect(err).NotTo(HaveOccurred())
			Expect(endpoints).To(ConsistOf(
				extensionsv1alpha1.ControlPlaneEndpoint{NodeName: "cp-2", Addresses: []corev1.NodeAddress{internalIP("10.0.0.2")}},
			))
		})

		It("should return an error if no node is healthy (keeping the last good endpoints)", func() {
			_, err := ControlPlaneEndpointsFromNodes([]corev1.Node{unhealthyNode("cp-1", internalIP("10.0.0.1"))})
			Expect(err).To(MatchError(ContainSubstring("no healthy control plane nodes found")))
		})

		It("should return an error if there are no nodes", func() {
			_, err := ControlPlaneEndpointsFromNodes(nil)
			Expect(err).To(MatchError(ContainSubstring("no healthy control plane nodes found")))
		})

		It("should return an error if a node has no addresses", func() {
			_, err := ControlPlaneEndpointsFromNodes([]corev1.Node{healthyNode("cp-1")})
			Expect(err).To(MatchError(ContainSubstring(`node "cp-1" has no addresses`)))
		})
	})
})
