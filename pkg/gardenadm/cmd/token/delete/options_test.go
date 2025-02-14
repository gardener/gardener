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
	var (
		options *Options

		tokenID = "token-id"
	)

	BeforeEach(func() {
		options = &Options{}
	})

	Describe("#ParseArgs", func() {
		It("should use the first argument as token ID", func() {
			Expect(options.ParseArgs([]string{tokenID})).To(Succeed())
			Expect(options.TokenID).To(Equal(tokenID))
		})
	})

	Describe("#Validate", func() {
		It("should pass for valid options", func() {
			options.TokenID = "foo123"

			Expect(options.Validate()).To(Succeed())
		})

		It("should fail because token ID is not set", func() {
			Expect(options.Validate()).To(MatchError(ContainSubstring("must provide a token ID to delete")))
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
