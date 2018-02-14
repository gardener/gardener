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

package vagrantbotanist

// GenerateCloudProviderConfig returns a cloud provider config for the Vagrant cloud provider
// as defined here: https://github.com/kubernetes/kubernetes/blob/release-1.7/pkg/cloudprovider/providers/azure/azure.go#L58.
func (b *VagrantBotanist) GenerateCloudProviderConfig() (string, error) {
	return "", nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *VagrantBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"securePort": 31443,
	}, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *VagrantBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *VagrantBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// DeployAutoNodeRepair deploys the auto-node-repair into the Seed cluster. It primary job is to repair
// unHealthy Nodes by replacing them by newer ones - Not needed on Vagrant yet. To be implemented later
func (b *VagrantBotanist) DeployAutoNodeRepair() error {
	return nil
}

// GenerateEtcdBackupSecretData generates the data for the secret which is required by the etcd-operator to
// store the backups on the ABS container, i.e. the secret contains the Vagrant storage account and the respective access key.
func (b *VagrantBotanist) GenerateEtcdBackupSecretData() (map[string][]byte, error) {
	return nil, nil
}

// GenerateEtcdConfig returns the etcd deployment configuration (including backup settings) for the etcd
// Helm chart.
func (b *VagrantBotanist) GenerateEtcdConfig(secretName string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"kind": "StatefulSet",
	}, nil
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
// TODO: implement backup functionality for Vagrant
func (b *VagrantBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	return nil, nil, nil
}
