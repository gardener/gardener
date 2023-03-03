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

package validation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
)

func makeDurationPointer(d time.Duration) *metav1.Duration {
	return &metav1.Duration{Duration: d}
}

var _ = Describe("Utils tests", func() {
	Describe("#ValidateFailureToleranceTypeValue", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("spec", "highAvailability", "failureTolerance", "type")
		})

		It("highAvailability is set to failureTolerance of node", func() {
			errorList := ValidateFailureToleranceTypeValue(core.FailureToleranceTypeNode, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("highAvailability is set to failureTolerance of zone", func() {
			errorList := ValidateFailureToleranceTypeValue(core.FailureToleranceTypeZone, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("highAvailability is set to an unsupported value", func() {
			errorList := ValidateFailureToleranceTypeValue("region", fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal(fldPath.String()),
				}))))
		})
	})

	Describe("#ValidateIPFamilies", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("ipFamilies")
		})

		It("should deny unsupported IP families", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{"foo", "bar"}, fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeNotSupported),
					"Field":    Equal(fldPath.Index(0).String()),
					"BadValue": BeEquivalentTo("foo"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeNotSupported),
					"Field":    Equal(fldPath.Index(1).String()),
					"BadValue": BeEquivalentTo("bar"),
				})),
			))
		})

		It("should deny duplicate IP families", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6, core.IPFamilyIPv4, core.IPFamilyIPv6}, fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal(fldPath.Index(2).String()),
					"BadValue": Equal(core.IPFamilyIPv4),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal(fldPath.Index(3).String()),
					"BadValue": Equal(core.IPFamilyIPv6),
				})),
			))
		})

		It("should deny dual-stack IP families", func() {
			ipFamilies := []core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6}
			errorList := ValidateIPFamilies(ipFamilies, fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(fldPath.String()),
					"BadValue": Equal(ipFamilies),
					"Detail":   Equal("dual-stack networking is not supported"),
				})),
			))
		})

		It("should allow IPv4 single-stack", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{core.IPFamilyIPv4}, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should deny IPv6 single-stack if feature gate is disabled", func() {
			ipFamilies := []core.IPFamily{core.IPFamilyIPv6}
			errorList := ValidateIPFamilies(ipFamilies, fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(fldPath.String()),
					"BadValue": Equal(ipFamilies),
					"Detail":   Equal("IPv6 single-stack networking is not supported"),
				})),
			))
		})

		It("should allow IPv6 single-stack if feature gate is enabled", func() {
			defer test.WithFeatureGate(utilfeature.DefaultMutableFeatureGate, features.IPv6SingleStack, true)()

			errorList := ValidateIPFamilies([]core.IPFamily{core.IPFamilyIPv6}, fldPath)
			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#Validate", func() {
		var seed *core.Seed

		BeforeEach(func() {
			seed = &core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-seed",
				},
				Spec: core.SeedSpec{
					Provider: core.SeedProvider{
						Type:   "test-provider",
						Region: "test-region",
					},
					Ingress: &core.Ingress{
						Domain: "someingress.example.com",
						Controller: core.IngressController{
							Kind: "nginx",
						},
					},
					DNS: core.SeedDNS{
						Provider: &core.SeedDNSProvider{
							Type: "provider",
							SecretRef: corev1.SecretReference{
								Name:      "secret",
								Namespace: "garden",
							},
						},
					},
					Networks: core.SeedNetworks{
						Pods:     "10.123.211.10/18",
						Services: "193.168.211.0/16",
					},
					Settings: &core.SeedSettings{
						DependencyWatchdog: &core.SeedSettingDependencyWatchdog{},
					},
				},
			}
		})

		It("should not allow if deprecated field is different than new field", func() {
			seed.Spec.Settings.DependencyWatchdog.Endpoint = &core.SeedSettingDependencyWatchdogEndpoint{
				Enabled: true,
			}
			seed.Spec.Settings.DependencyWatchdog.Weeder = &core.SeedSettingDependencyWatchdogWeeder{
				Enabled: false,
			}
			allErrs := ValidateSeed(seed)

			Expect(allErrs).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.settings.dependencyWatchdog"),
					"Detail": Equal(`weeder and endpoint cannot have different values`),
				}))))
		})

		It("should allow if deprecated fields are not set", func() {
			seed.Spec.Settings.DependencyWatchdog.Weeder = &core.SeedSettingDependencyWatchdogWeeder{
				Enabled: true,
			}
			seed.Spec.Settings.DependencyWatchdog.Prober = &core.SeedSettingDependencyWatchdogProber{
				Enabled: true,
			}
			allErrs := ValidateSeed(seed)

			Expect(allErrs).To(BeEmpty())
		})
	})
})
