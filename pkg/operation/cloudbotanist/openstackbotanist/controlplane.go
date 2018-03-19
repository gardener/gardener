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

package openstackbotanist

import (
	"fmt"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
	"github.com/gardener/gardener/pkg/utils"
)

// GenerateCloudProviderConfig generates the OpenStack cloud provider config.
// See this for more details:
// https://github.com/kubernetes/kubernetes/blob/master/pkg/cloudprovider/providers/openstack/openstack.go
func (b *OpenStackBotanist) GenerateCloudProviderConfig() (string, error) {
	cloudConfig := "cloud_config"
	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetStateOutputVariables(cloudConfig)
	if err != nil {
		return "", err
	}
	return stateVariables[cloudConfig], nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *OpenStackBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	loadBalancerIP, err := utils.WaitUntilDNSNameResolvable(b.APIServerAddress)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"additionalParameters": []string{
			fmt.Sprintf("--external-hostname=%s", loadBalancerIP),
		},
	}, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *OpenStackBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *OpenStackBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
// TODO: implement backup functionality for OpenStack
func (b *OpenStackBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	backupConfigData := map[string]interface{}{
		"schedule":         b.Shoot.Info.Spec.Backup.Schedule,
		"maxBackups":       b.Shoot.Info.Spec.Backup.Maximum,
		"storageProvider":  "Local",
		"storageContainer": "/var/etcd/default.bkp",
		"env":              []map[string]interface{}{},
		"volumeMount":      []map[string]interface{}{},
	}
	return nil, backupConfigData, nil
}
