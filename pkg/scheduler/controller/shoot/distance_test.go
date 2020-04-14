package shoot

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler_Control", func() {
	Context("orientation", func() {
		It("handles name without orientation", func() {
			n, o := orientation("europe")
			Expect(n).To(Equal("europe"))
			Expect(o).To(Equal(""))
		})

		It("handles orientation prefix", func() {
			n, o := orientation("west_europe")
			Expect(n).To(Equal(":_europe"))
			Expect(o).To(Equal("west"))
		})

		It("handles orientation suffix", func() {
			n, o := orientation("europe-west")
			Expect(n).To(Equal("europe-:"))
			Expect(o).To(Equal("west"))
		})
		It("handles orientation in the middle", func() {
			n, o := orientation("middle-west-europe")
			Expect(n).To(Equal("middle-:-europe"))
			Expect(o).To(Equal("west"))
		})
	})

	Context("orientation", func() {
		It("handles name without orientation", func() {
			d, l := distance("europe", "asia")
			Expect(l).To(Equal(6))
			Expect(d).To(Equal(l * 2))
		})

		It("handles name both with identical orientation", func() {
			d, l := distance("europe-west", "asia-west")
			Expect(l).To(Equal(6))
			Expect(d).To(Equal(l * 2))
		})

		It("handles name both with different orientation", func() {
			d, l := distance("europe-west", "asia-east")
			Expect(l).To(Equal(6))
			Expect(d).To(Equal(l*2 + 2))
		})
		It("handles name with different orientation", func() {
			d, l := distance("europe-west", "asia-:")
			Expect(l).To(Equal(6))
			Expect(d).To(Equal(l*2 + 1))
		})

		It("handles identical base with diffent orientation", func() {
			d, l := distance("europe-west", "europe-east")
			Expect(l).To(Equal(0))
			Expect(d).To(Equal(2))
		})

		It("handles identical base with mixes orientation", func() {
			d, l := distance("europe-west", "europe-:")
			Expect(l).To(Equal(0))
			Expect(d).To(Equal(1))
		})

		It("handles identical base with same orientation", func() {
			d, l := distance("europe-west", "europe-west")
			Expect(l).To(Equal(0))
			Expect(d).To(Equal(0))
		})
	})
})
