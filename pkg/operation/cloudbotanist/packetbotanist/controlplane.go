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

package packetbotanist

import (
	"encoding/base64"
	"fmt"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
)

// GenerateCloudProviderConfig generates the Packet cloud provider config.
// See this for more details:
// https://github.com/packethost/packet-ccm
func (b *PacketBotanist) GenerateCloudProviderConfig() (string, error) {
	return "", nil
}

// RefreshCloudProviderConfig refreshes the cloud provider credentials in the existing cloud
// provider config.
// Not needed on Packet (cloud provider config does not contain the credentials), hence, the
// original is returned back.
func (b *PacketBotanist) RefreshCloudProviderConfig(currentConfig map[string]string) map[string]string {
	return currentConfig
}

// GenerateKubeAPIServerServiceConfig generates the cloud provider specific values which are required to render the
// Service manifest of the kube-apiserver-service properly.
func (b *PacketBotanist) GenerateKubeAPIServerServiceConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"enableCSI": true,
	}, nil
}

// GenerateKubeAPIServerExposeConfig defines the cloud provider specific values which configure how the kube-apiserver
// is exposed to the public.
func (b *PacketBotanist) GenerateKubeAPIServerExposeConfig() (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *PacketBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"environment": getPacketCredentialsEnvironment(),
	}, nil
}

// GenerateCloudControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the cloud-controller-manager properly.
func (b *PacketBotanist) GenerateCloudControllerManagerConfig() (map[string]interface{}, string, error) {
	chartName := "packet-cloud-controller-manager"
	conf := map[string]interface{}{}
	newConf, err := b.InjectSeedShootImages(conf, common.PacketControllerManagerImageName)
	if err != nil {
		return conf, chartName, err
	}

	return newConf, chartName, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *PacketBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"enableCSI": true,
	}, nil
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *PacketBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateCSIConfig generates the configuration for CSI charts
func (b *PacketBotanist) GenerateCSIConfig() (map[string]interface{}, error) {
	conf := map[string]interface{}{
		"regionID": b.Shoot.Info.Spec.Cloud.Region,
		"credential": map[string]interface{}{
			"apiToken":  base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[PacketAPIKey]),
			"projectID": base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[ProjectID]),
		},
		"podAnnotations": map[string]interface{}{
			fmt.Sprintf("checksum/%s", common.CSIAttacher):                         b.CheckSums[common.CSIAttacher],
			fmt.Sprintf("checksum/%s", gardencorev1alpha1.SecretNameCloudProvider): b.CheckSums[gardencorev1alpha1.SecretNameCloudProvider],
			fmt.Sprintf("checksum/%s", common.CSIProvisioner):                      b.CheckSums[common.CSIProvisioner],
			fmt.Sprintf("checksum/%s", common.CSISnapshotter):                      b.CheckSums[common.CSISnapshotter],
		},
		"kubernetesVersion": b.ShootVersion(),
		"enabled":           true,
	}

	return b.InjectShootShootImages(conf,
		common.CSIAttacherImageName,
		common.CSIPluginPacketImageName,
		common.CSIProvisionerImageName,
		common.CSISnapshotterImageName,
		common.CSINodeDriverRegistrarImageName,
	)
}

// maps are mutable, so it's safer to create a new instance
func getPacketCredentialsEnvironment() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name": "PACKET_API_KEY",
			"valueFrom": map[string]interface{}{
				"secretKeyRef": map[string]interface{}{
					"key":  PacketAPIKey,
					"name": gardencorev1alpha1.SecretNameCloudProvider,
				},
			},
		},
	}
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *PacketBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	secretData := map[string][]byte{}
	backupConfigData := map[string]interface{}{}

	return secretData, backupConfigData, nil
}

// GenerateETCDStorageClassConfig generates values which are required to create etcd volume storageclass properly.
func (b *PacketBotanist) GenerateETCDStorageClassConfig() map[string]interface{} {
	return map[string]interface{}{
		"name":        "gardener.cloud-fast",
		"capacity":    "25Gi",
		"provisioner": "net.packet.csi",
		"parameters": map[string]interface{}{
			"plan": "standard",
		},
	}
}

// DeployCloudSpecificControlPlane does any last minute updates
func (b *PacketBotanist) DeployCloudSpecificControlPlane() error {
	return nil
}
