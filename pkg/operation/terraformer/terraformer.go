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
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewFromOperation takes an <o> operation object and initializes the Terraformer, and a
// string <purpose> and returns an initialized Terraformer.
func NewFromOperation(o *operation.Operation, purpose string) *Terraformer {
	return New(o.Logger, o.K8sSeedClient, purpose, o.Shoot.Info.Name, o.Shoot.SeedNamespace, o.ImageVector)
}

// New takes a <logger>, a <k8sClient>, a string <purpose>, which describes for what the
// Terraformer is used, a <name>, a <namespace> in which the Terraformer will run, and the
// <image> name for the to-be-used Docker image. It returns a Terraformer struct with initialized
// values for the namespace and the names which will be used for all the stored resources like
// ConfigMaps/Secrets.
func New(logger *logrus.Entry, k8sClient kubernetes.Client, purpose, name, namespace string, imageVector imagevector.ImageVector) *Terraformer {
	var (
		prefix    = fmt.Sprintf("%s.%s", name, purpose)
		podSuffix = utils.ComputeSHA256Hex([]byte(time.Now().String()))[:5]
		image     string
	)
	if img, _ := imageVector.FindImage("terraformer", k8sClient.Version()); img != nil {
		image = img.String()
	}

	return &Terraformer{
		logger:        logger,
		k8sClient:     k8sClient,
		chartRenderer: chartrenderer.New(k8sClient),

		namespace: namespace,
		purpose:   purpose,
		image:     image,

		configName:    prefix + common.TerraformerConfigSuffix,
		variablesName: prefix + common.TerraformerVariablesSuffix,
		stateName:     prefix + common.TerraformerStateSuffix,
		podName:       fmt.Sprintf("%s-%s", prefix+common.TerraformerPodSuffix, podSuffix),
		jobName:       prefix + common.TerraformerJobSuffix,
	}
}

// Apply executes the Terraform Job by running the 'terraform apply' command.
func (t *Terraformer) Apply() error {
	if !t.configurationDefined {
		return errors.New("Terraformer configuration has not been defined, cannot execute the Terraform scripts")
	}
	return t.execute("apply")
}

// Destroy executes the Terraform Job by running the 'terraform destroy' command.
func (t *Terraformer) Destroy() error {
	if err := t.execute("destroy"); err != nil {
		return err
	}
	return t.cleanupConfiguration()
}

// execute creates a Terraform Job which runs the provided scriptName (apply or destroy), waits for the Job to be completed
// (either successful or not), prints its logs, deletes it and returns whether it was successful or not.
func (t *Terraformer) execute(scriptName string) error {
	var (
		exitCode  int32 = 1     // Exit code of the Terraform validation pod
		succeeded       = true  // Success status of the Terraform execution job
		execute         = false // Should we skip the rest of the function depending on whether all ConfigMaps/Secrets exist/do not exist?
		skipPod         = false // Should we skip the execution of the Terraform Pod (validation of the Terraform config)?
		skipJob         = false // Should we skip the execution of the Terraform Job (actual execution of the Terraform config)?
	)

	// We should retry the preparation check in order to allow the kube-apiserver to actually create the ConfigMaps.
	if err := utils.Retry(t.logger, 30*time.Second, func() (bool, error) {
		numberOfExistingResources, err := t.prepare()
		if err != nil {
			return false, err
		}
		if numberOfExistingResources == 0 {
			t.logger.Debug("All ConfigMaps/Secrets do not exist, can not execute the Terraform Job.")
			return true, nil
		} else if numberOfExistingResources == 3 {
			t.logger.Debug("All ConfigMaps/Secrets exist, will execute the Terraform Job.")
			execute = true
			return true, nil
		} else {
			t.logger.Error("Can not execute Terraform Job as ConfigMaps/Secrets are missing!")
			return false, nil
		}
	}); err != nil {
		return err
	}
	if !execute {
		return nil
	}

	// In case of scriptName == 'destroy', we need to first check whether the Terraform state contains
	// something at all. If it does not contain anything, then the 'apply' could never be executed, probably
	// because of syntax errors. In this case, we want to skip the Terraform job (as it wouldn't do anything
	// anyway) and just delete the related ConfigMaps/Secrets.
	if scriptName == "destroy" {
		skipPod = true
		skipJob = t.IsStateEmpty()
	}

	if t.image == "" {
		return fmt.Errorf("cannot execute Terraformer because the image has not been set")
	}

	values := map[string]interface{}{
		"images": map[string]interface{}{
			"terraformer": t.image,
		},
		"terraformVariablesEnvironment": t.variablesEnvironment,
		"names": map[string]interface{}{
			"configuration": t.configName,
			"variables":     t.variablesName,
			"state":         t.stateName,
			"pod":           t.podName,
			"job":           t.jobName,
		},
	}

	if !skipPod {
		values["kind"] = "Pod"
		values["script"] = "validate"

		// Create Terraform Pod which validates the Terraform configuration
		if err := t.deployTerraformer(values); err != nil {
			return err
		}

		// Wait for the Terraform validation Pod to be completed
		exitCode = t.waitForPod()
		skipJob = exitCode == 0 || exitCode == 1

		switch exitCode {
		case 0:
			t.logger.Debug("Terraform validation succeeded but there is no difference between state and actual resources.")
		case 1:
			t.logger.Debug("Terraform validation failed, will not start the job.")
			succeeded = false
		default:
			t.logger.Debug("Terraform validation has been successful.")
		}
	}

	if !skipJob {
		values["kind"] = "Job"
		values["script"] = scriptName

		// Create Terraform Job which executes the provided scriptName
		if err := t.deployTerraformer(values); err != nil {
			return fmt.Errorf("Failed to deploy the Terraformer: %s", err.Error())
		}

		// Wait for the Terraform Job to be completed
		succeeded = t.waitForJob()
		t.logger.Infof("Terraform '%s' finished.", t.jobName)
	}

	// Retrieve the logs of the Pods belonging to the completed Job
	jobPodList, err := t.ListJobPods()
	if err != nil {
		t.logger.Errorf("Could not retrieve list of pods belonging to Terraform job '%s': %s", t.jobName, err.Error())
		jobPodList = &corev1.PodList{}
	}

	t.logger.Infof("Fetching the logs for all pods belonging to the Terraform job '%s'...", t.jobName)
	logList, err := t.retrievePodLogs(jobPodList)
	if err != nil {
		t.logger.Errorf("Could not retrieve the logs of the pods belonging to Terraform job '%s': %s", t.jobName, err.Error())
		logList = map[string]string{}
	}
	for podName, podLogs := range logList {
		t.logger.Infof("Logs of Pod '%s' belonging to Terraform job '%s':\n%s", podName, t.jobName, podLogs)
	}

	// Delete the Terraform Job and all its belonging Pods
	t.logger.Infof("Cleaning up pods created by Terraform job '%s'...", t.jobName)
	if err := t.CleanupJob(jobPodList); err != nil {
		return err
	}

	// Evaluate whether the execution was successful or not
	t.logger.Infof("Terraformer execution for job '%s' has been completed.", t.jobName)
	if !succeeded {
		errorMessage := fmt.Sprintf("Terraform execution job '%s' could not be completed.", t.jobName)
		if terraformErrors := retrieveTerraformErrors(logList); terraformErrors != nil {
			errorMessage += fmt.Sprintf(" The following issues have been found in the logs:\n\n%s", strings.Join(terraformErrors, "\n\n"))
		}
		return common.DetermineErrorCode(errorMessage)
	}
	return nil
}

// deployTerraformer renders the Terraformer chart which contains the Job/Pod manifest.
func (t *Terraformer) deployTerraformer(values map[string]interface{}) error {
	chartName := "terraformer"
	return common.ApplyChart(t.k8sClient, t.chartRenderer, filepath.Join(chartPath, chartName), chartName, t.namespace, nil, values)
}

// ListJobPods lists all pods which have a label 'job-name' whose value is equal to the Terraformer job name.
func (t *Terraformer) ListJobPods() (*corev1.PodList, error) {
	return t.k8sClient.ListPods(t.namespace, metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", t.jobName)})
}

// retrievePodLogs fetches the logs of the created Pods by the Terraform Job and returns them as a map whose
// keys are pod names and whose values are the corresponding logs.
func (t *Terraformer) retrievePodLogs(jobPodList *corev1.PodList) (map[string]string, error) {
	logChan := make(chan map[string]string, 1)
	go func() {
		var logList = map[string]string{}
		for _, jobPod := range jobPodList.Items {
			name := jobPod.Name
			logsBuffer, err := t.k8sClient.GetPodLogs(jobPod.Namespace, name, &corev1.PodLogOptions{})
			if err != nil {
				t.logger.Warnf("Could not retrieve the logs of Terraform job pod %s: '%v'", name, err)
				continue
			}
			logList[name] = logsBuffer.String()
		}
		logChan <- logList
	}()

	select {
	case result := <-logChan:
		return result, nil
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("Timeout when reading the logs of all pds created by Terraform job '%s'", t.jobName)
	}
}

// CleanupJob deletes the Terraform Job and all belonging Pods from the Garden cluster.
func (t *Terraformer) CleanupJob(jobPodList *corev1.PodList) error {
	// Delete the Terraform Job
	if err := t.k8sClient.DeleteJob(t.namespace, t.jobName); err == nil {
		t.logger.Infof("Deleted Terraform Job '%s'", t.jobName)
	} else {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Delete the belonging Terraform Pods
	for _, jobPod := range jobPodList.Items {
		if err := t.k8sClient.DeletePod(jobPod.Namespace, jobPod.Name); err == nil {
			t.logger.Infof("Deleted Terraform Job Pod '%s'", jobPod.Name)
		} else {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}
