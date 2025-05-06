// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token/create"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
)

var _ = Describe("Options", func() {
	var (
		createOptions *tokenutils.Options
		options       *Options

		token = "some.token"
	)

	BeforeEach(func() {
		createOptions = &tokenutils.Options{
			Validity: time.Hour,
		}
		options = &Options{CreateOptions: createOptions}
	})

	Describe("#ParseArgs", func() {
		It("should use the first argument as token", func() {
			Expect(options.ParseArgs([]string{token})).To(Succeed())
			Expect(options.Token.Combined).To(Equal(token))
		})

		It("should generate a random token", func() {
			Expect(options.ParseArgs(nil)).To(Succeed())
			Expect(options.Token.Combined).To(MatchRegexp(`^[a-z0-9]{6}.[a-z0-9]{16}$`))
		})
	})

	Describe("#Validate", func() {
		It("should pass for valid options", func() {
			options.Token.Combined = "abcdef.abcdef1234567890"
			Expect(options.Validate()).To(Succeed())
		})

		It("should fail because token does not match the expected format", func() {
			options.Token.Combined = "invalid-format"
			Expect(options.Validate()).To(MatchError(ContainSubstring("does not match the expected format")))
		})

		It("should fail because token is not set", func() {
			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a token to create")))
		})
	})

	Describe("#Complete", func() {
		It("should fail if the token has an unexpected format", func() {
			options.Token.Combined = "foo.bar.bar"

			Expect(options.Complete()).To(MatchError(ContainSubstring("token must be of the form")))
		})

		It("should succeed splitting the token", func() {
			options.Token.Combined = "foo.bar"

			Expect(options.Complete()).To(Succeed())
			Expect(options.Token.ID).To(Equal("foo"))
			Expect(options.Token.Secret).To(Equal("bar"))
		})
	})
})
