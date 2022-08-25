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
	"github.com/gardener/gardener/pkg/utils/version"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (k *kubeAPIServer) getPodDisruptionBudget() client.Object {
	pdbMaxUnavailable := intstr.FromInt(1)
	pdbObjectMeta := metav1.ObjectMeta{
		Name:      v1beta1constants.DeploymentNameKubeAPIServer,
		Namespace: k.namespace,
		Labels:    getLabels(),
	}
	pdbSelector := &metav1.LabelSelector{MatchLabels: getLabels()}

	if version.ConstraintK8sGreaterEqual121.Check(k.values.Version) {
		return &policyv1.PodDisruptionBudget{
			ObjectMeta: pdbObjectMeta,
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &pdbMaxUnavailable,
				Selector:       pdbSelector,
			},
		}
	}
	return &policyv1beta1.PodDisruptionBudget{
		ObjectMeta: pdbObjectMeta,
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			MaxUnavailable: &pdbMaxUnavailable,
			Selector:       pdbSelector,
		},
	}
}

func (k *kubeAPIServer) reconcilePodDisruptionBudget(ctx context.Context, podDisruptionBudget client.Object) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), podDisruptionBudget, func() error {
		return nil
	})
	return err
}
