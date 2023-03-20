// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/component-base/featuregate"

	. "github.com/gardener/gardener/pkg/features"
)

var _ = Describe("Features", func() {
	Describe("#GetFeatures", func() {
		It("should return the spec for the given feature gate", func() {
			Expect(GetFeatures("HVPA", "HVPAForShootedSeed", "Foo")).To(Equal(map[featuregate.Feature]featuregate.FeatureSpec{
				HVPA:               {Default: false, PreRelease: featuregate.Alpha},
				HVPAForShootedSeed: {Default: false, PreRelease: featuregate.Alpha},
			}))
		})
	})
})
