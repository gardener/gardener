// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Node", func() {
	Describe("CheckNode", func() {
		DescribeTable("nodes",
			func(node *corev1.Node, matcher types.GomegaMatcher) {
				err := health.CheckNode(node)
				Expect(err).To(matcher)
			},
			Entry("healthy", &corev1.Node{
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
			}, BeNil()),
			Entry("no ready condition", &corev1.Node{}, HaveOccurred()),
			Entry("ready condition not indicating true", &corev1.Node{
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}},
			}, HaveOccurred()),
		)
	})

	Describe("FilterHealthyNodes", func() {
		node := func(name string, conditions ...corev1.NodeCondition) corev1.Node {
			return corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Status:     corev1.NodeStatus{Conditions: conditions},
			}
		}

		It("should keep healthy nodes and drop unhealthy ones", func() {
			nodes := []corev1.Node{
				node("not-ready", corev1.NodeCondition{Type: corev1.NodeReady, Status: corev1.ConditionFalse}),
				node("pressured",
					corev1.NodeCondition{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					corev1.NodeCondition{Type: corev1.NodeDiskPressure, Status: corev1.ConditionTrue},
				),
				node("healthy", corev1.NodeCondition{Type: corev1.NodeReady, Status: corev1.ConditionTrue}),
			}

			result := health.FilterHealthyNodes(nodes)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Name).To(Equal("healthy"))
		})

		It("should return an empty slice when no nodes are healthy", func() {
			Expect(health.FilterHealthyNodes(nil)).To(BeEmpty())
		})
	})
})
