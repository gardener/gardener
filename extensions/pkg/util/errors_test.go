// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	"errors"
	"fmt"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

var _ = Describe("errors", func() {
	var (
		unauthenticatedRegexp               = regexp.MustCompile(`(?i)(InvalidAuthenticationTokenTenant|Authentication failed)`)
		unauthorizedRegexp                  = regexp.MustCompile(`(?i)(Unauthorized|AuthorizationFailed|AccessDenied|OperationNotAllowed)`)
		quotaExceededRegexp                 = regexp.MustCompile(`(?i)((?:^|[^t]|(?:[^s]|^)t|(?:[^e]|^)st|(?:[^u]|^)est|(?:[^q]|^)uest|(?:[^e]|^)quest|(?:[^r]|^)equest)LimitExceeded)`)
		rateLimitsExceededRegexp            = regexp.MustCompile(`(?i)(RequestLimitExceeded|Throttling)`)
		dependenciesRegexp                  = regexp.MustCompile(`(?i)(PendingVerification)`)
		retryableDependenciesRegexp         = regexp.MustCompile(`(?i)(RetryableError)`)
		resourcesDepletedRegexp             = regexp.MustCompile(`(?i)(not available in the current hardware cluster)`)
		configurationProblemRegexp          = regexp.MustCompile(`(?i)(InvalidParameter)`)
		retryableConfigurationProblemRegexp = regexp.MustCompile(`(?i)(is misconfigured and requires zero voluntary evictions)`)

		knownCodes = map[gardencorev1beta1.ErrorCode]func(string) bool{
			gardencorev1beta1.ErrorInfraUnauthenticated:          unauthenticatedRegexp.MatchString,
			gardencorev1beta1.ErrorInfraUnauthorized:             unauthorizedRegexp.MatchString,
			gardencorev1beta1.ErrorInfraQuotaExceeded:            quotaExceededRegexp.MatchString,
			gardencorev1beta1.ErrorInfraRateLimitsExceeded:       rateLimitsExceededRegexp.MatchString,
			gardencorev1beta1.ErrorInfraDependencies:             dependenciesRegexp.MatchString,
			gardencorev1beta1.ErrorRetryableInfraDependencies:    retryableDependenciesRegexp.MatchString,
			gardencorev1beta1.ErrorInfraResourcesDepleted:        resourcesDepletedRegexp.MatchString,
			gardencorev1beta1.ErrorConfigurationProblem:          configurationProblemRegexp.MatchString,
			gardencorev1beta1.ErrorRetryableConfigurationProblem: retryableConfigurationProblemRegexp.MatchString,
		}
	)

	Describe("#DetermineError", func() {
		It("should return nil for empty error", func() {
			Expect(DetermineError(nil, knownCodes)).To(Succeed())
		})
	})
	DescribeTable("#DetermineError",
		func(err error, expectedErr error) {
			Expect(DetermineError(err, knownCodes)).To(Equal(expectedErr))
		},

		Entry("no wrapped error",
			errors.New("foo"),
			errors.New("foo"),
		),
		Entry("no code to extract",
			errors.New("foo"),
			errors.New("foo"),
		),
		Entry("unauthenticated",
			errors.New("authentication failed"),
			v1beta1helper.NewErrorWithCodes(errors.New("authentication failed"), gardencorev1beta1.ErrorInfraUnauthenticated),
		),
		Entry("unauthenticated",
			errors.New("invalidauthenticationtokentenant"),
			v1beta1helper.NewErrorWithCodes(errors.New("invalidauthenticationtokentenant"), gardencorev1beta1.ErrorInfraUnauthenticated),
		),
		Entry("wrapped unauthenticated error with coder",
			fmt.Errorf("%w", v1beta1helper.NewErrorWithCodes(errors.New("unauthenticated"), gardencorev1beta1.ErrorInfraUnauthenticated)),
			fmt.Errorf("%w", v1beta1helper.NewErrorWithCodes(errors.New("unauthenticated"), gardencorev1beta1.ErrorInfraUnauthenticated)),
		),
		Entry("unauthorized",
			errors.New("unauthorized"),
			v1beta1helper.NewErrorWithCodes(errors.New("unauthorized"), gardencorev1beta1.ErrorInfraUnauthorized),
		),
		Entry("unauthorized with coder",
			v1beta1helper.NewErrorWithCodes(errors.New("operation not allowed"), gardencorev1beta1.ErrorInfraUnauthorized),
			v1beta1helper.NewErrorWithCodes(errors.New("operation not allowed"), gardencorev1beta1.ErrorInfraUnauthorized),
		),
		Entry("wrapped unauthorized error",
			fmt.Errorf("no sufficient permissions: %w", errors.New("AuthorizationFailed")),
			v1beta1helper.NewErrorWithCodes(fmt.Errorf("no sufficient permissions: %w", errors.New("AuthorizationFailed")), gardencorev1beta1.ErrorInfraUnauthorized),
		),
		Entry("wrapped unauthorized error with coder",
			fmt.Errorf("%w", v1beta1helper.NewErrorWithCodes(errors.New("unauthorized"), gardencorev1beta1.ErrorInfraUnauthorized)),
			fmt.Errorf("%w", v1beta1helper.NewErrorWithCodes(errors.New("unauthorized"), gardencorev1beta1.ErrorInfraUnauthorized)),
		),
		Entry("insufficient privileges",
			errors.New("accessdenied"),
			v1beta1helper.NewErrorWithCodes(errors.New("accessdenied"), gardencorev1beta1.ErrorInfraUnauthorized),
		),
		Entry("insufficient privileges with coder",
			v1beta1helper.NewErrorWithCodes(errors.New("accessdenied"), gardencorev1beta1.ErrorInfraUnauthorized),
			v1beta1helper.NewErrorWithCodes(errors.New("accessdenied"), gardencorev1beta1.ErrorInfraUnauthorized),
		),
		Entry("quota exceeded",
			errors.New("limitexceeded"),
			v1beta1helper.NewErrorWithCodes(errors.New("limitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
		),
		Entry("quota exceeded",
			errors.New("foolimitexceeded"),
			v1beta1helper.NewErrorWithCodes(errors.New("foolimitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
		),
		Entry("quota exceeded",
			errors.New("equestlimitexceeded"),
			v1beta1helper.NewErrorWithCodes(errors.New("equestlimitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
		),
		Entry("quota exceeded",
			errors.New("subnetlimitexceeded"),
			v1beta1helper.NewErrorWithCodes(errors.New("subnetlimitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
		),
		Entry("quota exceeded with coder",
			v1beta1helper.NewErrorWithCodes(errors.New("limitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
			v1beta1helper.NewErrorWithCodes(errors.New("limitexceeded"), gardencorev1beta1.ErrorInfraQuotaExceeded),
		),
		Entry("request throttling",
			errors.New("message=cannot get hosted zones: Throttling"),
			v1beta1helper.NewErrorWithCodes(errors.New("message=cannot get hosted zones: Throttling"), gardencorev1beta1.ErrorInfraRateLimitsExceeded),
		),
		Entry("request throttling",
			errors.New("requestlimitexceeded"),
			v1beta1helper.NewErrorWithCodes(errors.New("requestlimitexceeded"), gardencorev1beta1.ErrorInfraRateLimitsExceeded),
		),
		Entry("request throttling with coder",
			v1beta1helper.NewErrorWithCodes(errors.New("message=cannot get hosted zones: Throttling"), gardencorev1beta1.ErrorInfraRateLimitsExceeded),
			v1beta1helper.NewErrorWithCodes(errors.New("message=cannot get hosted zones: Throttling"), gardencorev1beta1.ErrorInfraRateLimitsExceeded),
		),
		Entry("infrastructure dependencies",
			errors.New("pendingverification"),
			v1beta1helper.NewErrorWithCodes(errors.New("pendingverification"), gardencorev1beta1.ErrorInfraDependencies),
		),
		Entry("infrastructure dependencies with coder",
			fmt.Errorf("error occurred: %w", v1beta1helper.NewErrorWithCodes(errors.New("pendingverification"), gardencorev1beta1.ErrorInfraDependencies)),
			fmt.Errorf("error occurred: %w", v1beta1helper.NewErrorWithCodes(errors.New("pendingverification"), gardencorev1beta1.ErrorInfraDependencies)),
		),
		Entry("resources depleted",
			fmt.Errorf("error occurred: not available in the current hardware cluster"),
			v1beta1helper.NewErrorWithCodes(errors.New("error occurred: not available in the current hardware cluster"), gardencorev1beta1.ErrorInfraResourcesDepleted),
		),
		Entry("resources depleted with coder",
			fmt.Errorf("error occurred: %w", v1beta1helper.NewErrorWithCodes(errors.New("not available in the current hardware cluster"), gardencorev1beta1.ErrorInfraResourcesDepleted)),
			fmt.Errorf("error occurred: %w", v1beta1helper.NewErrorWithCodes(errors.New("not available in the current hardware cluster"), gardencorev1beta1.ErrorInfraResourcesDepleted)),
		),
		Entry("configuration problem",
			fmt.Errorf("error occurred: %w", errors.New("InvalidParameterValue")),
			v1beta1helper.NewErrorWithCodes(fmt.Errorf("error occurred: %w", errors.New("InvalidParameterValue")), gardencorev1beta1.ErrorConfigurationProblem),
		),
		Entry("configuration problem with coder",
			fmt.Errorf("error occurred: %w", v1beta1helper.NewErrorWithCodes(errors.New("InvalidParameterValue"), gardencorev1beta1.ErrorConfigurationProblem)),
			fmt.Errorf("error occurred: %w", v1beta1helper.NewErrorWithCodes(errors.New("InvalidParameterValue"), gardencorev1beta1.ErrorConfigurationProblem)),
		),
		Entry("retryable configuration problem",
			errors.New("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions"),
			v1beta1helper.NewErrorWithCodes(errors.New("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions"), gardencorev1beta1.ErrorRetryableConfigurationProblem),
		),
		Entry("retryable configuration problem with coder",
			v1beta1helper.NewErrorWithCodes(errors.New("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions"), gardencorev1beta1.ErrorRetryableConfigurationProblem),
			v1beta1helper.NewErrorWithCodes(errors.New("pod disruption budget default/pdb is misconfigured and requires zero voluntary evictions"), gardencorev1beta1.ErrorRetryableConfigurationProblem),
		),
		Entry("retryable infrastructure dependencies",
			errors.New("Code=\"RetryableError\" Message=\"A retryable error occurred"),
			v1beta1helper.NewErrorWithCodes(errors.New("Code=\"RetryableError\" Message=\"A retryable error occurred"), gardencorev1beta1.ErrorRetryableInfraDependencies),
		),
		Entry("retryable infrastructure dependencies with coder",
			v1beta1helper.NewErrorWithCodes(errors.New("Code=\"RetryableError\" Message=\"A retryable error occurred"), gardencorev1beta1.ErrorRetryableInfraDependencies),
			v1beta1helper.NewErrorWithCodes(errors.New("Code=\"RetryableError\" Message=\"A retryable error occurred"), gardencorev1beta1.ErrorRetryableInfraDependencies)),
	)
})
