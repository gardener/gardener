// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package terraformer

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// WaitForCleanEnvironment waits until no Terraform Pod(s) exist for the current instance
// of the Terraformer.
func (t *terraformer) WaitForCleanEnvironment(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, t.deadlineCleaning)
	defer cancel()

	t.logger.Info("Waiting for clean environment")
	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		podList, err := t.listTerraformerPods(ctx)
		if err != nil {
			return retry.SevereError(err)
		}
		if len(podList.Items) > 0 {
			t.logger.Info("Waiting until all Terraformer Pods have been cleaned up")
			return retry.MinorError(fmt.Errorf("at least one terraformer pod still exists: %s", podList.Items[0].Name))
		}

		return retry.Ok()
	})
}

// waitForPod waits for the Terraform Pod to be completed (either successful or failed).
// It checks the Pod status field to identify the state.
func (t *terraformer) waitForPod(ctx context.Context, logger logr.Logger, pod *corev1.Pod, deadline time.Duration) int32 {
	// 'terraform plan' returns exit code 2 if the plan succeeded and there is a diff
	// If we can't read the terminated state of the container we simply force that the Terraform
	// job gets created.
	var exitCode int32 = 2
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	logger = logger.WithValues("pod", kutil.KeyFromObject(pod))

	logger.Info("Waiting for Terraformer Pod to be completed...")
	if err := retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		err = t.client.Get(ctx, kutil.KeyFromObject(pod), pod)
		if apierrors.IsNotFound(err) {
			logger.Info("Terraformer Pod disappeared unexpectedly, somebody must have manually deleted it")
			return retry.Ok()
		}
		if err != nil {
			logger.Error(err, "Error retrieving Pod")
			return retry.SevereError(err)
		}

		// Check whether the Pod has been successful
		var (
			phase             = pod.Status.Phase
			containerStatuses = pod.Status.ContainerStatuses
		)

		if (phase == corev1.PodSucceeded || phase == corev1.PodFailed) && len(containerStatuses) > 0 {
			if containerStateTerminated := containerStatuses[0].State.Terminated; containerStateTerminated != nil {
				exitCode = containerStateTerminated.ExitCode
			}
			return retry.Ok()
		}

		logger.Info("Waiting for terraformer pod to be completed, pod hasn't finished yet", "phase", phase, "len-of-containerstatuses", len(containerStatuses))
		return retry.MinorError(fmt.Errorf("pod was not successful: phase=%s, len-of-containerstatuses=%d", phase, len(containerStatuses)))
	}); err != nil {
		exitCode = 1
	}

	return exitCode
}
