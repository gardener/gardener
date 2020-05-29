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

	errors2 "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ErrorWithCodes contains error codes and an error message.
type ErrorWithCodes struct {
	message string
	codes   []gardencorev1beta1.ErrorCode
}

// NewErrorWithCodes creates a new error that additionally exposes the given codes via the Coder interface.
func NewErrorWithCodes(message string, codes ...gardencorev1beta1.ErrorCode) error {
	return &ErrorWithCodes{message, codes}
}

// Codes returns all error codes.
func (e *ErrorWithCodes) Codes() []gardencorev1beta1.ErrorCode {
	return e.codes
}

// Error returns the error message.
func (e *ErrorWithCodes) Error() string {
	return e.message
}

var (
	unauthorizedRegexp           = regexp.MustCompile(`(?i)(Unauthorized|InvalidClientTokenId|SignatureDoesNotMatch|Authentication failed|AuthFailure|AuthorizationFailed|invalid character|invalid_grant|invalid_client|Authorization Profile was not found|cannot fetch token|no active subscriptions|InvalidAccessKeyId|InvalidSecretAccessKey|query returned no results)`)
	quotaExceededRegexp          = regexp.MustCompile(`(?i)(LimitExceeded|Quota)`)
	insufficientPrivilegesRegexp = regexp.MustCompile(`(?i)(AccessDenied|Forbidden|deny|denied|OperationNotAllowed)`)
	dependenciesRegexp           = regexp.MustCompile(`(?i)(PendingVerification|Access Not Configured|accessNotConfigured|DependencyViolation|OptInRequired|DeleteConflict|Conflict|inactive billing state|ReadOnlyDisabledSubscription|is already being used|InUseSubnetCannotBeDeleted|VnetInUse|timeout while waiting for state to become|InvalidCidrBlock)`)
	resourcesDepletedRegexp      = regexp.MustCompile(`(?i)(not available in the current hardware cluster|InsufficientInstanceCapacity|SkuNotAvailable|ZonalAllocationFailed)`)
	configurationProblemRegexp   = regexp.MustCompile(`(?i)(AzureBastionSubnet|not supported in your requested Availability Zone|InvalidParameterValue|notFound|NetcfgInvalidSubnet|InvalidSubnet|Invalid value|KubeletHasInsufficientMemory|KubeletHasDiskPressure|KubeletHasInsufficientPID|violates constraint|no attached internet gateway found|Your query returned no results|PrivateEndpointNetworkPoliciesCannotBeEnabledOnPrivateEndpointSubnet)`)
)

// DetermineError determines the Garden error code for the given error and creates a new error with the given message.
func DetermineError(err error, message string) error {
	if err == nil {
		return errors.New(message)
	}

	errMsg := message
	if errMsg == "" {
		errMsg = err.Error()
	}

	codes := DetermineErrorCodes(err)
	if codes == nil {
		return errors.New(errMsg)
	}
	return &ErrorWithCodes{errMsg, codes}
}

// DetermineErrorCodes determines error codes based on the given error.
func DetermineErrorCodes(err error) []gardencorev1beta1.ErrorCode {
	var (
		coder   Coder
		message = err.Error()
		codes   = sets.NewString()
	)

	// try to re-use codes from error
	if errors.As(err, &coder) {
		for _, code := range coder.Codes() {
			codes.Insert(string(code))
		}
	}

	// determine error codes
	if unauthorizedRegexp.MatchString(message) {
		codes.Insert(string(gardencorev1beta1.ErrorInfraUnauthorized))
	}
	if quotaExceededRegexp.MatchString(message) {
		codes.Insert(string(gardencorev1beta1.ErrorInfraQuotaExceeded))
	}
	if insufficientPrivilegesRegexp.MatchString(message) {
		codes.Insert(string(gardencorev1beta1.ErrorInfraInsufficientPrivileges))
	}
	if dependenciesRegexp.MatchString(message) {
		codes.Insert(string(gardencorev1beta1.ErrorInfraDependencies))
	}
	if resourcesDepletedRegexp.MatchString(message) {
		codes.Insert(string(gardencorev1beta1.ErrorInfraResourcesDepleted))
	}
	if configurationProblemRegexp.MatchString(message) {
		codes.Insert(string(gardencorev1beta1.ErrorConfigurationProblem))
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

// NewWrappedLastErrors returns an error
func NewWrappedLastErrors(description string, err error) *WrappedLastErrors {
	var lastErrors []gardencorev1beta1.LastError

	for _, partError := range utilerrors.Errors(err) {
		lastErrors = append(lastErrors, *LastErrorWithTaskID(
			partError.Error(),
			utilerrors.GetID(partError),
			ExtractErrorCodes(errors2.Cause(partError))...))
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
			if code == gardencorev1beta1.ErrorInfraUnauthorized || code == gardencorev1beta1.ErrorConfigurationProblem {
				return true
			}
		}
	}
	return false
}
