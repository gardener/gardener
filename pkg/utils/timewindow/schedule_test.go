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
