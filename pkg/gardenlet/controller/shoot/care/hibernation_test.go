// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
)

var _ = Describe("#IsMaintenanceWindowInHibernationWindow", func() {
	const (
		maintBegin = "220000+0000"
		maintEnd   = "230000+0000"
	)

	makeShoot := func(begin, end string, schedules ...gardencorev1beta1.HibernationSchedule) *gardencorev1beta1.Shoot {
		return &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Maintenance: &gardencorev1beta1.Maintenance{
					TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
						Begin: begin,
						End:   end,
					},
				},
				Hibernation: &gardencorev1beta1.Hibernation{
					Schedules: schedules,
				},
			},
		}
	}

	makeShootDefault := func(schedules ...gardencorev1beta1.HibernationSchedule) *gardencorev1beta1.Shoot {
		return makeShoot(maintBegin, maintEnd, schedules...)
	}

	DescribeTable("should return the expected result",
		func(shoot *gardencorev1beta1.Shoot, expected bool) {
			Expect(IsShootAlwaysHibernatedDuringMaintenance(shoot)).To(Equal(expected))
		},
		Entry("no maintenance time window set",
			&gardencorev1beta1.Shoot{},
			false,
		),
		Entry("no hibernation schedules",
			makeShootDefault(),
			false,
		),
		Entry("schedule has only Start, no End",
			makeShootDefault(gardencorev1beta1.HibernationSchedule{Start: new("0 20 * * *")}),
			true,
		),
		Entry("schedule has only End, no Start",
			makeShootDefault(gardencorev1beta1.HibernationSchedule{End: new("0 8 * * *")}),
			false,
		),
		Entry("maintenance window inside hibernation window",
			makeShootDefault(gardencorev1beta1.HibernationSchedule{Start: new("0 20 * * *"), End: new("0 8 * * *")}),
			true,
		),
		Entry("maintenance window inside wake window",
			makeShootDefault(gardencorev1beta1.HibernationSchedule{Start: new("30 23 * * *"), End: new("0 8 * * *")}),
			false,
		),
		Entry("maintenance window starts in wake window, ends in hibernation window",
			makeShootDefault(gardencorev1beta1.HibernationSchedule{Start: new("30 22 * * *"), End: new("0 8 * * *")}),
			true,
		),
		Entry("maintenance window starts in hibernation window, ends in wake window",
			makeShootDefault(gardencorev1beta1.HibernationSchedule{Start: new("0 22 * * *"), End: new("30 22 * * *")}),
			true,
		),
		Entry("weekday-only schedule: maintenance window inside weekday hibernation window, no wake on weekends",
			makeShootDefault(gardencorev1beta1.HibernationSchedule{Start: new("0 20 * * 1-5"), End: new("0 8 * * 1-5")}),
			true,
		),
		Entry("two schedules covering all seven days: maintenance always inside hibernation window",
			makeShootDefault(
				gardencorev1beta1.HibernationSchedule{Start: new("0 20 * * 1-5"), End: new("0 8 * * 1-5")},
				gardencorev1beta1.HibernationSchedule{Start: new("0 20 * * 0,6"), End: new("0 8 * * 0,6")},
			),
			true,
		),
		Entry("weekday-only schedule: maintenance window on weekends falls outside hibernation window",
			makeShoot("100000+0000", "110000+0000",
				gardencorev1beta1.HibernationSchedule{Start: new("0 20 * * 1-5"), End: new("0 8 * * 1-5")}),
			false,
		),
		Entry("monthly schedule: no hibernation events in simulation window",
			makeShootDefault(gardencorev1beta1.HibernationSchedule{Start: new("0 20 1 * *"), End: new("0 8 2 * *")}),
			false,
		),
		Entry("two overlapping wake schedules: maintenance window inside hibernation window",
			makeShootDefault(
				gardencorev1beta1.HibernationSchedule{Start: new("0 20 * * *"), End: new("0 6 * * *")},
				gardencorev1beta1.HibernationSchedule{Start: new("0 21 * * *"), End: new("0 7 * * *")},
			),
			true,
		),
		Entry("two overlapping wake schedules: maintenance window inside wake window",
			makeShoot("210000+0000", "220000+0000",
				gardencorev1beta1.HibernationSchedule{Start: new("0 23 * * *"), End: new("0 6 * * *")},
				gardencorev1beta1.HibernationSchedule{Start: new("0 0 * * *"), End: new("0 7 * * *")},
			),
			false,
		),
		Entry("cross-midnight maintenance window inside hibernation window",
			makeShoot("230000+0000", "010000+0000",
				gardencorev1beta1.HibernationSchedule{Start: new("0 22 * * *"), End: new("0 6 * * *")}),
			true,
		),
		Entry("cross-midnight maintenance window inside wake window",
			makeShoot("230000+0000", "010000+0000",
				gardencorev1beta1.HibernationSchedule{Start: new("0 2 * * *"), End: new("0 21 * * *")}),
			false,
		),
	)
})
