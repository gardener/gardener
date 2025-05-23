// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package timewindow_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/gardener/gardener/pkg/utils/timewindow"
)

var _ = Describe("Schedule", func() {
	var (
		scheduleFormat = "%d %d"
		uid            = types.UID("uid")
		window         = NewMaintenanceTimeWindow(
			NewMaintenanceTime(14, 0, 0),
			NewMaintenanceTime(22, 0, 0),
		)
	)

	Describe("#DetermineSchedule", func() {
		var (
			begin             = "140000+0100"
			end               = "220000+0100"
			creationTimestamp = metav1.Time{}
			mutate            = func(string, MaintenanceTimeWindow, types.UID) string {
				return "foo"
			}
		)

		It("should return an error because the time window cannot be parsed", func() {
			schedule, err := DetermineSchedule(scheduleFormat, begin, "not-parseable", uid, creationTimestamp, mutate)
			Expect(err).To(HaveOccurred())
			Expect(schedule).To(BeEmpty())
		})

		It("should use the mutate function", func() {
			schedule, err := DetermineSchedule(scheduleFormat, begin, end, uid, creationTimestamp, mutate)
			Expect(err).NotTo(HaveOccurred())
			Expect(schedule).To(Equal("foo"))
		})

		It("should not use the mutate function because time window is equal to always window", func() {
			schedule, err := DetermineSchedule(scheduleFormat, "000000+0000", "235959+0000", uid, creationTimestamp, mutate)
			Expect(err).NotTo(HaveOccurred())
			Expect(schedule).To(Equal("0 0"))
		})
	})

	Describe("#RandomizeWithinTimeWindow", func() {
		It("should compute a pseudo-randomized time within the time window", func() {
			Expect(RandomizeWithinTimeWindow(scheduleFormat, *window, uid)).To(Equal("10 15"))
		})
	})

	Describe("#RandomizeWithinFirstHourOfTimeWindow", func() {
		It("should compute a pseudo-randomized time within the first hour of the time window", func() {
			Expect(RandomizeWithinFirstHourOfTimeWindow(scheduleFormat, *window, uid)).To(Equal("55 12"))
		})
	})
})
