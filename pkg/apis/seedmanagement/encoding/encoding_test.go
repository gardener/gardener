// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package encoding_test

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

var _ = Describe("Encoding", func() {
	var (
		config = &gardenletconfigv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
		}
	)

	Describe("#DecodeGardenletConfiguration", func() {
		It("should decode the raw config to a GardenletConfiguration without defaults", func() {
			result, err := DecodeGardenletConfiguration(&runtime.RawExtension{Raw: encode(config)}, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(config))
		})

		It("should decode the raw config to a GardenletConfiguration with defaults", func() {
			configWithDefaults := config.DeepCopy()
			gardenletconfigv1alpha1.SetObjectDefaults_GardenletConfiguration(configWithDefaults)

			result, err := DecodeGardenletConfiguration(&runtime.RawExtension{Raw: encode(config)}, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(configWithDefaults))
		})

		It("should return the raw config object if it's already set", func() {
			result, err := DecodeGardenletConfiguration(&runtime.RawExtension{Object: config}, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(config))
		})
	})

	Describe("#DecodeGardenletConfigurationFromBytes", func() {
		It("should decode the byte slice into a GardenletConfiguration", func() {
			result, err := DecodeGardenletConfigurationFromBytes(encode(config), false)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(config))
		})
	})

	Describe("#EncodeGardenletConfiguration", func() {
		It("should encode the GardenletConfiguration into a raw extension", func() {
			result, err := EncodeGardenletConfiguration(config)

			Expect(err).NotTo(HaveOccurred())
			// Test for equality doesn't work since there is one extra byte at the end of result compared to json.Marshal
			Expect(strings.HasPrefix(string(result.Raw), string(encode(config)))).To(BeTrue())
			Expect(result.Object).To(Equal(config))
		})
	})

	Describe("#EncodeGardenletConfigurationToBytes", func() {
		It("should encode the GardenletConfiguration into a byte slice", func() {
			result, err := EncodeGardenletConfigurationToBytes(config)

			Expect(err).NotTo(HaveOccurred())
			// Test for equality doesn't work since there is one extra byte at the end of result compared to json.Marshal
			Expect(strings.HasPrefix(string(result), string(encode(config)))).To(BeTrue())
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
