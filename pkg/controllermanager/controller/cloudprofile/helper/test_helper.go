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

package helper

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var (
	deprecated = gardencorev1beta1.ClassificationDeprecated
	preview    = gardencorev1beta1.ClassificationPreview
	supported  = gardencorev1beta1.ClassificationSupported

	dateInFuture = &metav1.Time{Time: time.Now().UTC().Add(time.Hour * 2)}
)

// GetVersions is a helper to return a slice of versions
func GetVersions(version ...gardencorev1beta1.ExpirableVersion) []gardencorev1beta1.ExpirableVersion {
	return version
}

// GetDeprecatedVersionNotExpired is a helper to return a 'deprecated' version that is not expired
func GetDeprecatedVersionNotExpired(version string) gardencorev1beta1.ExpirableVersion {
	return GetDeprecatedVersion(version, dateInFuture)
}

// GetDeprecatedVersion is a helper to return a 'deprecated' version
func GetDeprecatedVersion(version string, expiration *metav1.Time) gardencorev1beta1.ExpirableVersion {
	return gardencorev1beta1.ExpirableVersion{
		Version:        version,
		Classification: &deprecated,
		ExpirationDate: expiration,
	}
}

// GetVersionWithPreviewClassification is a helper to return a version with a 'preview' classification
func GetVersionWithPreviewClassification(version string) gardencorev1beta1.ExpirableVersion {
	return gardencorev1beta1.ExpirableVersion{
		Version:        version,
		Classification: &preview,
	}
}

// GetVersionWithSupportedClassification is a helper to return a version with a 'supported' classification
func GetVersionWithSupportedClassification(version string) gardencorev1beta1.ExpirableVersion {
	return gardencorev1beta1.ExpirableVersion{
		Version:        version,
		Classification: &supported,
	}
}

// GetVersionWithNoStatus is a helper to return a version with a no classification
func GetVersionWithNoStatus(version string) gardencorev1beta1.ExpirableVersion {
	return gardencorev1beta1.ExpirableVersion{
		Version: version,
	}
}

// SanitizeTimestampsForTesting rounds all the timestamps in a version to the hour in order to make it comparable during test execution
func SanitizeTimestampsForTesting(versions []gardencorev1beta1.ExpirableVersion) []gardencorev1beta1.ExpirableVersion {
	var updatedVersions []gardencorev1beta1.ExpirableVersion
	for _, version := range versions {
		version.ExpirationDate = roundToHour(version.ExpirationDate)
		updatedVersions = append(updatedVersions, version)
	}
	return updatedVersions
}

func roundToHour(toRound *metav1.Time) *metav1.Time {
	if toRound == nil {
		return nil
	}
	rounded := time.Date(toRound.Year(), toRound.Month(), toRound.Day(), toRound.Hour(), 0, 0, 0, toRound.Location())
	return &metav1.Time{Time: rounded}
}
