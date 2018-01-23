// Copyright 2018 The Gardener Authors.
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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

// waitForCleanEnvironment waits until no Terraform Job and Pod(s) exist for the current instance
// of the Terraformer.
func (t *Terraformer) waitForCleanEnvironment() error {
	return wait.PollImmediate(5*time.Second, 120*time.Second, func() (bool, error) {
		_, err := t.K8sSeedClient.GetJob(t.Namespace, t.JobName)
		if !apierrors.IsNotFound(err) {
			if err != nil {
				return false, err
			}
			t.Logger.Infof("Waiting until no Terraform Job with name '%s' exist any more...", t.JobName)
			return false, nil
		}

		jobPodList, err := t.listJobPods()
		if err != nil {
			return false, err
		}
		if len(jobPodList.Items) != 0 {
			t.Logger.Infof("Waiting until no Terraform Pods with label 'job-name=%s' exist any more...", t.JobName)
			return false, nil
		}

		return true, nil
	})
}

// waitForPod waits for the Terraform validation Pod to be completed (either successful or failed).
// It checks the Pod status field to identify the state.
func (t *Terraformer) waitForPod() int32 {
	// 'terraform plan' returns exit code 2 if the plan succeeded and there is a diff
	// If we can't read the terminated state of the container we simply force that the Terraform
	// job gets created.
	var exitCode int32 = 2

	wait.PollImmediate(5*time.Second, 120*time.Second, func() (bool, error) {
		t.Logger.Infof("Waiting for Terraform validation Pod '%s' to be completed...", t.PodName)
		pod, err := t.K8sSeedClient.GetPod(t.Namespace, t.PodName)
		if apierrors.IsNotFound(err) {
			t.Logger.Warn("Terraform validation Pod disappeared unexpectedly, somebody must have manually deleted it!")
			return true, nil
		}
		if err != nil {
			exitCode = 1 // 'terraform plan' exit code for "errors"
			return false, err
		}
		// Check whether the Job has been successful (at least one succeeded Pod)
		phase := pod.Status.Phase
		if phase == corev1.PodSucceeded || phase == corev1.PodFailed {
			containerStateTerminated := pod.Status.ContainerStatuses[0].State.Terminated
			if containerStateTerminated != nil {
				exitCode = containerStateTerminated.ExitCode
			}
			return true, nil
		}
		return false, nil
	})
	return exitCode
}

// waitForJob waits for the Terraform Job to be completed (either successful or failed). It checks the
// Job status field to identify the state.
func (t *Terraformer) waitForJob() bool {
	var succeeded = false
	wait.PollImmediate(5*time.Second, 3600*time.Second, func() (bool, error) {
		t.Logger.Infof("Waiting for Terraform Job '%s' to be completed...", t.JobName)
		job, err := t.K8sSeedClient.GetJob(t.Namespace, t.JobName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logger.Warn("Terraform Job disappeared unexpectedly, somebody must have manually deleted it!")
				return true, nil
			}
			return false, err
		}
		// Check whether the Job has been successful (at least one succeeded Pod)
		if job.Status.Succeeded >= 1 {
			succeeded = true
			return true, nil
		}
		// Check whether the Job is still running at all
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete || cond.Type == batchv1.JobFailed {
				return true, nil
			}
		}
		return false, nil
	})
	return succeeded
}
