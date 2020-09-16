// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubescheduler_test

import (
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubescheduler"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logging", func() {
	Describe("#LoggingConfiguration", func() {
		It("should return the expected logging parser and filter", func() {
			parser, filter, err := LoggingConfiguration()

			Expect(err).NotTo(HaveOccurred())
			Expect(parser).To(Equal(`[PARSER]
    Name        kubeSchedulerParser
    Format      regex
    Regex       ^(?<severity>\w)(?<time>\d{4} [^\s]*)\s+(?<pid>\d+)\s+(?<source>[^ \]]+)\] (?<log>.*)$
    Time_Key    time
    Time_Format %m%d %H:%M:%S.%L
`))

			Expect(filter).To(Equal(`[FILTER]
    Name                parser
    Match               kubernetes.*kube-scheduler*kube-scheduler*
    Key_Name            log
    Parser              kubeSchedulerParser
    Reserve_Data        True
`))
		})
	})
})
