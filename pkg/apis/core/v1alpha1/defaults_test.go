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

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
)

var _ = Describe("Defaults", func() {
	Describe("#SetObjectDefaults_Seed", func() {
		var obj *Seed

		BeforeEach(func() {
			obj = &Seed{}
		})

		It("should default the seed settings (w/o taints)", func() {
			SetObjectDefaults_Seed(obj)

			Expect(obj.Spec.Settings.DependencyWatchdog).NotTo(BeNil())
			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.Scheduling.Visible).To(BeTrue())
			Expect(obj.Spec.Settings.VerticalPodAutoscaler.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.OwnerChecks.Enabled).To(BeFalse())
			Expect(obj.Spec.Settings.TopologyAwareRouting.Enabled).To(BeFalse())
		})

		It("should allow taints that were not allowed in version v1.12", func() {
			taints := []SeedTaint{
				{Key: "seed.gardener.cloud/disable-capacity-reservation"},
				{Key: "seed.gardener.cloud/disable-dns"},
				{Key: "seed.gardener.cloud/invisible"},
			}
			obj.Spec.Taints = taints

			SetObjectDefaults_Seed(obj)

			Expect(obj.Spec.Settings.DependencyWatchdog).NotTo(BeNil())
			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.Scheduling.Visible).To(BeTrue())
			Expect(obj.Spec.Settings.VerticalPodAutoscaler.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.OwnerChecks.Enabled).To(BeFalse())
			Expect(obj.Spec.Settings.TopologyAwareRouting.Enabled).To(BeFalse())
			Expect(obj.Spec.Taints).To(HaveLen(3))
			Expect(obj.Spec.Taints).To(Equal(taints))
		})

		It("should not default the seed settings because they were provided", func() {
			var (
				dwdWeederEnabled          = false
				dwdProberEnabled          = false
				topologyAwareRouting      = true
				excessCapacityReservation = false
				scheduling                = true
				vpaEnabled                = false
				ownerChecks               = true
			)

			obj.Spec.Settings = &SeedSettings{
				DependencyWatchdog: &SeedSettingDependencyWatchdog{
					Weeder: &SeedSettingDependencyWatchdogWeeder{Enabled: dwdWeederEnabled},
					Prober: &SeedSettingDependencyWatchdogProber{Enabled: dwdProberEnabled},
				},
				TopologyAwareRouting: &SeedSettingTopologyAwareRouting{
					Enabled: topologyAwareRouting,
				},
				ExcessCapacityReservation: &SeedSettingExcessCapacityReservation{Enabled: excessCapacityReservation},
				Scheduling:                &SeedSettingScheduling{Visible: scheduling},
				VerticalPodAutoscaler:     &SeedSettingVerticalPodAutoscaler{Enabled: vpaEnabled},
				OwnerChecks:               &SeedSettingOwnerChecks{Enabled: ownerChecks},
			}

			SetObjectDefaults_Seed(obj)

			Expect(obj.Spec.Settings.DependencyWatchdog.Weeder.Enabled).To(Equal(dwdWeederEnabled))
			Expect(obj.Spec.Settings.DependencyWatchdog.Prober.Enabled).To(Equal(dwdProberEnabled))
			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(Equal(excessCapacityReservation))
			Expect(obj.Spec.Settings.Scheduling.Visible).To(Equal(scheduling))
			Expect(obj.Spec.Settings.VerticalPodAutoscaler.Enabled).To(Equal(vpaEnabled))
			Expect(obj.Spec.Settings.OwnerChecks.Enabled).To(Equal(ownerChecks))
			Expect(obj.Spec.Settings.TopologyAwareRouting.Enabled).To(Equal(topologyAwareRouting))
		})

		It("should default ipFamilies setting to IPv4 single-stack", func() {
			SetObjectDefaults_Seed(obj)

			Expect(obj.Spec.Networks.IPFamilies).To(ConsistOf(IPFamilyIPv4))
		})
	})
})
