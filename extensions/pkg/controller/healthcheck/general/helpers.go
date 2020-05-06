// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package general

import (
	"fmt"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
)

var (
	trueManagedResourceConditionTypes = []resourcesv1alpha1.ConditionType{
		resourcesv1alpha1.ResourcesApplied,
		resourcesv1alpha1.ResourcesHealthy,
	}
)

func getManagedResourceCondition(conditions []resourcesv1alpha1.ManagedResourceCondition, conditionType resourcesv1alpha1.ConditionType) *resourcesv1alpha1.ManagedResourceCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

func requiredConditionMissing(conditionType string) error {
	return fmt.Errorf("condition %q is missing", conditionType)
}

func checkConditionState(conditionType string, expected, actual, reason, message string) error {
	if expected != actual {
		return fmt.Errorf("condition %q has invalid status %s (expected %s) due to %s: %s",
			conditionType, actual, expected, reason, message)
	}
	return nil
}
