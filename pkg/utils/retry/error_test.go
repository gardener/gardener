// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package retry_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/retry"
)

var _ = Describe("Retriable Errors", func() {
	Describe("#RetriableError", func() {
		It("should mark an error as retriable", func() {
			err := errors.New("foo")
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
			Expect(IsRetriable(errors.New("foo"))).To(BeFalse())
		})
		It("should return true for retriable error", func() {
			Expect(IsRetriable(dummyRetriableError{})).To(BeTrue())
			Expect(IsRetriable(&dummyRetriableError{})).To(BeTrue())
		})
		It("should return true for error created by RetriableError", func() {
			Expect(IsRetriable(RetriableError(errors.New("foo")))).To(BeTrue())
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
