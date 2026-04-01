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
		Entry("empty name does not match domain", "", "example.com", false),
		Entry("name does not match empty domain", "example.com", "", false),
		Entry("both empty match", "", "", true),
		Entry("deeply nested subdomain matches", "a.b.c.d.e.example.com", "example.com", true),
		Entry("trailing dot mismatch", "test.example.com.", "example.com", false),
		Entry("single label name does not match different single label domain", "com", "net", false),
		Entry("single label name matches same single label domain", "com", "com", true),
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

		Entry("deeply nested name matches longest zone", zones, "api.test.example.com", "2"),
		Entry("exact zone domain match uses that zone", zones, "test.example.com", "2"),
		Entry("subdomain of shorter zone matches", zones, "foo.example.com", "1"),
		Entry("no matching zone returns empty string", zones, "bar.com", ""),
		Entry("prefix substring of zone still matches by subdomain", zones, "apitest.example.com", "1"),
		Entry("non-subdomain suffix returns empty string", zones, "testexample.com", ""),
		Entry("empty zones map returns empty string", map[string]string{}, "api.example.com", ""),
		Entry("nil zones map returns empty string", nil, "api.example.com", ""),
		Entry("exact match on shorter zone domain", zones, "example.com", "1"),
		Entry("exact match on separate zone", zones, "foo.com", "3"),
		Entry("subdomain of separate zone matches", zones, "bar.foo.com", "3"),
	)

	DescribeTable("#GetMetaRecordName",
		func(name, expected string) {
			Expect(GetMetaRecordName(name)).To(Equal(expected))
		},

		Entry("regular name", "api.test.example.com", "comment-api.test.example.com"),
		Entry("wildcard name", "*.test.example.com", "*.comment-test.example.com"),
		Entry("empty name", "", "comment-"),
		Entry("wildcard only", "*.", "*.comment-"),
		Entry("single label name", "example", "comment-example"),
		Entry("name starting with dot", ".example.com", "comment-.example.com"),
		Entry("wildcard with subdomain", "*.sub.test.example.com", "*.comment-sub.test.example.com"),
	)
})
