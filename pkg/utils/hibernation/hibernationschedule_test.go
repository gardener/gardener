// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hibernation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/hibernation"
)

var _ = Describe("hibernationschedule", func() {
	Describe("#Parse", func() {
		DescribeTable("should return the expected result",
			func(schedules []gardencorev1beta1.HibernationSchedule, expectErr bool, wantOps []Operation) {
				out, err := Parse(schedules)
				if expectErr {
					Expect(err).To(HaveOccurred())
					return
				}
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(HaveLen(len(wantOps)))
				for i, op := range wantOps {
					Expect(out[i].Operation).To(Equal(op))
				}
			},
			Entry("nil input → empty slice", nil, false, nil),
			Entry("invalid Start → error",
				[]gardencorev1beta1.HibernationSchedule{{Start: new("not-a-cron")}}, true, nil),
			Entry("invalid End → error",
				[]gardencorev1beta1.HibernationSchedule{{End: new("also-invalid")}}, true, nil),
			Entry("unknown location → error",
				[]gardencorev1beta1.HibernationSchedule{{Start: new("0 20 * * *"), Location: new("Moon/Crater")}}, true, nil),
			Entry("Start only → Hibernate",
				[]gardencorev1beta1.HibernationSchedule{{Start: new("0 20 * * *")}}, false, []Operation{Hibernate}),
			Entry("End only → WakeUp",
				[]gardencorev1beta1.HibernationSchedule{{End: new("0 8 * * *")}}, false, []Operation{WakeUp}),
			Entry("Start+End → Hibernate then WakeUp",
				[]gardencorev1beta1.HibernationSchedule{{Start: new("0 20 * * *"), End: new("0 8 * * *")}},
				false, []Operation{Hibernate, WakeUp}),
			Entry("two schedules → four entries",
				[]gardencorev1beta1.HibernationSchedule{
					{Start: new("0 20 * * 1-5"), End: new("0 8 * * 1-5")},
					{Start: new("0 22 * * 0,6"), End: new("0 10 * * 0,6")},
				}, false, []Operation{Hibernate, WakeUp, Hibernate, WakeUp}),
		)

		It("defaults location to UTC when nil", func() {
			out, err := Parse([]gardencorev1beta1.HibernationSchedule{{Start: new("0 20 * * *")}})
			Expect(err).NotTo(HaveOccurred())
			Expect(out[0].Location).To(Equal(*time.UTC))
		})

		It("loads a non-UTC location", func() {
			out, err := Parse([]gardencorev1beta1.HibernationSchedule{{Start: new("0 20 * * *"), Location: new("Europe/Berlin")}})
			Expect(err).NotTo(HaveOccurred())
			berlin, _ := time.LoadLocation("Europe/Berlin")
			Expect(out[0].Location).To(Equal(*berlin))
		})
	})

	// ref is Monday 2006-01-02 00:00:00 UTC — the same anchor used by IsMaintenanceWindowInHibernationWindow.
	ref := time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC)

	// dailyAt20 is a UTC schedule that fires every day at 20:00.
	mustDailyAt20 := func() ParsedSchedule {
		out, err := Parse([]gardencorev1beta1.HibernationSchedule{{Start: new("0 20 * * *")}})
		Expect(err).NotTo(HaveOccurred())
		return out[0]
	}

	Describe("#Next", func() {
		DescribeTable("should return the correct next occurrence in UTC",
			func(spec, location string, from, want time.Time) {
				var loc *string
				if location != "" {
					loc = new(location)
				}
				out, err := Parse([]gardencorev1beta1.HibernationSchedule{{Start: new(spec), Location: loc}})
				Expect(err).NotTo(HaveOccurred())
				Expect(out[0].Next(from)).To(Equal(want))
			},
			Entry("daily 20:00 UTC from midnight → same day 20:00",
				"0 20 * * *", "",
				ref,
				time.Date(2006, 1, 2, 20, 0, 0, 0, time.UTC),
			),
			Entry("daily 20:00 Europe/Berlin (CET=UTC+1) from midnight UTC → 19:00 UTC",
				"0 20 * * *", "Europe/Berlin",
				ref,
				time.Date(2006, 1, 2, 19, 0, 0, 0, time.UTC),
			),
		)
	})

	Describe("#Previous", func() {
		DescribeTable("should return the correct last occurrence in (from, to]",
			func(from, to time.Time, want *time.Time) {
				s := mustDailyAt20()
				prev := s.Previous(from, to)
				if want == nil {
					Expect(prev).To(BeNil())
				} else {
					Expect(prev).NotTo(BeNil())
					Expect(*prev).To(Equal(*want))
				}
			},
			// from=Jan2, to=Jan4 → last fire Jan3 20:00
			Entry("last occurrence in two-day window",
				ref, ref.Add(2*24*time.Hour),
				func() *time.Time { t := time.Date(2006, 1, 3, 20, 0, 0, 0, time.UTC); return &t }(),
			),
			// from=Jan3 21:00, to=Jan4 → no fire
			Entry("no occurrence in range",
				time.Date(2006, 1, 3, 21, 0, 0, 0, time.UTC), ref.Add(2*24*time.Hour),
				nil,
			),
			// to = Jan3 20:00 exactly → included
			Entry("occurrence exactly at to is included",
				time.Date(2006, 1, 2, 20, 0, 0, 0, time.UTC),
				time.Date(2006, 1, 3, 20, 0, 0, 0, time.UTC),
				func() *time.Time { t := time.Date(2006, 1, 3, 20, 0, 0, 0, time.UTC); return &t }(),
			),
			// from = Jan3 20:00 exactly → excluded (interval is open on the left)
			Entry("occurrence exactly at from is excluded",
				time.Date(2006, 1, 3, 20, 0, 0, 0, time.UTC),
				ref.Add(2*24*time.Hour),
				nil,
			),
		)
	})
})
