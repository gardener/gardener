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

package deployment

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (k *kubeAPIServer) deployPodDisruptionBudget(ctx context.Context) error {
	var (
		oneMaxUnavailable = intstr.FromInt(1)
		pdb               = k.emptyPodDisruptionBudget()
	)

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), pdb, func() error {
		pdb.Labels = map[string]string{
			v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
			v1beta1constants.LabelRole: labelRole,
		}
		pdb.Spec = policyv1beta1.PodDisruptionBudgetSpec{
			MaxUnavailable: &oneMaxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: getAPIServerPodLabels(),
			},
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (k *kubeAPIServer) emptyPodDisruptionBudget() *policyv1beta1.PodDisruptionBudget {
	return &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: k.seedNamespace}}
}
