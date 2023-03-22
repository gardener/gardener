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
