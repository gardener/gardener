// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction"
	. "github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction/validation"
)

var _ = Describe("Validation", func() {
	Describe("#ValidateConfiguration", func() {
		var config *shoottolerationrestriction.Configuration

		BeforeEach(func() {
			config = &shoottolerationrestriction.Configuration{}
		})

		It("should allow empty tolerations", func() {
			errorList := ValidateConfiguration(config)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow valid tolerations", func() {
			tolerations := []core.Toleration{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			}
			config.Defaults = tolerations
			config.Whitelist = tolerations

			errorList := ValidateConfiguration(config)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid invalid tolerations", func() {
			tolerations := []core.Toleration{
				{},
				{Key: "foo"},
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
				{Key: "bar", Value: ptr.To("baz")},
			}
			config.Defaults = tolerations
			config.Whitelist = tolerations

			errorList := ValidateConfiguration(config)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("defaults[0].key"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("defaults[2]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("defaults[4]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("whitelist[0].key"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("whitelist[2]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("whitelist[4]"),
				})),
			))
		})
	})
})
