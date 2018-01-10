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

package openstackbotanist

import (
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
	"github.com/gardener/gardener/pkg/utils"
)

// DeployAutoNodeRepair returns
func (b *OpenStackBotanist) DeployAutoNodeRepair() error {
	return nil
}

// GenerateCloudProviderConfig returns
func (b *OpenStackBotanist) GenerateCloudProviderConfig() (string, error) {
	stateConfigMap, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetState()
	if err != nil {
		return "", err
	}
	state := utils.ConvertJSONToMap(stateConfigMap)
	cloudConf, err := state.String("modules", "0", "outputs", "cloud_config", "value")
	if err != nil {
		return "", err
	}
	return cloudConf, nil
}

// GenerateKubeAPIServerConfig returns
func (b *OpenStackBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	loadBalancerIP, err := utils.WaitUntilDNSNameResolvable(b.APIServerAddress)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"AdditionalParameters": []string{
			fmt.Sprintf("--external-hostname=%s", loadBalancerIP),
		},
	}, nil
}

// GenerateKubeControllerManagerConfig returns
func (b *OpenStackBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateKubeSchedulerConfig returns
func (b *OpenStackBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateEtcdBackupSecretData returns
func (b *OpenStackBotanist) GenerateEtcdBackupSecretData() (map[string][]byte, error) {
	return nil, nil
}

// GenerateEtcdBackupDefaults returns
func (b *OpenStackBotanist) GenerateEtcdBackupDefaults() *gardenv1beta1.Backup {
	return nil
}

// GenerateEtcdConfig returns the etcd deployment configuration (including backup settings) for the etcd
// Helm chart.
func (b *OpenStackBotanist) GenerateEtcdConfig(string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"kind": "StatefulSet",
	}, nil
}
