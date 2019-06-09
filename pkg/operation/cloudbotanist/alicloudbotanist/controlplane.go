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
	"github.com/gardener/gardener/pkg/operation/common"
)

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *AlicloudBotanist) GenerateEtcdBackupConfig() (map[string][]byte, error) {
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return nil, err
	}
	stateVariables, err := tf.GetStateOutputVariables(BucketName, StorageEndpoint)
	if err != nil {
		return nil, err
	}

	secretData := map[string][]byte{
		common.BackupBucketName: []byte(stateVariables[BucketName]),
		StorageEndpoint:         []byte(stateVariables[StorageEndpoint]),
		AccessKeyID:             b.Seed.Secret.Data[AccessKeyID],
		AccessKeySecret:         b.Seed.Secret.Data[AccessKeySecret],
	}

	return secretData, nil
}

// GenerateCSIConfig generates the configuration for CSI charts
func (b *AlicloudBotanist) GenerateCSIConfig() (map[string]interface{}, error) {
	conf := map[string]interface{}{
		"credential": map[string]interface{}{
			"accessKeyID":     base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[AccessKeyID]),
			"accessKeySecret": base64.StdEncoding.EncodeToString(b.Shoot.Secret.Data[AccessKeySecret]),
		},
		"kubernetesVersion": b.ShootVersion(),
		"enabled":           true,
	}

	return b.InjectShootShootImages(conf,
		common.CSIPluginAlicloudImageName,
		common.CSINodeDriverRegistrarImageName,
	)
}

// DeployCloudSpecificControlPlane does nothing currently for Alicloud
func (b *AlicloudBotanist) DeployCloudSpecificControlPlane() error {
	return nil
}
