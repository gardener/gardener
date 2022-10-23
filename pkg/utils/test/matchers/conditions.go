// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package matchers

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
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
		"Message": Equal(message),
	})
}
