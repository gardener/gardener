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

package helper_test

import (
	"encoding/json"

	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Helper", func() {
	var (
		config  = &config.GardenletConfiguration{}
		configx = &configv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: configv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
		}
	)

	Describe("#DecodeGardenletConfiguration", func() {
		It("should decode the raw config to an internal GardenletConfiguration version without defaults", func() {
			result, err := DecodeGardenletConfiguration(&runtime.RawExtension{Raw: encode(configx)}, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(config))
		})

		It("should decode the raw config to an internal GardenletConfiguration version with defaults", func() {
			configxWithDefaults := configx.DeepCopy()
			configv1alpha1.SetObjectDefaults_GardenletConfiguration(configxWithDefaults)
			configWithDefaults, err := confighelper.ConvertGardenletConfiguration(configxWithDefaults)
			Expect(err).ToNot(HaveOccurred())

			result, err := DecodeGardenletConfiguration(&runtime.RawExtension{Raw: encode(configx)}, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(configWithDefaults))
		})

		It("should return the raw config object if it's already set", func() {
			result, err := DecodeGardenletConfiguration(&runtime.RawExtension{Object: configx}, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(config))
		})
	})

	Describe("#DecodeGardenletConfigurationExternal", func() {
		It("should decode the raw config to an external GardenletConfiguration version without defaults", func() {
			result, err := DecodeGardenletConfigurationExternal(&runtime.RawExtension{Raw: encode(configx)}, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(configx))
		})

		It("should decode the raw config to an external GardenletConfiguration version with defaults", func() {
			configxWithDefaults := configx.DeepCopy()
			configv1alpha1.SetObjectDefaults_GardenletConfiguration(configxWithDefaults)

			result, err := DecodeGardenletConfigurationExternal(&runtime.RawExtension{Raw: encode(configx)}, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(configxWithDefaults))
		})

		It("should return the raw config object if it's already set", func() {
			result, err := DecodeGardenletConfigurationExternal(&runtime.RawExtension{Object: configx}, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(configx))
		})
	})

	Describe("#GetBootstrap", func() {
		It("should return the correct Bootstrap value", func() {
			Expect(GetBootstrap(bootstrapPtr(seedmanagement.BootstrapToken))).To(Equal(seedmanagement.BootstrapToken))
			Expect(GetBootstrap(bootstrapPtr(seedmanagement.BootstrapServiceAccount))).To(Equal(seedmanagement.BootstrapServiceAccount))
			Expect(GetBootstrap(bootstrapPtr(seedmanagement.BootstrapNone))).To(Equal(seedmanagement.BootstrapNone))
			Expect(GetBootstrap(nil)).To(Equal(seedmanagement.BootstrapNone))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func bootstrapPtr(v seedmanagement.Bootstrap) *seedmanagement.Bootstrap { return &v }
