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

package awsbotanist

import (
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
)

// GenerateCloudProviderConfig generates the AWS cloud provider config.
// See this for more details:
// https://github.com/kubernetes/kubernetes/blob/master/pkg/cloudprovider/providers/aws/aws.go
func (b *AWSBotanist) GenerateCloudProviderConfig() (string, error) {
	var (
		vpcID    = "vpc_id"
		subnetID = "subnet_public_utility_z0"
	)

	stateVariables, err := terraformer.NewFromOperation(b.Operation, common.TerraformerPurposeInfra).GetStateOutputVariables(vpcID, subnetID)
	if err != nil {
		return "", err
	}

	return `[Global]
VPC="` + stateVariables[vpcID] + `"
SubnetID="` + stateVariables[subnetID] + `"
DisableSecurityGroupIngress=true
KubernetesClusterTag="` + b.Shoot.SeedNamespace + `"
KubernetesClusterID="` + b.Shoot.SeedNamespace + `"
Zone="` + b.Shoot.Info.Spec.Cloud.AWS.Zones[0] + `"`, nil
}

// RefreshCloudProviderConfig refreshes the cloud provider credentials in the existing cloud
// provider config.
// Not needed on AWS (cloud provider config does not contain the credentials), hence, the
// original is returned back.
func (b *AWSBotanist) RefreshCloudProviderConfig(currentConfig map[string]string) map[string]string {
	return currentConfig
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *AWSBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"environment": getAWSCredentialsEnvironment(),
	}, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *AWSBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"configureRoutes": false,
		"environment":     getAWSCredentialsEnvironment(),
	}, nil
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *AWSBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// maps are mutable, so it's safer to create a new instance
func getAWSCredentialsEnvironment() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name": "AWS_ACCESS_KEY_ID",
			"valueFrom": map[string]interface{}{
				"secretKeyRef": map[string]interface{}{
					"key":  AccessKeyID,
					"name": common.CloudProviderSecretName,
				},
			},
		},
		{
			"name": "AWS_SECRET_ACCESS_KEY",
			"valueFrom": map[string]interface{}{
				"secretKeyRef": map[string]interface{}{
					"key":  SecretAccessKey,
					"name": common.CloudProviderSecretName,
				},
			},
		},
	}
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *AWSBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	var (
		bucketName               = "bucketName"
		backupInfrastructureName = common.GenerateBackupInfrastructureName(b.Shoot.SeedNamespace, b.Shoot.Info.Status.UID)
		backupNamespace          = common.GenerateBackupNamespaceName(backupInfrastructureName)
	)

	stateVariables, err := terraformer.New(b.Logger, b.K8sSeedClient, common.TerraformerPurposeBackup, backupInfrastructureName, backupNamespace, b.ImageVector).GetStateOutputVariables(bucketName)
	if err != nil {
		return nil, nil, err
	}

	secretData := map[string][]byte{
		Region:          []byte(b.Seed.Info.Spec.Cloud.Region),
		AccessKeyID:     b.Seed.Secret.Data[AccessKeyID],
		SecretAccessKey: b.Seed.Secret.Data[SecretAccessKey],
	}

	backupConfigData := map[string]interface{}{
		"schedule":         b.Shoot.Info.Spec.Backup.Schedule,
		"maxBackups":       b.Shoot.Info.Spec.Backup.Maximum,
		"storageProvider":  "S3",
		"storageContainer": stateVariables[bucketName],
		"backupSecret":     common.BackupSecretName,
		"env": []map[string]interface{}{
			{
				"name": "AWS_REGION",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  Region,
					},
				},
			},
			{
				"name": "AWS_SECRET_ACCESS_KEY",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  SecretAccessKey,
					},
				},
			},
			{
				"name": "AWS_ACCESS_KEY_ID",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  AccessKeyID,
					},
				},
			},
		},
		"volumeMount": []map[string]interface{}{},
	}

	return secretData, backupConfigData, nil
}
