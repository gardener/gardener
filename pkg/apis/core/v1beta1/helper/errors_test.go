// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"errors"
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var _ = Describe("errors", func() {
	Describe("#ErrorWithCodes", func() {
		It("should be marked as a retriable error", func() {
			Expect(retry.IsRetriable(&ErrorWithCodes{})).To(BeTrue())
		})
	})

	DescribeTable("#ExtractErrorCodes",
		func(err error, matcher gomegatypes.GomegaMatcher) {
			Expect(ExtractErrorCodes(err)).To(matcher)
		},

		Entry("no error given", nil, BeEmpty()),
		Entry("no code error given", errors.New("error"), BeEmpty()),
		Entry("code error given",
			NewErrorWithCodes(errors.New("error"), gardencorev1beta1.ErrorInfraUnauthorized),
			ConsistOf(Equal(gardencorev1beta1.ErrorInfraUnauthorized))),
		Entry("multiple code error given", NewErrorWithCodes(errors.New("error"),
			gardencorev1beta1.ErrorInfraUnauthorized, gardencorev1beta1.ErrorConfigurationProblem),
			ConsistOf(Equal(gardencorev1beta1.ErrorInfraUnauthorized), Equal(gardencorev1beta1.ErrorConfigurationProblem))),
		Entry("wrapped code error", fmt.Errorf("error %w", NewErrorWithCodes(errors.New("error"), gardencorev1beta1.ErrorInfraUnauthorized)),
			ConsistOf(Equal(gardencorev1beta1.ErrorInfraUnauthorized))),
	)

	Describe("#MultiErrorWithCodes", func() {
		var (
			formatFn   func(errs []error) string
			multiError *MultiErrorWithCodes

			errs []error
		)

		JustBeforeEach(func() {
			formatFn = func([]error) string {
				return strconv.Itoa(len(errs))
			}
			multiError = NewMultiErrorWithCodes(formatFn)

			for _, err := range errs {
				multiError.Append(err)
			}
		})

		Context("when no errors added", func() {
			It("should return no codes", func() {
				Expect(multiError.Codes()).To(BeEmpty())
			})

			It("should return nil", func() {
				Expect(multiError.ErrorOrNil()).To(Succeed())
			})

			It("should return correct error string", func() {
				output := multiError.Error()
				numErrors, err := strconv.Atoi(output)
				Expect(err).ToNot(HaveOccurred())
				Expect(numErrors).To(Equal(len(errs)))
			})
		})

		Context("when errors are added", func() {
			BeforeEach(func() {
				errs = []error{
					NewErrorWithCodes(errors.New("InsufficientPrivileges"), gardencorev1beta1.ErrorInfraUnauthorized),
					NewErrorWithCodes(errors.New("InsufficientPrivileges"), gardencorev1beta1.ErrorInfraUnauthorized),
					NewErrorWithCodes(errors.New("ErrorConfigurationProblem"), gardencorev1beta1.ErrorConfigurationProblem),
					errors.New("foo"),
				}
			})

			It("should return unified codes", func() {
				Expect(multiError.Codes()).To(ConsistOf([]gardencorev1beta1.ErrorCode{
					gardencorev1beta1.ErrorInfraUnauthorized,
					gardencorev1beta1.ErrorConfigurationProblem,
				}))
			})

			It("should return error", func() {
				Expect(multiError.ErrorOrNil()).ToNot(Succeed())
			})

			It("should return correct error string", func() {
				output := multiError.Error()
				numErrors, err := strconv.Atoi(output)
				Expect(err).ToNot(HaveOccurred())
				Expect(numErrors).To(Equal(len(errs)))
			})
		})
	})

	var (
		unauthenticatedError         = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraUnauthenticated}}
		unauthorizedError            = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraUnauthorized}}
		configurationProblemError    = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorConfigurationProblem}}
		infraQuotaExceededError      = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded}}
		infraRateLimitsExceededError = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraRateLimitsExceeded}}
		infraDependenciesError       = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraDependencies}}
		infraResourcesDepletedError  = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraResourcesDepleted}}
		cleanupClusterResourcesError = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorCleanupClusterResources}}
		errorWithoutCodes            = gardencorev1beta1.LastError{}
	)

	DescribeTable("#HasNonRetryableErrorCode",
		func(lastErrors []gardencorev1beta1.LastError, matcher gomegatypes.GomegaMatcher) {
			Expect(HasNonRetryableErrorCode(lastErrors...)).To(matcher)
		},

		Entry("no error given", nil, BeFalse()),
		Entry("only errors with non-retryable error codes", []gardencorev1beta1.LastError{unauthenticatedError, unauthorizedError, infraQuotaExceededError, infraDependenciesError, configurationProblemError}, BeTrue()),
		Entry("only errors with retryable error codes", []gardencorev1beta1.LastError{infraResourcesDepletedError, cleanupClusterResourcesError}, BeFalse()),
		Entry("errors with both retryable and not retryable error codes", []gardencorev1beta1.LastError{unauthorizedError, unauthenticatedError, configurationProblemError, infraQuotaExceededError, infraRateLimitsExceededError, infraDependenciesError, infraResourcesDepletedError, cleanupClusterResourcesError}, BeTrue()),
		Entry("errors without error codes", []gardencorev1beta1.LastError{errorWithoutCodes}, BeFalse()),
	)

	DescribeTable("#HasErrorCode",
		func(lastErrors []gardencorev1beta1.LastError, code gardencorev1beta1.ErrorCode, matcher gomegatypes.GomegaMatcher) {
			Expect(HasErrorCode(lastErrors, code)).To(matcher)
		},

		Entry("should return false when no error given", nil, gardencorev1beta1.ErrorInfraRateLimitsExceeded, BeFalse()),
		Entry("should return false when error code is not present", []gardencorev1beta1.LastError{unauthorizedError, infraResourcesDepletedError}, gardencorev1beta1.ErrorInfraRateLimitsExceeded, BeFalse()),
		Entry("should return true when error code is present", []gardencorev1beta1.LastError{unauthorizedError, infraResourcesDepletedError, infraRateLimitsExceededError}, gardencorev1beta1.ErrorInfraRateLimitsExceeded, BeTrue()),
	)
})
