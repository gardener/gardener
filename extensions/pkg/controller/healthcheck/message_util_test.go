// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
		It("should return details when both counts are 0", func() {
			Expect(getUnsuccessfulDetailMessage(0, 0, details)).To(Equal("details"))
		})
		It("should use singular 'check' when progressing count is 1", func() {
			Expect(getUnsuccessfulDetailMessage(1, 1, details)).To(Equal("1 failing and 1 progressing check: details"))
		})
	})

	Describe("#getSingularOrPlural", func() {
		It("should return 'check' for count 1", func() {
			Expect(getSingularOrPlural(1)).To(Equal("check"))
		})

		It("should return 'checks' for count 2", func() {
			Expect(getSingularOrPlural(2)).To(Equal("checks"))
		})

		It("should return 'checks' for count 0", func() {
			Expect(getSingularOrPlural(0)).To(Equal("check"))
		})

		It("should return 'checks' for large count", func() {
			Expect(getSingularOrPlural(100)).To(Equal("checks"))
		})
	})

	Describe("#ensureTrailingDot", func() {
		It("should add a dot when there is no trailing dot", func() {
			Expect(ensureTrailingDot("hello")).To(Equal("hello."))
		})

		It("should not add a dot when there is already a trailing dot", func() {
			Expect(ensureTrailingDot("hello.")).To(Equal("hello."))
		})

		It("should add a dot to an empty string", func() {
			Expect(ensureTrailingDot("")).To(Equal("."))
		})

		It("should not modify a string that is just a dot", func() {
			Expect(ensureTrailingDot(".")).To(Equal("."))
		})

		It("should add a dot after a space", func() {
			Expect(ensureTrailingDot("some detail ")).To(Equal("some detail ."))
		})
	})

	Describe("#trimTrailingWhitespace", func() {
		It("should remove trailing space", func() {
			Expect(trimTrailingWhitespace("hello ")).To(Equal("hello"))
		})

		It("should not modify a string without trailing space", func() {
			Expect(trimTrailingWhitespace("hello")).To(Equal("hello"))
		})

		It("should handle an empty string", func() {
			Expect(trimTrailingWhitespace("")).To(Equal(""))
		})

		It("should only remove one trailing space", func() {
			Expect(trimTrailingWhitespace("hello  ")).To(Equal("hello "))
		})

		It("should not remove tabs", func() {
			Expect(trimTrailingWhitespace("hello\t")).To(Equal("hello\t"))
		})
	})

	DescribeTable("#append_ChecksDetails",
		func(input checkResultForConditionType, expected string) {
			var details strings.Builder
			Expect(input.appendFailedChecksDetails(&details)).To(Succeed())
			Expect(input.appendUnsuccessfulChecksDetails(&details)).To(Succeed())
			Expect(input.appendProgressingChecksDetails(&details)).To(Succeed())
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
		Entry("All three types: one failed, one unsuccessful, one progressing",
			checkResultForConditionType{
				failedChecks: []error{fmt.Errorf("could not reach API")},
				unsuccessfulChecks: []healthCheckUnsuccessful{
					{
						detail: "deployment unhealthy",
					},
				},
				progressingChecks: []healthCheckProgressing{
					{
						detail: "scaling up",
					},
				},
			},
			"Unable to execute check: could not reach API. Failed check: deployment unhealthy. Progressing check: scaling up."),
		Entry("Two failed checks only",
			checkResultForConditionType{
				failedChecks: []error{fmt.Errorf("error 1"), fmt.Errorf("error 2")},
			},
			"1) error 1. 2) error 2."),
		Entry("Two progressing checks only",
			checkResultForConditionType{
				progressingChecks: []healthCheckProgressing{
					{detail: "progressing 1"},
					{detail: "progressing 2"},
				},
			},
			"1) progressing 1. 2) progressing 2."),
		Entry("Two unsuccessful checks only",
			checkResultForConditionType{
				unsuccessfulChecks: []healthCheckUnsuccessful{
					{detail: "unhealthy 1"},
					{detail: "unhealthy 2"},
				},
			},
			"1) unhealthy 1. 2) unhealthy 2."),
		Entry("Detail with existing trailing dot should not double the dot",
			checkResultForConditionType{
				unsuccessfulChecks: []healthCheckUnsuccessful{
					{detail: "already has dot."},
				},
			},
			"already has dot."),
	)

})
