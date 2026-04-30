// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package calicoselfhostedshoot

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type mutator struct {
	client client.Client
}

func (m *mutator) Mutate(ctx context.Context, newObj, _ client.Object) error {
	if newObj.GetDeletionTimestamp() != nil {
		return nil
	}

	daemonSet, ok := newObj.(*appsv1.DaemonSet)
	if !ok {
		return fmt.Errorf("expected DaemonSet, got %T", newObj)
	}

	cluster, err := gardenerextensions.GetCluster(ctx, m.client, metav1.NamespaceSystem)
	if err != nil {
		return fmt.Errorf("failed reading Cluster resource: %w", err)
	}
	if cluster.Shoot != nil && v1beta1helper.HasManagedInfrastructure(cluster.Shoot) {
		return nil
	}

	return kubernetesutils.VisitPodSpec(daemonSet, func(podSpec *corev1.PodSpec) {
		kubernetesutils.VisitContainers(podSpec, func(container *corev1.Container) {
			// Restrict the default IPPool to 10.1.0.0/16 (a /16 subset of the full 10.0.0.0/15 pod network).
			// This leaves 10.0.0.0/16 free for dedicated shoot machine pod IPPools created by the Infrastructure controller.
			kubernetesutils.AddEnvVar(container, corev1.EnvVar{
				Name:  "CALICO_IPV4POOL_CIDR",
				Value: "10.1.0.0/16",
			}, true)
		}, "calico-node")
	})
}
