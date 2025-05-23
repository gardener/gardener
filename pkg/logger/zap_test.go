// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logger_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("zap", func() {
	Describe("#NewZapLogger", func() {
		It("should return a pointer to a Logger object ('debug' level)", func() {
			logger, err := NewZapLogger(DebugLevel, FormatText)
			Expect(err).NotTo(HaveOccurred())
			Expect(logger.V(0).Enabled()).To(BeTrue())
			Expect(logger.V(1).Enabled()).To(BeTrue())
		})

		It("should return a pointer to a Logger object ('info' level)", func() {
			logger, err := NewZapLogger(InfoLevel, FormatText)
			Expect(err).NotTo(HaveOccurred())
			Expect(logger.V(0).Enabled()).To(BeTrue())
			Expect(logger.V(1).Enabled()).To(BeFalse())
		})

		It("should default to 'info' level", func() {
			logger, err := NewZapLogger("", FormatText)
			Expect(err).NotTo(HaveOccurred())
			Expect(logger.V(0).Enabled()).To(BeTrue())
			Expect(logger.V(1).Enabled()).To(BeFalse())
		})

		It("should return a pointer to a Logger object ('error' level)", func() {
			logger, err := NewZapLogger(ErrorLevel, FormatText)
			Expect(err).NotTo(HaveOccurred())
			Expect(logger.V(0).Enabled()).To(BeFalse())
			Expect(logger.V(1).Enabled()).To(BeFalse())
		})

		It("should reject invalid log level", func() {
			_, err := NewZapLogger("invalid", FormatText)
			Expect(err).To(HaveOccurred())
		})
	})
})
