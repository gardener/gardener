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

package alicloudbotanist

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/pkg/operation/common"
)

type cloudConfig struct {
	Global struct {
		KubernetesClusterTag string
		UID                  string `json:"uid"`
		VpcID                string `json:"vpcid"`
		Region               string `json:"region"`
		ZoneID               string `json:"zoneid"`
		VswitchID            string `json:"vswitchid"`

		AccessKeyID     string `json:"accessKeyID"`
		AccessKeySecret string `json:"accessKeySecret"`
	}
}

// GenerateCloudProviderConfig generates the Alicloud cloud provider config.
// See this for more details:
// https://github.com/kubernetes/cloud-provider-alibaba-cloud/blob/master/cloud-controller-manager/alicloud.go#L62
func (b *AlicloudBotanist) GenerateCloudProviderConfig() (string, error) {
	var (
		vpcID     = "vpc_id"
		vswitchID = fmt.Sprintf("vswitch_id_z%d", 0)
	)
	tf, err := b.NewShootTerraformer(common.TerraformerPurposeInfra)
	if err != nil {
		return "", err
	}
	stateVariables, err := tf.GetStateOutputVariables(vpcID, vswitchID)
	if err != nil {
		return "", err
	}

	key := base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[AccessKeyID])
	secret := base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[AccessKeySecret])
	cfg := &cloudConfig{}
	cfg.Global.KubernetesClusterTag = b.Shoot.SeedNamespace
	cfg.Global.VpcID = stateVariables[vpcID]
	cfg.Global.ZoneID = b.Shoot.Info.Spec.Cloud.Alicloud.Zones[0]
	cfg.Global.VswitchID = stateVariables[vswitchID]
	cfg.Global.AccessKeyID = key
	cfg.Global.AccessKeySecret = secret
	cfg.Global.Region = b.Shoot.Info.Spec.Cloud.Region

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}

	return string(cfgJSON), nil
}

// RefreshCloudProviderConfig refreshes the cloud provider credentials in the existing cloud
// provider config.
// Not needed on Alicloud.
func (b *AlicloudBotanist) RefreshCloudProviderConfig(currentConfig map[string]string) map[string]string {
	existing := currentConfig[common.CloudProviderConfigMapKey]
	cfg := &cloudConfig{}
	if err := json.Unmarshal([]byte(existing), cfg); err != nil {
		return currentConfig
	}

	cfg.Global.AccessKeyID = base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[AccessKeyID])
	cfg.Global.AccessKeySecret = base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[AccessKeySecret])
	newProviderCfg, err := json.Marshal(cfg)
	if err == nil {
		currentConfig[common.CloudProviderConfigMapKey] = string(newProviderCfg)
	}

	return currentConfig
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *AlicloudBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"enableCSI": true,
	}, nil
}

// GenerateCloudControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the cloud-controller-manager properly.
func (b *AlicloudBotanist) GenerateCloudControllerManagerConfig() (map[string]interface{}, string, error) {
	chartName := "alicloud-cloud-controller-manager"
	conf := map[string]interface{}{
		"configureRoutes": false,
	}
	newConf, err := b.InjectSeedShootImages(conf, common.AlicloudControllerManagerImageName)
	if err != nil {
		return conf, chartName, err
	}

	return newConf, chartName, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *AlicloudBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"enableCSI": true,
	}, nil
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *AlicloudBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateETCDStorageClassConfig generates values which are required to create etcd volume storageclass properly.
func (b *AlicloudBotanist) GenerateETCDStorageClassConfig() map[string]interface{} {
	return map[string]interface{}{
		"name":        "gardener.cloud-fast",
		"capacity":    "25Gi",
		"provisioner": "diskplugin.csi.alibabacloud.com",
		"parameters": map[string]interface{}{
			"csi.storage.k8s.io/fstype": "ext4",
			"type":                      "cloud_ssd",
			"readOnly":                  "false",
		},
	}
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *AlicloudBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return nil, nil, err
	}
	stateVariables, err := tf.GetStateOutputVariables(BucketName, StorageEndpoint)
	if err != nil {
		return nil, nil, err
	}

	secretData := map[string][]byte{
		StorageEndpoint: []byte(stateVariables[StorageEndpoint]),
		AccessKeyID:     b.Seed.Secret.Data[AccessKeyID],
		AccessKeySecret: b.Seed.Secret.Data[AccessKeySecret],
	}

	backupConfigData := map[string]interface{}{
		"schedule":         b.ShootBackup.Schedule,
		"storageProvider":  "OSS",
		"storageContainer": stateVariables[BucketName],
		"backupSecret":     common.BackupSecretName,
		"env": []map[string]interface{}{
			{
				"name": "ALICLOUD_ENDPOINT",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  StorageEndpoint,
					},
				},
			},
			{
				"name": "ALICLOUD_ACCESS_KEY_ID",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  AccessKeyID,
					},
				},
			},
			{
				"name": "ALICLOUD_ACCESS_KEY_SECRET",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  AccessKeySecret,
					},
				},
			},
		},
		"volumeMount": []map[string]interface{}{},
	}

	return secretData, backupConfigData, nil
}

// GenerateKubeAPIServerExposeConfig defines the cloud provider specific values which configure how the kube-apiserver
// is exposed to the public.
func (b *AlicloudBotanist) GenerateKubeAPIServerExposeConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"advertiseAddress": b.APIServerAddress,
		"additionalParameters": []string{
			fmt.Sprintf("--external-hostname=%s", b.APIServerAddress),
		},
	}, nil
}

// GenerateCSIConfig generates the configuration for CSI charts
func (b *AlicloudBotanist) GenerateCSIConfig() (map[string]interface{}, error) {
	conf := map[string]interface{}{
		"regionID": b.Shoot.Info.Spec.Cloud.Region,
		"credential": map[string]interface{}{
			"accessKeyID":     base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[AccessKeyID]),
			"accessKeySecret": base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[AccessKeySecret]),
		},
		"podAnnotations": map[string]interface{}{
			fmt.Sprintf("checksum/%s", common.CSIAttacher):             b.CheckSums[common.CSIAttacher],
			fmt.Sprintf("checksum/%s", common.CloudProviderSecretName): b.CheckSums[common.CloudProviderSecretName],
			fmt.Sprintf("checksum/%s", common.CSIProvisioner):          b.CheckSums[common.CSIProvisioner],
			fmt.Sprintf("checksum/%s", common.CSISnapshotter):          b.CheckSums[common.CSISnapshotter],
		},
		"kubernetesVersion": b.ShootVersion(),
		"enabled":           true,
	}

	return b.InjectShootShootImages(conf,
		common.CSIAttacherImageName,
		common.CSIPluginAlicloudImageName,
		common.CSIProvisionerImageName,
		common.CSISnapshotterImageName,
		common.CSINodeDriverRegistrarImageName,
	)
}

// GenerateKubeAPIServerServiceConfig generates the cloud provider specific values which are required to render the
// Service manifest of the kube-apiserver-service properly.
func (b *AlicloudBotanist) GenerateKubeAPIServerServiceConfig() (map[string]interface{}, error) {
	return nil, nil
}

// DeployCloudSpecificControlPlane does nothing currently for Alicloud
func (b *AlicloudBotanist) DeployCloudSpecificControlPlane() error {
	return nil
}
