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
	"errors"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/aws"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	corev1 "k8s.io/api/core/v1"
)

// New takes an operation object <o> and creates a new AWSBotanist object.
func New(o *operation.Operation, purpose string) (*AWSBotanist, error) {
	var (
		cloudProvider gardenv1beta1.CloudProvider
		secret        *corev1.Secret
		region        string
	)

	switch purpose {
	case common.CloudPurposeShoot:
		cloudProvider = o.Shoot.CloudProvider
		secret = o.Shoot.Secret
		region = o.Shoot.Info.Spec.Cloud.Region
	case common.CloudPurposeSeed:
		cloudProvider = o.Seed.CloudProvider
		secret = o.Seed.Secret
		region = o.Seed.Info.Spec.Cloud.Region
	}

	if cloudProvider != gardenv1beta1.CloudProviderAWS {
		return nil, errors.New("cannot instantiate an AWS botanist if neither Shoot nor Seed cluster specifies AWS")
	}

	return &AWSBotanist{
		Operation:         o,
		CloudProviderName: "aws",
		AWSClient:         aws.NewClient(string(secret.Data[AccessKeyID]), string(secret.Data[SecretAccessKey]), region),
	}, nil
}

// GetCloudProviderName returns the Kubernetes cloud provider name for this cloud.
func (b *AWSBotanist) GetCloudProviderName() string {
	return b.CloudProviderName
}
