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

package helper_test

import (
	"encoding/json"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
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

		seed  = &gardencore.Seed{}
		seedx = &gardencorev1beta1.Seed{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Seed",
			},
		}

		rawConfig = &runtime.RawExtension{Raw: encode(configx)}
	)

	Describe("#DecodeGardenletConfig", func() {
		It("should decode the raw config to an internal GardenletConfiguration version without defaults", func() {
			result, err := DecodeGardenletConfig(rawConfig, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(config))
		})
	})

	Describe("#DecodeGardenletConfigExternal", func() {
		It("should decode the raw config to an external GardenletConfiguration version without defaults", func() {
			result, err := DecodeGardenletConfigExternal(rawConfig, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(configx))
		})

		It("should decode the raw config to an external GardenletConfiguration version with defaults", func() {
			configxWithDefaults := configx.DeepCopy()
			configv1alpha1.SetObjectDefaults_GardenletConfiguration(configxWithDefaults)

			result, err := DecodeGardenletConfigExternal(rawConfig, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(configxWithDefaults))
		})
	})

	Describe("#ConvertGardenletConfigExternal", func() {
		It("should convert the internal GardenletConfiguration version to an external one", func() {
			result, err := ConvertGardenletConfigExternal(config)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(configx))
		})
	})

	Describe("#ConvertSeed", func() {
		It("should convert the external Seed version to an internal one", func() {
			result, err := ConvertSeed(seedx)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(seed))
		})
	})

	Describe("#ConvertSeedExternal", func() {
		It("should convert the internal Seed version to an external one", func() {
			result, err := ConvertSeedExternal(seed)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(seedx))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
