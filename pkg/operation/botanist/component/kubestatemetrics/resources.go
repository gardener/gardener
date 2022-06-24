// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubestatemetrics

import (
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func (k *kubeStateMetrics) getResourceConfigs() component.ResourceConfigs {
	configs := component.ResourceConfigs{}

	if k.values.ClusterType == component.ClusterTypeSeed {
		serviceAccount := k.emptyServiceAccount()

		configs = append(configs, component.ResourceConfig{
			Obj: serviceAccount, Class: component.Runtime, MutateFn: func() { k.reconcileServiceAccount(serviceAccount) },
		})
	}

	return configs
}

func (k *kubeStateMetrics) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "kube-state-metrics", Namespace: k.namespace}}
}

func (k *kubeStateMetrics) reconcileServiceAccount(serviceAccount *corev1.ServiceAccount) {
	serviceAccount.Labels = k.getLabels()
	serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
}

func (k *kubeStateMetrics) getLabels() map[string]string {
	t := "seed"
	if k.values.ClusterType == component.ClusterTypeShoot {
		t = "shoot"
	}

	return map[string]string{
		labelKeyComponent: labelValueComponent,
		labelKeyType:      t,
	}
}
