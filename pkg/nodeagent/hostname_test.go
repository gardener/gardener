// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/nodeagent"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("HostName", func() {
	Describe("#GetHostName", func() {
		It("should convert the string to lower case", func() {
			DeferCleanup(test.WithVar(&Hostname, func() (string, error) {
				return "FooObAr", nil
			}))

			hostName, err := GetHostName()
			Expect(err).NotTo(HaveOccurred())

			Expect(hostName).To(Equal("fooobar"))
		})
	})
})
