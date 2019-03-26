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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("helper", func() {
	Describe("errors", func() {
		Describe("#DetermineError", func() {
			DescribeTable("appropriate error should be determined",
				func(msg string, expectedErr error) {
					Expect(DetermineError(msg)).To(Equal(expectedErr))
				},

				Entry("no code to extract", "foo", errors.New("foo")),
				Entry("unauthorized", "unauthorized", NewErrorWithCode(gardencorev1alpha1.ErrorInfraUnauthorized, "unauthorized")),
				Entry("quota exceeded", "limitexceeded", NewErrorWithCode(gardencorev1alpha1.ErrorInfraQuotaExceeded, "limitexceeded")),
				Entry("insufficient privileges", "accessdenied", NewErrorWithCode(gardencorev1alpha1.ErrorInfraInsufficientPrivileges, "accessdenied")),
				Entry("infrastructure dependencies", "pendingverification", NewErrorWithCode(gardencorev1alpha1.ErrorInfraDependencies, "pendingverification")),
			)
		})
	})
})
