// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/utils/retry"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
