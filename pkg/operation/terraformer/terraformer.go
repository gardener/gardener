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

	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New takes a Operation object <o> and a string <purpose> which describes for what the
// Terraformer is used, and returns a Terraformer struct with initialized values for the namespace
// and the names which will be used for all the stored resources like ConfigMaps/Secrets.
func New(o *operation.Operation, purpose string) *Terraformer {
	prefix := fmt.Sprintf("%s.%s", o.Shoot.Info.Name, purpose)

	if err := o.InitializeSeedClients(); err != nil {
		return nil
	}

	return &Terraformer{
		Operation:     o,
		Namespace:     o.Shoot.SeedNamespace,
		Purpose:       purpose,
		ConfigName:    prefix + common.TerraformerConfigSuffix,
		VariablesName: prefix + common.TerraformerVariablesSuffix,
		StateName:     prefix + common.TerraformerStateSuffix,
		PodName:       prefix + common.TerraformerPodSuffix + "-" + utils.ComputeSHA256Hex([]byte(time.Now().String()))[:5],
		JobName:       prefix + common.TerraformerJobSuffix,
	}
}

// Apply executes the Terraform Job by running the 'terraform apply' command.
func (t *Terraformer) Apply() error {
	if !t.ConfigurationDefined {
		return errors.New("Terraformer configuration has not been defined, cannot execute the Terraform scripts")
	}
	return t.execute("apply")
}

// Destroy executes the Terraform Job by running the 'terraform destroy' command.
func (t *Terraformer) Destroy() error {
	err := t.execute("destroy")
	if err != nil {
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
	err := utils.Retry(t.Logger, 30*time.Second, func() (bool, error) {
		numberOfExistingResources, err := t.prepare()
		if err != nil {
			return false, err
		}
		if numberOfExistingResources == 0 {
			t.Logger.Debug("All ConfigMaps/Secrets do not exist, can not execute the Terraform Job.")
			return true, nil
		} else if numberOfExistingResources == 3 {
			t.Logger.Debug("All ConfigMaps/Secrets exist, will execute the Terraform Job.")
			execute = true
			return true, nil
		} else {
			t.Logger.Error("Can not execute Terraform Job as ConfigMaps/Secrets are missing!")
			return false, nil
		}
	})
	if err != nil {
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

	defaultValues := map[string]interface{}{
		"terraformVariablesEnvironment": t.VariablesEnvironment,
		"names": map[string]interface{}{
			"configuration": t.ConfigName,
			"variables":     t.VariablesName,
			"state":         t.StateName,
			"pod":           t.PodName,
			"job":           t.JobName,
		},
	}
	values, err := t.InjectImages(defaultValues, t.K8sSeedClient.Version(), map[string]string{"terraformer": "terraformer"})
	if err != nil {
		return err
	}

	if !skipPod {
		values["kind"] = "Pod"
		values["script"] = "validate"

		// Create Terraform Pod which validates the Terraform configuration
		err := t.deployTerraformer(values)
		if err != nil {
			return err
		}

		// Wait for the Terraform validation Pod to be completed
		exitCode = t.waitForPod()
		skipJob = exitCode == 0 || exitCode == 1
		if exitCode == 0 {
			t.Logger.Debug("Terraform validation succeeded but there is no difference between state and actual resources.")
		} else if exitCode == 1 {
			t.Logger.Debug("Terraform validation failed, will not start the job.")
			succeeded = false
		} else {
			t.Logger.Debug("Terraform validation has been successful.")
		}
	}

	if !skipJob {
		values["kind"] = "Job"
		values["script"] = scriptName

		// Create Terraform Job which executes the provided scriptName
		err := t.deployTerraformer(values)
		if err != nil {
			return fmt.Errorf("Failed to deploy the Terraformer: %s", err.Error())
		}

		// Wait for the Terraform Job to be completed
		succeeded = t.waitForJob()
		t.Logger.Infof("Terraform '%s' finished.", t.JobName)
	}

	// Retrieve the logs of the Pods belonging to the completed Job
	jobPodList, err := t.listJobPods()
	if err != nil {
		t.Logger.Errorf("Could not retrieve list of pods belonging to Terraform job '%s': %s", t.JobName, err.Error())
		jobPodList = &corev1.PodList{}
	}

	t.Logger.Infof("Fetching the logs for all pods belonging to the Terraform job '%s'...", t.JobName)
	logList, err := t.retrievePodLogs(jobPodList)
	if err != nil {
		t.Logger.Errorf("Could not retrieve the logs of the pods belonging to Terraform job '%s': %s", t.JobName, err.Error())
		logList = map[string]string{}
	}
	for podName, podLogs := range logList {
		t.Logger.Infof("Logs of Pod '%s' belonging to Terraform job '%s':\n%s", podName, t.JobName, podLogs)
	}

	// Delete the Terraform Job and all its belonging Pods
	t.Logger.Infof("Cleaning up pods created by Terraform job '%s'...", t.JobName)
	err = t.cleanupJob(jobPodList)
	if err != nil {
		return err
	}

	// Evaluate whether the execution was successful or not
	t.Logger.Infof("Terraformer execution for job '%s' has been completed.", t.JobName)
	if !succeeded {
		errorMessage := fmt.Sprintf("Terraform execution job '%s' could not be completed.", t.JobName)
		terraformErrors := retrieveTerraformErrors(logList)
		if terraformErrors != nil {
			errorMessage += fmt.Sprintf(" The following issues have been found in the logs:\n\n%s", strings.Join(terraformErrors, "\n\n"))
		}
		return determineErrorCode(errorMessage)
	}
	return nil
}

// deployTerraformer renders the Terraformer chart which contains the Job/Pod manifest.
func (t *Terraformer) deployTerraformer(values map[string]interface{}) error {
	chartName := "terraformer"
	return t.ApplyChartSeed(filepath.Join(chartPath, chartName), chartName, t.Namespace, nil, values)
}

// listJobPods lists all pods which have a label 'job-name' whose value is equal to the Terraformer job name.
func (t *Terraformer) listJobPods() (*corev1.PodList, error) {
	return t.K8sSeedClient.ListPods(t.Namespace, metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", t.JobName)})
}

// retrievePodLogs fetches the logs of the created Pods by the Terraform Job and returns them as a map whose
// keys are pod names and whose values are the corresponding logs.
func (t *Terraformer) retrievePodLogs(jobPodList *corev1.PodList) (map[string]string, error) {
	logChan := make(chan map[string]string, 1)
	go func() {
		var logList = map[string]string{}
		for _, jobPod := range jobPodList.Items {
			name := jobPod.Name
			logsBuffer, err := t.K8sSeedClient.GetPodLogs(jobPod.Namespace, name, &corev1.PodLogOptions{})
			if err != nil {
				t.Logger.Warnf("Could not retrieve the logs of Terraform job pod %s: '%v'", name, err)
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
		return nil, fmt.Errorf("Timeout when reading the logs of all pds created by Terraform job '%s'", t.JobName)
	}
}

// cleanupJob deletes the Terraform Job and all belonging Pods from the Garden cluster.
func (t *Terraformer) cleanupJob(jobPodList *corev1.PodList) error {
	// Delete the Terraform Job
	err := t.K8sSeedClient.DeleteJob(t.Namespace, t.JobName)
	if err == nil {
		t.Logger.Infof("Deleted Terraform Job '%s'", t.JobName)
	} else {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Delete the belonging Terraform Pods
	for _, jobPod := range jobPodList.Items {
		err = t.K8sSeedClient.DeletePod(jobPod.ObjectMeta.Namespace, jobPod.ObjectMeta.Name)
		if err == nil {
			t.Logger.Infof("Deleted Terraform Job Pod '%s'", jobPod.ObjectMeta.Name)
		} else {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}
