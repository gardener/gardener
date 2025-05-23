// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package errors_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
)

var _ = Describe("Unwrap", func() {
	var (
		singleErr     = errors.New("single")
		wrappedErr    = fmt.Errorf("outer err:%w", singleErr)
		nilErr        error
		mulWrappedErr = fmt.Errorf(
			"error 3: %w",
			fmt.Errorf(
				"error 2: %w",
				fmt.Errorf("error 1:%w", singleErr)),
		)

		fatalErr       = errors.New("fatal")
		suppressedErr1 = errorsutils.WithSuppressed(singleErr, fatalErr)
		wrappedErr1    = fmt.Errorf("err 1: %w", suppressedErr1)
		seppressedErr2 = errorsutils.WithSuppressed(wrappedErr1, singleErr)
		wrappedErr2    = fmt.Errorf("err 2: %w", seppressedErr2)

		wrappedErr3 = fmt.Errorf("err 3: %w", wrappedErr2)
		supperssed3 = errorsutils.WithSuppressed(wrappedErr3, singleErr)
	)

	DescribeTable("#Wrapped Errors",
		func(in error, m types.GomegaMatcher) { Expect(errorsutils.Unwrap(in)).To(m) },

		Entry("return same error", singleErr, BeIdenticalTo(singleErr)),
		Entry("return root error", wrappedErr, BeIdenticalTo(singleErr)),
		Entry("return nil error", nilErr, BeNil()),
		Entry("return root err from nested err", mulWrappedErr, BeIdenticalTo(singleErr)),
		Entry("return cause error", suppressedErr1, BeIdenticalTo(singleErr)),
		Entry("return nested cause error", wrappedErr2, BeIdenticalTo(singleErr)),
		Entry("return deeper nested cause error", supperssed3, BeIdenticalTo(singleErr)),
	)
})
