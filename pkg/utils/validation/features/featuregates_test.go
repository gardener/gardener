// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/utils/validation/features"
)

var _ = Describe("featuregates", func() {
	DescribeTable("#IsFeatureGateSupported",
		func(featureGate, version string, supported, success bool) {
			result, err := IsFeatureGateSupported(featureGate, version)
			if success {
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Equal(supported))
			} else {
				Expect(err).To(HaveOccurred())
			}
		},

		Entry("TopologyAwareHints is supported in 1.27.4", "TopologyAwareHints", "1.27.4", true, true),                   // AddedInVersion: 1.21
		Entry("StorageVersionMigrator is not supported in 1.29.8", "StorageVersionMigrator", "1.29.8", false, true),      // AddedInVersion: 1.30
		Entry("CSIMigrationAzureFile is not supported in 1.31.4", "CSIMigrationAzureFile", "1.31.4", false, true),        // RemovedInVersion: 1.30
		Entry("AllowServiceLBStatusOnNonLB is supported in 1.30.3", "AllowServiceLBStatusOnNonLB", "1.30.3", true, true), // AddedInVersion: 1.29
		Entry("SecurityContextDeny is supported in 1.29.3", "SecurityContextDeny", "1.29.3", true, true),                 // RemovedInVersion: 1.30
		Entry("Foo is unknown in 1.25.8", "Foo", "1.25.8", false, false),                                                 // Unknown

		Entry("AnyVolumeDataSource is supported in 1.24.9", "AnyVolumeDataSource", "1.24.9", true, true),                                    // AddedInVersion: 1.18
		Entry("SELinuxMountReadWriteOncePod is supported in 1.29.1", "SELinuxMountReadWriteOncePod", "1.29.1", true, true),                  // AddedInVersion: 1.25
		Entry("PodHasNetworkCondition is not supported in 1.29.2", "PodHasNetworkCondition", "1.29.2", false, true),                         // RemovedInVersion: 1.28
		Entry("CSIMigrationRBD is supported in 1.27.1", "CSIMigrationRBD", "1.27.1", true, true),                                            // RemovedInVersion: 1.31
		Entry("UserNamespacesStatelessPodsSupport is not supported in 1.29.4", "UserNamespacesStatelessPodsSupport", "1.29.4", false, true), // RemovedInVersion: 1.28
		Entry("Foo is unknown in 1.27.0", "Foo", "1.27.0", false, false),                                                                    // Unknown

		Entry("AllAlpha is supported in 1.17.0", "AllAlpha", "1.17.0", true, true),        // AddedInVersion: 1.17
		Entry("AllAlpha is not supported in 1.16.15", "AllAlpha", "1.16.15", false, true), // AddedInVersion: 1.17
	)

	Describe("#ValidateFeatureGates", func() {
		DescribeTable("validate feature gates",
			func(featureGates map[string]bool, version string, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateFeatureGates(featureGates, version, nil)
				Expect(errList).To(matcher)
			},

			Entry("empty list", nil, "1.18.14", BeEmpty()),
			Entry("supported feature gate", map[string]bool{"AnyVolumeDataSource": true}, "1.18.14", BeEmpty()),
			Entry("unsupported feature gate", map[string]bool{"WatchListClient": true}, "1.29.10", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("WatchListClient").String()),
				"Detail": Equal("not supported in Kubernetes version 1.29.10"),
			})))),
			Entry("unknown feature gate", map[string]bool{"Foo": true}, "1.25.10", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal(field.NewPath("Foo").String()),
				"BadValue": Equal("Foo"),
				"Detail":   Equal("unknown feature gate Foo"),
			})))),
			Entry("setting non-default value for locked feature gate", map[string]bool{"CPUManager": false}, "1.27.5", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("CPUManager").String()),
				"Detail": Equal("cannot set feature gate to false, feature is locked to true"),
			})))),
		)
	})
})
