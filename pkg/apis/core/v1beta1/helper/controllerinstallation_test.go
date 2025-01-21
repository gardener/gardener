// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

var _ = Describe("Helper", func() {
	DescribeTable("#IsControllerInstallationSuccessful",
		func(conditions []gardencorev1beta1.Condition, expectation bool) {
			controllerInstallation := gardencorev1beta1.ControllerInstallation{
				Status: gardencorev1beta1.ControllerInstallationStatus{
					Conditions: conditions,
				},
			}
			Expect(IsControllerInstallationSuccessful(controllerInstallation)).To(Equal(expectation))
		},
		Entry("expect true",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationInstalled,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationProgressing,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			true,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationInstalled,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationHealthy,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationProgressing,
					Status: gardencorev1beta1.ConditionTrue,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationInstalled,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationHealthy,
					Status: gardencorev1beta1.ConditionFalse,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationProgressing,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationInstalled,
					Status: gardencorev1beta1.ConditionFalse,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationProgressing,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationInstalled,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationProgressing,
					Status: gardencorev1beta1.ConditionTrue,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{},
			false,
		),
	)

	DescribeTable("#IsControllerInstallationRequired",
		func(conditions []gardencorev1beta1.Condition, expectation bool) {
			controllerInstallation := gardencorev1beta1.ControllerInstallation{
				Status: gardencorev1beta1.ControllerInstallationStatus{
					Conditions: conditions,
				},
			}
			Expect(IsControllerInstallationRequired(controllerInstallation)).To(Equal(expectation))
		},
		Entry("expect true",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationRequired,
					Status: gardencorev1beta1.ConditionTrue,
				},
			},
			true,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationRequired,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{},
			false,
		),
	)

})
