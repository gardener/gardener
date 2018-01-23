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

package awsbotanist

import (
	"path/filepath"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
)

// GenerateCloudProviderConfig generates the AWS cloud provider config.
// See this for more details:
// https://github.com/kubernetes/kubernetes/blob/release-1.7/pkg/cloudprovider/providers/aws/aws.go#L399-L444
func (b *AWSBotanist) GenerateCloudProviderConfig() (string, error) {
	var (
		vpcID    = "vpc_id"
		subnetID = "subnet_id"
	)

	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetStateOutputVariables(vpcID, subnetID)
	if err != nil {
		return "", err
	}

	return `[Global]
VPC = ` + stateVariables[vpcID] + `
SubnetID = ` + stateVariables[subnetID] + `
DisableSecurityGroupIngress = true
KubernetesClusterTag = ` + b.Shoot.SeedNamespace + `
KubernetesClusterID = ` + b.Shoot.SeedNamespace + `
Zone = ` + b.Shoot.Info.Spec.Cloud.AWS.Zones[0], nil
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

// DeployAutoNodeRepair deploys the auto-node-repair into the Seed cluster. It primary job is to repair
// unHealthy Nodes by replacing them by newer ones.
func (b *AWSBotanist) DeployAutoNodeRepair() error {
	var (
		name                 = "auto-node-repair"
		autoscalingGroups    = b.GetASGs()
		imagePullSecrets     = b.GetImagePullSecretsMap()
		environmentVariables = getAWSCredentialsEnvironment()
	)

	environmentVariables = append(environmentVariables, map[string]interface{}{
		"name":  "AWS_REGION",
		"value": b.Shoot.Info.Spec.Cloud.Region,
	})

	defaultValues := map[string]interface{}{
		"namespace":         b.Shoot.SeedNamespace,
		"autoscalingGroups": autoscalingGroups,
		"imagePullSecrets":  imagePullSecrets,
		"environment":       environmentVariables,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-auto-node-repair": b.CheckSums[name],
		},
	}

	values, err := b.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{"auto-node-repair": "auto-node-repair"})
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-controlplane", "charts", name), name, b.Shoot.SeedNamespace, values, nil)
}

// maps are mutable, so it's safer to create a new instance
func getAWSCredentialsEnvironment() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name": "AWS_ACCESS_KEY_ID",
			"valueFrom": map[string]interface{}{
				"secretKeyRef": map[string]interface{}{
					"key":  AccessKeyID,
					"name": "cloudprovider",
				},
			},
		},
		{
			"name": "AWS_SECRET_ACCESS_KEY",
			"valueFrom": map[string]interface{}{
				"secretKeyRef": map[string]interface{}{
					"key":  SecretAccessKey,
					"name": "cloudprovider",
				},
			},
		},
	}
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *AWSBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	bucketName := "bucketName"
	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeBackup).GetStateOutputVariables(AccessKeyID, SecretAccessKey, bucketName)
	if err != nil {
		return nil, nil, err
	}

	credentials := `[default]
aws_access_key_id = ` + stateVariables[AccessKeyID] + `
aws_secret_access_key = ` + stateVariables[SecretAccessKey]

	config := `[default]
region = ` + b.Seed.Info.Spec.Cloud.Region

	secretData := map[string][]byte{
		"credentials": []byte(credentials),
		"config":      []byte(config),
	}

	backupConfigData := map[string]interface{}{
		"backupIntervalInSecond": b.Shoot.Info.Spec.Backup.IntervalInSecond,
		"maxBackups":             b.Shoot.Info.Spec.Backup.Maximum,
		"storageType":            "S3",
		"s3": map[string]interface{}{
			"s3Bucket":  stateVariables[bucketName],
			"awsSecret": common.BackupSecretName,
		},
	}

	return secretData, backupConfigData, nil
}
