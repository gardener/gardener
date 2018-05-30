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

package cloudbotanist

import (
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
)

// CloudBotanist is an interface which must be implemented by cloud-specific Botanists. The Cloud Botanist
// is responsible for all operations which require IaaS specific knowledge.
type CloudBotanist interface {
	GetCloudProviderName() string

	// Infrastructure
	DeployInfrastructure() error
	DestroyInfrastructure() error
	DeployBackupInfrastructure() error
	DestroyBackupInfrastructure() error

	// Control Plane
	GenerateCloudProviderConfig() (string, error)
	RefreshCloudProviderConfig(map[string]string) map[string]string
	GenerateCloudConfigUserDataConfig() *common.CloudConfigUserDataConfig
	GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error)
	GenerateKubeAPIServerConfig() (map[string]interface{}, error)
	GenerateKubeControllerManagerConfig() (map[string]interface{}, error)
	GenerateKubeSchedulerConfig() (map[string]interface{}, error)

	// Machines
	GetMachineClassInfo() (string, string, string)
	GenerateMachineConfig() ([]map[string]interface{}, []operation.MachineDeployment, error)
	GenerateMachineClassSecretData() map[string][]byte

	// Addons
	DeployKube2IAMResources() error
	DestroyKube2IAMResources() error
	GenerateKube2IAMConfig() (map[string]interface{}, error)
	GenerateAdmissionControlConfig() (map[string]interface{}, error)
	GenerateNginxIngressConfig() (map[string]interface{}, error)

	// Hooks
	ApplyCreateHook() error
}
