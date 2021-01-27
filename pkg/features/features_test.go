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

package features_test

import (
	. "github.com/gardener/gardener/pkg/features"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/component-base/featuregate"
)

var _ = Describe("Features", func() {
	var (
		fooGate = featuregate.Feature("Foo")

		emptyFeatureGates    = featuregate.NewFeatureGate()
		nonEmptyFeatureGates = featuregate.NewFeatureGate()
	)

	BeforeEach(func() {
		Expect(nonEmptyFeatureGates.Add(map[featuregate.Feature]featuregate.FeatureSpec{
			fooGate: {Default: false, PreRelease: featuregate.Alpha},
		})).To(Succeed())
	})

	DescribeTable("IsFeatureGateKnown",
		func(featureGates featuregate.FeatureGate, feature featuregate.Feature, matcher gomegatypes.GomegaMatcher) {
			Expect(IsFeatureGateKnown(featureGates, feature)).To(matcher)
		},

		Entry("known", nonEmptyFeatureGates, fooGate, BeTrue()),
		Entry("unknown", emptyFeatureGates, fooGate, BeFalse()),
	)
})
