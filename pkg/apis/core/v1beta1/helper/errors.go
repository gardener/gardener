// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	"errors"
	"regexp"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ErrorWithCodes contains the error and Gardener error codes.
type ErrorWithCodes struct {
	err   error
	codes []gardencorev1beta1.ErrorCode
}

// Retriable marks ErrorWithCodes as retriable.
func (e *ErrorWithCodes) Retriable() {}

// NewErrorWithCodes creates a new error that additionally exposes the given codes via the Coder interface.
func NewErrorWithCodes(err error, codes ...gardencorev1beta1.ErrorCode) error {
	return &ErrorWithCodes{err, codes}
}

// Codes returns all error codes.
func (e *ErrorWithCodes) Codes() []gardencorev1beta1.ErrorCode {
	return e.codes
}

// Unwrap rettieves the error from ErrorWithCodes.
func (e *ErrorWithCodes) Unwrap() error {
	return e.err
}

// Error returns the error message.
func (e *ErrorWithCodes) Error() string {
	return e.err.Error()
}

var (
	unauthenticatedRegexp               = regexp.MustCompile(`(?i)(InvalidAuthenticationTokenTenant|Authentication failed|AuthFailure|invalid character|invalid_client|query returned no results|InvalidAccessKeyId|cannot fetch token|InvalidSecretAccessKey|InvalidSubscriptionId)`)
	unauthorizedRegexp                  = regexp.MustCompile(`(?i)(Unauthorized|InvalidClientTokenId|SignatureDoesNotMatch|AuthorizationFailed|invalid_grant|Authorization Profile was not found|no active subscriptions|UnauthorizedOperation|not authorized|AccessDenied|OperationNotAllowed|Error 403|SERVICE_ACCOUNT_ACCESS_DENIED)`)
	quotaExceededRegexp                 = regexp.MustCompile(`(?i)((?:^|[^t]|(?:[^s]|^)t|(?:[^e]|^)st|(?:[^u]|^)est|(?:[^q]|^)uest|(?:[^e]|^)quest|(?:[^r]|^)equest)LimitExceeded|Quotas|Quota.*exceeded|exceeded quota|Quota has been met|QUOTA_EXCEEDED|Maximum number of ports exceeded|ZONE_RESOURCE_POOL_EXHAUSTED_WITH_DETAILS|VolumeSizeExceedsAvailableQuota)`)
	rateLimitsExceededRegexp            = regexp.MustCompile(`(?i)(RequestLimitExceeded|Throttling|Too many requests)`)
	dependenciesRegexp                  = regexp.MustCompile(`(?i)(PendingVerification|Access Not Configured|accessNotConfigured|DependencyViolation|OptInRequired|DeleteConflict|Conflict|inactive billing state|ReadOnlyDisabledSubscription|is already being used|InUseSubnetCannotBeDeleted|VnetInUse|InUseRouteTableCannotBeDeleted|timeout while waiting for state to become|InvalidCidrBlock|already busy for|InsufficientFreeAddressesInSubnet|InternalServerError|internalerror|internal server error|A resource with the ID|VnetAddressSpaceCannotChangeDueToPeerings|InternalBillingError|There are not enough hosts available)`)
	retryableDependenciesRegexp         = regexp.MustCompile(`(?i)(RetryableError)`)
	resourcesDepletedRegexp             = regexp.MustCompile(`(?i)(not available in the current hardware cluster|InsufficientInstanceCapacity|SkuNotAvailable|ZonalAllocationFailed|out of stock|Zone.NotOnSale)`)
	configurationProblemRegexp          = regexp.MustCompile(`(?i)(AzureBastionSubnet|not supported in your requested Availability Zone|InvalidParameter|InvalidParameterValue|notFound|NetcfgInvalidSubnet|InvalidSubnet|Invalid value|KubeletHasInsufficientMemory|KubeletHasDiskPressure|KubeletHasInsufficientPID|violates constraint|no attached internet gateway found|Your query returned no results|PrivateEndpointNetworkPoliciesCannotBeEnabledOnPrivateEndpointSubnet|invalid VPC attributes|PrivateLinkServiceNetworkPoliciesCannotBeEnabledOnPrivateLinkServiceSubnet|unrecognized feature gate|runtime-config invalid key|LoadBalancingRuleMustDisableSNATSinceSameFrontendIPConfigurationIsReferencedByOutboundRule|strict decoder error|not allowed to configure an unsupported|error during apply of object .* is invalid:|OverconstrainedZonalAllocationRequest|duplicate zones|overlapping zones)`)
	retryableConfigurationProblemRegexp = regexp.MustCompile(`(?i)(is misconfigured and requires zero voluntary evictions|SDK.CanNotResolveEndpoint|The requested configuration is currently not supported)`)
)

// DeprecatedDetermineError determines the Gardener error codes for the given error and returns an ErrorWithCodes with the error and codes.
// This function is deprecated and will be removed in a future version.
func DeprecatedDetermineError(err error) error {
	if err == nil {
		return nil
	}

	// try to re-use codes from error
	var coder Coder
	if errors.As(err, &coder) {
		return err
	}

	codes := DeprecatedDetermineErrorCodes(err)
	if len(codes) == 0 {
		return err
	}

	return &ErrorWithCodes{err, codes}
}

// DeprecatedDetermineErrorCodes determines error codes based on the given error.
// This function is deprecated and will be removed in a future version.
func DeprecatedDetermineErrorCodes(err error) []gardencorev1beta1.ErrorCode {
	var (
		coder   Coder
		message = err.Error()
		codes   = sets.NewString()

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

	// try to re-use codes from error
	if errors.As(err, &coder) {
		for _, code := range coder.Codes() {
			codes.Insert(string(code))
			// found codes don't need to be checked any more
			delete(knownCodes, code)
		}
	}

	// determine error codes
	for code, matchFn := range knownCodes {
		if !codes.Has(string(code)) && matchFn(message) {
			codes.Insert(string(code))
		}
	}

	// compute error code list based on code string set
	var out []gardencorev1beta1.ErrorCode
	for _, c := range codes.List() {
		out = append(out, gardencorev1beta1.ErrorCode(c))
	}
	return out
}

// Coder is an error that may produce a ErrorCodes visible to the outside.
type Coder interface {
	error
	Codes() []gardencorev1beta1.ErrorCode
}

// ExtractErrorCodes extracts all error codes from the given error by using utilerrors.Errors
func ExtractErrorCodes(err error) []gardencorev1beta1.ErrorCode {
	var codes []gardencorev1beta1.ErrorCode
	for _, err := range utilerrors.Errors(err) {
		var coder Coder
		if errors.As(err, &coder) {
			codes = append(codes, coder.Codes()...)
		}
	}
	return codes
}

var _ error = (*MultiErrorWithCodes)(nil)

// MultiErrorWithCodes is a struct that contains multiple errors and ErrorCodes.
type MultiErrorWithCodes struct {
	errors      []error
	errorFormat func(errs []error) string

	errorCodeStr sets.String
	codes        []gardencorev1beta1.ErrorCode
}

// NewMultiErrorWithCodes returns a new instance of `MultiErrorWithCodes`.
func NewMultiErrorWithCodes(errorFormat func(errs []error) string) *MultiErrorWithCodes {
	return &MultiErrorWithCodes{
		errorFormat:  errorFormat,
		errorCodeStr: sets.NewString(),
	}
}

// Append appends the given error to the `MultiErrorWithCodes`.
func (m *MultiErrorWithCodes) Append(err error) {
	for _, code := range ExtractErrorCodes(err) {
		if m.errorCodeStr.Has(string(code)) {
			continue
		}
		m.errorCodeStr.Insert(string(code))
		m.codes = append(m.codes, code)
	}

	m.errors = append(m.errors, err)
}

// Codes returns all underlying `gardencorev1beta1.ErrorCode` codes.
func (m *MultiErrorWithCodes) Codes() []gardencorev1beta1.ErrorCode {
	if m.codes == nil {
		return nil
	}

	cp := make([]gardencorev1beta1.ErrorCode, len(m.codes))
	copy(cp, m.codes)
	return cp
}

// ErrorOrNil returns nil if no underlying errors are given.
func (m *MultiErrorWithCodes) ErrorOrNil() error {
	if len(m.errors) == 0 {
		return nil
	}
	return m
}

// Error implements the error interface.
func (m *MultiErrorWithCodes) Error() string {
	return m.errorFormat(m.errors)
}

// FormatLastErrDescription formats the error message string for the last occurred error.
func FormatLastErrDescription(err error) string {
	errString := err.Error()
	if len(errString) > 0 {
		errString = strings.ToUpper(string(errString[0])) + errString[1:]
	}
	return errString
}

// WrappedLastErrors is a structure which contains the general description of the lastErrors which occurred and an array of all lastErrors
type WrappedLastErrors struct {
	Description string
	LastErrors  []gardencorev1beta1.LastError
}

// DeprecatedNewWrappedLastErrors returns a list of last errors.
func DeprecatedNewWrappedLastErrors(description string, err error) *WrappedLastErrors {
	var lastErrors []gardencorev1beta1.LastError

	for _, partError := range utilerrors.Errors(err) {
		lastErrors = append(lastErrors, *LastErrorWithTaskID(
			partError.Error(),
			utilerrors.GetID(partError),
			DeprecatedDetermineErrorCodes(partError)...))
	}

	return &WrappedLastErrors{
		Description: description,
		LastErrors:  lastErrors,
	}
}

// NewWrappedLastErrors returns a list of last errors.
func NewWrappedLastErrors(description string, err error) *WrappedLastErrors {
	var lastErrors []gardencorev1beta1.LastError

	for _, partError := range utilerrors.Errors(err) {
		lastErrors = append(lastErrors, *LastErrorWithTaskID(
			partError.Error(),
			utilerrors.GetID(partError),
			ExtractErrorCodes(partError)...))
	}

	return &WrappedLastErrors{
		Description: description,
		LastErrors:  lastErrors,
	}
}

// LastError creates a new LastError with the given description, optional codes and sets timestamp when the error is lastly observed.
func LastError(description string, codes ...gardencorev1beta1.ErrorCode) *gardencorev1beta1.LastError {
	return &gardencorev1beta1.LastError{
		Description: description,
		Codes:       codes,
		LastUpdateTime: &metav1.Time{
			Time: time.Now(),
		},
	}
}

// LastErrorWithTaskID creates a new LastError with the given description, the ID of the task when the error occurred, optional codes and sets timestamp when the error is lastly observed.
func LastErrorWithTaskID(description string, taskID string, codes ...gardencorev1beta1.ErrorCode) *gardencorev1beta1.LastError {
	return &gardencorev1beta1.LastError{
		Description: description,
		Codes:       codes,
		TaskID:      &taskID,
		LastUpdateTime: &metav1.Time{
			Time: time.Now(),
		},
	}
}

// HasNonRetryableErrorCode returns true if at least one of given list of last errors has at least one error code that
// indicates that an automatic retry would not help fixing the problem.
func HasNonRetryableErrorCode(lastErrors ...gardencorev1beta1.LastError) bool {
	for _, lastError := range lastErrors {
		for _, code := range lastError.Codes {
			if code == gardencorev1beta1.ErrorInfraUnauthenticated ||
				code == gardencorev1beta1.ErrorInfraUnauthorized ||
				code == gardencorev1beta1.ErrorInfraDependencies ||
				code == gardencorev1beta1.ErrorInfraQuotaExceeded ||
				code == gardencorev1beta1.ErrorInfraRateLimitsExceeded ||
				code == gardencorev1beta1.ErrorConfigurationProblem ||
				code == gardencorev1beta1.ErrorProblematicWebhook {
				return true
			}
		}
	}
	return false
}

// HasErrorCode checks whether at least one LastError from the given slice of LastErrors <lastErrors>
// contains the given ErrorCode <code>.
func HasErrorCode(lastErrors []gardencorev1beta1.LastError, code gardencorev1beta1.ErrorCode) bool {
	for _, lastError := range lastErrors {
		for _, current := range lastError.Codes {
			if current == code {
				return true
			}
		}
	}

	return false
}
