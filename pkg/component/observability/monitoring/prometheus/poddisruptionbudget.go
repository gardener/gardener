// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func (p *prometheus) podDisruptionBudget() *policyv1.PodDisruptionBudget {
	if p.values.Replicas <= 1 || p.values.RuntimeVersion == nil {
		return nil
	}

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name(),
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable:             ptr.To(intstr.FromInt32(1)),
			Selector:                   &metav1.LabelSelector{MatchLabels: p.getLabels()},
			UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
		},
	}

	return pdb
}
