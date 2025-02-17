// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token/create"
)

var _ = Describe("Options", func() {
	var (
		options *Options

		token = "some.token"
	)

	BeforeEach(func() {
		options = &Options{}
	})

	Describe("#ParseArgs", func() {
		It("should use the first argument as token", func() {
			Expect(options.ParseArgs([]string{token})).To(Succeed())
			Expect(options.Token).To(Equal(token))
		})

		It("should generate a random token", func() {
			Expect(options.ParseArgs(nil)).To(Succeed())
			Expect(options.Token).To(Equal("foo123.bar4567890baz123"))
		})
	})

	Describe("#Validate", func() {
		It("should pass for valid options", func() {
			options.Token = "foo"

			Expect(options.Validate()).To(Succeed())
		})

		It("should fail because token is not set", func() {
			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a token to create")))
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
