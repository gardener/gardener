// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package highavailabilityconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Reconciler", func() {
	Describe("#needsHostSpreadUpdate", func() {
		It("should return false when there is no hostname constraint", func() {
			Expect(needsHostSpreadUpdate([]corev1.TopologySpreadConstraint{{TopologyKey: "topology.kubernetes.io/zone"}}, true)).To(BeFalse())
		})

		It("should return true when multiple nodes are available but the constraint allows scheduling anyway", func() {
			Expect(needsHostSpreadUpdate([]corev1.TopologySpreadConstraint{{TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: corev1.ScheduleAnyway}}, true)).To(BeTrue())
		})

		It("should return false when multiple nodes are available and the constraint already enforces spreading", func() {
			Expect(needsHostSpreadUpdate([]corev1.TopologySpreadConstraint{{TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: corev1.DoNotSchedule}}, true)).To(BeFalse())
		})

		It("should return true when a single node is available but the constraint still enforces spreading", func() {
			Expect(needsHostSpreadUpdate([]corev1.TopologySpreadConstraint{{TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: corev1.DoNotSchedule}}, false)).To(BeTrue())
		})
	})

	Describe("#needsZoneSpreadUpdate", func() {
		It("should return false when there is no hostname constraint", func() {
			Expect(needsZoneSpreadUpdate([]corev1.TopologySpreadConstraint{{TopologyKey: "topology.kubernetes.io/zone"}}, true)).To(BeFalse())
		})

		It("should return true when multiple zones are available but there is no zone constraint", func() {
			Expect(needsZoneSpreadUpdate([]corev1.TopologySpreadConstraint{{TopologyKey: "kubernetes.io/hostname"}}, true)).To(BeTrue())
		})

		It("should return false when multiple zones are available and a zone constraint exists", func() {
			Expect(needsZoneSpreadUpdate([]corev1.TopologySpreadConstraint{{TopologyKey: "kubernetes.io/hostname"}, {TopologyKey: "topology.kubernetes.io/zone"}}, true)).To(BeFalse())
		})

		It("should return true when less than two zones are available but a zone constraint still exists", func() {
			Expect(needsZoneSpreadUpdate([]corev1.TopologySpreadConstraint{{TopologyKey: "kubernetes.io/hostname"}, {TopologyKey: "topology.kubernetes.io/zone"}}, false)).To(BeTrue())
		})

		It("should return false when less than two zones are available and there is no zone constraint", func() {
			Expect(needsZoneSpreadUpdate([]corev1.TopologySpreadConstraint{{TopologyKey: "kubernetes.io/hostname"}}, false)).To(BeFalse())
		})
	})
})
