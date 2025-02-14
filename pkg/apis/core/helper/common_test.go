// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/helper"
)

var _ = Describe("Helper", func() {
	Describe("#GetCondition", func() {
		It("should return the found condition", func() {
			var (
				conditionType core.ConditionType = "test-1"
				condition                        = core.Condition{Type: conditionType}
				conditions                       = []core.Condition{condition}
			)

			cond := GetCondition(conditions, conditionType)

			Expect(cond).NotTo(BeNil())
			Expect(*cond).To(Equal(condition))
		})

		It("should return nil because the required condition could not be found", func() {
			var (
				conditionType core.ConditionType = "test-1"
				conditions    []core.Condition
			)

			cond := GetCondition(conditions, conditionType)

			Expect(cond).To(BeNil())
		})
	})

	DescribeTable("#QuotaScope",
		func(apiVersion, kind, expectedScope string, expectedErr gomegatypes.GomegaMatcher) {
			scope, err := QuotaScope(corev1.ObjectReference{APIVersion: apiVersion, Kind: kind})
			Expect(scope).To(Equal(expectedScope))
			Expect(err).To(expectedErr)
		},

		Entry("project", "core.gardener.cloud/v1beta1", "Project", "project", BeNil()),
		Entry("secret", "v1", "Secret", "credentials", BeNil()),
		Entry("workloadidentity", "security.gardener.cloud/v1alpha1", "WorkloadIdentity", "credentials", BeNil()),
		Entry("unknown", "v2", "Foo", "", HaveOccurred()),
	)

	Describe("#DeterminePrimaryIPFamily", func() {
		It("should return IPv4 for empty ipFamilies", func() {
			Expect(DeterminePrimaryIPFamily(nil)).To(Equal(core.IPFamilyIPv4))
			Expect(DeterminePrimaryIPFamily([]core.IPFamily{})).To(Equal(core.IPFamilyIPv4))
		})

		It("should return IPv4 if it's the first entry", func() {
			Expect(DeterminePrimaryIPFamily([]core.IPFamily{core.IPFamilyIPv4})).To(Equal(core.IPFamilyIPv4))
			Expect(DeterminePrimaryIPFamily([]core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6})).To(Equal(core.IPFamilyIPv4))
		})

		It("should return IPv6 if it's the first entry", func() {
			Expect(DeterminePrimaryIPFamily([]core.IPFamily{core.IPFamilyIPv6})).To(Equal(core.IPFamilyIPv6))
			Expect(DeterminePrimaryIPFamily([]core.IPFamily{core.IPFamilyIPv6, core.IPFamilyIPv4})).To(Equal(core.IPFamilyIPv6))
		})
	})
})
