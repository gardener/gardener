// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package delete_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token/delete"
)

var _ = Describe("Options", func() {
	var options *Options

	BeforeEach(func() {
		options = &Options{}
	})

	Describe("#ParseArgs", func() {
		It("should trim the spaces form the arguments", func() {
			Expect(options.ParseArgs([]string{" first", "second "})).To(Succeed())
			Expect(options.TokenValues).To(ConsistOf("first", "second"))
		})
	})

	Describe("#Validate", func() {
		It("should pass for valid options", func() {
			options.TokenValues = []string{"foo123", "bootstrap-token-foo123", "foo123.abcdef0123456789"}
			Expect(options.Validate()).To(Succeed())
		})

		It("should fail because a token value is invalid", func() {
			options.TokenValues = []string{"foo"}
			Expect(options.Validate()).To(MatchError(ContainSubstring("invalid token value")))
		})

		It("should fail because no token value is set", func() {
			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide at least one token value to delete")))
		})
	})

	Describe("#Complete", func() {
		It("should properly parse the token IDs from the values", func() {
			options.TokenValues = []string{"foo123", "bootstrap-token-123abc", "987654.abcdef0123456789"}
			Expect(options.Complete()).To(Succeed())
			Expect(options.TokenIDs).To(ConsistOf("foo123", "123abc", "987654"))
		})
	})
})
