// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package kubeapiserver

import (
	"context"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
)

func (k *kubeAPIServer) emptyPodDisruptionBudget() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcilePodDisruptionBudget(ctx context.Context, pdb *policyv1.PodDisruptionBudget) error {
	var pdbMaxUnavailable = intstr.FromInt32(1)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), pdb, func() error {
		pdb.Labels = getLabels()
		pdb.Spec = policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: &pdbMaxUnavailable,
			Selector:       &metav1.LabelSelector{MatchLabels: getLabels()},
		}
		return nil
	})

	return err
}
