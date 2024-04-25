// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (g *gardenerScheduler) podDisruptionBudget() *policyv1.PodDisruptionBudget {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: g.namespace,
			Labels:    GetLabels(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: ptr.To(intstr.FromInt32(1)),
			Selector:       &metav1.LabelSelector{MatchLabels: GetLabels()},
		},
	}

	kubernetesutils.SetAlwaysAllowEviction(pdb, g.values.RuntimeVersion)

	return pdb
}
