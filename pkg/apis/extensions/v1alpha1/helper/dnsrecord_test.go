// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
)

var _ = Describe("helper", func() {
	DescribeTable("#GetDNSRecordType",
		func(address string, expected extensionsv1alpha1.DNSRecordType) {
			Expect(GetDNSRecordType(address)).To(Equal(expected))
		},

		Entry("valid IPv4 address", "1.2.3.4", extensionsv1alpha1.DNSRecordTypeA),
		Entry("valid IPv6 address", "2001:db8:f00::1", extensionsv1alpha1.DNSRecordTypeAAAA),
		Entry("anything else", "foo", extensionsv1alpha1.DNSRecordTypeCNAME),
	)

	DescribeTable("#GetDNSRecordTTL",
		func(ttl *int64, expected int64) {
			Expect(GetDNSRecordTTL(ttl)).To(Equal(expected))
		},

		Entry("nil value", nil, int64(120)),
		Entry("non-nil value", ptr.To[int64](300), int64(300)),
	)
})
