// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DeletePriorNode deletes the Node object of the prior control plane Node that was lost during the disaster.
func (b *GardenadmBotanist) DeletePriorNode(ctx context.Context, priorNodeName string) error {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: priorNodeName}}
	b.Logger.Info("Deleting prior Node", "node", client.ObjectKeyFromObject(node))

	return kubernetesutils.DeleteObject(ctx, b.SeedClientSet.Client(), node)
}

// ForceDeletePriorNodePods force-deletes all Pods that were running on the prior control plane Node
// that was lost during the disaster.
func (b *GardenadmBotanist) ForceDeletePriorNodePods(ctx context.Context, priorNodeName string) error {
	podList := &corev1.PodList{}
	if err := b.SeedClientSet.Client().List(ctx, podList, client.MatchingFields{"spec.nodeName": priorNodeName}); err != nil {
		return fmt.Errorf("failed listing Pods: %w", err)
	}

	forceDeleteOptions := &client.DeleteOptions{
		GracePeriodSeconds: new(int64(0)),
		PropagationPolicy:  new(metav1.DeletePropagationBackground),
	}
	for _, pod := range podList.Items {
		b.Logger.Info("Force deleting Pod", "pod", client.ObjectKeyFromObject(&pod), "nodeName", pod.Spec.NodeName)
		if err := b.SeedClientSet.Client().Delete(ctx, &pod, forceDeleteOptions); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed force deleting Pod %s: %w", client.ObjectKeyFromObject(&pod), err)
		}
	}

	return nil
}
