// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	//utilerrors "github.com/gardener/gardener/pkg/utils/errors"

	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
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

		fatalErr   = errors.New("fatal")
		err1       = utilerrors.WithSuppressed(singleErr, fatalErr)
		wrErr1     = fmt.Errorf("err 1: %w", err1)
		causedErr1 = utilerrors.WithSuppressed(wrErr1, singleErr)
		wrErr2     = fmt.Errorf("err 2: %w", causedErr1)
	)

	DescribeTable("#Wrapped Errors",
		func(in error, m types.GomegaMatcher) { Expect(utilerrors.Unwrap(in)).To(m) },

		Entry("return same error", singleErr, BeIdenticalTo(singleErr)),
		Entry("return root error", wrappedErr, BeIdenticalTo(singleErr)),
		Entry("return nil error", nilErr, BeNil()),
		Entry("return root err from nested err", mulWrappedErr, BeIdenticalTo(singleErr)),
		Entry("return cause error", err1, BeIdenticalTo(singleErr)),
		Entry("return nested cause error", wrErr2, BeIdenticalTo(singleErr)),
	)
})
