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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type errorWithCode struct {
	code    gardencorev1alpha1.ErrorCode
	message string
}

// NewErrorWithCode creates a new error that additionally exposes the given code via the Coder interface.
func NewErrorWithCode(code gardencorev1alpha1.ErrorCode, message string) error {
	return &errorWithCode{code, message}
}

func (e *errorWithCode) Code() gardencorev1alpha1.ErrorCode {
	return e.code
}

func (e *errorWithCode) Error() string {
	return e.message
}

var (
	unauthorizedRegexp           = regexp.MustCompile(`(?i)(Unauthorized|InvalidClientTokenId|SignatureDoesNotMatch|Authentication failed|AuthFailure|AuthorizationFailed|invalid character|invalid_grant|invalid_client|Authorization Profile was not found|cannot fetch token|no active subscriptions|InvalidAccessKeyId|InvalidSecretAccessKey)`)
	quotaExceededRegexp          = regexp.MustCompile(`(?i)(LimitExceeded|Quota)`)
	insufficientPrivilegesRegexp = regexp.MustCompile(`(?i)(AccessDenied|Forbidden|deny|denied)`)
	dependenciesRegexp           = regexp.MustCompile(`(?i)(PendingVerification|Access Not Configured|accessNotConfigured|DependencyViolation|OptInRequired|DeleteConflict|Conflict|inactive billing state)`)
)

// DetermineError determines the Garden error code for the given error message.
func DetermineError(message string) error {
	code := determineErrorCode(message)
	if code == "" {
		return errors.New(message)
	}

	return &errorWithCode{code, message}
}

func determineErrorCode(message string) gardencorev1alpha1.ErrorCode {
	switch {
	case unauthorizedRegexp.MatchString(message):
		return gardencorev1alpha1.ErrorInfraUnauthorized
	case quotaExceededRegexp.MatchString(message):
		return gardencorev1alpha1.ErrorInfraQuotaExceeded
	case insufficientPrivilegesRegexp.MatchString(message):
		return gardencorev1alpha1.ErrorInfraInsufficientPrivileges
	case dependenciesRegexp.MatchString(message):
		return gardencorev1alpha1.ErrorInfraDependencies
	default:
		return ""
	}
}

// Coder is an error that may produce an ErrorCode visible to the outside.
type Coder interface {
	error
	Code() gardencorev1alpha1.ErrorCode
}

// ExtractErrorCodes extracts all error codes from the given error by using utils.Errors
func ExtractErrorCodes(err error) []gardencorev1alpha1.ErrorCode {
	var codes []gardencorev1alpha1.ErrorCode
	for _, err := range utils.Errors(err) {
		if coder, ok := err.(Coder); ok {
			codes = append(codes, coder.Code())
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

// LastError creates a new LastError with the given description, optional codes and sets timestamp when the error is lastly observed.
func LastError(description string, codes ...gardencorev1alpha1.ErrorCode) *gardencorev1alpha1.LastError {
	return &gardencorev1alpha1.LastError{
		Description: description,
		Codes:       codes,
		LastUpdateTime: &metav1.Time{
			Time: time.Now(),
		},
	}
}
