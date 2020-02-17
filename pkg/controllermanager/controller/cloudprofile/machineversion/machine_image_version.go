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

package machineversion

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	controllermgrconfig "github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile/helper"
)

// ReconcileMachineImageVersions reconciles the versions of each machine image in the CloudProfile
// returns true if the CloudProfile has to be updated, the updated CloudProfile or an error
func ReconcileMachineImageVersions(logger *logrus.Entry, config *controllermgrconfig.CloudProfileControllerConfiguration, profile *gardencorev1beta1.CloudProfile) (*gardencorev1beta1.CloudProfile, error) {
	if config == nil || config.MachineImageVersionManagement == nil || config.MachineImageVersionManagement.Enabled == false {
		return profile, nil
	}

	for machineIndex, machineImage := range profile.Spec.MachineImages {
		supportedVersionsMap := make(map[string]helper.HighestSupportedVersion, len(profile.Spec.MachineImages[machineIndex].Versions))
		for versionIndex, currentImageVersion := range machineImage.Versions {
			currentSemverVersion, err := semver.NewVersion(currentImageVersion.Version)
			if err != nil {
				return profile, err
			}

			isLatestNonPreviewVersion, err := gardencorev1beta1helper.IsLatestMachineImageVersion(machineImage, currentImageVersion)
			if err != nil {
				return profile, err
			}

			if currentImageVersion.Classification == nil && isLatestNonPreviewVersion {
				// highest patch with no status -> supported
				logger.Infof("[MACHINE IMAGE VERSION MANAGEMENT] setting '%s' classification for machine image (%s, %s) that had no classification", gardencorev1beta1.ClassificationSupported, machineImage.Name, currentImageVersion.Version)
				profile.Spec.MachineImages[machineIndex].Versions[versionIndex] = helper.SetSupportedClassification(currentImageVersion)
				continue
			} else if currentImageVersion.Classification == nil {
				profile.Spec.MachineImages[machineIndex].Versions[versionIndex] = deprecateMachineImageVersion(logger, currentImageVersion, *config.MachineImageVersionManagement.ExpirationDuration, machineImage.Name)
				continue
			}

			// only keep one supported version
			if currentImageVersion.Classification != nil && *currentImageVersion.Classification == gardencorev1beta1.ClassificationSupported {
				key := getSupportedVersionsKey(*currentSemverVersion)
				currentlyHighestSupportedVersion, found := supportedVersionsMap[key]
				if !found {
					supportedVersionsMap[key] = helper.HighestSupportedVersion{
						Version: currentSemverVersion,
						Index:   versionIndex,
					}
					continue
				}
				if currentlyHighestSupportedVersion.Version.LessThan(currentSemverVersion) {
					deprecatedVersion := deprecateMachineImageVersion(logger, profile.Spec.MachineImages[machineIndex].Versions[currentlyHighestSupportedVersion.Index], *config.MachineImageVersionManagement.ExpirationDuration, machineImage.Name)
					if err != nil {
						return profile, err
					}
					profile.Spec.MachineImages[machineIndex].Versions[currentlyHighestSupportedVersion.Index] = deprecatedVersion
					supportedVersionsMap[key] = helper.HighestSupportedVersion{
						Version: currentSemverVersion,
						Index:   versionIndex,
					}
					continue
				}

				// deprecate current version, as there is already another higher supported version of the same minor
				deprecatedVersion := deprecateMachineImageVersion(logger, currentImageVersion, *config.MachineImageVersionManagement.ExpirationDuration, machineImage.Name)
				profile.Spec.MachineImages[machineIndex].Versions[versionIndex] = deprecatedVersion
				continue
			}
		}
	}
	return profile, nil
}

func deprecateMachineImageVersion(logger *logrus.Entry, version gardencorev1beta1.ExpirableVersion, expirationDuration metav1.Duration, machineImageName string) gardencorev1beta1.ExpirableVersion {
	add := time.Now().UTC().Add(expirationDuration.Duration)
	version.ExpirationDate = &metav1.Time{Time: add}
	logger.Infof("[MACHINE IMAGE VERSION MANAGEMENT] deprecating machine image (%s, %s) - expiration time set to: '%s'", machineImageName, version.Version, version.ExpirationDate.Round(time.Minute))
	return helper.SetDeprecatedClassification(version)
}

func getSupportedVersionsKey(version semver.Version) string {
	return fmt.Sprintf("%d.%d", version.Major(), version.Minor())
}
