// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeobjects

import (
	"github.com/Masterminds/semver/v3"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func makePDB(namespace string, kubernetesVersion *semver.Version) *policyv1.PodDisruptionBudget {
	labels := map[string]string{
		"gardener.cloud/role": gcmxBaseName,
	}

	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app":                 gcmxBaseName,
			"gardener.cloud/role": gcmxBaseName,
		},
	}

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gcmxBaseName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			Selector:       selector,
		},
	}

	kubernetesutils.SetAlwaysAllowEviction(pdb, kubernetesVersion)

	return pdb
}
