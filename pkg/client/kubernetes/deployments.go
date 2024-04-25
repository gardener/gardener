// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/retry"
)

// HasDeploymentRolloutCompleted checks for the number of updated &
// available replicas to be equal to the deployment's desired replicas count.
// Thus confirming a successful rollout of the deployment.
func HasDeploymentRolloutCompleted(ctx context.Context, c client.Client, namespace, name string) (bool, error) {
	var (
		deployment      = &appsv1.Deployment{}
		desiredReplicas = int32(0)
	)

	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
		return retry.SevereError(err)
	}

	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}

	if deployment.Generation != deployment.Status.ObservedGeneration {
		return retry.MinorError(fmt.Errorf("%q not observed at latest generation (%d/%d)", name,
			deployment.Status.ObservedGeneration, deployment.Generation))
	}

	if deployment.Status.Replicas == desiredReplicas && deployment.Status.UpdatedReplicas == desiredReplicas && deployment.Status.AvailableReplicas == desiredReplicas {
		return retry.Ok()
	}

	return retry.MinorError(fmt.Errorf("deployment %q currently has Updated/Available: %d/%d replicas. Desired: %d", name, deployment.Status.UpdatedReplicas, deployment.Status.AvailableReplicas, desiredReplicas))
}

// WaitUntilDeploymentRolloutIsComplete waits for the number of updated &
// available replicas to be equal to the deployment's desired replicas count.
// It keeps retrying until timeout
func WaitUntilDeploymentRolloutIsComplete(ctx context.Context, client client.Client, namespace string, name string, interval, timeout time.Duration) error {
	return retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		return HasDeploymentRolloutCompleted(ctx, client, namespace, name)
	})
}
