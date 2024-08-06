// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import corev1 "k8s.io/api/core/v1"

// MergeMinValuesIntoResourceList merges the values from extraValues into mainList. Collisions are resolved by taking the
// smaller value.
//
// For keys in extraValues which are missing from mainList, the values are inserted into mainList.
// For keys which have a value in both lists, mainList ends up containing the smaller of the two values.
// Values in main list, which do not have a correspondent in extraValues, are retained in mainList.
func MergeMinValuesIntoResourceList(extraValues, mainList corev1.ResourceList) {
	for key, extraValue := range extraValues {
		mainValue, ok := mainList[key]
		if !ok || mainValue.Cmp(extraValue) > 0 {
			mainList[key] = extraValue
		}
	}
}
