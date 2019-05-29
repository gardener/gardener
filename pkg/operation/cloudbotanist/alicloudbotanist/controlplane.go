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
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/gardener/etcd-backup-restore/pkg/snapstore"
	"github.com/gardener/gardener/pkg/operation/common"
)

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *AlicloudBotanist) GenerateEtcdBackupConfig() (map[string][]byte, error) {
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return nil, err
	}
	stateVariables, err := tf.GetStateOutputVariables(common.BackupBucketName, StorageEndpoint)
	if err != nil {
		return nil, err
	}

	secretData := map[string][]byte{
		common.BackupBucketName: []byte(stateVariables[common.BackupBucketName]),
		StorageEndpoint:         []byte(stateVariables[StorageEndpoint]),
		AccessKeyID:             b.Seed.Secret.Data[AccessKeyID],
		AccessKeySecret:         b.Seed.Secret.Data[AccessKeySecret],
	}

	return secretData, nil
}

// GetEtcdBackupSnapstore returns the etcd backup snapstore object.
func (b *AlicloudBotanist) GetEtcdBackupSnapstore(secretData map[string][]byte) (snapstore.SnapStore, error) {
	var (
		accessKeyID     = string(secretData[AccessKeyID])
		secretAccessKey = string(secretData[AccessKeySecret])
		bucket          = string(secretData[common.BackupBucketName])
		storageEndpoint = string(secretData[StorageEndpoint])
	)

	client, err := oss.New(storageEndpoint, accessKeyID, secretAccessKey)
	if err != nil {
		return nil, err
	}

	bucketOSS, err := client.Bucket(bucket)
	if err != nil {
		return nil, err
	}

	return snapstore.NewOSSFromBucket("etcd-main/v1", "", 10, bucketOSS), nil
}
