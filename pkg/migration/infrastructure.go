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

package migration

import (
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	helper "github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"

	"k8s.io/apimachinery/pkg/runtime"
)

const (
	infrastructureConfig = "InfrastructureConfig"
)

// ShootToInfrastructureConfig computes the provider specific infrastructure config.
func ShootToInfrastructureConfig(shoot *gardenv1beta1.Shoot) (runtime.Object, error) {
	cloudProvider, err := helper.GetShootCloudProvider(shoot)
	if err != nil {
		return nil, err
	}

	switch cloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		return GardenV1beta1ShootToAWSV1alpha1InfrastructureConfig(shoot)
	case gardenv1beta1.CloudProviderAzure:
		return GardenV1beta1ShootToAzureV1alpha1InfrastructureConfig(shoot)
	case gardenv1beta1.CloudProviderGCP:
		return GardenV1beta1ShootToGCPV1alpha1InfrastructureConfig(shoot)
	case gardenv1beta1.CloudProviderOpenStack:
		return GardenV1beta1ShootToOpenStackV1alpha1InfrastructureConfig(shoot)
	case gardenv1beta1.CloudProviderAlicloud:
		return GardenV1beta1ShootToAlicloudV1alpha1InfrastructureConfig(shoot)
	case gardenv1beta1.CloudProviderPacket:
		return GardenV1beta1ShootToPacketV1alpha1InfrastructureConfig(shoot)
	}

	return nil, fmt.Errorf("cannot compute infrastructure config for shoot: unknown cloud provider: %+v", cloudProvider)
}
