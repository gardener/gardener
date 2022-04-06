// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_test

import (
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/robfig/cron"
)

// MustParseStandard parses the standardSpec and errors otherwise.
func MustParseStandard(standardSpec string) cron.Schedule {
	sched, err := cron.ParseStandard(standardSpec)
	Expect(err).NotTo(HaveOccurred())
	return sched
}

var _ = Describe("Shoot Hibernation", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("HibernationSchedule", func() {
		Describe("#GroupHibernationSchedulesByLocation", func() {
			It("should group the hibernation schedules with the same location together", func() {
				var (
					locationEuropeBerlin = "Europe/Berlin"
					locationUSCentral    = "US/Central"

					s1 = gardencorev1beta1.HibernationSchedule{Location: &locationEuropeBerlin}
					s2 = gardencorev1beta1.HibernationSchedule{Location: &locationEuropeBerlin}
					s3 = gardencorev1beta1.HibernationSchedule{Location: &locationUSCentral}
					s4 = gardencorev1beta1.HibernationSchedule{}
				)

				grouped := GroupHibernationSchedulesByLocation([]gardencorev1beta1.HibernationSchedule{s1, s2, s3, s4})
				Expect(grouped).To(Equal(map[string][]gardencorev1beta1.HibernationSchedule{
					locationEuropeBerlin: {s1, s2},
					locationUSCentral:    {s3},
					time.UTC.String():    {s4},
				}))
			})
		})
	})
})
