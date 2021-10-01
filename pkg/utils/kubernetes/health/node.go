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
)

func getNodeCondition(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

var (
	trueNodeConditionTypes = []corev1.NodeConditionType{
		corev1.NodeReady,
	}

	falseNodeConditionTypes = []corev1.NodeConditionType{
		corev1.NodeDiskPressure,
		corev1.NodeMemoryPressure,
		corev1.NodeNetworkUnavailable,
		corev1.NodePIDPressure,
	}
)

// CheckNode checks whether the given Node is healthy.
// A node is considered healthy if it has a `corev1.NodeReady` condition and this condition reports
// `corev1.ConditionTrue`.
func CheckNode(node *corev1.Node) error {
	for _, trueConditionType := range trueNodeConditionTypes {
		conditionType := string(trueConditionType)
		condition := getNodeCondition(node.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(string(condition.Type), string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseConditionType := range falseNodeConditionTypes {
		condition := getNodeCondition(node.Status.Conditions, falseConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(string(condition.Type), string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}
