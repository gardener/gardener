// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"github.com/gardener/gardener/pkg/apis/core"
)

// GetConditionIndex returns the index of the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns -1.
func GetConditionIndex(conditions []core.Condition, conditionType core.ConditionType) int {
	for index, condition := range conditions {
		if condition.Type == conditionType {
			return index
		}
	}
	return -1
}

// GetCondition returns the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns nil.
func GetCondition(conditions []core.Condition, conditionType core.ConditionType) *core.Condition {
	if index := GetConditionIndex(conditions, conditionType); index != -1 {
		return &conditions[index]
	}
	return nil
}

// DeterminePrimaryIPFamily determines the primary IP family out of a specified list of IP families.
func DeterminePrimaryIPFamily(ipFamilies []core.IPFamily) core.IPFamily {
	if len(ipFamilies) == 0 {
		return core.IPFamilyIPv4
	}
	return ipFamilies[0]
}
