// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("helper", func() {
	Describe("#SeedNameFromSeedConfig", func() {
		It("should return an empty string", func() {
			Expect(SeedNameFromSeedConfig(nil)).To(BeEmpty())
		})

		It("should return the seed name", func() {
			seedName := "some-name"

			config := &config.SeedConfig{
				Seed: gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Name: seedName,
					},
				},
			}
			Expect(SeedNameFromSeedConfig(config)).To(Equal(seedName))
		})
	})
})
