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

package helper

import (
	"errors"

	"github.com/gardener/gardener/pkg/apis/garden"
)

// DetermineCloudProviderInProfile takes a CloudProfile specification and returns the cloud provider this profile is used for.
// If it is not able to determine it, an error will be returned.
func DetermineCloudProviderInProfile(spec garden.CloudProfileSpec) (garden.CloudProvider, error) {
	var (
		cloud     garden.CloudProvider
		numClouds = 0
	)

	if spec.AWS != nil {
		numClouds++
		cloud = garden.CloudProviderAWS
	}
	if spec.Azure != nil {
		numClouds++
		cloud = garden.CloudProviderAzure
	}
	if spec.GCP != nil {
		numClouds++
		cloud = garden.CloudProviderGCP
	}
	if spec.OpenStack != nil {
		numClouds++
		cloud = garden.CloudProviderOpenStack
	}
	if spec.Local != nil {
		numClouds++
		cloud = garden.CloudProviderLocal
	}

	if numClouds != 1 {
		return "", errors.New("cloud profile must only contain exactly one field of aws/azure/gcp/openstack/local")
	}
	return cloud, nil
}

// DetermineCloudProviderInShoot takes a Shoot cloud object and returns the cloud provider this profile is used for.
// If it is not able to determine it, an error will be returned.
func DetermineCloudProviderInShoot(cloudObj garden.Cloud) (garden.CloudProvider, error) {
	var (
		cloud     garden.CloudProvider
		numClouds = 0
	)

	if cloudObj.AWS != nil {
		numClouds++
		cloud = garden.CloudProviderAWS
	}
	if cloudObj.Azure != nil {
		numClouds++
		cloud = garden.CloudProviderAzure
	}
	if cloudObj.GCP != nil {
		numClouds++
		cloud = garden.CloudProviderGCP
	}
	if cloudObj.OpenStack != nil {
		numClouds++
		cloud = garden.CloudProviderOpenStack
	}
	if cloudObj.Local != nil {
		numClouds++
		cloud = garden.CloudProviderLocal
	}

	if numClouds != 1 {
		return "", errors.New("cloud object must only contain exactly one field of aws/azure/gcp/openstack/local")
	}
	return cloud, nil
}

// GetCondition returns the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns nil.
func GetCondition(conditions []garden.Condition, conditionType garden.ConditionType) *garden.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			c := condition
			return &c
		}
	}
	return nil
}
