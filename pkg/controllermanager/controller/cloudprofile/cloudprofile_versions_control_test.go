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

package cloudprofile

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	controllermgrconfig "github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile/helper"
	"github.com/gardener/gardener/pkg/logger"
)

var (
	testlogger                          = logger.NewFieldLogger(logger.NewLogger("info"), "cloudprofile", "test")
	expirationDuration                  = metav1.Duration{Duration: 24 * 30 * 4 * time.Hour}
	expirationDurationUnmaintainedMinor = metav1.Duration{Duration: 24 * 30 * 1 * time.Hour}

	expirationDateSupportedMinor   = &metav1.Time{Time: time.Now().UTC().Add(expirationDuration.Duration)}
	expirationDateUnsupportedMinor = &metav1.Time{Time: time.Now().UTC().Add(expirationDurationUnmaintainedMinor.Duration)}
)

func testKubernetesVersions(testVersions []gardencorev1beta1.ExpirableVersion, expectedVersions []gardencorev1beta1.ExpirableVersion, versionManagementConfig *controllermgrconfig.VersionManagement) {
	testVersionCopy := make([]gardencorev1beta1.ExpirableVersion, len(testVersions))
	copy(testVersionCopy, testVersions)
	cp := gardencorev1beta1.CloudProfile{
		Spec: gardencorev1beta1.CloudProfileSpec{
			Kubernetes: gardencorev1beta1.KubernetesSettings{Versions: testVersionCopy},
		},
	}

	updatedCloudProfile, err := ReconcileKubernetesVersions(testlogger, versionManagementConfig, &cp)
	Expect(err).ToNot(HaveOccurred())
	Expect(updatedCloudProfile.Spec.Kubernetes.Versions).ToNot(BeNil())
	versions := helper.SanitizeTimestampsForTesting(updatedCloudProfile.Spec.Kubernetes.Versions)
	expectedVersions = helper.SanitizeTimestampsForTesting(expectedVersions)
	Expect(versions).To(Equal(expectedVersions))
}

func testMachineImageVersions(testVersions []gardencorev1beta1.ExpirableVersion, expectedVersions []gardencorev1beta1.ExpirableVersion, versionManagementConfig *controllermgrconfig.VersionManagement) {
	testVersionCopy := make([]gardencorev1beta1.ExpirableVersion, len(testVersions))
	copy(testVersionCopy, testVersions)
	cp := gardencorev1beta1.CloudProfile{
		Spec: gardencorev1beta1.CloudProfileSpec{
			MachineImages: []gardencorev1beta1.MachineImage{{Name: "gardenlinux", Versions: testVersionCopy}},
		},
	}

	updatedCloudProfile, err := ReconcileMachineImageVersions(versionManagementConfig, &cp)

	Expect(err).ToNot(HaveOccurred())
	Expect(updatedCloudProfile.Spec.MachineImages).ToNot(BeNil())
	Expect(updatedCloudProfile.Spec.MachineImages[0].Versions).ToNot(BeNil())
	Expect(updatedCloudProfile.Spec.MachineImages[0].Versions).To(HaveLen(len(expectedVersions)))

	// sanitize updated versions
	updatedCloudProfile.Spec.MachineImages[0].Versions = helper.SanitizeTimestampsForTesting(updatedCloudProfile.Spec.MachineImages[0].Versions)

	// sanitize expected versions
	expectedVersions = helper.SanitizeTimestampsForTesting(expectedVersions)
	Expect(updatedCloudProfile.Spec.MachineImages[0].Versions).To(Equal(expectedVersions))
}

var _ = Describe("Version Management", func() {
	DescribeTable("#Reconcile",
		func(testVersions []gardencorev1beta1.ExpirableVersion, expectedK8sVersions []gardencorev1beta1.ExpirableVersion) {
			config := &controllermgrconfig.VersionManagement{
				ExpirationDuration: &expirationDuration,
			}
			testKubernetesVersions(testVersions, expectedK8sVersions, config)
			testMachineImageVersions(testVersions, expectedK8sVersions, config)
		},

		// version without classification
		Entry("latest version: should set supported status",
			helper.GetVersions(
				helper.GetVersionWithNoStatus("1.0.2"),
				helper.GetDeprecatedVersionNotExpired("1.0.1"),
				helper.GetDeprecatedVersionNotExpired("1.0.0")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.2"),
				helper.GetDeprecatedVersionNotExpired("1.0.1"),
				helper.GetDeprecatedVersionNotExpired("1.0.0")),
		),

		Entry("non latest version: should deprecate and keep highest supported",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.3"),
				helper.GetVersionWithNoStatus("1.0.2")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.3"),
				helper.GetDeprecatedVersion("1.0.2", expirationDateSupportedMinor),
			)),

		// preview version
		Entry("operator sets only preview version - do not change version",
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.3.1"),
			),
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.3.1")),
		),
		Entry("added another (latest) preview version: allow multiple preview versions",
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.0.3"),
				helper.GetVersionWithPreviewClassification("1.0.2")),
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.0.3"),
				helper.GetVersionWithPreviewClassification("1.0.2")),
		),

		// Supported version
		Entry("operator sets only supported version - do not change version",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1")),
		),

		Entry("latest version: should overall only define one supported version",
			helper.GetVersions(
				helper.GetVersionWithNoStatus("1.18.1"),
				helper.GetVersionWithNoStatus("1.18.0"),
				helper.GetVersionWithSupportedClassification("1.17.1"),
				helper.GetVersionWithNoStatus("1.17.0"),
				helper.GetVersionWithSupportedClassification("1.16.1"),
				helper.GetVersionWithSupportedClassification("1.15.1")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.18.1"),
				helper.GetDeprecatedVersion("1.18.0", expirationDateSupportedMinor),
				helper.GetDeprecatedVersion("1.17.1", expirationDateSupportedMinor),
				helper.GetDeprecatedVersion("1.17.0", expirationDateSupportedMinor),
				helper.GetDeprecatedVersion("1.16.1", expirationDateSupportedMinor),
				helper.GetDeprecatedVersion("1.15.1", expirationDateSupportedMinor)),
		),

		Entry("should enforce a supported version even though there are higher preview versions",
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.18.1"),
				helper.GetVersionWithNoStatus("1.18.0")),
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.18.1"),
				helper.GetVersionWithSupportedClassification("1.18.0")),
		),
	)

	DescribeTable("#Reconcile with maintained versions configuration",
		func(testVersions []gardencorev1beta1.ExpirableVersion, expectedK8sVersions []gardencorev1beta1.ExpirableVersion) {
			config := &controllermgrconfig.VersionManagement{
				ExpirationDuration: &expirationDuration,
				VersionMaintenance: &controllermgrconfig.VersionMaintenance{
					MaintainedVersions:                    3,
					ExpirationDurationUnmaintainedVersion: expirationDurationUnmaintainedMinor,
				},
			}
			testKubernetesVersions(testVersions, expectedK8sVersions, config)
			testMachineImageVersions(testVersions, expectedK8sVersions, config)
		},

		// add version without classification - maintained minor
		Entry("latest version for maintained minor: should set supported status and deprecate currently supported",
			helper.GetVersions(
				helper.GetVersionWithNoStatus("1.0.2"),
				helper.GetVersionWithSupportedClassification("1.0.1"),
				helper.GetDeprecatedVersionNotExpired("1.0.0")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.2"),
				helper.GetDeprecatedVersion("1.0.1", expirationDateSupportedMinor),
				helper.GetDeprecatedVersionNotExpired("1.0.0")),
		),
		Entry("non-latest version for maintained minor: should deprecate with expiration date",
			helper.GetVersions(
				helper.GetVersionWithNoStatus("1.0.2"),
				helper.GetDeprecatedVersionNotExpired("1.0.1"),
				helper.GetVersionWithNoStatus("1.0.0")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.2"),
				helper.GetDeprecatedVersionNotExpired("1.0.1"),
				helper.GetDeprecatedVersion("1.0.0", expirationDateSupportedMinor)),
		),

		// add new latest version without classification - should set supported status and deprecate unmaintained versions
		Entry("new latest version without classification: should set supported status and deprecate other",
			helper.GetVersions(
				helper.GetVersionWithNoStatus("1.18.0"),
				helper.GetVersionWithSupportedClassification("1.17.1"),
				helper.GetVersionWithSupportedClassification("1.16.1"),
				helper.GetVersionWithSupportedClassification("1.15.1"),
				helper.GetDeprecatedVersionNotExpired("1.15.0")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.18.0"),
				helper.GetVersionWithSupportedClassification("1.17.1"),
				helper.GetVersionWithSupportedClassification("1.16.1"),
				helper.GetDeprecatedVersion("1.15.1", nil),
				helper.GetDeprecatedVersionNotExpired("1.15.0")),
		),

		Entry("non-latest version for maintained minor: should deprecate with expiration date",
			helper.GetVersions(
				helper.GetVersionWithNoStatus("1.0.2"),
				helper.GetDeprecatedVersionNotExpired("1.0.1"),
				helper.GetVersionWithNoStatus("1.0.0")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.2"),
				helper.GetDeprecatedVersionNotExpired("1.0.1"),
				helper.GetDeprecatedVersion("1.0.0", expirationDateSupportedMinor)),
		),

		// add version without classification - unmaintained minor
		Entry("latest version for unmaintained minor - deprecate without expiration date & set deprecation timestamp on all other versions of that minor",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetVersionWithNoStatus("1.0.1"),
				helper.GetDeprecatedVersion("1.0.0", nil),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetDeprecatedVersion("1.0.1", nil),
				helper.GetDeprecatedVersion("1.0.0", expirationDateUnsupportedMinor)),
		),
		Entry("non-latest version for unmaintained minor: should deprecate with expiration date",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetDeprecatedVersion("1.0.2", nil),
				helper.GetVersionWithNoStatus("1.0.1")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetDeprecatedVersion("1.0.2", nil),
				helper.GetDeprecatedVersion("1.0.1", expirationDateUnsupportedMinor)),
		),

		// add preview version - maintained minor
		Entry("operator sets only preview version - do not do anything",
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.3.1"),
			),
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.3.1")),
		),
		Entry("added another (latest) preview version for supported minor: allow multiple preview versions",
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.0.3"),
				helper.GetVersionWithPreviewClassification("1.0.2")),
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.0.3"),
				helper.GetVersionWithPreviewClassification("1.0.2")),
		),
		// add preview version - unmaintained minor
		Entry("operator adds version (preview & latest) for unsupported minor - deprecate without expiration date and set deprecation timestamp on other versions",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetVersionWithPreviewClassification("1.0.2"),
				helper.GetDeprecatedVersion("1.0.1", nil),
				helper.GetDeprecatedVersion("1.0.0", nil),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetDeprecatedVersion("1.0.2", nil),
				// if version 1.0.2 would not be a preview version, the expiration date would be set.
				// will be set during the second reconcile
				helper.GetDeprecatedVersion("1.0.1", nil),
				helper.GetDeprecatedVersion("1.0.0", expirationDateUnsupportedMinor)),
		),

		// edge case: only preview versions for an unmaintained minor
		Entry("only preview versions for an unmaintained minor - deprecate all versions without an expiration date",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetVersionWithPreviewClassification("1.0.3"),
				helper.GetVersionWithPreviewClassification("1.0.2"),
				helper.GetVersionWithPreviewClassification("1.0.1"),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetDeprecatedVersion("1.0.3", nil),
				helper.GetDeprecatedVersion("1.0.2", nil),
				helper.GetDeprecatedVersion("1.0.1", nil)),
		),

		// add supported version - maintained minor
		Entry("operator sets only supported version - do not do anything",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
			),
		),

		Entry("should enforce one supported version per maintained minor version",
			helper.GetVersions(
				helper.GetVersionWithNoStatus("1.18.1"),
				helper.GetVersionWithNoStatus("1.18.0"),
				helper.GetVersionWithNoStatus("1.17.1"),
				helper.GetVersionWithNoStatus("1.17.0"),
				helper.GetVersionWithNoStatus("1.16.1"),
				helper.GetVersionWithNoStatus("1.15.1")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.18.1"),
				helper.GetDeprecatedVersion("1.18.0", expirationDateSupportedMinor),
				helper.GetVersionWithSupportedClassification("1.17.1"),
				helper.GetDeprecatedVersion("1.17.0", expirationDateSupportedMinor),
				helper.GetVersionWithSupportedClassification("1.16.1"),
				helper.GetDeprecatedVersion("1.15.1", nil)),
		),

		// add supported version - unmaintained minor
		Entry("operator sets supported version for unmaintained minor - deprecate",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetDeprecatedVersion("1.0.1", nil),
				helper.GetVersionWithSupportedClassification("1.0.0"),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetDeprecatedVersion("1.0.1", nil),
				helper.GetDeprecatedVersion("1.0.0", expirationDateUnsupportedMinor)),
		),
		Entry("do not override (operator set) expiration date for latest version of unsupported minor",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				// operator decided to deprecate
				helper.GetDeprecatedVersion("1.0.1", expirationDateUnsupportedMinor),
				helper.GetDeprecatedVersionNotExpired("1.0.0"),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1"),
				helper.GetVersionWithSupportedClassification("1.2.1"),
				helper.GetVersionWithSupportedClassification("1.1.1"),
				helper.GetDeprecatedVersion("1.0.1", expirationDateUnsupportedMinor),
				helper.GetDeprecatedVersionNotExpired("1.0.0")),
		),
	)
})
