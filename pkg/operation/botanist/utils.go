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

package botanist

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
)

// GetFailureToleranceType determines the FailureToleranceType by looking at both the alpha HA annotations and shoot spec ControlPlane
func GetFailureToleranceType(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.FailureToleranceType {
	if gardenletfeatures.FeatureGate.Enabled(features.HAControlPlanes) {
		if haAnnot, ok := shoot.Annotations[v1beta1constants.ShootAlphaControlPlaneHighAvailability]; ok {
			var failureToleranceType gardencorev1beta1.FailureToleranceType
			if haAnnot == v1beta1constants.ShootAlphaControlPlaneHighAvailabilityMultiZone {
				failureToleranceType = gardencorev1beta1.FailureToleranceTypeZone
			} else {
				failureToleranceType = gardencorev1beta1.FailureToleranceTypeNode
			}
			return &failureToleranceType
		}
		if shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil {
			return &shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.FailureToleranceType
		}
	}
	return nil
}
