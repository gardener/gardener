// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd"
)

var _ = Describe("Options", func() {
	var (
		options *Options
	)

	BeforeEach(func() {
		options = &Options{}
	})

	Describe("#Validate", func() {
		It("should return nil", func() {
			options.LogLevel, options.LogFormat = "info", "json"
			Expect(options.Validate()).To(Succeed())
		})

		It("should return an error due to an invalid log level", func() {
			options.LogLevel, options.LogFormat = "foo", "json"
			Expect(options.Validate()).To(MatchError(ContainSubstring("log-level must be one of")))
		})

		It("should return an error due to an invalid log format", func() {
			options.LogLevel, options.LogFormat = "info", "foo"
			Expect(options.Validate()).To(MatchError(ContainSubstring("log-format must be one of")))
		})
	})

	Describe("#Complete", func() {
		It("should succeed", func() {
			Expect(options.Complete()).To(Succeed())
		})

		It("should fail when log level is unknown", func() {
			options.LogLevel = "foo"
			Expect(options.Complete()).To(MatchError(ContainSubstring("error instantiating zap logger: invalid log level")))
		})

		It("should fail when log format is unknown", func() {
			options.LogFormat = "foo"
			Expect(options.Complete()).To(MatchError(ContainSubstring("error instantiating zap logger: invalid log format")))
		})
	})
})
