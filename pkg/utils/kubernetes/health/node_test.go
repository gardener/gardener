// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"

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

	Describe("IsNodePreservedAndUnhealthy", func() {
		DescribeTable("nodes",
			func(node corev1.Node, expected bool) {
				Expect(health.IsNodePreservedAndUnhealthy(node)).To(Equal(expected))
			},
			Entry("preserved and not ready (False)",
				corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
					{Type: machinev1alpha1.NodePreserved, Status: corev1.ConditionTrue},
					{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
				}}},
				true),
			Entry("preserved and not ready (Unknown)",
				corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
					{Type: machinev1alpha1.NodePreserved, Status: corev1.ConditionTrue},
					{Type: corev1.NodeReady, Status: corev1.ConditionUnknown},
				}}},
				true),
			Entry("preserved and ready",
				corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
					{Type: machinev1alpha1.NodePreserved, Status: corev1.ConditionTrue},
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				}}},
				false),
			Entry("not preserved and not ready",
				corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
				}}},
				false),
			Entry("no conditions",
				corev1.Node{},
				false),
		)
	})
})
