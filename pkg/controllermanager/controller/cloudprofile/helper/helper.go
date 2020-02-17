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
	"fmt"

	"github.com/Masterminds/semver"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// HighestSupportedVersion is a helper struct to map a version with to its index in a for-loop
type HighestSupportedVersion struct {
	Version *semver.Version
	Index   int
}

// GetSupportedVersionsKey returns a string containing the major and minor version as a standardised key
func GetSupportedVersionsKey(version semver.Version) string {
	return fmt.Sprintf("%d.%d", version.Major(), version.Minor())
}

// SetSupportedClassification sets the 'supported' classification to a version
func SetSupportedClassification(version gardencorev1beta1.ExpirableVersion) gardencorev1beta1.ExpirableVersion {
	return setClassification(version, gardencorev1beta1.ClassificationSupported)
}

// SetDeprecatedClassification sets the 'deprecated' classification to a version
func SetDeprecatedClassification(version gardencorev1beta1.ExpirableVersion) gardencorev1beta1.ExpirableVersion {
	return setClassification(version, gardencorev1beta1.ClassificationDeprecated)
}

func setClassification(version gardencorev1beta1.ExpirableVersion, classification gardencorev1beta1.VersionClassification) gardencorev1beta1.ExpirableVersion {
	version.Classification = &classification
	return version
}
