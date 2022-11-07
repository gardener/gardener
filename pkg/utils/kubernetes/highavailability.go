// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// GetReplicaCount returns the replica count based on the criteria, failure tolerance type, and component type.
func GetReplicaCount(criteria string, failureToleranceType *gardencorev1beta1.FailureToleranceType, componentType string) *int32 {
	if len(componentType) == 0 {
		return nil
	}

	switch criteria {
	case resourcesv1alpha1.HighAvailabilityConfigCriteriaZones:
		return pointer.Int32(2)

	case resourcesv1alpha1.HighAvailabilityConfigCriteriaFailureToleranceType:
		if componentType == resourcesv1alpha1.HighAvailabilityConfigTypeController &&
			(failureToleranceType == nil || *failureToleranceType == "") {
			return pointer.Int32(1)
		}
		return pointer.Int32(2)
	}

	return nil
}
