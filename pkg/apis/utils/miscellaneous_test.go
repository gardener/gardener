// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"slices"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/apis/utils"
)

var _ = Describe("utils", func() {
	Describe("#TransformElements", func() {
		It("should transform a slice of integers to strings", func() {
			elements := []int{1, 2, 3, 4, 5}
			transform := func(i int) string {
				return strconv.Itoa(i)
			}

			result := slices.Collect(TransformElements(elements, transform))

			Expect(result).To(Equal([]string{"1", "2", "3", "4", "5"}))
		})

		It("should handle an empty slice", func() {
			var elements []int
			transform := func(i int) string {
				return strconv.Itoa(i)
			}

			result := slices.Collect(TransformElements(elements, transform))

			Expect(result).To(BeEmpty())
		})

		It("should transform a slice of structs to a specific field", func() {
			type person struct {
				name string
				age  int
			}

			elements := []person{
				{name: "Alice", age: 30},
				{name: "Bob", age: 25},
				{name: "Charlie", age: 35},
			}
			transform := func(p person) string {
				return p.name
			}

			result := slices.Collect(TransformElements(elements, transform))

			Expect(result).To(Equal([]string{"Alice", "Bob", "Charlie"}))
		})
	})

	Describe("#FilterElements", func() {
		It("should filter a slice of integers to only even numbers", func() {
			elements := []int{1, 2, 3, 4, 5, 6}
			match := func(i int) bool {
				return i%2 == 0
			}

			result := slices.Collect(FilterElements(elements, match))

			Expect(result).To(Equal([]int{2, 4, 6}))
		})

		It("should handle an empty slice", func() {
			var elements []int
			match := func(i int) bool {
				return i%2 == 0
			}

			result := slices.Collect(FilterElements(elements, match))

			Expect(result).To(BeEmpty())
		})

		It("should return empty when no elements match", func() {
			elements := []int{1, 3, 5, 7}
			match := func(i int) bool {
				return i%2 == 0
			}

			result := slices.Collect(FilterElements(elements, match))

			Expect(result).To(BeEmpty())
		})

		It("should return all elements when all match", func() {
			elements := []int{2, 4, 6, 8}
			match := func(i int) bool {
				return i%2 == 0
			}

			result := slices.Collect(FilterElements(elements, match))

			Expect(result).To(Equal([]int{2, 4, 6, 8}))
		})
	})
})
