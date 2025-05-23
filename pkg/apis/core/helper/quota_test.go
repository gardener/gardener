// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/apis/core/helper"
)

var _ = Describe("Helper", func() {
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
})
