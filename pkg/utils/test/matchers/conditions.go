// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ContainCondition returns a matchers for checking whether a condition is contained.
func ContainCondition(matchers ...gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return ContainElement(And(matchers...))
}

// OfType returns a matcher for checking whether a condition has a certain type.
func OfType(conditionType gardencorev1beta1.ConditionType) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Type": Equal(conditionType),
	})
}

// WithStatus returns a matcher for checking whether a condition has a certain status.
func WithStatus(status gardencorev1beta1.ConditionStatus) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Status": Equal(status),
	})
}

// WithReason returns a matcher for checking whether a condition has a certain reason.
func WithReason(reason string) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Reason": Equal(reason),
	})
}

// WithMessage returns a matcher for checking whether a condition has a certain message.
func WithMessage(message string) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Message": ContainSubstring(message),
	})
}

// WithCodes returns a matcher for checking whether a condition contains certain error codes.
func WithCodes(codes ...gardencorev1beta1.ErrorCode) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Codes": ContainElements(codes),
	})
}

// WithMessageSubstrings returns a matcher for checking whether a condition's message contains certain substrings.
func WithMessageSubstrings(messages ...string) gomegatypes.GomegaMatcher {
	var substringMatchers = make([]gomegatypes.GomegaMatcher, 0, len(messages))
	for _, message := range messages {
		substringMatchers = append(substringMatchers, ContainSubstring(message))
	}
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Message": SatisfyAll(substringMatchers...),
	})
}
