// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

		Entry("TopologyAwareHints is supported in 1.27.4", "TopologyAwareHints", "1.27.4", true, true),                        // AddedInVersion: 1.21
		Entry("AggregatedDiscoveryEndpoint is not supported in 1.25.8", "AggregatedDiscoveryEndpoint", "1.25.8", false, true), // AddedInVersion: 1.26
		Entry("CSIMigrationOpenStack is not supported in 1.26.2", "CSIMigrationOpenStack", "1.26.2", false, true),             // RemovedInVersion: 1.25
		Entry("SuspendJob is supported in 1.25.9", "SuspendJob", "1.25.9", true, true),                                        // AddedInVersion: 1.24, RemovedInVersion: 1.26
		Entry("DaemonSetUpdateSurge is supported in 1.26.7", "DaemonSetUpdateSurge", "1.26.7", true, true),                    // RemovedInVersion: 1.27
		Entry("Foo is unknown in 1.25.8", "Foo", "1.25.8", false, false),                                                      // Unknown

		Entry("AnyVolumeDataSource is supported in 1.24.9", "AnyVolumeDataSource", "1.24.9", true, true),                     // AddedInVersion: 1.18
		Entry("SELinuxMountReadWriteOncePod is supported in 1.26.10", "SELinuxMountReadWriteOncePod", "1.26.10", true, true), // AddedInVersion: 1.25
		Entry("EphemeralContainers is not supported in 1.28.2", "EphemeralContainers", "1.28.2", false, true),                // RemovedInVersion: 1.27
		Entry("DownwardAPIHugePages is supported in 1.27.1", "DownwardAPIHugePages", "1.27.1", true, true),                   // AddedInVersion: 1.20, RemovedInVersion: 1.27
		Entry("CSRDuration is not supported in 1.27.4", "CSRDuration", "1.27.4", false, true),                                // RemovedInVersion: 1.26
		Entry("Foo is unknown in 1.27.0", "Foo", "1.27.0", false, false),                                                     // Unknown

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
			Entry("unsupported feature gate", map[string]bool{"WatchList": true}, "1.26.10", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("WatchList").String()),
				"Detail": Equal("not supported in Kubernetes version 1.26.10"),
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
