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

package shoot

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("gardenletAnnotations", func() {
	BeforeEach(func() {
		gardenletfeatures.RegisterFeatureGates()
	})

	DescribeTable("with different values annotation", func(added bool, version string, SNIEnabled bool) {
		Expect(gardenletfeatures.FeatureGate.SetFromMap(map[string]bool{"APIServerSNI": SNIEnabled})).NotTo(HaveOccurred())

		s := &gardencorev1beta1.Shoot{
			Status: gardencorev1beta1.ShootStatus{
				Gardener: gardencorev1beta1.Gardener{
					Version: version,
				},
			},
		}

		actualAnnotations := gardenletAnnotations(s)

		if added {
			Expect(actualAnnotations).To(HaveKeyWithValue("networking.gardener.cloud/seed-sni-enabled", "true"))
			Expect(actualAnnotations).To(HaveLen(1))
		} else {
			Expect(actualAnnotations).To(BeEmpty())
		}
	},
		Entry("should be added for SNIEnabled release 1.14.1", true, "1.14.1", true),
		Entry("should be added for SNIEnabled pre-release 1.14", true, "1.14-dev", true),
		Entry("should be added for SNIEnabled pre-release 1.14.0", true, "1.14.0-dev", true),
		Entry("should be added for SNIEnabled release 1.13.3", true, "1.13.3", true),
		Entry("should be added for SNIEnabled pre-release 1.13", true, "1.13-dev", true),
		Entry("should be added for SNIEnabled pre-release 1.13.0", true, "1.13.0-dev", true),
		Entry("should not be added for SNIEnabled release 1.12.8", false, "1.12.8", true),
		Entry("should not be added for SNIEnabled unparsable version", false, "not a semver", true),

		Entry("should not be added for SNIDisabled release 1.14.1", false, "1.14.1", false),
		Entry("should not be added for SNIDisabled pre-release 1.14", false, "1.14-dev", false),
		Entry("should not be added for SNIDisabled pre-release 1.14.0", false, "1.14.0-dev", false),
		Entry("should not be added for SNIDisabled release 1.13.3", false, "1.13.3", false),
		Entry("should not be added for SNIDisabled pre-release 1.13", false, "1.13-dev", false),
		Entry("should not be added for SNIDisabled pre-release 1.13.0", false, "1.13.0-dev", false),
		Entry("should not be added for SNIDisabled release 1.12.8", false, "1.12.8", false),
		Entry("should not be added for SNIDisabled unparsable version", false, "not a semver", false),
	)
})
