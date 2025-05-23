// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
)

var _ = Describe("Utils", func() {
	DescribeTable("#MatchesDomain",
		func(name, domain string, expected bool) {
			Expect(MatchesDomain(name, domain)).To(Equal(expected))
		},

		Entry("sub-sub-domain matches domain", "api.test.example.com", "example.com", true),
		Entry("sub-domain matches domain", "test.example.com", "example.com", true),
		Entry("domain matches domain", "example.com", "example.com", true),
		Entry("different domain doesn't match domain (even though it's a suffix)", "testexample.com", "example.com", false),
	)

	var zones = map[string]string{
		"example.com":      "1",
		"test.example.com": "2",
		"foo.com":          "3",
	}

	DescribeTable("#FindZoneForName",
		func(zones map[string]string, name, expected string) {
			Expect(FindZoneForName(zones, name)).To(Equal(expected))
		},

		Entry("", zones, "api.test.example.com", "2"),
		Entry("", zones, "test.example.com", "2"),
		Entry("", zones, "foo.example.com", "1"),
		Entry("", zones, "bar.com", ""),
		Entry("", zones, "apitest.example.com", "1"),
		Entry("", zones, "testexample.com", ""),
	)

	DescribeTable("#GetMetaRecordName",
		func(name, expected string) {
			Expect(GetMetaRecordName(name)).To(Equal(expected))
		},

		Entry("regular name", "api.test.example.com", "comment-api.test.example.com"),
		Entry("wildcard name", "*.test.example.com", "*.comment-test.example.com"),
		Entry("empty name", "", "comment-"),
	)
})
