// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	pvcautoscalingv1alpha1 "github.com/gardener/pvc-autoscaler/api/autoscaling/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("PersistentVolumeClaimAutoscaler", func() {
	DescribeTable("CheckPersistentVolumeClaimAutoscaler",
		func(conditions []metav1.Condition, matcher types.GomegaMatcher) {
			pvca := &pvcautoscalingv1alpha1.PersistentVolumeClaimAutoscaler{
				Status: pvcautoscalingv1alpha1.PersistentVolumeClaimAutoscalerStatus{Conditions: conditions},
			}
			Expect(health.CheckPersistentVolumeClaimAutoscaler(pvca)).To(matcher)
		},
		Entry("no conditions", nil, BeNil()),
		Entry("RecommendationAvailable True", []metav1.Condition{
			{Type: string(pvcautoscalingv1alpha1.ConditionTypeRecommendationAvailable), Status: metav1.ConditionTrue},
		}, BeNil()),
		Entry("RecommendationAvailable True and Resizing True", []metav1.Condition{
			{Type: string(pvcautoscalingv1alpha1.ConditionTypeRecommendationAvailable), Status: metav1.ConditionTrue},
			{Type: string(pvcautoscalingv1alpha1.ConditionTypeResizing), Status: metav1.ConditionTrue},
		}, BeNil()),
		Entry("RecommendationAvailable False is ignored", []metav1.Condition{
			{Type: string(pvcautoscalingv1alpha1.ConditionTypeRecommendationAvailable), Status: metav1.ConditionFalse, Reason: "RecommendationError", Message: "storage class does not support expansion"},
		}, BeNil()),
		Entry("Resizing Unknown", []metav1.Condition{
			{Type: string(pvcautoscalingv1alpha1.ConditionTypeRecommendationAvailable), Status: metav1.ConditionTrue},
			{Type: string(pvcautoscalingv1alpha1.ConditionTypeResizing), Status: metav1.ConditionUnknown, Reason: "Reconcile", Message: "could not parse gardener.cloud/previous-size annotation"},
		}, MatchError(ContainSubstring(`condition "Resizing" has invalid status Unknown (expected True) due to Reconcile: could not parse gardener.cloud/previous-size annotation`))),
		Entry("Resizing False", []metav1.Condition{
			{Type: string(pvcautoscalingv1alpha1.ConditionTypeResizing), Status: metav1.ConditionFalse},
		}, MatchError(ContainSubstring(`condition "Resizing" has invalid status False (expected True)`))),
	)
})
