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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/types"
)

var _ = Describe("errors", func() {
	DescribeTable("#DetermineError",
		func(err error, msg string, expectedErr error) {
			Expect(DetermineError(err, msg)).To(Equal(expectedErr))
		},

		Entry("no error", nil, "foo", errors.New("foo")),
		Entry("no code to extract", errors.New("foo"), "", errors.New("foo")),
		Entry("unauthorized", errors.New("unauthorized"), "", NewErrorWithCodes("unauthorized", gardencorev1beta1.ErrorInfraUnauthorized)),
		Entry("unauthorized with coder", NewErrorWithCodes("", gardencorev1beta1.ErrorInfraUnauthorized), "", NewErrorWithCodes("", gardencorev1beta1.ErrorInfraUnauthorized)),
		Entry("quota exceeded", errors.New("limitexceeded"), "", NewErrorWithCodes("limitexceeded", gardencorev1beta1.ErrorInfraQuotaExceeded)),
		Entry("quota exceeded with coder", NewErrorWithCodes("limitexceeded", gardencorev1beta1.ErrorInfraQuotaExceeded), "", NewErrorWithCodes("limitexceeded", gardencorev1beta1.ErrorInfraQuotaExceeded)),
		Entry("insufficient privileges", errors.New("accessdenied"), "", NewErrorWithCodes("accessdenied", gardencorev1beta1.ErrorInfraInsufficientPrivileges)),
		Entry("insufficient privileges with coder", NewErrorWithCodes("accessdenied", gardencorev1beta1.ErrorInfraInsufficientPrivileges), "", NewErrorWithCodes("accessdenied", gardencorev1beta1.ErrorInfraInsufficientPrivileges)),
		Entry("infrastructure dependencies", errors.New("pendingverification"), "", NewErrorWithCodes("pendingverification", gardencorev1beta1.ErrorInfraDependencies)),
		Entry("infrastructure dependencies with coder", NewErrorWithCodes("pendingverification", gardencorev1beta1.ErrorInfraDependencies), "error occurred: pendingverification", NewErrorWithCodes("error occurred: pendingverification", gardencorev1beta1.ErrorInfraDependencies)),
		Entry("resources depleted", errors.New("not available in the current hardware cluster"), "error occurred: not available in the current hardware cluster", NewErrorWithCodes("error occurred: not available in the current hardware cluster", gardencorev1beta1.ErrorInfraResourcesDepleted)),
		Entry("resources depleted with coder", NewErrorWithCodes("not available in the current hardware cluster", gardencorev1beta1.ErrorInfraResourcesDepleted), "error occurred: not available in the current hardware cluster", NewErrorWithCodes("error occurred: not available in the current hardware cluster", gardencorev1beta1.ErrorInfraResourcesDepleted)),
		Entry("configuration problem", errors.New("InvalidParameterValue"), "error occurred: InvalidParameterValue", NewErrorWithCodes("error occurred: InvalidParameterValue", gardencorev1beta1.ErrorConfigurationProblem)),
		Entry("configuration problem with coder", NewErrorWithCodes("InvalidParameterValue", gardencorev1beta1.ErrorConfigurationProblem), "error occurred: InvalidParameterValue", NewErrorWithCodes("error occurred: InvalidParameterValue", gardencorev1beta1.ErrorConfigurationProblem)),
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
		Entry("only errors with non-retryable error codes", []gardencorev1beta1.LastError{unauthorizedError, configurationProblemError}, BeTrue()),
		Entry("only errors with retryable error codes", []gardencorev1beta1.LastError{infraInsufficientPrivilegesError, infraQuotaExceededError, infraDependenciesError, infraResourcesDepletedError, cleanupClusterResourcesError}, BeFalse()),
		Entry("errors with both retryable and not retryable error codes", []gardencorev1beta1.LastError{unauthorizedError, configurationProblemError, infraInsufficientPrivilegesError, infraQuotaExceededError, infraDependenciesError, infraResourcesDepletedError, cleanupClusterResourcesError}, BeTrue()),
		Entry("errors without error codes", []gardencorev1beta1.LastError{errorWithoutCodes}, BeFalse()),
	)
})
