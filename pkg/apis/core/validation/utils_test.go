// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func makeDurationPointer(d time.Duration) *metav1.Duration {
	return &metav1.Duration{Duration: d}
}

var _ = Describe("Utils tests", func() {
	Describe("#ValidateFailureToleranceTypeValue", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("spec", "highAvailability", "failureTolerance", "type")
		})

		It("highAvailability is set to failureTolerance of node", func() {
			errorList := validation.ValidateFailureToleranceTypeValue(core.FailureToleranceTypeNode, fldPath)
			Expect(errorList).To(HaveLen(0))
		})

		It("highAvailability is set to failureTolerance of zone", func() {
			errorList := validation.ValidateFailureToleranceTypeValue(core.FailureToleranceTypeZone, fldPath)
			Expect(errorList).To(HaveLen(0))
		})

		It("highAvailability is set to an unsupported value", func() {
			errorList := validation.ValidateFailureToleranceTypeValue("region", fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal(fldPath.String()),
				}))))
		})
	})
})
