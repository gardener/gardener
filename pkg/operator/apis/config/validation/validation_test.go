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

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	"github.com/gardener/gardener/pkg/operator/apis/config"
	. "github.com/gardener/gardener/pkg/operator/apis/config/validation"
)

var _ = Describe("#ValidateOperatorConfiguration", func() {
	DescribeTable("Logging configuration",
		func(logLevel, logFormat string, matcher gomegatypes.GomegaMatcher) {
			conf := &config.OperatorConfiguration{
				LogLevel:  logLevel,
				LogFormat: logFormat,
			}

			errs := ValidateOperatorConfiguration(conf)
			Expect(errs).To(matcher)
		},
		Entry("should be a valid logging configuration", "debug", "json", BeEmpty()),
		Entry("should be a valid logging configuration", "info", "json", BeEmpty()),
		Entry("should be a valid logging configuration", "error", "json", BeEmpty()),
		Entry("should be a valid logging configuration", "info", "text", BeEmpty()),
		Entry("should be an invalid logging level configuration", "foo", "json",
			ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("logLevel")}))),
		),
		Entry("should be an invalid logging format configuration", "info", "foo",
			ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("logFormat")}))),
		),
	)
})
