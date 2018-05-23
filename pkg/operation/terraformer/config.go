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
	"path/filepath"
	"time"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// SetVariablesEnvironment sets the provided <tfvarsEnvironment> on the Terraformer object.
func (t *Terraformer) SetVariablesEnvironment(tfvarsEnvironment []map[string]interface{}) *Terraformer {
	t.variablesEnvironment = tfvarsEnvironment
	return t
}

// SetImage sets the provided <image> on the Terraformer object.
func (t *Terraformer) SetImage(image string) *Terraformer {
	t.image = image
	return t
}

// DefineConfig creates a ConfigMap for the tf state (if it does not exist, otherwise it won't update it),
// as well as a ConfigMap for the tf configuration (if it does not exist, otherwise it will update it).
// The tfvars are stored in a Secret as the contain confidential information like credentials.
func (t *Terraformer) DefineConfig(chartName string, values map[string]interface{}) *Terraformer {
	values["names"] = map[string]interface{}{
		"configuration": t.configName,
		"variables":     t.variablesName,
		"state":         t.stateName,
	}
	values["initializeEmptyState"] = t.IsStateEmpty()

	if err := utils.Retry(t.logger, 30*time.Second, func() (bool, error) {
		if err := common.ApplyChart(t.k8sClient, t.chartRenderer, filepath.Join(chartPath, chartName), chartName, t.namespace, nil, values); err != nil {
			t.logger.Errorf("could not create Terraform ConfigMaps/Secrets: %s", err.Error())
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.logger.Errorf("Could not create the Terraform ConfigMaps/Secrets: %s", err.Error())
	} else {
		t.configurationDefined = true
	}

	return t
}

// prepare checks whether all required ConfigMaps and Secrets exist. It returns the number of
// existing ConfigMaps/Secrets, or the error in case something unexpected happens.
func (t *Terraformer) prepare() (int, error) {
	// Check whether the required ConfigMaps and the Secret exist
	numberOfExistingResources := 3

	_, err := t.k8sClient.GetConfigMap(t.namespace, t.stateName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return -1, err
		}
		numberOfExistingResources--
	}
	_, err = t.k8sClient.GetSecret(t.namespace, t.variablesName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return -1, err
		}
		numberOfExistingResources--
	}
	_, err = t.k8sClient.GetConfigMap(t.namespace, t.configName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return -1, err
		}
		numberOfExistingResources--
	}
	if t.variablesEnvironment == nil {
		return -1, errors.New("no Terraform variable environment provided")
	}

	// Clean up possible existing job/pod artifacts from previous runs
	jobPodList, err := t.ListJobPods()
	if err != nil {
		return -1, err
	}
	if err := t.CleanupJob(jobPodList); err != nil {
		return -1, err
	}
	if err := t.WaitForCleanEnvironment(); err != nil {
		return -1, err
	}
	return numberOfExistingResources, nil
}

// cleanupConfiguration deletes the two ConfigMaps which store the Terraform configuration and state. It also deletes
// the Secret which stores the Terraform variables.
func (t *Terraformer) cleanupConfiguration() error {
	t.logger.Debugf("Deleting Terraform variables Secret '%s'", t.variablesName)
	if err := t.k8sClient.DeleteSecret(t.namespace, t.variablesName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	t.logger.Debugf("Deleting Terraform configuration ConfigMap '%s'", t.configName)
	if err := t.k8sClient.DeleteConfigMap(t.namespace, t.configName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	t.logger.Debugf("Deleting Terraform state ConfigMap '%s'", t.stateName)
	if err := t.k8sClient.DeleteConfigMap(t.namespace, t.stateName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}
