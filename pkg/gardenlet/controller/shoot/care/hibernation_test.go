// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
)

var _ = Describe("#IsShootHibernatedDuringNextMaintenanceWindow", func() {
	now := time.Date(2024, time.January, 8, 15, 0, 0, 0, time.UTC)

	// nowHibernated is inside a typical overnight hibernation window:
	nowHibernated := time.Date(2024, time.January, 8, 21, 0, 0, 0, time.UTC)

	const (
		// maintenance window 22:00–23:00 UTC (nightly)
		maintBegin = "220000+0000"
		maintEnd   = "230000+0000"
	)

	makeShoot := func(begin, end string, isHibernated bool, schedules ...gardencorev1beta1.HibernationSchedule) *gardencorev1beta1.Shoot {
		shoot := &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Maintenance: &gardencorev1beta1.Maintenance{
					TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
						Begin: begin,
						End:   end,
					},
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				IsHibernated: isHibernated,
			},
		}
		if len(schedules) > 0 {
			shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
				Schedules: schedules,
			}
		}
		return shoot
	}

	makeShootDefault := func(isHibernated bool, schedules ...gardencorev1beta1.HibernationSchedule) *gardencorev1beta1.Shoot {
		return makeShoot(maintBegin, maintEnd, isHibernated, schedules...)
	}

	nightlySchedule := gardencorev1beta1.HibernationSchedule{Start: new("0 20 * * *"), End: new("0 8 * * *")}
	weekdaySchedule := gardencorev1beta1.HibernationSchedule{Start: new("0 20 * * 1-5"), End: new("0 8 * * 1-5")}

	DescribeTable("should return the expected result",
		func(shoot *gardencorev1beta1.Shoot, t time.Time, expected bool) {
			Expect(IsShootHibernatedDuringNextMaintenanceWindow(shoot, t)).To(Equal(expected))
		},

		Entry("no maintenance time window set",
			&gardencorev1beta1.Shoot{}, now,
			false,
		),
		Entry("no schedules, not hibernated",
			makeShootDefault(false), now,
			false,
		),

		Entry("hibernated with no schedules — stuck hibernated",
			makeShootDefault(true), now,
			true,
		),

		// Currently AWAKE

		Entry("awake: maintenance inside nightly hibernation window",
			makeShootDefault(false, nightlySchedule), now,
			true,
		),
		Entry("awake: hibernate after maintenance window end",
			makeShootDefault(false, gardencorev1beta1.HibernationSchedule{
				Start: new("30 23 * * *"), End: new("0 8 * * *"),
			}), now,
			false,
		),
		Entry("awake: hibernate inside maintenance window",
			makeShootDefault(false, gardencorev1beta1.HibernationSchedule{
				Start: new("30 22 * * *"), End: new("0 8 * * *"),
			}), now,
			true,
		),
		Entry("awake: shoot wakes up before maintenance",
			makeShootDefault(false, gardencorev1beta1.HibernationSchedule{
				Start: new("0 20 * * *"), End: new("0 21 * * *"),
			}), now,
			false,
		),
		Entry("awake: hibernate-only schedule, no wake-up",
			makeShootDefault(false, gardencorev1beta1.HibernationSchedule{
				Start: new("0 20 * * *"),
			}), now,
			true,
		),
		Entry("awake: wake-only schedule, no hibernate",
			makeShootDefault(false, gardencorev1beta1.HibernationSchedule{
				End: new("0 8 * * *"),
			}), now,
			false,
		),
		Entry("awake: weekday schedule, maintenance inside weeknight hibernation window",
			makeShootDefault(false, weekdaySchedule), now,
			true,
		),
		Entry("awake: cross-midnight maintenance inside hibernation window",
			makeShoot("230000+0000", "010000+0000", false,
				gardencorev1beta1.HibernationSchedule{Start: new("0 20 * * *"), End: new("0 6 * * *")}), now,
			true,
		),
		Entry("awake: cross-midnight maintenance before next hibernate",
			makeShoot("230000+0000", "010000+0000", false,
				gardencorev1beta1.HibernationSchedule{Start: new("0 2 * * *"), End: new("0 21 * * *")}), now,
			false,
		),

		// Currently HIBERNATED

		Entry("hibernated: manually hibernated, but wakes up in time",
			makeShootDefault(true, gardencorev1beta1.HibernationSchedule{
				End: new("0 20 * * *"),
			}), now,
			false,
		),

		Entry("hibernated: nightly schedule, next wake-up is after tonight's maintenance",
			makeShootDefault(true, nightlySchedule), nowHibernated,
			true,
		),
		Entry("hibernated: wake-up inside maintenance window",
			makeShootDefault(true, gardencorev1beta1.HibernationSchedule{
				Start: new("0 20 * * *"), End: new("30 22 * * *"),
			}), nowHibernated,
			false,
		),
		Entry("hibernated: wake-up after maintenance window end",
			makeShootDefault(true, gardencorev1beta1.HibernationSchedule{
				Start: new("0 20 * * *"), End: new("30 23 * * *"),
			}), nowHibernated,
			true,
		),
		Entry("hibernated: wake-up before maintenance start",
			makeShootDefault(true, gardencorev1beta1.HibernationSchedule{
				Start: new("0 20 * * *"), End: new("30 21 * * *"),
			}), nowHibernated,
			false,
		),
		Entry("hibernated: weekday schedule, no wake-up before tonight's maintenance",
			makeShootDefault(true, weekdaySchedule), nowHibernated,
			true,
		),
		Entry("hibernated: hibernate-only schedule, no wake-up ever",
			makeShootDefault(true, gardencorev1beta1.HibernationSchedule{
				Start: new("0 20 * * *"),
			}), nowHibernated,
			true,
		),
		Entry("hibernated: wakes up before maintenance but re-hibernates before it",
			makeShootDefault(true, gardencorev1beta1.HibernationSchedule{
				Start: new("45 21 * * *"), End: new("30 21 * * *"),
			}), nowHibernated,
			true,
		),
	)
})
