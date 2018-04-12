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

package gcpbotanist

import (
	"fmt"
	"path"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
)

// GenerateCloudProviderConfig generates the GCE cloud provider config.
// See this for more details:
// https://github.com/kubernetes/kubernetes/blob/master/pkg/cloudprovider/providers/gce/gce.go
func (b *GCPBotanist) GenerateCloudProviderConfig() (string, error) {
	networkName := b.VPCName
	if networkName == "" {
		networkName = b.Shoot.SeedNamespace
	}

	return `[Global]
project-id = ` + b.Project + `
network-name = ` + networkName + `
multizone = true
token-url = nil
node-tags = ` + b.Shoot.SeedNamespace, nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *GCPBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"environment": getGCPCredentialsEnvironment(),
	}, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *GCPBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"environment": getGCPCredentialsEnvironment(),
	}, nil
}

// maps are mutable, so it's safer to create a new instance
func getGCPCredentialsEnvironment() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":  "GOOGLE_APPLICATION_CREDENTIALS",
			"value": fmt.Sprintf("/srv/cloudprovider/%s", ServiceAccountJSON),
		},
	}
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *GCPBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *GCPBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	mountPath := "/root/.gcp/"
	bucketName := "bucketName"
	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeBackup).GetStateOutputVariables(bucketName)
	if err != nil {
		return nil, nil, err
	}

	secretData := map[string][]byte{
		ServiceAccountJSON: []byte(b.MinifiedServiceAccount),
	}
	backupConfigData := map[string]interface{}{
		"schedule":         b.Shoot.Info.Spec.Backup.Schedule,
		"maxBackups":       b.Shoot.Info.Spec.Backup.Maximum,
		"storageProvider":  "GCS",
		"storageContainer": stateVariables[bucketName],
		"env": []map[string]interface{}{
			{
				"name":  "GOOGLE_APPLICATION_CREDENTIALS",
				"value": path.Join(mountPath, ServiceAccountJSON),
			},
		},
		"volumeMounts": []map[string]interface{}{
			{
				"mountPath": mountPath,
				"name":      common.BackupSecretName,
			},
		},
	}

	return secretData, backupConfigData, nil
}
