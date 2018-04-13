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

package localbotanist

// GenerateCloudProviderConfig returns a cloud provider config for the Local cloud provider.
// Not needed on Local.
func (b *LocalBotanist) GenerateCloudProviderConfig() (string, error) {
	return "", nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *LocalBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"securePort": 31443,
	}, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *LocalBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *LocalBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *LocalBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	backupConfigData := map[string]interface{}{
		"schedule":         b.Shoot.Info.Spec.Backup.Schedule,
		"maxBackups":       b.Shoot.Info.Spec.Backup.Maximum,
		"storageProvider":  "",
		"storageContainer": "/var/etcd/default.bkp",
		"env":              []map[string]interface{}{},
		"volumeMount":      []map[string]interface{}{},
	}

	return nil, backupConfigData, nil
}
