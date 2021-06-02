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

package kubernetes_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("featuregates", func() {
	DescribeTable("#IsFeatureGateSupported",
		func(featureGate, version string, supported, success bool) {
			result, err := kubernetes.IsFeatureGateSupported(featureGate, version)
			if success {
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Equal(supported))
			} else {
				Expect(err).To(HaveOccurred())
			}
		},

		Entry("AnyVolumeDataSource is supported in 1.18.14", "AnyVolumeDataSource", "1.18.14", true, true),                // MinVersion: 1.18
		Entry("CSIStorageCapacity is not supported in 1.18.14", "CSIStorageCapacity", "1.18.14", false, true),             // MinVersion: 1.19
		Entry("CustomResourceValidation is not supported in 1.18.14", "CustomResourceValidation", "1.18.14", false, true), // MaxVersion: 1.18
		Entry("DefaultIngressClass is supported in 1.18.14", "DefaultIngressClass", "1.18.14", true, true),                // MinVersion: 1.18, MaxVersion: 1.20
		Entry("DynamicAuditing is supported in 1.18.14", "DynamicAuditing", "1.18.14", true, true),                        // MaxVersion: 1.19
		Entry("Foo is unknown in 1.18.14", "Foo", "1.18.14", false, false),                                                // Unknown

		Entry("AnyVolumeDataSource is supported in 1.19.10", "AnyVolumeDataSource", "1.19.10", true, true),                // MinVersion: 1.18
		Entry("CSIStorageCapacity is supported in 1.19.10", "CSIStorageCapacity", "1.19.10", true, true),                  // MinVersion: 1.19
		Entry("CustomResourceValidation is not supported in 1.19.10", "CustomResourceValidation", "1.19.10", false, true), // MaxVersion: 1.18
		Entry("DefaultIngressClass is supported in 1.19.10", "DefaultIngressClass", "1.19.10", true, true),                // MinVersion: 1.18, MaxVersion: 1.20
		Entry("DynamicAuditing is not supported in 1.19.10", "DynamicAuditing", "1.19.10", false, true),                   // MaxVersion: 1.19
		Entry("Foo is unknown in 1.19.10", "Foo", "1.19.10", false, false),                                                // Unknown
	)
})
