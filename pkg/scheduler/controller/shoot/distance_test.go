// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler_Control", func() {
	Context("orientation", func() {
		It("handles name without orientation", func() {
			base, orient := orientation("europe")
			Expect(base).To(Equal("europe"))
			Expect(orient).To(Equal(""))
		})

		It("handles orientation prefix", func() {
			base, orient := orientation("west_europe")
			Expect(base).To(Equal(":_europe"))
			Expect(orient).To(Equal("west"))
		})

		It("handles orientation suffix", func() {
			base, orient := orientation("europe-west")
			Expect(base).To(Equal("europe-:"))
			Expect(orient).To(Equal("west"))
		})
		It("handles orientation in the middle", func() {
			base, orient := orientation("middle-west-europe")
			Expect(base).To(Equal("middle-:-europe"))
			Expect(orient).To(Equal("west"))
		})
	})

	Context("distance", func() {
		It("handles name without orientation", func() {
			dist, leven := distanceValues("europe", "asia")
			Expect(leven).To(Equal(6))
			Expect(dist).To(Equal(leven * 2))
		})

		It("handles name both with identical orientation", func() {
			dist, leven := distanceValues("europe-west", "asia-west")
			Expect(leven).To(Equal(6))
			Expect(dist).To(Equal(leven * 2))
		})

		It("handles name both with different orientation", func() {
			dist, leven := distanceValues("europe-west", "asia-east")
			Expect(leven).To(Equal(6))
			Expect(dist).To(Equal(leven*2 + 2))
		})
		It("handles name with different orientation", func() {
			dist, leven := distanceValues("europe-west", "asia-:")
			Expect(leven).To(Equal(6))
			Expect(dist).To(Equal(leven*2 + 1))
		})

		It("handles identical base with different orientation", func() {
			dist, leven := distanceValues("europe-west", "europe-east")
			Expect(leven).To(Equal(0))
			Expect(dist).To(Equal(2))
		})

		It("handles identical base with mixes orientation", func() {
			dist, leven := distanceValues("europe-west", "europe-:")
			Expect(leven).To(Equal(0))
			Expect(dist).To(Equal(1))
		})

		It("handles identical base with same orientation", func() {
			dist, leven := distanceValues("europe-west", "europe-west")
			Expect(leven).To(Equal(0))
			Expect(dist).To(Equal(0))
		})
	})
})
