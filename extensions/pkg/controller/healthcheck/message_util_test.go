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

package healthcheck

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("message_util", func() {
	var details = "details"
	Describe("#getUnsuccessfulDetailMessage", func() {
		It("should return message when progressing checks > 0 && unsuccessful checks > 0", func() {
			Expect(getUnsuccessfulDetailMessage(2, 2, details)).To(Equal("2 failing and 2 progressing checks: details"))
		})
		It("should return message when unsuccessful checks > 1", func() {
			Expect(getUnsuccessfulDetailMessage(2, 0, details)).To(Equal("details"))
		})
		It("should return message when unsuccessful checks == 1", func() {
			Expect(getUnsuccessfulDetailMessage(1, 0, details)).To(Equal("details"))
		})
		It("should return message when progressing checks > 1", func() {
			Expect(getUnsuccessfulDetailMessage(0, 2, details)).To(Equal("details"))
		})
		It("should return message when progressing checks == 1", func() {
			Expect(getUnsuccessfulDetailMessage(0, 1, details)).To(Equal("details"))
		})
	})

	DescribeTable("#append_ChecksDetails",
		func(input checkResultForConditionType, expected string) {
			var details strings.Builder
			input.appendFailedChecksDetails(&details)
			input.appendUnsuccessfulChecksDetails(&details)
			input.appendProgressingChecksDetails(&details)
			Expect(trimTrailingWhitespace(details.String())).To(Equal(expected))
		},
		Entry("no unsuccessful checks", checkResultForConditionType{}, ""),
		Entry("Only one unsuccessful check",
			checkResultForConditionType{
				unsuccessfulChecks: []healthCheckUnsuccessful{
					{
						detail: "MyBad",
					}},
			},
			"MyBad."),
		Entry("Only one failed check",
			checkResultForConditionType{
				failedChecks: []error{fmt.Errorf("fail")},
			},
			"fail."),
		Entry("Only one progressing check",
			checkResultForConditionType{
				progressingChecks: []healthCheckProgressing{
					{
						detail: "fail again",
					},
				},
			},
			"fail again."),
		Entry("One unsuccessful check and one progressing check",
			checkResultForConditionType{
				unsuccessfulChecks: []healthCheckUnsuccessful{
					{
						detail: "MyBad",
					}},
				progressingChecks: []healthCheckProgressing{
					{
						detail: "xyz",
					},
				},
			},
			"Failed check: MyBad. Progressing check: xyz."),
		Entry("Two unsuccessful check and two progressing check",
			checkResultForConditionType{
				unsuccessfulChecks: []healthCheckUnsuccessful{
					{
						detail: "MyBad",
					},
					{
						detail: "MyBad2",
					},
				},
				progressingChecks: []healthCheckProgressing{
					{
						detail: "xyz",
					},
					{
						detail: "xtc",
					},
				},
			},
			"Failed checks: 1) MyBad. 2) MyBad2. Progressing checks: 1) xyz. 2) xtc."),
		Entry("One unsuccessful check and two progressing checks",
			checkResultForConditionType{
				unsuccessfulChecks: []healthCheckUnsuccessful{
					{
						detail: "MyBad",
					}},
				progressingChecks: []healthCheckProgressing{
					{
						detail: "xyz",
					},
					{
						detail: "abc",
					},
				},
			},
			"Failed check: MyBad. Progressing checks: 1) xyz. 2) abc."),
		Entry("One unsuccessful check and one failed check",
			checkResultForConditionType{
				unsuccessfulChecks: []healthCheckUnsuccessful{
					{
						detail: "MyBad",
					}},
				failedChecks: []error{fmt.Errorf("super bad")},
			},
			"Unable to execute check: super bad. Failed check: MyBad."),
		Entry("One unsuccessful check and two failed checks",
			checkResultForConditionType{
				unsuccessfulChecks: []healthCheckUnsuccessful{
					{
						detail: "MyBad",
					}},
				failedChecks: []error{fmt.Errorf("super bad"), fmt.Errorf("super bad2")},
			},
			"Unable to execute checks: 1) super bad. 2) super bad2. Failed check: MyBad."),
	)

})
