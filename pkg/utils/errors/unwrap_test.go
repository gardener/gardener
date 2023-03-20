// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
