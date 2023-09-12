// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/registry/core/seed"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.TODO()
		strategy = Strategy{}
	)

	Describe("#PrepareForUpdate", func() {
		var oldSeed, newSeed *core.Seed

		BeforeEach(func() {
			oldSeed = &core.Seed{}
			newSeed = &core.Seed{}
		})

		It("should preserve the status", func() {
			newSeed.Status = core.SeedStatus{KubernetesVersion: pointer.String("1.2.3")}
			strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
			Expect(newSeed.Status).To(Equal(oldSeed.Status))
		})

		Context("generation increment", func() {
			It("should not bump the generation if nothing changed", func() {
				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
			})

			It("should bump the generation if the spec changed", func() {
				newSeed.Spec.Provider.Type = "foo"

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation if the deletionTimestamp was set", func() {
				now := metav1.Now()
				newSeed.DeletionTimestamp = &now

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should not bump the generation if the deletionTimestamp was already set", func() {
				now := metav1.Now()
				oldSeed.DeletionTimestamp = &now
				newSeed.DeletionTimestamp = &now

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
			})

			It("should bump the generation if the operation annotation was set to renew-garden-access-secrets", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-garden-access-secrets")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation if the operation annotation was set to renew-kubeconfig", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-kubeconfig")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation and remove the annotation if the operation annotation was set to reconcile", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "reconcile")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
				Expect(newSeed.Annotations).NotTo(ContainElement("gardener.cloud/operation"))
			})

			It("should not bump the generation if the operation annotation didn't change", func() {
				metav1.SetMetaDataAnnotation(&oldSeed.ObjectMeta, "gardener.cloud/operation", "renew-garden-access-secrets")
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-garden-access-secrets")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
			})
		})

		Context("syncDependencyWatchdogSettings", func() {
			BeforeEach(func() {
				oldSeed.Spec = core.SeedSpec{
					Settings: &core.SeedSettings{
						DependencyWatchdog: &core.SeedSettingDependencyWatchdog{},
					},
				}
				newSeed.Spec = core.SeedSpec{
					Settings: &core.SeedSettings{
						DependencyWatchdog: &core.SeedSettingDependencyWatchdog{},
					},
				}
			})

			It("should default new and deprecated fields for prober and weeder when both aren't specified", func() {
				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Weeder.Enabled).To(BeTrue())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Endpoint.Enabled).To(BeTrue())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Prober.Enabled).To(BeTrue())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Probe.Enabled).To(BeTrue())
			})

			It("should set new field equal to deprecated field , if new field is not set", func() {
				newSeed.Spec.Settings.DependencyWatchdog.Endpoint = &core.SeedSettingDependencyWatchdogEndpoint{Enabled: false}
				newSeed.Spec.Settings.DependencyWatchdog.Probe = &core.SeedSettingDependencyWatchdogProbe{Enabled: false}
				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Weeder.Enabled).To(BeFalse())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Endpoint.Enabled).To(BeFalse())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Prober.Enabled).To(BeFalse())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Probe.Enabled).To(BeFalse())
			})

			It("should set deprecated field equal to new field, even if just new field is set", func() {
				newSeed.Spec.Settings.DependencyWatchdog.Weeder = &core.SeedSettingDependencyWatchdogWeeder{Enabled: false}
				newSeed.Spec.Settings.DependencyWatchdog.Prober = &core.SeedSettingDependencyWatchdogProber{Enabled: false}
				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Weeder.Enabled).To(BeFalse())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Endpoint.Enabled).To(BeFalse())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Prober.Enabled).To(BeFalse())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Probe.Enabled).To(BeFalse())
			})

			It("should overwrite deprecated field with value of new field, if new field is set", func() {
				newSeed.Spec.Settings.DependencyWatchdog.Endpoint = &core.SeedSettingDependencyWatchdogEndpoint{Enabled: true}
				newSeed.Spec.Settings.DependencyWatchdog.Probe = &core.SeedSettingDependencyWatchdogProbe{Enabled: true}
				newSeed.Spec.Settings.DependencyWatchdog.Weeder = &core.SeedSettingDependencyWatchdogWeeder{Enabled: false}
				newSeed.Spec.Settings.DependencyWatchdog.Prober = &core.SeedSettingDependencyWatchdogProber{Enabled: false}
				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Weeder.Enabled).To(BeFalse())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Endpoint.Enabled).To(BeFalse())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Prober.Enabled).To(BeFalse())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Probe.Enabled).To(BeFalse())
			})

			It("should update deprecated fields with updated value of new field", func() {
				oldSeed.Spec.Settings.DependencyWatchdog.Endpoint = &core.SeedSettingDependencyWatchdogEndpoint{Enabled: false}
				oldSeed.Spec.Settings.DependencyWatchdog.Probe = &core.SeedSettingDependencyWatchdogProbe{Enabled: false}
				oldSeed.Spec.Settings.DependencyWatchdog.Weeder = &core.SeedSettingDependencyWatchdogWeeder{Enabled: false}
				oldSeed.Spec.Settings.DependencyWatchdog.Prober = &core.SeedSettingDependencyWatchdogProber{Enabled: false}

				newSeed.Spec.Settings.DependencyWatchdog.Endpoint = &core.SeedSettingDependencyWatchdogEndpoint{Enabled: false}
				newSeed.Spec.Settings.DependencyWatchdog.Probe = &core.SeedSettingDependencyWatchdogProbe{Enabled: false}
				newSeed.Spec.Settings.DependencyWatchdog.Weeder = &core.SeedSettingDependencyWatchdogWeeder{Enabled: true}
				newSeed.Spec.Settings.DependencyWatchdog.Prober = &core.SeedSettingDependencyWatchdogProber{Enabled: true}

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Weeder.Enabled).To(BeTrue())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Endpoint.Enabled).To(BeTrue())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Prober.Enabled).To(BeTrue())
				Expect(newSeed.Spec.Settings.DependencyWatchdog.Probe.Enabled).To(BeTrue())
			})
		})
	})
})
