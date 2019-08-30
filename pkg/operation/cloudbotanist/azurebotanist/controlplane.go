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

package azurebotanist

import (
	"fmt"
	"net/url"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/gardener/etcd-backup-restore/pkg/snapstore"
	"github.com/gardener/gardener/pkg/operation/common"
)

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *AzureBotanist) GenerateEtcdBackupConfig() (map[string][]byte, error) {
	var (
		storageAccountName = "storageAccountName"
		storageAccessKey   = "storageAccessKey"
		containerName      = "containerName"
	)
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return nil, err
	}
	stateVariables, err := tf.GetStateOutputVariables(storageAccountName, storageAccessKey, containerName)
	if err != nil {
		return nil, err
	}

	secretData := map[string][]byte{
		common.BackupBucketName: []byte(stateVariables[containerName]),
		"storage-account":       []byte(stateVariables[storageAccountName]),
		"storage-key":           []byte(stateVariables[storageAccessKey]),
	}

	return secretData, nil
}

// GetEtcdBackupSnapstore returns the etcd backup snapstore object.
func (b *AzureBotanist) GetEtcdBackupSnapstore(secretData map[string][]byte) (snapstore.SnapStore, error) {
	var (
		storageAccount = string(secretData["storage-account"])
		storageKey     = string(secretData["storage-key"])
		container      = string(secretData[common.BackupBucketName])
	)
	credentials, err := azblob.NewSharedKeyCredential(storageAccount, storageKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create shared key credentials: %v", err)
	}

	p := azblob.NewPipeline(credentials, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{},
	})
	u, err := url.Parse(fmt.Sprintf("https://%s.%s", storageAccount, snapstore.AzureBlobStorageHostName))
	if err != nil {
		return nil, fmt.Errorf("failed to parse service url: %v", err)
	}
	serviceURL := azblob.NewServiceURL(*u, p)
	containerURL := serviceURL.NewContainerURL(container)
	return snapstore.GetABSSnapstoreFromClient(container, "etcd-main/v1", "", 10, &containerURL)
}
