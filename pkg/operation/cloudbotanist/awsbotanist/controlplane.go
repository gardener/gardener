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
	"github.com/gardener/gardener/pkg/utils"
)

// GenerateCloudProviderConfig generates the AWS cloud provider config.
// See this for more details:
// https://github.com/kubernetes/kubernetes/blob/release-1.7/pkg/cloudprovider/providers/aws/aws.go#L399-L444
func (b *AWSBotanist) GenerateCloudProviderConfig() (string, error) {
	stateConfigMap, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetState()
	if err != nil {
		return "", err
	}
	state := utils.ConvertJSONToMap(stateConfigMap)

	vpcID, err := state.String("modules", "0", "outputs", "vpc_id", "value")
	if err != nil {
		return "", err
	}
	subnetID, err := state.String("modules", "0", "outputs", "subnet_id", "value")
	if err != nil {
		return "", err
	}

	return `[Global]
VPC = ` + vpcID + `
SubnetID = ` + subnetID + `
DisableSecurityGroupIngress = true
KubernetesClusterTag = ` + b.Shoot.SeedNamespace + `
KubernetesClusterID = ` + b.Shoot.SeedNamespace, nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *AWSBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"Environment": getAWSCredentialsEnvironment(),
	}, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *AWSBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"ConfigureRoutes": false,
		"Environment":     getAWSCredentialsEnvironment(),
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

	return b.ApplyChartSeed(
		filepath.Join(common.ChartPath, "seed-controlplane", "charts", name),
		name,
		b.Shoot.SeedNamespace,
		defaultValues,
		nil,
	)
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

// GenerateEtcdBackupSecretData generates the data for the secret which is required by the etcd-operator to
// store the backups on the S3 backup, i.e. the secret contains the AWS credentials and the respective region.
func (b *AWSBotanist) GenerateEtcdBackupSecretData() (map[string][]byte, error) {
	stateConfigMap, err := terraformer.New(b.Operation, common.TerraformerPurposeBackup).GetState()
	if err != nil {
		return nil, err
	}
	state := utils.ConvertJSONToMap(stateConfigMap)

	accessKeyID, err := state.String("modules", "0", "outputs", AccessKeyID, "value")
	if err != nil {
		return nil, err
	}
	secretAccessKey, err := state.String("modules", "0", "outputs", SecretAccessKey, "value")
	if err != nil {
		return nil, err
	}

	credentials := `[default]
aws_access_key_id = ` + string(accessKeyID) + `
aws_secret_access_key = ` + string(secretAccessKey)

	config := `[default]
region = ` + b.Shoot.Info.Spec.Cloud.Region

	return map[string][]byte{
		"credentials": []byte(credentials),
		"config":      []byte(config),
	}, nil
}

// GenerateEtcdConfig returns the etcd deployment configuration (including backup settings) for the etcd
// Helm chart.
func (b *AWSBotanist) GenerateEtcdConfig(secretName string) (map[string]interface{}, error) {
	stateConfigMap, err := terraformer.New(b.Operation, common.TerraformerPurposeBackup).GetState()
	if err != nil {
		return nil, err
	}
	state := utils.ConvertJSONToMap(stateConfigMap)

	bucketName, err := state.String("modules", "0", "outputs", "bucketName", "value")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"kind": "EtcdCluster",
		"backup": map[string]interface{}{
			"backupIntervalInSecond": b.Shoot.Info.Spec.Backup.IntervalInSecond,
			"maxBackups":             b.Shoot.Info.Spec.Backup.Maximum,
			"storageType":            "S3",
			"s3": map[string]interface{}{
				"s3Bucket":  bucketName,
				"awsSecret": secretName,
			},
		},
	}, nil
}
