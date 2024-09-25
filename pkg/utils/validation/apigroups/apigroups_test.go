// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apigroups_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/utils/validation/apigroups"
)

var _ = Describe("apigroups", func() {
	DescribeTable("#IsAPISupported",
		func(api, version string, supported, success bool) {
			result, supportedRange, err := IsAPISupported(api, version)
			if success {
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Equal(supported))
				Expect(supportedRange).NotTo(Equal(""))
			} else {
				Expect(err).To(HaveOccurred())
			}
		},
		Entry("Unknown API Group Version", "Unknown", "core/v2", false, false),
		Entry("Unknown API Group Version Resource", "Unknown", "core/v1/random", false, false),
		Entry("Known API Group Version but kubernetes version not present in supported range", "certificates.k8s.io/v1alpha1", "1.25", false, true),
		Entry("Known API Group Version Resource but kubernetes version not present in supported range", "resource.k8s.io/v1alpha1/podschedulings", "1.25", false, true),
		Entry("Known API Group Version and kubernetes version present in supported range", "resource.k8s.io/v1alpha1", "1.26", true, true),
		Entry("Known API Group Version Resource and kubernetes version present in supported range", "resource.k8s.io/v1alpha1/podschedulings", "1.26", true, true),
		Entry("Known API Group Version but kubernetes version range not present", "policy/v1", "1.25", true, true),
		Entry("Known API Group Version Resource but kubernetes version range not present", "policy/v1/poddisruptionbudgets", "1.25", true, true),
	)

	DescribeTable("#SplitAPI",
		func(api, expectedGV, expectedGVR string, matcher gomegatypes.GomegaMatcher) {
			gv, gvr, err := SplitAPI(api)
			Expect(gv).To(Equal(expectedGV))
			Expect(gvr).To(Equal(expectedGVR))
			Expect(err).To(matcher)
		},
		Entry("v1 group", "v1", "v1", "", BeNil()),
		Entry("v1 group and configmaps resource", "v1/configmaps", "v1", "v1/configmaps", BeNil()),
		Entry("apps/v1 group", "apps/v1", "apps/v1", "", BeNil()),
		Entry("apps/v1 group and deployments resource", "apps/v1/deployments", "apps/v1", "apps/v1/deployments", BeNil()),
		Entry("Invalid format", "apps/v1/foo/bar", "", "", MatchError("invalid API Group format \"apps/v1/foo/bar\"")),
	)

	Describe("#ValidateAPIGroupVersions", func() {
		DescribeTable("validate API ",
			func(runtimeConfig map[string]bool, version string, workerless bool, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateAPIGroupVersions(runtimeConfig, version, workerless, field.NewPath("runtimeConfig"))
				Expect(errList).To(matcher)
			},
			Entry("empty list", nil, "1.27.1", false, BeEmpty()),
			Entry("unknown API group version", map[string]bool{"Foo": true}, "1.26.8", false, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("runtimeConfig[Foo]"),
				"BadValue": Equal("Foo"),
				"Detail":   Equal("unknown API group version \"Foo\""),
			})))),
			Entry("supported API group version", map[string]bool{"v1": true}, "1.27.1", false, BeEmpty()),
			Entry("unsupported API group version", map[string]bool{"certificates.k8s.io/v1alpha1": true}, "1.25.10", false, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("runtimeConfig[certificates.k8s.io/v1alpha1]"),
				"Detail": Equal("api \"certificates.k8s.io/v1alpha1\" is not supported in Kubernetes version 1.25.10, only supported in versions >= 1.27"),
			})))),
			Entry("unsupported API group version", map[string]bool{"resource.k8s.io/v1alpha1": true}, "1.27.4", false, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("runtimeConfig[resource.k8s.io/v1alpha1]"),
				"Detail": Equal("api \"resource.k8s.io/v1alpha1\" is not supported in Kubernetes version 1.27.4, only supported in versions >= 1.26, < 1.27"),
			})))),
			Entry("unsupported API group version", map[string]bool{"resource.k8s.io/v1alpha2": false}, "1.26.8", false, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("runtimeConfig[resource.k8s.io/v1alpha2]"),
				"Detail": Equal("api \"resource.k8s.io/v1alpha2\" is not supported in Kubernetes version 1.26.8, only supported in versions >= 1.27, < 1.31"),
			})))),
			Entry("disabling non-required API group version", map[string]bool{"batch/v1": false}, "1.26.8", false, BeEmpty()),
			Entry("disabling non-required API group version for workerless shoot", map[string]bool{"apps/v1": false}, "1.26.8", true, BeEmpty()),
			Entry("disabling particular API in a non-required API group version", map[string]bool{"batch/v1/jobs": false}, "1.26.8", false, BeEmpty()),
			Entry("disabling particular API in a non-required API group version for workerless shoot", map[string]bool{"storage.k8s.io/v1/csidrivers": false}, "1.26.8", true, BeEmpty()),
			Entry("disabling required API group version", map[string]bool{"v1": false}, "1.26.8", false, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("runtimeConfig[v1]"),
				"Detail": Equal("api \"v1\" cannot be disabled"),
			})))),
			Entry("disabling required API group version for workerless shoot", map[string]bool{"v1": false}, "1.26.8", true, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("runtimeConfig[v1]"),
				"Detail": Equal("api \"v1\" cannot be disabled for workerless clusters"),
			})))),
			Entry("disabling particular API in a required API group version", map[string]bool{"apps/v1/deployments": false}, "1.26.8", false, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("runtimeConfig[apps/v1/deployments]"),
				"Detail": Equal("api \"apps/v1\" cannot be disabled"),
			})))),
			Entry("disabling particular API in a required API group version for workerless clusters", map[string]bool{"v1/services": false}, "1.26.8", true, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("runtimeConfig[v1/services]"),
				"Detail": Equal("api \"v1\" cannot be disabled for workerless clusters"),
			})))),
			Entry("disabling non-required API group version as a whole when a resource in the group is required", map[string]bool{"scheduling.k8s.io/v1": false}, "1.26.8", false, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("runtimeConfig[scheduling.k8s.io/v1]"),
				"Detail": Equal("api \"scheduling.k8s.io/v1/priorityclasses\" cannot be disabled"),
			})))),
		)
	})
})
