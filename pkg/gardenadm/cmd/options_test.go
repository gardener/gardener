// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"k8s.io/klog/v2"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/utils/test"
	clitest "github.com/gardener/gardener/pkg/utils/test/cli"
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
		var stdOut, stdErr *Buffer

		BeforeEach(func() {
			options.IOStreams, _, stdOut, stdErr = clitest.NewTestIOStreams()
		})

		It("should not log anything if the logger is uninitialized", func() {
			options.Log.Info("Some example log message")
			Consistently(stdErr.Contents).Should(BeEmpty())
			Consistently(stdOut.Contents).Should(BeEmpty())
		})

		It("should fail when log level is unknown", func() {
			options.LogLevel = "foo"
			Expect(options.Complete()).To(MatchError(ContainSubstring("error instantiating zap logger: invalid log level")))
		})

		It("should fail when log format is unknown", func() {
			options.LogFormat = "foo"
			Expect(options.Complete()).To(MatchError(ContainSubstring("error instantiating zap logger: invalid log format")))
		})

		It("should initialize the logger to write to stderr", func() {
			Expect(options.Complete()).To(Succeed())

			options.Log.Info("Some example log message")
			Eventually(stdErr).Should(Say("Some example log message"))
			Consistently(stdOut.Contents).Should(BeEmpty())
		})

		It("should initialize the global logger in controller-runtime", func() {
			var logfLogger logr.Logger
			DeferCleanup(test.WithVar(&LogfSetLogger, func(l logr.Logger) {
				logfLogger = l
			}))

			Expect(options.Complete()).To(Succeed())

			Expect(logfLogger.GetSink()).NotTo(BeNil())
			logfLogger.Info("Some example log message")
			Eventually(stdErr).Should(Say("Some example log message"))
		})

		It("should initialize the global logger in klog", func() {
			Expect(options.Complete()).To(Succeed())

			klog.Info("Some example log message")
			Eventually(stdErr).Should(Say("Some example log message"))
		})
	})
})
