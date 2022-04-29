// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper_test

import (
	"errors"
	"fmt"
	"strconv"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/retry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
)

var _ = Describe("errors", func() {
	Describe("#ErrorWithCodes", func() {
		It("should be marked as a retriable error", func() {
			Expect(retry.IsRetriable(&ErrorWithCodes{})).To(BeTrue())
		})
	})

	Describe("#DetermineError", func() {
		It("should return nil for empty error", func() {
			Expect(DeprecatedDetermineError(nil)).To(BeNil())
		})
	})
	DescribeTable("#DetermineError",
		func(err error, expectedErr error) {
			Expect(DeprecatedDetermineError(err)).To(Equal(expectedErr))
		},

		Entry("no wrapped error",
			fmt.Errorf("foo"),
			errors.New("foo"),
		),
		Entry("no code to extract",
			errors.New("foo"),
			errors.New("foo"),
		),
		Entry("unauthenticated",
			errors.New("authentication failed"),
			NewErrorWithCodes(errors.New("authentication failed"), gardencorev1beta1.ErrorInfraUnauthenticated),
		),
		Entry("unauthenticated",
			errors.New("invalidauthenticationtokentenant"),
			NewErrorWithCodes(errors.New("invalidauthenticationtokentenant"), gardencorev1beta1.ErrorInfraUnauthenticated),
		),
		Entry("wrapped unauthenticated error",
			fmt.Errorf("%w", NewErrorWithCodes(errors.New("unauthenticated"), gardencorev1beta1.ErrorInfraUnauthenticated)),
			NewErrorWithCodes(errors.New("unauthenticated"), gardencorev1beta1.ErrorInfraUnauthenticated),
		),
		Entry("unauthorized",
			errors.New("unauthorized"),
			NewErrorWithCodes(errors.New("unauthorized"), gardencorev1beta1.ErrorInfraUnauthorized),
		),
		Entry("unauthorized with coder",
			NewErrorWithCodes(errors.New("operation not allowed"), gardencorev1beta1.ErrorInfraUnauthorized),
			NewErrorWithCodes(errors.New("operation not allowed"), gardencorev1beta1.ErrorInfraUnauthorized),
		),
		Entry("wrapped unauthorized error",
			fmt.Errorf("no sufficient permissions: %w", errors.New("AuthorizationFailed")),
			NewErrorWithCodes(fmt.Errorf("no sufficient permissions: %w", errors.New("AuthorizationFailed")), gardencorev1beta1.ErrorInfraUnauthorized),
		),
		Entry("wrapped unauthorized error with coder",
			fmt.Errorf("%w", NewErrorWithCodes(errors.New("unauthorized"), gardencorev1beta1.ErrorInfraUnauthorized)),
			NewErrorWithCodes(errors.New("unauthorized"), gardencorev1beta1.ErrorInfraUnauthorized),
		),
		Entry("insufficient privileges",
			errors.New("accessdenied"),
			NewErrorWithCodes(errors.New("accessdenied"), gardencorev1beta1.ErrorInfraUnauthorized)),
		Entry("insufficient privileges with coder",
			NewErrorWithCodes(errors.New("accessdenied"), gardencorev1beta1.ErrorInfraUnauthorized),
			NewErrorWithCodes(errors.New("accessdenied"), gardencorev1beta1.ErrorInfraUnauthorized)),
		Entry("quota exceeded",
			errors.New("limitexceeded"),
			NewErrorWithCodes(errors.New("limitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded)),
		Entry("quota exceeded",
			errors.New("foolimitexceeded"),
			NewErrorWithCodes(errors.New("foolimitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
		),
		Entry("quota exceeded",
			errors.New("equestlimitexceeded"),
			NewErrorWithCodes(errors.New("equestlimitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
		),
		Entry("quota exceeded",
			errors.New("subnetlimitexceeded"),
			NewErrorWithCodes(errors.New("subnetlimitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
		),
		Entry("quota exceeded with coder",
			NewErrorWithCodes(errors.New("limitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
			NewErrorWithCodes(errors.New("limitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
		),
		Entry("request throttling",
			errors.New("message=cannot get hosted zones: Throttling"),
			NewErrorWithCodes(errors.New("message=cannot get hosted zones: Throttling"), gardencorev1beta1.ErrorInfraRateLimitsExceeded),
		),
		Entry("request throttling",
			errors.New("requestlimitexceeded"),
			NewErrorWithCodes(errors.New("requestlimitexceeded"), gardencorev1beta1.ErrorInfraRateLimitsExceeded),
		),
		Entry("request throttling with coder",
			NewErrorWithCodes(errors.New("message=cannot get hosted zones: Throttling"), gardencorev1beta1.ErrorInfraRateLimitsExceeded),
			NewErrorWithCodes(errors.New("message=cannot get hosted zones: Throttling"), gardencorev1beta1.ErrorInfraRateLimitsExceeded),
		),
		Entry("infrastructure dependencies",
			errors.New("pendingverification"),
			NewErrorWithCodes(errors.New("pendingverification"), gardencorev1beta1.ErrorInfraDependencies),
		),
		Entry("infrastructure dependencies with coder",
			fmt.Errorf("error occurred: %w", NewErrorWithCodes(errors.New("pendingverification"), gardencorev1beta1.ErrorInfraDependencies)),
			NewErrorWithCodes(errors.New("pendingverification"), gardencorev1beta1.ErrorInfraDependencies),
		),
		Entry("resources depleted",
			fmt.Errorf("error occurred: not available in the current hardware cluster"),
			NewErrorWithCodes(errors.New("error occurred: not available in the current hardware cluster"), gardencorev1beta1.ErrorInfraResourcesDepleted),
		),
		Entry("resources depleted with coder",
			fmt.Errorf("error occurred: %w", NewErrorWithCodes(errors.New("not available in the current hardware cluster"), gardencorev1beta1.ErrorInfraResourcesDepleted)),
			NewErrorWithCodes(errors.New("not available in the current hardware cluster"), gardencorev1beta1.ErrorInfraResourcesDepleted),
		),
		Entry("configuration problem",
			fmt.Errorf("error occurred: %w", errors.New("InvalidParameterValue")),
			NewErrorWithCodes(fmt.Errorf("error occurred: %w", errors.New("InvalidParameterValue")), gardencorev1beta1.ErrorConfigurationProblem),
		),
		Entry("configuration problem with coder",
			fmt.Errorf("error occurred: %w", NewErrorWithCodes(errors.New("InvalidParameterValue"), gardencorev1beta1.ErrorConfigurationProblem)),
			NewErrorWithCodes(errors.New("InvalidParameterValue"), gardencorev1beta1.ErrorConfigurationProblem),
		),
		Entry("retryable configuration problem",
			errors.New("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions"),
			NewErrorWithCodes(errors.New("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions"), gardencorev1beta1.ErrorRetryableConfigurationProblem),
		),
		Entry("retryable configuration problem with coder",
			NewErrorWithCodes(errors.New("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions"), gardencorev1beta1.ErrorRetryableConfigurationProblem),
			NewErrorWithCodes(errors.New("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions"), gardencorev1beta1.ErrorRetryableConfigurationProblem)),
		Entry("retryable infrastructure dependencies",
			errors.New("Code=\"RetryableError\" Message=\"A retryable error occurred"),
			NewErrorWithCodes(errors.New("Code=\"RetryableError\" Message=\"A retryable error occurred"), gardencorev1beta1.ErrorRetryableInfraDependencies),
		),
		Entry("retryable infrastructure dependencies with coder",
			NewErrorWithCodes(errors.New("Code=\"RetryableError\" Message=\"A retryable error occurred"), gardencorev1beta1.ErrorRetryableInfraDependencies),
			NewErrorWithCodes(errors.New("Code=\"RetryableError\" Message=\"A retryable error occurred"), gardencorev1beta1.ErrorRetryableInfraDependencies)),
	)

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
			formatFn = func(err []error) string {
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
				Expect(multiError.ErrorOrNil()).To(BeNil())
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
				Expect(multiError.ErrorOrNil()).ToNot(BeNil())
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
