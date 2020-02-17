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

package machineversion_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	controllermgrconfig "github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile/helper"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile/machineversion"
	"github.com/gardener/gardener/pkg/logger"
)

var (
	klogger              = logger.NewFieldLogger(logger.NewLogger("info"), "cloudprofile", "test")
	defaultMachineConfig = controllermgrconfig.CloudProfileControllerConfiguration{
		ConcurrentSyncs: 1,
		MachineImageVersionManagement: &controllermgrconfig.MachineImageVersionManagement{
			Enabled:            true,
			ExpirationDuration: &metav1.Duration{Duration: 24 * 30 * 4 * time.Hour},
		},
	}
	now      = &metav1.Time{Time: time.Now()}
	falseVar = false
	trueVar  = true

	dateInThePast              = &metav1.Time{Time: time.Now().UTC().AddDate(-5, 0, 0)}
	expirationDateMachineImage = &metav1.Time{Time: time.Now().UTC().Add(defaultMachineConfig.MachineImageVersionManagement.ExpirationDuration.Duration)}
)

var _ = Describe("MachineImage Version Management", func() {
	Describe("#Machine Image Version Management not enabled", func() {
		It("should not update the version", func() {
			cp := gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					MachineImages: []gardencorev1beta1.MachineImage{
						{
							Name:     "test",
							Versions: helper.GetVersions(helper.GetVersionWithNoStatus("1.0.0")),
						},
					},
				},
			}
			updatedProfile, err := machineversion.ReconcileMachineImageVersions(klogger, &controllermgrconfig.CloudProfileControllerConfiguration{
				KubernetesVersionManagement: &controllermgrconfig.KubernetesVersionManagement{
					Enabled: false,
				},
			}, &cp)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedProfile.Spec.MachineImages[0].Versions).To(Equal(helper.GetVersions(helper.GetVersionWithNoStatus("1.0.0"))))
		})
	})

	DescribeTable("#Check Status Updates",
		func(machines []gardencorev1beta1.MachineImage, expectedMachines []gardencorev1beta1.MachineImage) {
			cp := gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					MachineImages: machines,
				},
			}
			updatedProfile, err := machineversion.ReconcileMachineImageVersions(klogger, &defaultMachineConfig, &cp)
			Expect(err).ToNot(HaveOccurred())
			Expect(cp.Spec.MachineImages).ToNot(BeNil())

			for i, machine := range cp.Spec.MachineImages {
				cp.Spec.MachineImages[i].Versions = helper.SanitizeTimestampsForTesting(machine.Versions)
			}
			for i, machine := range expectedMachines {
				expectedMachines[i].Versions = helper.SanitizeTimestampsForTesting(machine.Versions)
			}
			Expect(updatedProfile.Spec.MachineImages).To(Equal(expectedMachines))
		},

		// MIGRATION - version without classification
		Entry("Migration: latest machine image version: should set supported status",
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithNoStatus("1.0.2"),
				helper.GetDeprecatedVersionNotExpired("1.0.1", dateInThePast),
				helper.GetDeprecatedVersionNotExpired("1.0.0", dateInThePast)),
			)),
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.2", now),
				helper.GetDeprecatedVersionNotExpired("1.0.1", dateInThePast),
				helper.GetDeprecatedVersionNotExpired("1.0.0", dateInThePast)),
			)),
		),

		Entry("Migration: non latest version: should deprecate and keep highest supported",
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.3", now),
				helper.GetVersionWithNoStatus("1.0.2")),
			)),
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.3", now),
				helper.GetDeprecatedVersion("1.0.2", now, expirationDateMachineImage)),
			)),
		),

		// ADD PREVIEW VERSION
		// - AdmissionPlugin only allows to add latest patch versions as preview versions
		Entry("operator sets only preview version - do not do anything",
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.3.1", dateInThePast),
			))),
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.3.1", dateInThePast)),
			)),
		),

		Entry("added another (latest) preview version: allow multiple preview versions",
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.0.3", now),
				helper.GetVersionWithPreviewClassification("1.0.2", now)),
			)),
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.0.3", now),
				helper.GetVersionWithPreviewClassification("1.0.2", now)),
			)),
		),

		// SUPPORTED
		// - AdmissionPlugin only allows to supported version that is higher than the current supported version
		Entry("operator sets only supported version - do not do anything",
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
			))),
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
			))),
		),

		Entry("operator sets another supported version -  deprecate the currently supported & lower version",
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.3.0", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.2.0", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.2.1", dateInThePast),
			))),
			getMachines(getTestMachine(helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
				helper.GetDeprecatedVersion("1.3.0", now, expirationDateMachineImage),
				helper.GetDeprecatedVersion("1.2.0", now, expirationDateMachineImage),
				helper.GetVersionWithSupportedClassification("1.2.1", dateInThePast),
			))),
		),
	)
})

func getMachines(version ...gardencorev1beta1.MachineImage) []gardencorev1beta1.MachineImage {
	return version
}

func getTestMachine(version []gardencorev1beta1.ExpirableVersion) gardencorev1beta1.MachineImage {
	return gardencorev1beta1.MachineImage{Name: "test", Versions: version}
}
