// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health

import (
	"fmt"

	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	landscaperv1alpha1helper "github.com/gardener/landscaper/apis/core/v1alpha1/helper"
)

var (
	installationConditionTypesTrue = []landscaperv1alpha1.ConditionType{
		landscaperv1alpha1.EnsureSubInstallationsCondition,
		landscaperv1alpha1.ReconcileExecutionCondition,
		landscaperv1alpha1.CreateImportsCondition,
		landscaperv1alpha1.CreateExportsCondition,
		landscaperv1alpha1.EnsureExecutionsCondition,
		landscaperv1alpha1.ValidateExportCondition,
	}

	installationConditionTypesFalse = []landscaperv1alpha1.ConditionType{
		landscaperv1alpha1.ValidateImportsCondition,
	}
)

// CheckInstallation checks if the given Installation is healthy
// An installation is healthy if
// * Its observed generation is up-to-date
// * No annotation landscaper.gardener.cloud/operation is set
// * No lastError is in the status
// * A last operation is state succeeded is present
// * landscaperv1alpha1.ComponentPhaseSucceeded
func CheckInstallation(installation *landscaperv1alpha1.Installation) error {
	if installation.Status.ObservedGeneration < installation.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", installation.Status.ObservedGeneration, installation.Generation)
	}

	if installation.Status.LastError != nil {
		return fmt.Errorf("last errors is set: %s", installation.Status.LastError.Message)
	}

	if installation.Status.Phase != landscaperv1alpha1.ComponentPhaseSucceeded {
		return fmt.Errorf("installation phase is not suceeded, but %s", installation.Status.Phase)
	}

	if op, ok := installation.Annotations[landscaperv1alpha1.OperationAnnotation]; ok {
		return fmt.Errorf("landscaper operation %q is not yet picked up by controller", op)
	}

	for _, conditionType := range installationConditionTypesTrue {
		condition := landscaperv1alpha1helper.GetCondition(installation.Status.Conditions, conditionType)
		if condition == nil {
			// conditions vary based on what is configured in the installation
			continue
		}

		if err := checkConditionState(string(conditionType), string(landscaperv1alpha1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, conditionType := range installationConditionTypesFalse {
		condition := landscaperv1alpha1helper.GetCondition(installation.Status.Conditions, conditionType)
		if condition == nil {
			continue
		}

		if err := checkConditionState(string(conditionType), string(landscaperv1alpha1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

func checkConditionState(conditionType string, expected, actual, reason, message string) error {
	if expected != actual {
		return fmt.Errorf("condition %q has invalid status %s (expected %s) due to %s: %s",
			conditionType, actual, expected, reason, message)
	}
	return nil
}
