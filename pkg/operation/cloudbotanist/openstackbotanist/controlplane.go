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

package openstackbotanist

import (
	"github.com/gardener/etcd-backup-restore/pkg/snapstore"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *OpenStackBotanist) GenerateEtcdBackupConfig() (map[string][]byte, error) {
	containerName := "containerName"

	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return nil, err
	}

	stateVariables, err := tf.GetStateOutputVariables(containerName)
	if err != nil {
		return nil, err
	}

	secretData := map[string][]byte{
		common.BackupBucketName: []byte(stateVariables[containerName]),
		UserName:                b.Seed.Secret.Data[UserName],
		Password:                b.Seed.Secret.Data[Password],
		TenantName:              b.Seed.Secret.Data[TenantName],
		AuthURL:                 []byte(b.Seed.CloudProfile.Spec.OpenStack.KeyStoneURL),
		DomainName:              b.Seed.Secret.Data[DomainName],
	}

	return secretData, nil
}

// GetEtcdBackupSnapstore returns the etcd backup snapstore object.
func (b *OpenStackBotanist) GetEtcdBackupSnapstore(secretData map[string][]byte) (snapstore.SnapStore, error) {
	var (
		bucket = string(secretData[common.BackupBucketName])
	)
	opts := &clientconfig.ClientOpts{
		AuthInfo: &clientconfig.AuthInfo{
			AuthURL:     string(secretData[AuthURL]),
			Username:    string(secretData[UserName]),
			Password:    string(secretData[Password]),
			DomainName:  string(secretData[DomainName]),
			ProjectName: string(secretData[TenantName]),
		},
	}
	authOpts, err := clientconfig.AuthOptions(opts)
	if err != nil {
		return nil, err
	}

	// AllowReauth should be set to true if you grant permission for Gophercloud to
	// cache your credentials in memory, and to allow Gophercloud to attempt to
	// re-authenticate automatically if/when your token expires.
	authOpts.AllowReauth = true
	provider, err := openstack.AuthenticatedClient(*authOpts)
	if err != nil {
		return nil, err

	}

	client, err := openstack.NewObjectStorageV1(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err

	}

	return snapstore.NewSwiftSnapstoreFromClient(bucket, "etcd-main/v1", "", 10, client), nil
}
