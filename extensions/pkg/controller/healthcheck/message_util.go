// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
func (h *checkResultForConditionType) appendUnsuccessfulChecksDetails(details *strings.Builder) error {
	if len(h.unsuccessfulChecks) > 0 && (len(h.progressingChecks) != 0 || len(h.failedChecks) != 0) {
		if _, err := fmt.Fprintf(details, "Failed %s: ", getSingularOrPlural(len(h.unsuccessfulChecks))); err != nil {
			return err
		}
	}

	if len(h.unsuccessfulChecks) == 1 {
		if _, err := fmt.Fprintf(details, "%s ", ensureTrailingDot(h.unsuccessfulChecks[0].detail)); err != nil {
			return err
		}
		return nil
	}

	for index, check := range h.unsuccessfulChecks {
		if _, err := fmt.Fprintf(details, "%d) %s ", index+1, ensureTrailingDot(check.detail)); err != nil {
			return err
		}
	}

	return nil
}

// appendProgressingChecksDetails appends a formatted detail message to the given string builder
func (h *checkResultForConditionType) appendProgressingChecksDetails(details *strings.Builder) error {
	if len(h.progressingChecks) > 0 && (len(h.unsuccessfulChecks) != 0 || len(h.failedChecks) != 0) {
		if _, err := fmt.Fprintf(details, "Progressing %s: ", getSingularOrPlural(len(h.progressingChecks))); err != nil {
			return err
		}
	}

	if len(h.progressingChecks) == 1 {
		if _, err := fmt.Fprintf(details, "%s ", ensureTrailingDot(h.progressingChecks[0].detail)); err != nil {
			return err
		}
		return nil
	}

	for index, check := range h.progressingChecks {
		if _, err := fmt.Fprintf(details, "%d) %s ", index+1, ensureTrailingDot(check.detail)); err != nil {
			return err
		}
	}

	return nil
}

// appendFailedChecksDetails appends a formatted detail message to the given string builder
func (h *checkResultForConditionType) appendFailedChecksDetails(details *strings.Builder) error {
	if len(h.failedChecks) > 0 && (len(h.unsuccessfulChecks) != 0 || len(h.progressingChecks) != 0) {
		if _, err := fmt.Fprintf(details, "Unable to execute %s: ", getSingularOrPlural(len(h.failedChecks))); err != nil {
			return err
		}
	}

	if len(h.failedChecks) == 1 {
		if _, err := fmt.Fprintf(details, "%s ", ensureTrailingDot(h.failedChecks[0].Error())); err != nil {
			return err
		}
		return nil
	}

	for index, check := range h.failedChecks {
		if _, err := fmt.Fprintf(details, "%d) %s ", index+1, ensureTrailingDot(check.Error())); err != nil {
			return err
		}
	}

	return nil
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
