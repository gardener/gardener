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

package awsbotanist

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gardener/etcd-backup-restore/pkg/snapstore"
	"github.com/gardener/gardener/pkg/operation/common"
)

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *AWSBotanist) GenerateEtcdBackupConfig() (map[string][]byte, error) {
	bucketName := "bucketName"

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
		Region:                  []byte(b.Seed.Info.Spec.Cloud.Region),
		AccessKeyID:             b.Seed.Secret.Data[AccessKeyID],
		SecretAccessKey:         b.Seed.Secret.Data[SecretAccessKey],
	}

	return secretData, nil
}

// GetEtcdBackupSnapstore returns the etcd backup snapstore object.
func (b *AWSBotanist) GetEtcdBackupSnapstore(secretData map[string][]byte) (snapstore.SnapStore, error) {
	var (
		accessKeyID     = string(secretData[AccessKeyID])
		secretAccessKey = string(secretData[SecretAccessKey])
		region          = string(secretData[Region])
		bucket          = string(secretData[common.BackupBucketName])
		awsConfig       = &aws.Config{
			Credentials: credentials.NewStaticCredentials(accessKeyID, secretAccessKey, ""),
		}
		config = &aws.Config{Region: aws.String(region)}
	)

	s, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, err
	}

	cli := s3.New(s, config)

	return snapstore.NewS3FromClient(bucket, "etcd-main/v1", "", 10, cli), nil
}
