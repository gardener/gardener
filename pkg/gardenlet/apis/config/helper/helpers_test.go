// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

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

	Describe("#StaleExtensionHealthChecksThreshold", func() {
		It("should return nil when the config is nil", func() {
			Expect(StaleExtensionHealthChecksThreshold(nil)).To(BeNil())
		})

		It("should return nil when the check is not enabled", func() {
			threshold := &metav1.Duration{Duration: time.Minute}
			c := &config.StaleExtensionHealthChecks{
				Enabled:   false,
				Threshold: threshold,
			}
			Expect(StaleExtensionHealthChecksThreshold(c)).To(BeNil())
		})

		It("should return the threshold", func() {
			threshold := &metav1.Duration{Duration: time.Minute}
			c := &config.StaleExtensionHealthChecks{
				Enabled:   true,
				Threshold: threshold,
			}
			Expect(StaleExtensionHealthChecksThreshold(c)).To(Equal(threshold))
		})
	})
})
