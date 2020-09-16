// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package utils_test

import (
	. "github.com/gardener/gardener/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/types"
	"k8s.io/utils/pointer"
)

var _ = Describe("utils", func() {
	Describe("#MergeStringMaps", func() {
		It("should return nil", func() {
			result := MergeStringMaps(nil, nil)

			Expect(result).To(BeNil())
		})

		It("should return an empty map", func() {
			emptyMap := map[string]string{}

			result := MergeStringMaps(emptyMap, nil)

			Expect(result).To(Equal(emptyMap))
		})

		It("should return a merged map", func() {
			var (
				oldMap = map[string]string{
					"a": "1",
					"b": "2",
				}
				newMap = map[string]string{
					"b": "20",
					"c": "3",
				}
			)

			result := MergeStringMaps(oldMap, newMap)

			Expect(result).To(Equal(map[string]string{
				"a": "1",
				"b": "20",
				"c": "3",
			}))
		})
	})

	DescribeTable("#IsTrue",
		func(value *bool, matcher GomegaMatcher) {
			Expect(IsTrue(value)).To(matcher)
		},
		Entry("nil", nil, BeFalse()),
		Entry("false", pointer.BoolPtr(false), BeFalse()),
		Entry("true", pointer.BoolPtr(true), BeTrue()),
	)

	DescribeTable("#IDForKeyWithOptionalValue",
		func(key string, value *string, expectation string) {
			Expect(IDForKeyWithOptionalValue(key, value)).To(Equal(expectation))
		},
		Entry("only key", "foo", nil, "foo"),
		Entry("key and value", "foo", pointer.StringPtr("bar"), "foo=bar"),
	)
})
