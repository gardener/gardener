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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// waitForCleanEnvironment waits until no Terraform Job and Pod(s) exist for the current instance
// of the Terraformer.
func (t *Terraformer) waitForCleanEnvironment(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, t.deadlineCleaning)
	defer cancel()

	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		err = t.client.Get(ctx, kutil.Key(t.namespace, t.jobName), &batchv1.Job{})
		if !apierrors.IsNotFound(err) {
			if err != nil {
				return retry.SevereError(err)
			}
			t.logger.Infof("Waiting until no Terraform Job with name '%s' exist any more...", t.jobName)
			return retry.MinorError(fmt.Errorf("terraform job %q still exists", t.jobName))
		}

		jobPodList, err := t.listJobPods(ctx)
		if err != nil {
			return retry.SevereError(err)
		}
		if len(jobPodList.Items) != 0 {
			t.logger.Infof("Waiting until no Terraform Pods with label 'job-name=%s' exist any more...", t.jobName)
			return retry.MinorError(fmt.Errorf("terraform pods with label 'job-name%s' still exist", t.jobName))
		}

		return retry.Ok()
	})
}

// waitForPod waits for the Terraform validation Pod to be completed (either successful or failed).
// It checks the Pod status field to identify the state.
func (t *Terraformer) waitForPod(ctx context.Context) int32 {
	// 'terraform plan' returns exit code 2 if the plan succeeded and there is a diff
	// If we can't read the terminated state of the container we simply force that the Terraform
	// job gets created.
	var exitCode int32 = 2
	ctx, cancel := context.WithTimeout(ctx, t.deadlinePod)
	defer cancel()

	if err := retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		t.logger.Infof("Waiting for Terraform validation Pod '%s' to be completed...", t.podName)
		pod := &corev1.Pod{}
		err = t.client.Get(ctx, kutil.Key(t.namespace, t.podName), pod)
		if apierrors.IsNotFound(err) {
			t.logger.Warn("Terraform validation Pod disappeared unexpectedly, somebody must have manually deleted it!")
			return retry.Ok()
		}
		if err != nil {
			return retry.SevereError(err)
		}

		// Check whether the Job has been successful (at least one succeeded Pod)
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

		return retry.MinorError(fmt.Errorf("job was not successful (phase=%s, no-of-container-states=%d)", phase, len(containerStatuses)))
	}); err != nil {
		exitCode = 1
	}

	return exitCode
}

// waitForJob waits for the Terraform Job to be completed (either successful or failed). It checks the
// Job status field to identify the state.
func (t *Terraformer) waitForJob(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, t.deadlineJob)
	defer cancel()

	var succeeded = false
	if err := retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		t.logger.Infof("Waiting for Terraform Job '%s' to be completed...", t.jobName)
		job := &batchv1.Job{}
		err = t.client.Get(ctx, kutil.Key(t.namespace, t.jobName), job)
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.logger.Warnf("Terraform Job %s disappeared unexpectedly, somebody must have manually deleted it!", t.jobName)
				return retry.Ok()
			}
			return retry.SevereError(err)
		}
		// Check the job conditions to identify whether the job has been completed or failed.
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
				succeeded = true
				return retry.Ok()
			}
			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				t.logger.Errorf("Terraform Job %s failed for reason '%s': '%s'", t.jobName, cond.Reason, cond.Message)
				return retry.Ok()
			}
		}
		return retry.MinorError(fmt.Errorf("job %q is not yet completed", t.jobName))
	}); err != nil {
		t.logger.Errorf("Error while waiting for Terraform job: '%s'", err.Error())
	}
	return succeeded
}
