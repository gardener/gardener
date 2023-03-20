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

package retry_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/retry"
)

var _ = Describe("Retriable Errors", func() {
	Describe("#RetriableError", func() {
		It("should mark an error as retriable", func() {
			err := fmt.Errorf("foo")
			r := RetriableError(err)
			Expect(r).To(MatchError("foo"))

			var re interface{ Retriable() }
			Expect(errors.As(r, &re)).To(BeTrue())
		})
		It("should allow unwrapping the given error", func() {
			err := &specialError{}

			r := RetriableError(err)
			Expect(r).To(MatchError("special"))

			var re interface{ Retriable() }
			Expect(errors.As(r, &re)).To(BeTrue())

			var special interface{ Special() }
			Expect(errors.As(r, &special)).To(BeTrue())
		})
	})

	Describe("#IsRetriable", func() {
		It("should return false for non-retriable error", func() {
			Expect(IsRetriable(fmt.Errorf("foo"))).To(BeFalse())
		})
		It("should return true for retriable error", func() {
			Expect(IsRetriable(dummyRetriableError{})).To(BeTrue())
			Expect(IsRetriable(&dummyRetriableError{})).To(BeTrue())
		})
		It("should return true for error created by RetriableError", func() {
			Expect(IsRetriable(RetriableError(fmt.Errorf("foo")))).To(BeTrue())
			Expect(IsRetriable(RetriableError(&specialError{}))).To(BeTrue())
		})
	})
})

type specialError struct{}

func (s *specialError) Error() string { return "special" }
func (s *specialError) Special()      {}

type dummyRetriableError struct{}

func (s dummyRetriableError) Error() string { return "dummy" }
func (s dummyRetriableError) Retriable()    {}
