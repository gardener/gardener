// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package healthcheck

import (
	"fmt"
	"strings"
)

// getUnsuccessfulDetailMessage returns a message depending on the number of
// unsuccessful and pending checks
func getUnsuccessfulDetailMessage(unsuccessfulChecks, progressingChecks int, details string) string {
	if progressingChecks > 0 && unsuccessfulChecks > 0 {
		return fmt.Sprintf("%d failing and %d progressing %s: %s", unsuccessfulChecks, progressingChecks, getSingularOrPlural("check", progressingChecks), details)
	}

	return details
}

// getSingularOrPlural returns the given verb in either singular or plural
func getSingularOrPlural(verb string, count int) string {
	if count > 1 {
		return fmt.Sprintf("%ss", verb)
	}
	return verb
}

// appendUnsuccessfulChecksDetails appends a formatted detail message to the given string builder
func (h *checkResultForConditionType) appendUnsuccessfulChecksDetails(details *strings.Builder) {
	if len(h.unsuccessfulChecks) > 0 && (len(h.progressingChecks) != 0 || len(h.failedChecks) != 0) {
		details.WriteString(fmt.Sprintf("Failed %s: ", getSingularOrPlural("check", len(h.unsuccessfulChecks))))
	}

	if len(h.unsuccessfulChecks) == 1 {
		details.WriteString(fmt.Sprintf("%s ", ensureTrailingDot(h.unsuccessfulChecks[0].detail)))
		return
	}

	for index, check := range h.unsuccessfulChecks {
		details.WriteString(fmt.Sprintf("%d) %s ", index+1, ensureTrailingDot(check.detail)))
	}
}

// appendProgressingChecksDetails appends a formatted detail message to the given string builder
func (h *checkResultForConditionType) appendProgressingChecksDetails(details *strings.Builder) {
	if len(h.progressingChecks) > 0 && (len(h.unsuccessfulChecks) != 0 || len(h.failedChecks) != 0) {
		details.WriteString(fmt.Sprintf("Progressing %s: ", getSingularOrPlural("check", len(h.progressingChecks))))
	}

	if len(h.progressingChecks) == 1 {
		details.WriteString(fmt.Sprintf("%s ", ensureTrailingDot(h.progressingChecks[0].detail)))
		return
	}

	for index, check := range h.progressingChecks {
		details.WriteString(fmt.Sprintf("%d) %s ", index+1, ensureTrailingDot(check.detail)))
	}
}

// appendFailedChecksDetails appends a formatted detail message to the given string builder
func (h *checkResultForConditionType) appendFailedChecksDetails(details *strings.Builder) {
	if len(h.failedChecks) > 0 && (len(h.unsuccessfulChecks) != 0 || len(h.progressingChecks) != 0) {
		details.WriteString(fmt.Sprintf("Unable to execute %s: ", getSingularOrPlural("check", len(h.failedChecks))))
	}

	if len(h.failedChecks) == 1 {
		details.WriteString(fmt.Sprintf("%s ", ensureTrailingDot(h.failedChecks[0].Error())))
		return
	}

	for index, check := range h.failedChecks {
		details.WriteString(fmt.Sprintf("%d) %s ", index+1, ensureTrailingDot(check.Error())))
	}
}

// ensureTrailingDot adds a trailing dot if it does not exist
func ensureTrailingDot(details string) string {
	if !strings.HasSuffix(details, ".") {
		return fmt.Sprintf("%s.", details)
	}
	return details
}

// trimTrailingWhitespace removes a trailing whitespace character
func trimTrailingWhitespace(details string) string {
	return strings.TrimSuffix(details, " ")
}
