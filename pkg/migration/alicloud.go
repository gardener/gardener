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

	alicloudv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-alicloud/pkg/apis/alicloud/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GardenV1beta1ShootToAlicloudV1alpha1InfrastructureConfig converts a garden.sapcloud.io/v1beta1.Shoot to alicloudv1alpha1.InfrastructureConfig.
// This function is only required temporarily for migration purposes and can be removed in the future when we switched to
// core.gardener.cloud/v1alpha1.Shoot.
func GardenV1beta1ShootToAlicloudV1alpha1InfrastructureConfig(shoot *gardenv1beta1.Shoot) (*alicloudv1alpha1.InfrastructureConfig, error) {
	if shoot.Spec.Cloud.Alicloud == nil {
		return nil, fmt.Errorf("shoot is not of type Alicloud")
	}
	if len(shoot.Spec.Cloud.Alicloud.Networks.Workers) != len(shoot.Spec.Cloud.Alicloud.Zones) {
		return nil, fmt.Errorf("alicloud workers networks must have same number of entries like zones")
	}

	zones := make([]alicloudv1alpha1.Zone, 0, len(shoot.Spec.Cloud.Alicloud.Zones))
	for i, zone := range shoot.Spec.Cloud.Alicloud.Zones {
		zones = append(zones, alicloudv1alpha1.Zone{
			Name:   zone,
			Worker: shoot.Spec.Cloud.Alicloud.Networks.Workers[i],
		})
	}

	return &alicloudv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: alicloudv1alpha1.SchemeGroupVersion.String(),
			Kind:       infrastructureConfig,
		},
		Networks: alicloudv1alpha1.Networks{
			VPC: alicloudv1alpha1.VPC{
				ID:   shoot.Spec.Cloud.Alicloud.Networks.VPC.ID,
				CIDR: shoot.Spec.Cloud.Alicloud.Networks.VPC.CIDR,
			},
			Zones: zones,
		},
	}, nil
}

// GardenV1beta1ShootToAlicloudV1alpha1ControlPlaneConfig converts a garden.sapcloud.io/v1beta1.Shoot to alicloudv1alpha1.ControlPlaneConfig.
// This function is only required temporarily for migration purposes and can be removed in the future when we switched to
// core.gardener.cloud/v1alpha1.Shoot.
func GardenV1beta1ShootToAlicloudV1alpha1ControlPlaneConfig(shoot *gardenv1beta1.Shoot) (*alicloudv1alpha1.ControlPlaneConfig, error) {
	if shoot.Spec.Cloud.Alicloud == nil {
		return nil, fmt.Errorf("shoot is not of type Alicloud")
	}

	var cloudControllerManager *alicloudv1alpha1.CloudControllerManagerConfig
	if shoot.Spec.Kubernetes.CloudControllerManager != nil {
		cloudControllerManager = &alicloudv1alpha1.CloudControllerManagerConfig{
			KubernetesConfig: shoot.Spec.Kubernetes.CloudControllerManager.KubernetesConfig,
		}
	}

	return &alicloudv1alpha1.ControlPlaneConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: alicloudv1alpha1.SchemeGroupVersion.String(),
			Kind:       controlPlaneConfig,
		},
		Zone:                   shoot.Spec.Cloud.Alicloud.Zones[0],
		CloudControllerManager: cloudControllerManager,
	}, nil
}
