// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create_test

import (
	"time"

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
		options = &Options{Validity: time.Hour}
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
			options.Validity = time.Hour
			options.PrintJoinCommand = true
			options.WorkerPoolName = "worker-pool"
			Expect(options.Validate()).To(Succeed())
		})

		It("should fail because token does not match the expected format", func() {
			options.Token.Combined = "invalid-format"
			Expect(options.Validate()).To(MatchError(ContainSubstring("does not match the expected format")))
		})

		It("should fail because token is not set", func() {
			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a token to create")))
		})

		It("should return an error when the validity is less than 10m", func() {
			options.Token.Combined = "abcdef.abcdef1234567890"
			options.Validity = time.Minute
			Expect(options.Validate()).To(MatchError(ContainSubstring("minimum validity duration is 10m0s")))
		})

		It("should return an error when the validity is longer than 24h", func() {
			options.Token.Combined = "abcdef.abcdef1234567890"
			options.Validity = 25 * time.Hour
			Expect(options.Validate()).To(MatchError(ContainSubstring("maximum validity duration is 24h0m0s")))
		})

		When("the print-join-command flag is set", func() {
			BeforeEach(func() {
				options.PrintJoinCommand = true
				options.Token.Combined = "abcdef.abcdef1234567890"
			})

			It("should an error when no worker pool name is provided", func() {
				options.Validity = time.Hour
				Expect(options.Validate()).To(MatchError(ContainSubstring("must specify a worker pool name when using --print-join-command")))
			})
		})

		When("the print-connect-command flag is set", func() {
			BeforeEach(func() {
				options.PrintConnectCommand = true
				options.Token.Combined = "abcdef.abcdef1234567890"
			})

			It("should return an error when no shoot namespace is provided", func() {
				options.Shoot.Name = "foo"
				Expect(options.Validate()).To(MatchError(ContainSubstring("must specify a shoot namespace and name when using --print-connect-command")))
			})

			It("should return an error when no shoot name is provided", func() {
				options.Shoot.Namespace = "foo"
				Expect(options.Validate()).To(MatchError(ContainSubstring("must specify a shoot namespace and name when using --print-connect-command")))
			})
		})

		It("should return an error when a custom description is specified while shoot info is provided", func() {
			options.Shoot.Name = "foo"
			options.Shoot.Namespace = "bar"
			options.Description = "custom description"
			options.Token.Combined = "abcdef.abcdef1234567890"
			Expect(options.Validate()).To(MatchError(ContainSubstring("cannot specify a custom description when creating a bootstrap token for the 'gardenadm connect' command")))
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

		It("should default the description for 'gardenadm connect' when shoot info is provided", func() {
			options.Token.Combined = "foo.bar"
			options.Shoot.Name = "foo"
			options.Shoot.Namespace = "bar"

			Expect(options.Complete()).To(Succeed())
			Expect(options.Description).To(Equal("Used for connecting the self-hosted Shoot bar/foo to Gardener via 'gardenadm connect'"))
		})

		It("should default the description for 'gardenadm join' when no shoot info is provided", func() {
			options.Token.Combined = "foo.bar"

			Expect(options.Complete()).To(Succeed())
			Expect(options.Description).To(Equal("Used for joining nodes to a self-hosted shoot cluster via 'gardenadm join'"))
		})
	})
})
