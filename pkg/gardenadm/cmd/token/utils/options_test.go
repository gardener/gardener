// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
)

var _ = Describe("Options", func() {
	var (
		options *Options
	)

	BeforeEach(func() {
		options = &Options{}
	})

	Describe("#ParseArgs", func() {
		It("should return nil", func() {
			Expect(options.ParseArgs(nil)).To(Succeed())
		})
	})

	Describe("#Validate", func() {
		It("should return an error when the validity is less than 10m", func() {
			options.Validity = time.Minute
			Expect(options.Validate()).To(MatchError(ContainSubstring("minimum validity duration is 10m0s")))
		})

		It("should return an error when the validity is longer than 24h", func() {
			options.Validity = 25 * time.Hour
			Expect(options.Validate()).To(MatchError(ContainSubstring("maximum validity duration is 24h0m0s")))
		})

		It("should an error when the print-join-command flag is set but no worker pool name is provided", func() {
			options.Validity = time.Hour
			options.PrintJoinCommand = true
			Expect(options.Validate()).To(MatchError(ContainSubstring("must specify a worker pool name when using --print-join-command")))
		})

		It("should succeed when options are valid", func() {
			options.Validity = time.Hour
			options.PrintJoinCommand = true
			options.WorkerPoolName = "worker-pool"
			Expect(options.Validate()).To(Succeed())
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
