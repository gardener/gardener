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

package cloudbotanist

import "github.com/gardener/gardener/pkg/operation/common"

// CloudBotanist is an interface which must be implemented by cloud-specific Botanists. The Cloud Botanist
// is responsible for all operations which require IaaS specific knowledge.
type CloudBotanist interface {
	// Infrastructure
	DeployInfrastructure() error
	DestroyInfrastructure() error
	DeployBackupInfrastructure() error
	DestroyBackupInfrastructure() error

	// Control Plane
	DeployAutoNodeRepair() error
	GenerateCloudProviderConfig() (string, error)
	GenerateCloudConfigUserDataConfig() *common.CloudConfigUserDataConfig
	GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error)
	GenerateKubeAPIServerConfig() (map[string]interface{}, error)
	GenerateKubeControllerManagerConfig() (map[string]interface{}, error)
	GenerateKubeSchedulerConfig() (map[string]interface{}, error)

	// Addons
	DeployKube2IAMResources() error
	DestroyKube2IAMResources() error
	GenerateKube2IAMConfig() (map[string]interface{}, error)
	GenerateClusterAutoscalerConfig() (map[string]interface{}, error)
	GenerateAdmissionControlConfig() (map[string]interface{}, error)
	GenerateCalicoConfig() (map[string]interface{}, error)
	GenerateNginxIngressConfig() (map[string]interface{}, error)

	// Hooks
	ApplyCreateHook() error
	ApplyDeleteHook() error

	// Miscellaneous (Health check, ...)
	CheckIfClusterGetsScaled() (bool, int, error)
	GetCloudProviderName() string
}
