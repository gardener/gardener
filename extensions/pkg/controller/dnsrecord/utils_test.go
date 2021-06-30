// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Utils", func() {
	DescribeTable("#MatchesDomain",
		func(hostname, domain string, expected bool) {
			Expect(MatchesDomain(hostname, domain)).To(Equal(expected))
		},

		Entry("sub-sub-domain matches domain", "api.test.example.com", "example.com", true),
		Entry("sub-domain matches domain", "test.example.com", "example.com", true),
		Entry("domain matches domain", "example.com", "example.com", true),
		Entry("different domain doesn't match domain (even though it's a suffix)", "testexample.com", "example.com", false),
	)
})
