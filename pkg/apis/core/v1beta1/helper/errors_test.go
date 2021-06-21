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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/retry"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/types"
)

var _ = Describe("errors", func() {
	Describe("#ErrorWithCodes", func() {
		It("should be marked as a retriable error", func() {
			Expect(retry.IsRetriable(&ErrorWithCodes{})).To(BeTrue())
		})
	})

	DescribeTable("#DetermineError",
		func(err error, msg string, expectedErr error) {
			Expect(DetermineError(err, msg)).To(Equal(expectedErr))
		},

		Entry("no error", nil, "foo", errors.New("foo")),
		Entry("no code to extract", errors.New("foo"), "", errors.New("foo")),
		Entry("unauthorized", errors.New("unauthorized"), "", NewErrorWithCodes("unauthorized", gardencorev1beta1.ErrorInfraUnauthorized)),
		Entry("unauthorized with coder", NewErrorWithCodes("", gardencorev1beta1.ErrorInfraUnauthorized), "", NewErrorWithCodes("", gardencorev1beta1.ErrorInfraUnauthorized)),
		Entry("quota exceeded", errors.New("limitexceeded"), "", NewErrorWithCodes("limitexceeded", gardencorev1beta1.ErrorInfraQuotaExceeded)),
		Entry("quota exceeded", errors.New("foolimitexceeded"), "", NewErrorWithCodes("foolimitexceeded", gardencorev1beta1.ErrorInfraQuotaExceeded)),
		Entry("quota exceeded", errors.New("equestlimitexceeded"), "", NewErrorWithCodes("equestlimitexceeded", gardencorev1beta1.ErrorInfraQuotaExceeded)),
		Entry("quota exceeded", errors.New("subnetlimitexceeded"), "", NewErrorWithCodes("subnetlimitexceeded", gardencorev1beta1.ErrorInfraQuotaExceeded)),
		Entry("quota exceeded with coder", NewErrorWithCodes("limitexceeded", gardencorev1beta1.ErrorInfraQuotaExceeded), "", NewErrorWithCodes("limitexceeded", gardencorev1beta1.ErrorInfraQuotaExceeded)),
		Entry("request throttling", errors.New("message=cannot get hosted zones: Throttling"), "", NewErrorWithCodes("message=cannot get hosted zones: Throttling", gardencorev1beta1.ErrorInfraRateLimitsExceeded)),
		Entry("request throttling", errors.New("requestlimitexceeded"), "", NewErrorWithCodes("requestlimitexceeded", gardencorev1beta1.ErrorInfraRateLimitsExceeded)),
		Entry("request throttling coder", NewErrorWithCodes("message=cannot get hosted zones: Throttling", gardencorev1beta1.ErrorInfraRateLimitsExceeded), "", NewErrorWithCodes("message=cannot get hosted zones: Throttling", gardencorev1beta1.ErrorInfraRateLimitsExceeded)),
		Entry("insufficient privileges", errors.New("accessdenied"), "", NewErrorWithCodes("accessdenied", gardencorev1beta1.ErrorInfraInsufficientPrivileges)),
		Entry("insufficient privileges with coder", NewErrorWithCodes("accessdenied", gardencorev1beta1.ErrorInfraInsufficientPrivileges), "", NewErrorWithCodes("accessdenied", gardencorev1beta1.ErrorInfraInsufficientPrivileges)),
		Entry("infrastructure dependencies", errors.New("pendingverification"), "", NewErrorWithCodes("pendingverification", gardencorev1beta1.ErrorInfraDependencies)),
		Entry("infrastructure dependencies with coder", NewErrorWithCodes("pendingverification", gardencorev1beta1.ErrorInfraDependencies), "error occurred: pendingverification", NewErrorWithCodes("error occurred: pendingverification", gardencorev1beta1.ErrorInfraDependencies)),
		Entry("resources depleted", errors.New("not available in the current hardware cluster"), "error occurred: not available in the current hardware cluster", NewErrorWithCodes("error occurred: not available in the current hardware cluster", gardencorev1beta1.ErrorInfraResourcesDepleted)),
		Entry("resources depleted with coder", NewErrorWithCodes("not available in the current hardware cluster", gardencorev1beta1.ErrorInfraResourcesDepleted), "error occurred: not available in the current hardware cluster", NewErrorWithCodes("error occurred: not available in the current hardware cluster", gardencorev1beta1.ErrorInfraResourcesDepleted)),
		Entry("configuration problem", errors.New("InvalidParameterValue"), "error occurred: InvalidParameterValue", NewErrorWithCodes("error occurred: InvalidParameterValue", gardencorev1beta1.ErrorConfigurationProblem)),
		Entry("configuration problem with coder", NewErrorWithCodes("InvalidParameterValue", gardencorev1beta1.ErrorConfigurationProblem), "error occurred: InvalidParameterValue", NewErrorWithCodes("error occurred: InvalidParameterValue", gardencorev1beta1.ErrorConfigurationProblem)),
		Entry("retryable configuration problem", errors.New("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions"), "", NewErrorWithCodes("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions", gardencorev1beta1.ErrorRetryableConfigurationProblem)),
		Entry("retryable configuration problem with coder", NewErrorWithCodes("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions", gardencorev1beta1.ErrorRetryableConfigurationProblem), "", NewErrorWithCodes("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions", gardencorev1beta1.ErrorRetryableConfigurationProblem)),
		Entry("retryable infrastructure dependencies", errors.New("Code=\"RetryableError\" Message=\"A retryable error occurred"), "", NewErrorWithCodes("Code=\"RetryableError\" Message=\"A retryable error occurred", gardencorev1beta1.ErrorRetryableInfraDependencies)),
		Entry("retryable infrastructure dependencies with coder", NewErrorWithCodes("Code=\"RetryableError\" Message=\"A retryable error occurred", gardencorev1beta1.ErrorRetryableInfraDependencies), "", NewErrorWithCodes("Code=\"RetryableError\" Message=\"A retryable error occurred", gardencorev1beta1.ErrorRetryableInfraDependencies)),
	)

	DescribeTable("#ExtractErrorCodes",
		func(err error, matcher GomegaMatcher) {
			Expect(ExtractErrorCodes(err)).To(matcher)
		},

		Entry("no error given", nil, BeEmpty()),
		Entry("no code error given", errors.New("error"), BeEmpty()),
		Entry("code error given", NewErrorWithCodes("", gardencorev1beta1.ErrorInfraUnauthorized), ConsistOf(Equal(gardencorev1beta1.ErrorInfraUnauthorized))),
		Entry("multiple code error given", NewErrorWithCodes("", gardencorev1beta1.ErrorInfraUnauthorized, gardencorev1beta1.ErrorConfigurationProblem), ConsistOf(Equal(gardencorev1beta1.ErrorInfraUnauthorized), Equal(gardencorev1beta1.ErrorConfigurationProblem))),
		Entry("wrapped code error", fmt.Errorf("error %w", NewErrorWithCodes("", gardencorev1beta1.ErrorInfraUnauthorized)), ConsistOf(Equal(gardencorev1beta1.ErrorInfraUnauthorized))),
	)

	var (
		unauthorizedError                = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraUnauthorized}}
		configurationProblemError        = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorConfigurationProblem}}
		infraInsufficientPrivilegesError = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraInsufficientPrivileges}}
		infraQuotaExceededError          = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded}}
		infraReteLimitsExceededError     = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraRateLimitsExceeded}}
		infraDependenciesError           = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraDependencies}}
		infraResourcesDepletedError      = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraResourcesDepleted}}
		cleanupClusterResourcesError     = gardencorev1beta1.LastError{Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorCleanupClusterResources}}
		errorWithoutCodes                = gardencorev1beta1.LastError{}
	)

	DescribeTable("#HasNonRetryableErrorCode",
		func(lastErrors []gardencorev1beta1.LastError, matcher GomegaMatcher) {
			Expect(HasNonRetryableErrorCode(lastErrors...)).To(matcher)
		},

		Entry("no error given", nil, BeFalse()),
		Entry("only errors with non-retryable error codes", []gardencorev1beta1.LastError{unauthorizedError, infraInsufficientPrivilegesError, infraQuotaExceededError, infraDependenciesError, configurationProblemError}, BeTrue()),
		Entry("only errors with retryable error codes", []gardencorev1beta1.LastError{infraResourcesDepletedError, cleanupClusterResourcesError}, BeFalse()),
		Entry("errors with both retryable and not retryable error codes", []gardencorev1beta1.LastError{unauthorizedError, configurationProblemError, infraInsufficientPrivilegesError, infraQuotaExceededError, infraReteLimitsExceededError, infraDependenciesError, infraResourcesDepletedError, cleanupClusterResourcesError}, BeTrue()),
		Entry("errors without error codes", []gardencorev1beta1.LastError{errorWithoutCodes}, BeFalse()),
	)

	DescribeTable("#HasErrorCode",
		func(lastErrors []gardencorev1beta1.LastError, code gardencorev1beta1.ErrorCode, matcher GomegaMatcher) {
			Expect(HasErrorCode(lastErrors, code)).To(matcher)
		},

		Entry("should return false when no error given", nil, gardencorev1beta1.ErrorInfraRateLimitsExceeded, BeFalse()),
		Entry("should return false when error code is not present", []gardencorev1beta1.LastError{unauthorizedError, infraInsufficientPrivilegesError}, gardencorev1beta1.ErrorInfraRateLimitsExceeded, BeFalse()),
		Entry("should return true when error code is present", []gardencorev1beta1.LastError{unauthorizedError, infraInsufficientPrivilegesError, infraReteLimitsExceededError}, gardencorev1beta1.ErrorInfraRateLimitsExceeded, BeTrue()),
	)
})
