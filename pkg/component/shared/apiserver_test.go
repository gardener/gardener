// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/component/shared"
)

var _ = Describe("APIServer", func() {
	Describe("#GetResourcesForEncryptionFromConfig", func() {
		It("should return nil when encryptionConfig is nil", func() {
			Expect(GetResourcesForEncryptionFromConfig(nil)).To(BeNil())
		})

		It("should return the correct list of resources when encryptionConfig is not nil", func() {
			encryptionConfig := &gardencorev1beta1.EncryptionConfig{
				Resources: []string{"deployments.apps", "fancyresource.customoperator.io", "configmaps", "daemonsets.apps"},
			}

			Expect(GetResourcesForEncryptionFromConfig(encryptionConfig)).To(ConsistOf(
				"deployments.apps",
				"fancyresource.customoperator.io",
				"configmaps",
				"daemonsets.apps",
			))
		})
	})

	Describe("#GetModifiedResources", func() {
		It("should return the correct list of modified resources", func() {
			oldResources := []string{
				"secrets",
				"configmaps",
				"foo.bar",
				"custom.operator.io",
			}

			newResources := []string{
				"secrets",
				"custom.operator.io",
				"zig.zag",
				"deployments.apps",
			}

			Expect(GetModifiedResources(oldResources, newResources)).To(ConsistOf(
				"configmaps",
				"deployments.apps",
				"foo.bar",
				"zig.zag",
			))
		})
	})
})
