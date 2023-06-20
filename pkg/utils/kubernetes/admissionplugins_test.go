// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("kubernetes", func() {
	Describe("#GetAdmissionPluginsForVersion", func() {
		It("should return the correct list for 1.22", func() {
			expected := []string{
				"Priority",
				"NamespaceLifecycle",
				"LimitRanger",
				"PodSecurityPolicy",
				"ServiceAccount",
				"NodeRestriction",
				"DefaultStorageClass",
				"DefaultTolerationSeconds",
				"ResourceQuota",
				"StorageObjectInUseProtection",
				"MutatingAdmissionWebhook",
				"ValidatingAdmissionWebhook",
			}

			plugins := GetAdmissionPluginsForVersion("1.22.0")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the correct list for 1.23 upto 1.25", func() {
			expected := []string{"Priority",
				"NamespaceLifecycle",
				"LimitRanger",
				"PodSecurityPolicy",
				"PodSecurity",
				"ServiceAccount",
				"NodeRestriction",
				"DefaultStorageClass",
				"DefaultTolerationSeconds",
				"ResourceQuota",
				"StorageObjectInUseProtection",
				"MutatingAdmissionWebhook",
				"ValidatingAdmissionWebhook",
			}

			plugins := GetAdmissionPluginsForVersion("1.24.0")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the correct list for 1.25 or higher", func() {
			expected := []string{"Priority",
				"NamespaceLifecycle",
				"LimitRanger",
				"PodSecurity",
				"ServiceAccount",
				"NodeRestriction",
				"DefaultStorageClass",
				"DefaultTolerationSeconds",
				"ResourceQuota",
				"StorageObjectInUseProtection",
				"MutatingAdmissionWebhook",
				"ValidatingAdmissionWebhook",
			}

			plugins := GetAdmissionPluginsForVersion("1.26.0")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return copy of the default plugins", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("1.22.0")
			plugins2 := GetAdmissionPluginsForVersion("1.22.0")
			plugins[0].Name = "MissingPlugin"

			for _, plugin := range expected {
				Expect(plugins2).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})
	})
})
