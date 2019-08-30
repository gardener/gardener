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
	"context"

	"cloud.google.com/go/storage"
	"github.com/gardener/etcd-backup-restore/pkg/snapstore"
	"github.com/gardener/etcd-backup-restore/pkg/snapstore/gcs"
	"github.com/gardener/gardener/pkg/operation/common"
	"google.golang.org/api/option"
)

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *GCPBotanist) GenerateEtcdBackupConfig() (map[string][]byte, error) {
	var (
		bucketName = "bucketName"
	)
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return nil, err
	}
	stateVariables, err := tf.GetStateOutputVariables(bucketName)
	if err != nil {
		return nil, err
	}

	secretData := map[string][]byte{
		common.BackupBucketName: []byte(stateVariables[bucketName]),
		ServiceAccountJSON:      []byte(b.MinifiedServiceAccount),
	}

	return secretData, nil
}

// GetEtcdBackupSnapstore returns the etcd backup snapstore object.
func (b *GCPBotanist) GetEtcdBackupSnapstore(secretData map[string][]byte) (snapstore.SnapStore, error) {
	var (
		serviceAccountJSON = secretData[ServiceAccountJSON]
		bucket             = string(secretData[common.BackupBucketName])
	)
	client, err := storage.NewClient(context.TODO(), option.WithCredentialsJSON(serviceAccountJSON), option.WithScopes(storage.ScopeFullControl))
	if err != nil {
		return nil, err
	}

	gcsClient := gcs.AdaptClient(client)
	return snapstore.NewGCSSnapStoreFromClient(bucket, "etcd-main/v1", "", 10, gcsClient), nil
}
