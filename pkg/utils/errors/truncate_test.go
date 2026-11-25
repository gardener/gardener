// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package errors_test

import (
	"strings"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
)

var _ = Describe("#TruncateLastErrors", func() {
	It("should truncate an overly long description to 4 KiB", func() {
		result := errorsutils.TruncateLastErrors([]gardencorev1beta1.LastError{{
			Description: strings.Repeat("a", 5000),
		}}, logr.Discard())

		Expect(result).To(Equal([]gardencorev1beta1.LastError{{
			Description: strings.Repeat("a", 4091) + "[...]",
		}}))
	})
})

var _ = Describe("#TruncateOperationDescription", func() {
	It("should truncate an overly long description to 4 KiB while preserving the retry suffix", func() {
		result := errorsutils.TruncateOperationDescription(strings.Repeat("a", 5000) + errorsutils.OperationWillBeRetried)

		Expect(result).To(Equal(strings.Repeat("a", 4064) + "[...]" + errorsutils.OperationWillBeRetried))
	})
})
