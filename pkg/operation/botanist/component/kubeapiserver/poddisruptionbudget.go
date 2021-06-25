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

package kubeapiserver

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"

	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (k *kubeAPIServer) emptyPodDisruptionBudget() *policyv1beta1.PodDisruptionBudget {
	return &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcilePodDisruptionBudget(ctx context.Context, podDisruptionBudget *policyv1beta1.PodDisruptionBudget) error {
	pdbMaxUnavailable := intstr.FromInt(1)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), podDisruptionBudget, func() error {
		podDisruptionBudget.Labels = getLabels()
		podDisruptionBudget.Spec = policyv1beta1.PodDisruptionBudgetSpec{
			MaxUnavailable: &pdbMaxUnavailable,
			Selector:       &metav1.LabelSelector{MatchLabels: getLabels()},
		}
		return nil
	})
	return err
}
