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
	"errors"
	"path/filepath"
	"time"

	"github.com/gardener/gardener/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// SetVariablesEnvironment sets the provided <tfvarsEnvironment> on the Terraformer object.
func (t *Terraformer) SetVariablesEnvironment(tfvarsEnvironment []map[string]interface{}) *Terraformer {
	t.VariablesEnvironment = tfvarsEnvironment
	return t
}

// DefineConfig creates a ConfigMap for the tf state (if it does not exist, otherwise it won't update it),
// as well as a ConfigMap for the tf configuration (if it does not exist, otherwise it will update it).
// The tfvars are stored in a Secret as the contain confidental information like credentials.
func (t *Terraformer) DefineConfig(chartName string, values map[string]interface{}) *Terraformer {
	values["names"] = map[string]interface{}{
		"configuration": t.ConfigName,
		"variables":     t.VariablesName,
		"state":         t.StateName,
	}
	values["initializeEmptyState"] = t.IsStateEmpty()

	err := utils.Retry(t.Logger, 60*time.Second, func() (bool, error) {
		if err := t.ApplyChartSeed(filepath.Join(chartPath, chartName), chartName, t.Namespace, nil, values); err != nil {
			t.Logger.Infof("could not create Terraform ConfigMaps/Secrets: %s", err.Error())
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Logger.Errorf("Could not create the Terraform ConfigMaps/Secrets: %s", err.Error())
	} else {
		t.ConfigurationDefined = true
	}

	return t
}

// prepare checks whether all required ConfigMaps and Secrets exist. It returns the number of
// existing ConfigMaps/Secrets, or the error in case something unexpected happens.
func (t *Terraformer) prepare() (int, error) {
	// Check whether the required ConfigMaps and the Secret exist
	numberOfExistingResources := 3

	_, err := t.K8sSeedClient.GetConfigMap(t.Namespace, t.StateName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return -1, err
		}
		numberOfExistingResources--
	}
	_, err = t.K8sSeedClient.GetSecret(t.Namespace, t.VariablesName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return -1, err
		}
		numberOfExistingResources--
	}
	_, err = t.K8sSeedClient.GetConfigMap(t.Namespace, t.ConfigName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return -1, err
		}
		numberOfExistingResources--
	}
	if t.VariablesEnvironment == nil {
		return -1, errors.New("no Terraform variable environment provided")
	}

	// Clean up possible existing job/pod artifacts from previous runs
	jobPodList, err := t.listJobPods()
	if err != nil {
		return -1, err
	}
	err = t.cleanupJob(jobPodList)
	if err != nil {
		return -1, err
	}
	err = t.waitForCleanEnvironment()
	if err != nil {
		return -1, err
	}
	return numberOfExistingResources, nil
}

// cleanupConfiguration deletes the two ConfigMaps which store the Terraform configuration and state. It also deletes
// the Secret which stores the Terraform variables.
func (t *Terraformer) cleanupConfiguration() error {
	t.Logger.Infof("Deleting Terraform variables Secret '%s'", t.VariablesName)
	err := t.K8sSeedClient.DeleteSecret(t.Namespace, t.VariablesName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	t.Logger.Infof("Deleting Terraform configuration ConfigMap '%s'", t.ConfigName)
	err = t.K8sSeedClient.DeleteConfigMap(t.Namespace, t.ConfigName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	t.Logger.Infof("Deleting Terraform state ConfigMap '%s'", t.StateName)
	err = t.K8sSeedClient.DeleteConfigMap(t.Namespace, t.StateName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
