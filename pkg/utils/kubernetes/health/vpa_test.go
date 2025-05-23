// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("VPA", func() {
	DescribeTable("CheckVerticalPodAutoscaler",
		func(vpa *vpaautoscalingv1.VerticalPodAutoscaler, matcher types.GomegaMatcher) {
			err := health.CheckVerticalPodAutoscaler(vpa)
			Expect(err).To(matcher)
		},
		Entry("healthy", &vpaautoscalingv1.VerticalPodAutoscaler{}, BeNil()),
		Entry("condition False", &vpaautoscalingv1.VerticalPodAutoscaler{
			Status: vpaautoscalingv1.VerticalPodAutoscalerStatus{Conditions: []vpaautoscalingv1.VerticalPodAutoscalerCondition{{Type: vpaautoscalingv1.ConfigUnsupported, Status: corev1.ConditionFalse}}},
		}, BeNil()),
		Entry("condition True", &vpaautoscalingv1.VerticalPodAutoscaler{
			Status: vpaautoscalingv1.VerticalPodAutoscalerStatus{Conditions: []vpaautoscalingv1.VerticalPodAutoscalerCondition{{Type: vpaautoscalingv1.ConfigUnsupported, Status: corev1.ConditionTrue}}},
		}, MatchError(ContainSubstring(`condition "ConfigUnsupported" has invalid status True (expected False)`))),
	)
})
