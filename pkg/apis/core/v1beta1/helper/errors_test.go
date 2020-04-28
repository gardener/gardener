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

var _ = Describe("helper", func() {
	Describe("errors", func() {
		Describe("#DetermineError", func() {
			DescribeTable("appropriate error should be determined",
				func(err error, msg string, expectedErr error) {
					Expect(DetermineError(err, msg)).To(Equal(expectedErr))
				},

				Entry("no error", nil, "foo", errors.New("foo")),
				Entry("no code to extract", errors.New("foo"), "", errors.New("foo")),
				Entry("unauthorized", errors.New("unauthorized"), "", NewErrorWithCode(gardencorev1beta1.ErrorInfraUnauthorized, "unauthorized")),
				Entry("unauthorized with coder", NewErrorWithCode(gardencorev1beta1.ErrorInfraUnauthorized, ""), "", NewErrorWithCode(gardencorev1beta1.ErrorInfraUnauthorized, "")),
				Entry("quota exceeded", errors.New("limitexceeded"), "", NewErrorWithCode(gardencorev1beta1.ErrorInfraQuotaExceeded, "limitexceeded")),
				Entry("quota exceeded with coder", NewErrorWithCode(gardencorev1beta1.ErrorInfraQuotaExceeded, "limitexceeded"), "", NewErrorWithCode(gardencorev1beta1.ErrorInfraQuotaExceeded, "limitexceeded")),
				Entry("insufficient privileges", errors.New("accessdenied"), "", NewErrorWithCode(gardencorev1beta1.ErrorInfraInsufficientPrivileges, "accessdenied")),
				Entry("insufficient privileges with coder", NewErrorWithCode(gardencorev1beta1.ErrorInfraInsufficientPrivileges, "accessdenied"), "", NewErrorWithCode(gardencorev1beta1.ErrorInfraInsufficientPrivileges, "accessdenied")),
				Entry("infrastructure dependencies", errors.New("pendingverification"), "", NewErrorWithCode(gardencorev1beta1.ErrorInfraDependencies, "pendingverification")),
				Entry("infrastructure dependencies with coder", NewErrorWithCode(gardencorev1beta1.ErrorInfraDependencies, "pendingverification"), "error occurred: pendingverification", NewErrorWithCode(gardencorev1beta1.ErrorInfraDependencies, "error occurred: pendingverification")),
				Entry("resources depleted", errors.New("not available in the current hardware cluster"), "error occurred: not available in the current hardware cluster", NewErrorWithCode(gardencorev1beta1.ErrorInfraResourcesDepleted, "error occurred: not available in the current hardware cluster")),
				Entry("resources depleted with coder", NewErrorWithCode(gardencorev1beta1.ErrorInfraResourcesDepleted, "not available in the current hardware cluster"), "error occurred: not available in the current hardware cluster", NewErrorWithCode(gardencorev1beta1.ErrorInfraResourcesDepleted, "error occurred: not available in the current hardware cluster")),
				Entry("configuration problem", errors.New("InvalidParameterValue"), "error occurred: InvalidParameterValue", NewErrorWithCode(gardencorev1beta1.ErrorConfigurationProblem, "error occurred: InvalidParameterValue")),
				Entry("configuration problem with coder", NewErrorWithCode(gardencorev1beta1.ErrorConfigurationProblem, "InvalidParameterValue"), "error occurred: InvalidParameterValue", NewErrorWithCode(gardencorev1beta1.ErrorConfigurationProblem, "error occurred: InvalidParameterValue")),
			)
		})

		Describe("#ExtractErrorCodes", func() {
			DescribeTable("appropriate error code should be extracted",
				func(err error, matcher GomegaMatcher) {
					Expect(ExtractErrorCodes(err)).To(matcher)
				},

				Entry("no error given", nil, BeEmpty()),
				Entry("no code error given", errors.New("error"), BeEmpty()),
				Entry("code error given", NewErrorWithCode(gardencorev1beta1.ErrorInfraUnauthorized, ""), ConsistOf(Equal(gardencorev1beta1.ErrorInfraUnauthorized))),
				Entry("wrapped code error", fmt.Errorf("error %w", NewErrorWithCode(gardencorev1beta1.ErrorInfraUnauthorized, "")), ConsistOf(Equal(gardencorev1beta1.ErrorInfraUnauthorized))),
			)
		})
	})
})
