// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helper", func() {
	DescribeTable("#IsResourceSupported",
		func(resources []gardencorev1beta1.ControllerResource, resourceKind, resourceType string, expectation bool) {
			Expect(IsResourceSupported(resources, resourceKind, resourceType)).To(Equal(expectation))
		},
		Entry("expect true",
			[]gardencorev1beta1.ControllerResource{
				{
					Kind: "foo",
					Type: "bar",
				},
			},
			"foo",
			"bar",
			true,
		),
		Entry("expect true",
			[]gardencorev1beta1.ControllerResource{
				{
					Kind: "foo",
					Type: "bar",
				},
			},
			"foo",
			"BAR",
			true,
		),
		Entry("expect false",
			[]gardencorev1beta1.ControllerResource{
				{
					Kind: "foo",
					Type: "bar",
				},
			},
			"foo",
			"baz",
			false,
		),
	)
})
