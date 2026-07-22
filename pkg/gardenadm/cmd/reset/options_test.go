// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reset_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/reset"
)

var _ = Describe("Options", func() {
	var (
		options *Options
	)

	BeforeEach(func() {
		options = &Options{}
	})

	Describe("#ParseArgs", func() {
		It("should do nothing when no argument is set", func() {
			Expect(options.ParseArgs(nil)).To(Succeed())
			Expect(options.ControlPlaneAddress).To(BeEmpty())
		})

		It("should trim spaces when the argument is set", func() {
			Expect(options.ParseArgs([]string{" foo.bar   "})).To(Succeed())
			Expect(options.ControlPlaneAddress).To(Equal("foo.bar"))
		})
	})

	Describe("#Validate", func() {
		It("should succeed when proper values were provided", func() {
			options.Token = "some-token"

			Expect(options.Validate()).To(Succeed())
		})

		It("should fail when no token is provided", func() {
			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a token")))
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
