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
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// DeployBackupInfrastructure kicks off a Terraform job which deploys the infrastructure resources for backup.
// It sets up the User and the Bucket to store the backups. Allocate permission to the User to access the bucket.
func (b *AlicloudBotanist) DeployBackupInfrastructure() error {
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		InitializeWith(b.ChartInitializer("alicloud-backup", b.generateTerraformBackupConfig())).
		Apply()
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for etcd backup.
func (b *AlicloudBotanist) DestroyBackupInfrastructure() error {
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return err
	}

	// Must clean snapshots before deleting the bucket
	stateVariables, err := tf.GetStateOutputVariables(common.BackupBucketName, StorageEndpoint)
	if err != nil {
		if terraformer.IsVariablesNotFoundError(err) {
			b.Logger.Infof("Skipping Alicloud backup storage bucket deletion because no storage endpoint has been found in the Terraform state.")
			return nil
		}
		return err
	}

	err = cleanSnapshots(stateVariables[common.BackupBucketName], stateVariables[StorageEndpoint],
		string(b.Seed.Secret.Data[AccessKeyID]), string(b.Seed.Secret.Data[AccessKeySecret]))
	if err != nil {
		return err
	}

	// Clean the bucket using terraformer
	return tf.
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		Destroy()
}

func (b *AlicloudBotanist) generateTerraformBackupVariablesEnvironment() map[string]string {
	return terraformer.GenerateVariablesEnvironment(b.Seed.Secret, map[string]string{
		"ACCESS_KEY_ID":     AccessKeyID,
		"ACCESS_KEY_SECRET": AccessKeySecret,
	})
}

func (b *AlicloudBotanist) generateTerraformBackupConfig() map[string]interface{} {
	return map[string]interface{}{
		"alicloud": map[string]interface{}{
			"region": b.Seed.Info.Spec.Cloud.Region,
		},
		"bucket": map[string]interface{}{
			"name": b.Operation.BackupInfrastructure.Name,
		},
	}
}

func cleanSnapshots(bucketName, storageEndpoint, accessKeyID, accessKeySecret string) error {
	client, err := oss.New(storageEndpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return err
	}

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return err
	}

	for {
		var snapshots []string
		lsRes, err := bucket.ListObjects()
		if err != nil {
			return err
		}
		for _, object := range lsRes.Objects {
			snapshots = append(snapshots, object.Key)
		}
		if len(snapshots) > 0 {
			if _, err = bucket.DeleteObjects(snapshots); err != nil {
				return err
			}
		}
		if !lsRes.IsTruncated {
			break
		}
	}
	return nil
}
