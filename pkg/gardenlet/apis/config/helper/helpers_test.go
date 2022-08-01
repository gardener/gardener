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

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("helper", func() {
	Describe("#SeedNameFromSeedConfig", func() {
		It("should return an empty string", func() {
			Expect(SeedNameFromSeedConfig(nil)).To(BeEmpty())
		})

		It("should return the seed name", func() {
			seedName := "some-name"

			config := &config.SeedConfig{
				SeedTemplate: gardencore.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: seedName,
					},
				},
			}
			Expect(SeedNameFromSeedConfig(config)).To(Equal(seedName))
		})
	})

	Describe("#OwnerChecksEnabledInSeedConfig", func() {
		It("should return true with nil config", func() {
			Expect(OwnerChecksEnabledInSeedConfig(nil)).To(BeTrue())
		})

		It("should return false if owner checks are disabled", func() {
			config := &config.SeedConfig{
				SeedTemplate: gardencore.SeedTemplate{
					Spec: gardencore.SeedSpec{
						Settings: &gardencore.SeedSettings{
							OwnerChecks: &gardencore.SeedSettingOwnerChecks{
								Enabled: false,
							},
						},
					},
				},
			}
			Expect(OwnerChecksEnabledInSeedConfig(config)).To(BeFalse())
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

	Describe("#ConvertGardenletConfiguration", func() {
		It("should convert the external GardenletConfiguration version to an internal one", func() {
			result, err := ConvertGardenletConfiguration(&configv1alpha1.GardenletConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: configv1alpha1.SchemeGroupVersion.String(),
					Kind:       "GardenletConfiguration",
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&config.GardenletConfiguration{}))
		})
	})

	Describe("#ConvertGardenletConfigurationExternal", func() {
		It("should convert the internal GardenletConfiguration version to an external one", func() {
			result, err := ConvertGardenletConfigurationExternal(&config.GardenletConfiguration{})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&configv1alpha1.GardenletConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: configv1alpha1.SchemeGroupVersion.String(),
					Kind:       "GardenletConfiguration",
				},
			}))
		})
	})

	Describe("#IsMonitoringEnabled", func() {
		It("should return false when Monitoring.Shoot.Enabled is false", func() {
			gardenletConfig := &config.GardenletConfiguration{
				Monitoring: &config.MonitoringConfig{
					Shoot: &config.ShootMonitoringConfig{
						Enabled: pointer.Bool(false),
					},
				},
			}
			Expect(IsMonitoringEnabled(gardenletConfig)).To(BeFalse())
		})

		It("should return true when Monitoring.Shoot.Enabled is true", func() {
			gardenletConfig := &config.GardenletConfiguration{
				Monitoring: &config.MonitoringConfig{
					Shoot: &config.ShootMonitoringConfig{
						Enabled: pointer.Bool(true),
					},
				},
			}
			Expect(IsMonitoringEnabled(gardenletConfig)).To(BeTrue())
		})

		It("should return true when nothing is set", func() {
			gardenletConfig := &config.GardenletConfiguration{}
			Expect(IsMonitoringEnabled(gardenletConfig)).To(BeTrue())
		})

		It("should return true when Monitoring.Shoot is nil", func() {
			gardenletConfig := &config.GardenletConfiguration{
				Monitoring: &config.MonitoringConfig{Shoot: nil},
			}
			Expect(IsMonitoringEnabled(gardenletConfig)).To(BeTrue())
		})

		It("should return true when Monitoring.Shoot.Enabled is nil", func() {
			gardenletConfig := &config.GardenletConfiguration{
				Monitoring: &config.MonitoringConfig{Shoot: &config.ShootMonitoringConfig{Enabled: nil}},
			}
			Expect(IsMonitoringEnabled(gardenletConfig)).To(BeTrue())
		})
	})

	Describe("#LoggingConfiguration", func() {
		It("should return false when the GardenletConfiguration is nil", func() {
			Expect(IsLoggingEnabled(nil)).To(BeFalse())
		})

		It("should return false when the logging is nil", func() {
			gardenletConfig := &config.GardenletConfiguration{}

			Expect(IsLoggingEnabled(gardenletConfig)).To(BeFalse())
		})

		It("should return false when the logging is not enabled", func() {
			gardenletConfig := &config.GardenletConfiguration{
				Logging: &config.Logging{
					Enabled: pointer.Bool(false),
				},
			}

			Expect(IsLoggingEnabled(gardenletConfig)).To(BeFalse())
		})

		It("should return true when the logging is enabled", func() {
			gardenletConfig := &config.GardenletConfiguration{
				Logging: &config.Logging{
					Enabled: pointer.Bool(true),
				},
			}

			Expect(IsLoggingEnabled(gardenletConfig)).To(BeTrue())
		})
	})

	Describe("#LokiConfiguration", func() {
		It("should return true when the GardenletConfiguration is nil", func() {
			Expect(IsLokiEnabled(nil)).To(BeTrue())
		})

		It("should return true when the logging is nil", func() {
			gardenletConfig := &config.GardenletConfiguration{}

			Expect(IsLokiEnabled(gardenletConfig)).To(BeTrue())
		})

		It("should return false when the loki is not enabled", func() {
			gardenletConfig := &config.GardenletConfiguration{
				Logging: &config.Logging{
					Loki: &config.Loki{
						Enabled: pointer.Bool(false),
					},
				},
			}

			Expect(IsLokiEnabled(gardenletConfig)).To(BeFalse())
		})

		It("should return true when the loki is enabled", func() {
			gardenletConfig := &config.GardenletConfiguration{
				Logging: &config.Logging{
					Loki: &config.Loki{
						Enabled: pointer.Bool(true),
					},
				},
			}

			Expect(IsLokiEnabled(gardenletConfig)).To(BeTrue())
		})
	})
})
