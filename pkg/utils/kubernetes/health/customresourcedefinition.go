// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
