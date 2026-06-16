// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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

	DescribeTable("#ContinuousEndpointUpdateEnabled",
		func(registrations []gardencorev1beta1.ControllerRegistration, extensionType string, expected bool) {
			Expect(ContinuousEndpointUpdateEnabled(registrations, extensionType)).To(Equal(expected))
		},
		Entry("no registrations defaults to true (not-yet-synced must not silently disable)",
			nil,
			"local",
			true,
		),
		Entry("no matching registration defaults to true",
			[]gardencorev1beta1.ControllerRegistration{{
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "foo", Type: "bar"},
					},
				},
			}},
			"local",
			true,
		),
		Entry("matching registration with field unset defaults to true",
			[]gardencorev1beta1.ControllerRegistration{{
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "SelfHostedShootExposure", Type: "local"},
					},
				},
			}},
			"local",
			true,
		),
		Entry("matching registration with field explicitly set to false",
			[]gardencorev1beta1.ControllerRegistration{{
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "SelfHostedShootExposure", Type: "local", ContinuousEndpointUpdate: new(false)},
					},
				},
			}},
			"local",
			false,
		),
		Entry("type lookup is case-insensitive",
			[]gardencorev1beta1.ControllerRegistration{{
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "SelfHostedShootExposure", Type: "Local", ContinuousEndpointUpdate: new(false)},
					},
				},
			}},
			"local",
			false,
		),
	)
})
