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

package kubernetesversion_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	controllermgrconfig "github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile/helper"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile/kubernetesversion"
	"github.com/gardener/gardener/pkg/logger"
)

var (
	testlogger    = logger.NewFieldLogger(logger.NewLogger("info"), "cloudprofile", "test")
	three         = 3
	defaultConfig = controllermgrconfig.CloudProfileControllerConfiguration{
		ConcurrentSyncs: 1,
		KubernetesVersionManagement: &controllermgrconfig.KubernetesVersionManagement{
			Enabled:                               true,
			MaintainedKubernetesVersions:          &three,
			ExpirationDurationMaintainedVersion:   &metav1.Duration{Duration: 24 * 30 * 4 * time.Hour},
			ExpirationDurationUnmaintainedVersion: &metav1.Duration{Duration: 24 * 30 * 1 * time.Hour},
		},
	}

	now     = &metav1.Time{Time: time.Now()}
	trueVar = true

	expirationDateSupportedMinor   = &metav1.Time{Time: time.Now().UTC().Add(defaultConfig.KubernetesVersionManagement.ExpirationDurationMaintainedVersion.Duration)}
	expirationDateUnsupportedMinor = &metav1.Time{Time: time.Now().UTC().Add(defaultConfig.KubernetesVersionManagement.ExpirationDurationUnmaintainedVersion.Duration)}
	dateInThePast                  = &metav1.Time{Time: time.Now().UTC().AddDate(-5, 0, 0)}
)

var _ = Describe("Kubernetes Version Management", func() {
	Describe("#Kubernetes Version Management not enabled", func() {
		It("should not update the version", func() {
			cp := gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					Kubernetes: gardencorev1beta1.KubernetesSettings{Versions: helper.GetVersions(helper.GetVersionWithNoStatus("1.0.0"))},
				},
			}
			updatedCloudProfile, err := kubernetesversion.ReconcileKubernetesVersions(testlogger, &controllermgrconfig.CloudProfileControllerConfiguration{
				KubernetesVersionManagement: &controllermgrconfig.KubernetesVersionManagement{
					Enabled: false,
				},
			}, &cp)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedCloudProfile.Spec.Kubernetes.Versions).To(Equal(helper.GetVersions(helper.GetVersionWithNoStatus("1.0.0"))))
		})
	})

	DescribeTable("#Check Status Updates",
		func(k8Versions []gardencorev1beta1.ExpirableVersion, expectedK8sVersions []gardencorev1beta1.ExpirableVersion) {
			cp := gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					Kubernetes: gardencorev1beta1.KubernetesSettings{Versions: k8Versions},
				},
			}
			updatedCloudProfile, err := kubernetesversion.ReconcileKubernetesVersions(testlogger, &defaultConfig, &cp)
			Expect(err).ToNot(HaveOccurred())
			Expect(cp.Spec.Kubernetes.Versions).ToNot(BeNil())
			versions := helper.SanitizeTimestampsForTesting(updatedCloudProfile.Spec.Kubernetes.Versions)
			expectedVersions := helper.SanitizeTimestampsForTesting(expectedK8sVersions)
			Expect(versions).To(Equal(expectedVersions))
		},

		// MIGRATION - supported Minor (version without classification)
		Entry("Migration: latest k8 version for supported minor: should set supported status",
			helper.GetVersions(
				helper.GetVersionWithNoStatus("1.0.2"),
				helper.GetDeprecatedVersionNotExpired("1.0.1", dateInThePast),
				helper.GetDeprecatedVersionNotExpired("1.0.0", dateInThePast)),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.2", now),
				helper.GetDeprecatedVersionNotExpired("1.0.1", dateInThePast),
				helper.GetDeprecatedVersionNotExpired("1.0.0", dateInThePast)),
		),
		Entry("Migration: non latest version for supported minor: should deprecate and keep highest supported",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.3", now),
				helper.GetVersionWithNoStatus("1.0.2")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.0.3", now),
				helper.GetDeprecatedVersion("1.0.2", now, expirationDateSupportedMinor)),
		),

		// MIGRATION - Unsupported Minor (version without classification)
		Entry("Migration: latest version for unsupported minor - deprecate without expiration date && set deprecation timestamp on all other versions",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.2.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.1.1", dateInThePast),
				helper.GetVersionWithNoStatus("1.0.1"),
				helper.GetDeprecatedVersion("1.0.0", dateInThePast, nil),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.2.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.1.1", dateInThePast),
				helper.GetDeprecatedVersion("1.0.1", now, nil),
				helper.GetDeprecatedVersion("1.0.0", now, expirationDateUnsupportedMinor)),
		),
		Entry("Migration: non-latest version for unsupported minor: should deprecate with expiration date",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.2.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.1.1", dateInThePast),
				helper.GetDeprecatedVersion("1.0.2", dateInThePast, nil),
				helper.GetVersionWithNoStatus("1.0.1")),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.2.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.1.1", dateInThePast),
				helper.GetDeprecatedVersion("1.0.2", dateInThePast, nil),
				helper.GetDeprecatedVersion("1.0.1", now, expirationDateUnsupportedMinor)),
		),

		// ADD PREVIEW VERSION
		// - AdmissionPlugin only allows to add latest patch versions as preview versions
		Entry("operator sets only preview version - do not do anything",
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.3.1", dateInThePast),
			),
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.3.1", dateInThePast)),
		),
		Entry("operator adds preview version for unsupported minor - deprecate without expiration date && set deprecation timestamp on all other versions",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.2.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.1.1", dateInThePast),
				helper.GetVersionWithPreviewClassification("1.0.1", dateInThePast),
				helper.GetDeprecatedVersion("1.0.0", dateInThePast, nil),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.2.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.1.1", dateInThePast),
				helper.GetDeprecatedVersion("1.0.1", now, nil),
				helper.GetDeprecatedVersion("1.0.0", now, expirationDateUnsupportedMinor)),
		),
		Entry("added another (latest) preview version for supported minor: allow multiple preview versions",
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.0.3", now),
				helper.GetVersionWithPreviewClassification("1.0.2", now)),
			helper.GetVersions(
				helper.GetVersionWithPreviewClassification("1.0.3", now),
				helper.GetVersionWithPreviewClassification("1.0.2", now)),
		),

		// SUPPORTED
		// - AdmissionPlugin only allows to supported version that is higher than the current supported version
		Entry("operator sets only supported version - do not do anything",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast)),
		),
		Entry("operator sets another supported version -  deprecate the currently supported & lower version (of that minor only)",
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
				helper.GetVersionWithSupportedClassification("1.3.0", dateInThePast),
			),
			helper.GetVersions(
				helper.GetVersionWithSupportedClassification("1.3.1", dateInThePast),
				helper.GetDeprecatedVersion("1.3.0", now, expirationDateSupportedMinor)),
		),
	)
})
