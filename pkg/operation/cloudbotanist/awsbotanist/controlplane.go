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
	"path/filepath"

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

// DeployCloudSpecificControlPlane updates the AWS ELB health check to SSL and deploys the aws-lb-readvertiser.
// https://github.com/gardener/aws-lb-readvertiser
func (b *AWSBotanist) DeployCloudSpecificControlPlane() error {
	var (
		name          = "aws-lb-readvertiser"
		defaultValues = map[string]interface{}{
			"domain":   b.APIServerAddress,
			"replicas": b.Shoot.GetReplicas(1),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-aws-lb-readvertiser": b.CheckSums[name],
			},
		}
	)

	values, err := b.InjectSeedShootImages(defaultValues, common.AWSLBReadvertiserImageName)
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-controlplane", "charts", name), b.Shoot.SeedNamespace, name, nil, values)
}
