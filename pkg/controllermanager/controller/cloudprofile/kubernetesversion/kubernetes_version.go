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

package kubernetesversion

import (
	"time"

	"github.com/Masterminds/semver"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	controllermgrconfig "github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile/helper"
)

// ReconcileKubernetesVersions reconciles the Spec.Kubernetes.Versions section of the CloudProfile
// returns true if the CloudProfile has to be updated, the updated CloudProfile or an error
func ReconcileKubernetesVersions(logger *logrus.Entry, config *controllermgrconfig.CloudProfileControllerConfiguration, profile *gardencorev1beta1.CloudProfile) (*gardencorev1beta1.CloudProfile, error) {
	if config == nil || config.KubernetesVersionManagement == nil || config.KubernetesVersionManagement.Enabled == false {
		return profile, nil
	}

	supportedVersionsMap := make(map[string]helper.HighestSupportedVersion, len(profile.Spec.Kubernetes.Versions))
	for cloudProfileIndex, version := range profile.Spec.Kubernetes.Versions {
		currentSemverVersion, err := semver.NewVersion(version.Version)
		if err != nil {
			return profile, err
		}

		hasMaintainedMinorVersion, err := gardencorev1beta1helper.HasMaintainedKubernetesMinorVersion(*config.KubernetesVersionManagement.MaintainedKubernetesVersions, profile, *currentSemverVersion)
		if err != nil {
			return profile, err
		}

		newerPatchVersionFound, _, err := gardencorev1beta1helper.DetermineLatestKubernetesPatchVersion(profile, version.Version)
		if err != nil {
			return profile, err
		}

		// if there is no classification set & version is highest patch version of supported minor version -> declare as supported
		if version.Classification == nil && !newerPatchVersionFound && hasMaintainedMinorVersion {
			logger.Infof("[KUBERNETES VERSION MANAGEMENT] setting '%s' classification for kubernetes version '%s' that had no classification", gardencorev1beta1.ClassificationSupported, version.Version)
			profile.Spec.Kubernetes.Versions[cloudProfileIndex] = helper.SetSupportedClassification(version)
		}

		// if there is no classification set & version is not the highest patch version -> deprecate
		if version.Classification == nil && newerPatchVersionFound {
			// deprecate
			deprecatedVersion, err := deprecateKubernetesVersion(logger, config, profile, version, *currentSemverVersion, hasMaintainedMinorVersion, !newerPatchVersionFound)
			if err != nil {
				return profile, err
			}
			profile.Spec.Kubernetes.Versions[cloudProfileIndex] = *deprecatedVersion
			continue
		}

		// there cannot be a preview or supported version for 'unmaintained' Kubernetes versions
		if !hasMaintainedMinorVersion && (version.Classification == nil || *version.Classification == gardencorev1beta1.ClassificationPreview || *version.Classification == gardencorev1beta1.ClassificationSupported) {
			deprecatedVersion, err := deprecateKubernetesVersion(logger, config, profile, version, *currentSemverVersion, false, !newerPatchVersionFound)
			if err != nil {
				return profile, err
			}
			profile.Spec.Kubernetes.Versions[cloudProfileIndex] = *deprecatedVersion
			continue
		}

		// keep only one supported version.
		// This is important when another 'supported' version has been added. The automation makes sure that
		// only the highest 'supported' version is kept, and the other 'supported versions' are deprecated according
		// to the deprecation times for 'maintained' Kubernetes versions configured in the CloudProfileController of the GCM
		if version.Classification != nil && *version.Classification == gardencorev1beta1.ClassificationSupported {
			key := helper.GetSupportedVersionsKey(*currentSemverVersion)
			currentlyHighestSupportedVersion, found := supportedVersionsMap[key]
			if !found {
				supportedVersionsMap[key] = helper.HighestSupportedVersion{
					Version: currentSemverVersion,
					Index:   cloudProfileIndex,
				}
				continue
			}
			if currentlyHighestSupportedVersion.Version.LessThan(currentSemverVersion) {
				deprecatedVersion, err := deprecateKubernetesVersion(logger, config, profile, profile.Spec.Kubernetes.Versions[currentlyHighestSupportedVersion.Index], *currentSemverVersion, true, false)
				if err != nil {
					return profile, err
				}
				profile.Spec.Kubernetes.Versions[currentlyHighestSupportedVersion.Index] = *deprecatedVersion
				supportedVersionsMap[key] = helper.HighestSupportedVersion{
					Version: currentSemverVersion,
					Index:   cloudProfileIndex,
				}
				continue
			}

			// deprecate current version, as there is already another higher supported version of the same minor
			deprecatedVersion, err := deprecateKubernetesVersion(logger, config, profile, version, *currentSemverVersion, true, false)
			if err != nil {
				return profile, err
			}
			profile.Spec.Kubernetes.Versions[cloudProfileIndex] = *deprecatedVersion
			continue
		}
	}
	return profile, nil
}

// deprecateKubernetesVersion deprecates a version, taking into consideration if the version
// is the latest patch version & has a supported minor version
func deprecateKubernetesVersion(logger *logrus.Entry, config *controllermgrconfig.CloudProfileControllerConfiguration, profile *gardencorev1beta1.CloudProfile, currentVersion gardencorev1beta1.ExpirableVersion, currentSemVerVersion semver.Version, hasMaintainedMinorVersion, isLatestPatchVersion bool) (*gardencorev1beta1.ExpirableVersion, error) {
	currentVersion = helper.SetDeprecatedClassification(currentVersion)
	if isLatestPatchVersion && !hasMaintainedMinorVersion {
		// case: adding latest patch version of unmaintained minor -> set deprecation dates for all other version of same major and minor
		// do not add expiration date for latest patch version
		for i, version := range profile.Spec.Kubernetes.Versions {
			semverVersion, err := semver.NewVersion(version.Version)
			if err != nil {
				return nil, err
			}
			if semverVersion.Minor() == currentSemVerVersion.Minor() && semverVersion.Major() == currentSemVerVersion.Major() && version.Version != currentVersion.Version {
				v, err := deprecateKubernetesVersion(logger, config, profile, version, *semverVersion, false, false)
				if err != nil {
					return nil, err
				}
				profile.Spec.Kubernetes.Versions[i] = *v
			}
		}

		return &currentVersion, nil
	} else if isLatestPatchVersion {
		return &currentVersion, nil
	}

	// set expiration date depending on supported minor
	if hasMaintainedMinorVersion {
		currentVersion.ExpirationDate = &metav1.Time{Time: time.Now().UTC().Add(config.KubernetesVersionManagement.ExpirationDurationMaintainedVersion.Duration)}
		logger.Infof("[KUBERNETES VERSION MANAGEMENT] deprecating kubernetes version '%s' (unmaintained minor version) - expiration time set to: '%s'", currentVersion.Version, currentVersion.ExpirationDate.Round(time.Minute))
		return &currentVersion, nil
	}

	currentVersion.ExpirationDate = &metav1.Time{Time: time.Now().UTC().Add(config.KubernetesVersionManagement.ExpirationDurationUnmaintainedVersion.Duration)}
	logger.Infof("[KUBERNETES VERSION MANAGEMENT] deprecating kubernetes version '%s' (unmaintained minor version) - expiration time set to: '%s'", currentVersion.Version, currentVersion.ExpirationDate.Round(time.Minute))
	return &currentVersion, nil
}
