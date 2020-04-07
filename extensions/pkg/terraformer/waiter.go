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

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// waitForCleanEnvironment waits until no Terraform Pod(s) exist for the current instance
// of the Terraformer.
func (t *terraformer) waitForCleanEnvironment(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, t.deadlineCleaning)
	defer cancel()

	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		podList, err := t.listTerraformerPods(ctx)
		if err != nil {
			return retry.SevereError(err)
		}
		if len(podList.Items) != 0 {
			labels := fmt.Sprintf("%s=%s,%s=%s", TerraformerLabelKeyName, t.name, TerraformerLabelKeyPurpose, t.purpose)
			t.logger.Infof("Waiting until no Terraform Pods with labels '%s' exist any more in namespace '%s'...", labels, t.namespace)
			return retry.MinorError(fmt.Errorf("terraform pods with labels '%s' still exist in namespace '%s'", labels, t.namespace))
		}

		return retry.Ok()
	})
}

// waitForPod waits for the Terraform Pod to be completed (either successful or failed).
// It checks the Pod status field to identify the state.
func (t *terraformer) waitForPod(ctx context.Context, podName string, deadline time.Duration) int32 {
	// 'terraform plan' returns exit code 2 if the plan succeeded and there is a diff
	// If we can't read the terminated state of the container we simply force that the Terraform
	// job gets created.
	var exitCode int32 = 2
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	if err := retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		t.logger.Infof("Waiting for Terraform Pod '%s' to be completed...", podName)
		pod := &corev1.Pod{}
		err = t.client.Get(ctx, kutil.Key(t.namespace, podName), pod)
		if apierrors.IsNotFound(err) {
			t.logger.Warnf("Terraform Pod '%s' disappeared unexpectedly, somebody must have manually deleted it!", podName)
			return retry.Ok()
		}
		if err != nil {
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

		return retry.MinorError(fmt.Errorf("pod was not successful (phase=%s, no-of-container-states=%d)", phase, len(containerStatuses)))
	}); err != nil {
		exitCode = 1
	}

	return exitCode
}
