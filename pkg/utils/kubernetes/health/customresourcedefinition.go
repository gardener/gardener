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
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var (
	trueCrdConditionTypes          = []apiextensionsv1.CustomResourceDefinitionConditionType{apiextensionsv1.NamesAccepted, apiextensionsv1.Established}
	falseOptionalCrdConditionTypes = []apiextensionsv1.CustomResourceDefinitionConditionType{apiextensionsv1.Terminating}
)

// CheckCustomResourceDefinition checks whether the given CustomResourceDefinition is healthy.
// A CRD is considered healthy if its `NamesAccepted` and `Established` conditions are with status `True`
// and its `Terminating` condition is missing or has status `False`.
func CheckCustomResourceDefinition(crd *apiextensionsv1.CustomResourceDefinition) error {
	for _, trueConditionType := range trueCrdConditionTypes {
		conditionType := string(trueConditionType)
		condition := getCustomResourceDefinitionCondition(crd.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseOptionalConditionType := range falseOptionalCrdConditionTypes {
		conditionType := string(falseOptionalConditionType)
		condition := getCustomResourceDefinitionCondition(crd.Status.Conditions, falseOptionalConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(conditionType, string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

func getCustomResourceDefinitionCondition(conditions []apiextensionsv1.CustomResourceDefinitionCondition, conditionType apiextensionsv1.CustomResourceDefinitionConditionType) *apiextensionsv1.CustomResourceDefinitionCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}
