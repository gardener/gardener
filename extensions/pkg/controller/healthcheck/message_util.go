// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"fmt"
	"strings"
)

// getUnsuccessfulDetailMessage returns a message depending on the number of
// unsuccessful and pending checks
func getUnsuccessfulDetailMessage(unsuccessfulChecks, progressingChecks int, details string) string {
	if progressingChecks > 0 && unsuccessfulChecks > 0 {
		return fmt.Sprintf("%d failing and %d progressing %s: %s", unsuccessfulChecks, progressingChecks, getSingularOrPlural(progressingChecks), details)
	}

	return details
}

// getSingularOrPlural returns the given verb in either singular or plural
func getSingularOrPlural(count int) string {
	if count > 1 {
		return "checks"
	}
	return "check"
}

// appendUnsuccessfulChecksDetails appends a formatted detail message to the given string builder
func (h *checkResultForConditionType) appendUnsuccessfulChecksDetails(details *strings.Builder) {
	if len(h.unsuccessfulChecks) > 0 && (len(h.progressingChecks) != 0 || len(h.failedChecks) != 0) {
		details.WriteString(fmt.Sprintf("Failed %s: ", getSingularOrPlural(len(h.unsuccessfulChecks))))
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
		details.WriteString(fmt.Sprintf("Progressing %s: ", getSingularOrPlural(len(h.progressingChecks))))
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
		details.WriteString(fmt.Sprintf("Unable to execute %s: ", getSingularOrPlural(len(h.failedChecks))))
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
