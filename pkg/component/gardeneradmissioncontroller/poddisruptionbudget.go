// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package gardeneradmissioncontroller

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardenerutils "github.com/gardener/gardener/pkg/utils"
)

func (a *gardenerAdmissionController) podDisruptionBudget() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: a.namespace,
			Labels:    GetLabels(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: gardenerutils.IntStrPtrFromInt32(1),
			Selector:       &metav1.LabelSelector{MatchLabels: GetLabels()},
		},
	}
}
