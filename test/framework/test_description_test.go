// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/test/framework"
)

var _ = Describe("Test Descriptions tests", func() {

	DescribeTable("define test labels",
		func(td framework.TestDescription, expected string) {
			Expect(td.String()).To(Equal(expected))
		},
		Entry("beta default - beta default", framework.TestDescription{}.Beta().Default(), "[BETA] [DEFAULT]"),
		Entry("serial beta - beta serial", framework.TestDescription{}.Serial().Beta(), "[BETA] [SERIAL]"),
		Entry("serial beta release - beta release serial", framework.TestDescription{}.Serial().Beta().Release(), "[BETA] [RELEASE] [SERIAL]"),
		Entry("serial beta beta - beta serial", framework.TestDescription{}.Serial().Beta().Beta(), "[BETA] [SERIAL]"),
	)

})
