// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

// CheckAPIService checks whether the given APIService is healthy.
// An APIService is considered healthy if it has the `Available` condition and its status is `True`.
func CheckAPIService(apiService *apiregistrationv1.APIService) error {
	const (
		requiredCondition       = apiregistrationv1.Available
		requiredConditionStatus = apiregistrationv1.ConditionTrue
	)

	for _, condition := range apiService.Status.Conditions {
		if condition.Type == requiredCondition {
			return checkConditionState(
				string(condition.Type),
				string(requiredConditionStatus),
				string(condition.Status),
				condition.Reason,
				condition.Message,
			)
		}
	}
	return requiredConditionMissing(string(requiredCondition))
}
