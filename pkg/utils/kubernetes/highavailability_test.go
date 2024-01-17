// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("HighAvailability", func() {
	DescribeTable("#GetReplicaCount",
		func(failureToleranceType *gardencorev1beta1.FailureToleranceType, componentType string, matcher gomegatypes.GomegaMatcher) {
			Expect(GetReplicaCount(failureToleranceType, componentType)).To(matcher)
		},

		Entry("component type is empty", nil, "", BeNil()),
		Entry("component type 'server', failure-tolerance-type nil", nil, "server", Equal(ptr.To(2))),
		Entry("component type 'server', failure-tolerance-type empty", failureToleranceTypePtr(""), "server", Equal(ptr.To(2))),
		Entry("component type 'server', failure-tolerance-type non-empty", failureToleranceTypePtr("foo"), "server", Equal(ptr.To(2))),
		Entry("component type 'controller', failure-tolerance-type nil", nil, "controller", Equal(ptr.To(2))),
		Entry("component type 'controller', failure-tolerance-type empty", failureToleranceTypePtr(""), "controller", Equal(ptr.To(1))),
		Entry("component type 'controller', failure-tolerance-type non-empty", failureToleranceTypePtr("foo"), "controller", Equal(ptr.To(2))),
	)

	zones := []string{"a", "b", "c"}

	DescribeTable("#GetNodeSelectorRequirementForZones",
		func(isZonePinningEnabled bool, zones []string, matcher gomegatypes.GomegaMatcher) {
			Expect(GetNodeSelectorRequirementForZones(isZonePinningEnabled, zones)).To(matcher)
		},

		Entry("no zones", false, nil, BeNil()),
		Entry("zone pinning disabled", false, zones, BeNil()),
		Entry("zone pinning enabled", true, zones, Equal(&corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: zones})),
		Entry("zones, but zone pinning disabled", false, zones, BeNil()),
		Entry("zones and zone pinning enabled", true, zones, Equal(&corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: zones})),
	)

	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}

	DescribeTable("#GetTopologySpreadConstraints",
		func(
			failureToleranceType *gardencorev1beta1.FailureToleranceType,
			replicas int,
			maxReplicas int,
			numberOfZones int,
			labelSelector metav1.LabelSelector,
			enforceSpreadAcrossHosts bool,
			matcher gomegatypes.GomegaMatcher,
		) {
			Expect(GetTopologySpreadConstraints(int32(replicas), int32(maxReplicas), labelSelector, int32(numberOfZones), failureToleranceType, enforceSpreadAcrossHosts)).To(matcher)
		},

		Entry("less than 2 replicas", nil, 1, 1, 1, labelSelector, false, BeNil()),
		Entry("1 zone, failure-tolerance-type nil", nil, 2, 2, 1, labelSelector, false, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &labelSelector})),
		Entry("1 zone, failure-tolerance-type nil, but host spread enforced", nil, 2, 2, 1, labelSelector, true, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector})),
		Entry("1 zone, failure-tolerance-type empty", failureToleranceTypePtr(""), 2, 2, 1, labelSelector, false, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &labelSelector})),
		Entry("1 zone, failure-tolerance-type non-empty", failureToleranceTypePtr("foo"), 2, 2, 1, labelSelector, false, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector})),
		Entry("2 zones, maxReplicas less twice the number of zones", nil, 2, 2, 2, labelSelector, false, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &labelSelector}, corev1.TopologySpreadConstraint{TopologyKey: "topology.kubernetes.io/zone", MaxSkew: 1, MinDomains: ptr.To(int32(2)), WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector})),
		Entry("2 zones, failure-tolerance-type nil", nil, 2, 2, 2, labelSelector, false, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &labelSelector}, corev1.TopologySpreadConstraint{TopologyKey: "topology.kubernetes.io/zone", MaxSkew: 1, MinDomains: ptr.To(int32(2)), WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector})),
		Entry("2 zones, failure-tolerance-type nil, but host spread enforced", nil, 2, 2, 2, labelSelector, true, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector}, corev1.TopologySpreadConstraint{TopologyKey: "topology.kubernetes.io/zone", MaxSkew: 1, MinDomains: ptr.To(int32(2)), WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector})),
		Entry("2 zones, failure-tolerance-type empty", failureToleranceTypePtr(""), 2, 2, 2, labelSelector, false, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &labelSelector})),
		Entry("2 zones, failure-tolerance-type non-empty", failureToleranceTypePtr("foo"), 2, 2, 2, labelSelector, false, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector})),
		Entry("2 zones, failure-tolerance-type 'zone'", failureToleranceTypePtr("zone"), 2, 2, 2, labelSelector, false, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector}, corev1.TopologySpreadConstraint{TopologyKey: "topology.kubernetes.io/zone", MaxSkew: 1, MinDomains: ptr.To(int32(2)), WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector})),
		Entry("2 zones, maxReplicas at least twice the number of zones", nil, 2, 4, 2, labelSelector, false, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &labelSelector}, corev1.TopologySpreadConstraint{TopologyKey: "topology.kubernetes.io/zone", MaxSkew: 2, MinDomains: ptr.To(int32(2)), WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector})),
		Entry("3 zones, maxReplicas less than zones", nil, 2, 2, 3, labelSelector, false, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &labelSelector}, corev1.TopologySpreadConstraint{TopologyKey: "topology.kubernetes.io/zone", MaxSkew: 1, MinDomains: ptr.To(int32(2)), WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector})),
		Entry("3 zones, maxReplicas less than zones, but host spread enforced", nil, 2, 2, 3, labelSelector, true, ConsistOf(corev1.TopologySpreadConstraint{TopologyKey: "kubernetes.io/hostname", MaxSkew: 1, WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector}, corev1.TopologySpreadConstraint{TopologyKey: "topology.kubernetes.io/zone", MaxSkew: 1, MinDomains: ptr.To(int32(2)), WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &labelSelector})),
	)
})

func failureToleranceTypePtr(v gardencorev1beta1.FailureToleranceType) *gardencorev1beta1.FailureToleranceType {
	return &v
}
