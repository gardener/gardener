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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/controller/shoot"
)

var _ = Describe("ShootMaintenanceControl", func() {
	Describe("#NowWithinTimeWindow", func() {
		var now = time.Date(0, time.January, 1, 0, 0, 0, 0, time.UTC)

		It("should return an error due to invalid formats", func() {
			invalidFormat := "98123723921023"

			_, err := NowWithinTimeWindow(invalidFormat, invalidFormat, now)

			Expect(err).To(HaveOccurred())
		})

		Context("begin and end on the same day", func() {
			const (
				begin = "160000+0000"
				end   = "190000+0000"
			)

			It("should return false", func() {
				now := time.Date(0, time.January, 1, 15, 0, 0, 0, time.UTC)

				res, err := NowWithinTimeWindow(begin, end, now)

				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(BeFalse())
			})

			It("should return true", func() {
				now := time.Date(0, time.January, 1, 17, 0, 0, 0, time.UTC)

				res, err := NowWithinTimeWindow(begin, end, now)

				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(BeTrue())
			})
		})

		Context("begin and end on different days", func() {
			const (
				begin = "230000+0000"
				end   = "010000+0000"
			)

			It("should return false", func() {
				now := time.Date(0, time.January, 1, 22, 0, 0, 0, time.UTC)

				res, err := NowWithinTimeWindow(begin, end, now)

				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(BeFalse())
			})

			It("should return true", func() {
				now := time.Date(0, time.January, 1, 0, 59, 0, 0, time.UTC)

				res, err := NowWithinTimeWindow(begin, end, now)

				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(BeTrue())
			})
		})
	})
})
