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
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	controllermgrconfig "github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile/helper"
	"github.com/gardener/gardener/pkg/logger"
)

const loggerPrefixKubernetes = "KUBERNETES"
const loggerPrefixMachineImage = "MACHINE IMAGE"

// ReconcileKubernetesVersions reconciles the Kubernetes versions of the CloudProfile
func ReconcileKubernetesVersions(logger *logrus.Entry, config *controllermgrconfig.VersionManagement, profile *gardencorev1beta1.CloudProfile) (*gardencorev1beta1.CloudProfile, error) {
	var err error
	profile.Spec.Kubernetes.Versions, err = ReconcileVersions(logger, loggerPrefixKubernetes, profile.Spec.Kubernetes.Versions, config)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

// ReconcileMachineImageVersions reconciles the Machine image versions of the CloudProfile
func ReconcileMachineImageVersions(config *controllermgrconfig.VersionManagement, profile *gardencorev1beta1.CloudProfile) (*gardencorev1beta1.CloudProfile, error) {
	var err error
	for index, machineImage := range profile.Spec.MachineImages {
		log := logger.NewFieldLogger(logger.Logger, "machine image name", machineImage.Name)
		profile.Spec.MachineImages[index].Versions, err = ReconcileVersions(log, loggerPrefixMachineImage, machineImage.Versions, config)
		if err != nil {
			return nil, err
		}
	}
	return profile, nil
}

// ReconcileVersions reconciles the slice of expirable versions and returns the reconciled result.
func ReconcileVersions(logger *logrus.Entry, loggerPrefix string, versions []gardencorev1beta1.ExpirableVersion, config *controllermgrconfig.VersionManagement) ([]gardencorev1beta1.ExpirableVersion, error) {
	var (
		err                       error
		unmaintainedMinorVersions = sets.String{}
	)

	if config.VersionMaintenance != nil {
		unmaintainedMinorVersions, err = gardencorev1beta1helper.GetUnmaintainedMinorVersions(versions, config.VersionMaintenance.MaintainedVersions)
		if err != nil {
			return nil, err
		}
	}

	latestVersionPerMinor, latestVersion, err := gardencorev1beta1helper.GetLatestVersionsPerMinor(versions)
	if err != nil {
		return nil, err
	}

	for index, v := range versions {
		version, err := semver.NewVersion(v.Version)
		if err != nil {
			return nil, err
		}

		var reconciledVersion *gardencorev1beta1.ExpirableVersion
		if config.VersionMaintenance != nil {
			key := fmt.Sprintf("%d.%d", version.Major(), version.Minor())
			latestVersionForMinor := latestVersionPerMinor[key]
			hasUnmaintainedMinor := unmaintainedMinorVersions.Has(key)

			// there are only preview versions for that particular minor version
			if latestVersionForMinor == nil {
				if hasUnmaintainedMinor {
					// edge case: if there are only preview versions in an unmaintained minor: deprecate without expiration date.
					// Next reconciliation will determine the expiration dates.
					reconciledVersion = reconcileUnmaintainedVersion(logger, loggerPrefix, v, true, config.VersionMaintenance.ExpirationDurationUnmaintainedVersion)
					versions[index] = *reconciledVersion
				}
				continue
			}

			isLatestVersionForMinor := version.Equal(latestVersionForMinor) || version.GreaterThan(latestVersionForMinor)
			if hasUnmaintainedMinor {
				reconciledVersion = reconcileUnmaintainedVersion(logger, loggerPrefix, v, isLatestVersionForMinor, config.VersionMaintenance.ExpirationDurationUnmaintainedVersion)
			} else {
				reconciledVersion = reconcileVersion(logger, loggerPrefix, v, isLatestVersionForMinor, *config.ExpirationDuration)
			}
			versions[index] = *reconciledVersion
			continue
		}

		// hands over the information it the version is overall the latest version. Not only scoped to the minor version.
		reconciledVersion = reconcileVersion(logger, loggerPrefix, v, latestVersion == v.Version, *config.ExpirationDuration)
		versions[index] = *reconciledVersion
	}
	return versions, nil
}

// overrides preview classifications for unmaintained minors
func reconcileUnmaintainedVersion(logger *logrus.Entry, loggerPrefix string, version gardencorev1beta1.ExpirableVersion, isLatestVersionForMinor bool, expirationDuration metav1.Duration) *gardencorev1beta1.ExpirableVersion {
	// not deprecating latest version of unsupported minor
	if isLatestVersionForMinor {
		// already properly maintained
		if version.Classification != nil && *version.Classification == gardencorev1beta1.ClassificationDeprecated && version.ExpirationDate == nil {
			return &version
		}

		version = helper.SetDeprecatedClassification(version)

		logger.Infof("[%s VERSION MANAGEMENT] setting '%s' classification for unmaintained version '%s'.", loggerPrefix, gardencorev1beta1.ClassificationDeprecated, version.Version)
		return &version
	}
	version = helper.SetDeprecatedClassification(version)
	return setExpirationDate(logger, loggerPrefix, version, expirationDuration)
}

func reconcileVersion(logger *logrus.Entry, loggerPrefix string, version gardencorev1beta1.ExpirableVersion, isLatestVersion bool, expirationDuration metav1.Duration) *gardencorev1beta1.ExpirableVersion {
	// do not touch preview versions
	if version.Classification != nil && (*version.Classification == gardencorev1beta1.ClassificationPreview) {
		return &version
	}
	// setting supported classification works if there is no supported version yet for the minor version yet
	// However it will fail if there is already a supported version. The Gardener operator has to deprecate the current supported version first.
	if isLatestVersion {
		version.ExpirationDate = nil
		if version.Classification != nil && *version.Classification == gardencorev1beta1.ClassificationSupported {
			return &version
		}

		logger.Infof("[%s VERSION MANAGEMENT] setting '%s' classification for version '%s'.", loggerPrefix, gardencorev1beta1.ClassificationSupported, version.Version)
		version = helper.SetSupportedClassification(version)
		return &version
	}

	version = helper.SetDeprecatedClassification(version)
	return setExpirationDate(logger, loggerPrefix, version, expirationDuration)
}

func setExpirationDate(logger *logrus.Entry, loggerPrefix string, version gardencorev1beta1.ExpirableVersion, expirationDuration metav1.Duration) *gardencorev1beta1.ExpirableVersion {
	// do not change expiration date
	if version.ExpirationDate != nil {
		return &version
	}

	version.ExpirationDate = &metav1.Time{Time: time.Now().UTC().Add(expirationDuration.Duration)}
	logger.Infof("[%s VERSION MANAGEMENT] setting '%s' classification for version '%s'. Expiration date set to: '%s'.", loggerPrefix, gardencorev1beta1.ClassificationDeprecated, version.Version, version.ExpirationDate.Round(time.Minute))
	return &version
}
