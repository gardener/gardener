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
	"github.com/gardener/gardener/pkg/operation/terraformer"
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
	tf, err := terraformer.NewFromOperation(b.Operation, common.TerraformerPurposeInfra)
	if err != nil {
		return "", err
	}
	stateVariables, err := tf.GetStateOutputVariables(vpcID, vswitchID)
	if err != nil {
		return "", err
	}

	key := base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[AccessKeyID])
	secret := base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[AccessKeySecret])
	cfg := `{
"Global":
		{
		  "kubernetesClusterTag": "` + b.Shoot.SeedNamespace + `",
		  "vpcid": "` + stateVariables[vpcID] + `",
		  "zoneID": "` + b.Shoot.Info.Spec.Cloud.Alicloud.Zones[0] + `",
		  "region": "` + b.Shoot.Info.Spec.Cloud.Region + `",
		  "vswitchid": "` + stateVariables[vswitchID] + `",
		  "accessKeyID": "` + key + `",
		  "accessKeySecret": "` + secret + `"
		}
}`

	return cfg, nil
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
	return nil, nil
}

// GenerateCloudControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the cloud-controller-manager properly.
func (b *AlicloudBotanist) GenerateCloudControllerManagerConfig() (map[string]interface{}, error) {
	conf := map[string]interface{}{
		"defaultCCM":      false,
		"configureRoutes": true,
	}
	newConf, err := b.InjectImages(conf, b.SeedVersion(), b.ShootVersion(), common.AlicloudControllerManagerImageName)
	if err != nil {
		return conf, err
	}

	return newConf, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *AlicloudBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *AlicloudBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *AlicloudBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	return map[string][]byte{}, map[string]interface{}{}, nil
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

// GenerateKubeAPIServerServiceConfig generates the cloud provider specific values which are required to render the
// Service manifest of the kube-apiserver-service properly.
func (b *AlicloudBotanist) GenerateKubeAPIServerServiceConfig() (map[string]interface{}, error) {
	return nil, nil
}

// DeployCloudSpecificControlPlane does nothing currently for Alicloud
func (b *AlicloudBotanist) DeployCloudSpecificControlPlane() error {
	return nil
}
