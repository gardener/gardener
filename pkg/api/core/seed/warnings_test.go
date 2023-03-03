// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/gardener/gardener/pkg/api/core/seed"
	"github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("Warnings", func() {
	Describe("#GetWarnings", func() {
		var (
			ctx  = context.TODO()
			seed *core.Seed
		)

		BeforeEach(func() {
			seed = &core.Seed{
				Spec: core.SeedSpec{
					Settings: &core.SeedSettings{
						DependencyWatchdog: &core.SeedSettingDependencyWatchdog{
							Weeder: &core.SeedSettingDependencyWatchdogWeeder{Enabled: true},
							Prober: &core.SeedSettingDependencyWatchdogProber{Enabled: true},
						},
					},
				},
			}
		})

		It("should return nil when seed is nil", func() {
			Expect(GetWarnings(ctx, nil, nil)).To(BeEmpty())
		})

		It("should return nil when seed does not have any problematic configuration", func() {
			Expect(GetWarnings(ctx, seed, nil)).To(BeEmpty())
		})

		It("should return a warning when field endpoint is set", func() {
			seed.Spec.Settings.DependencyWatchdog.Endpoint = &core.SeedSettingDependencyWatchdogEndpoint{Enabled: true}
			Expect(GetWarnings(ctx, seed, nil)).To(ContainElement(ContainSubstring("you are setting spec.settings.dependencyWatchdog.endpoint field. This field is deprecated and will be removed in future versions of Gardener. Instead, use the spec.settings.dependencyWatchdog.weeder field.")))
		})

		It("should return a warning when field probe is set", func() {
			seed.Spec.Settings.DependencyWatchdog.Probe = &core.SeedSettingDependencyWatchdogProbe{Enabled: true}
			Expect(GetWarnings(ctx, seed, nil)).To(ContainElement(ContainSubstring("you are setting spec.settings.dependencyWatchdog.probe field. This field is deprecated and will be removed in future versions of Gardener. Instead, use the spec.settings.dependencyWatchdog.prober field.")))
		})
	})
})
