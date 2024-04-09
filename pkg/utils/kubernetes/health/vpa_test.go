// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
