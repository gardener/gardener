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

	gcpv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/apis/gcp/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GardenV1beta1ShootToGCPV1alpha1InfrastructureConfig converts a garden.sapcloud.io/v1beta1.Shoot to gcpv1alpha1.InfrastructureConfig.
// This function is only required temporarily for migration purposes and can be removed in the future when we switched to
// core.gardener.cloud/v1alpha1.Shoot.
func GardenV1beta1ShootToGCPV1alpha1InfrastructureConfig(shoot *gardenv1beta1.Shoot) (*gcpv1alpha1.InfrastructureConfig, error) {
	if shoot.Spec.Cloud.GCP == nil {
		return nil, fmt.Errorf("shoot is not of type GCP")
	}

	if len(shoot.Spec.Cloud.GCP.Networks.Workers) != 1 {
		return nil, fmt.Errorf("gcp worker networks must only have exactly one entry")
	}

	var vpc *gcpv1alpha1.VPC
	if shoot.Spec.Cloud.GCP.Networks.VPC != nil {
		vpc = &gcpv1alpha1.VPC{
			Name: shoot.Spec.Cloud.GCP.Networks.VPC.Name,
		}
	}

	return &gcpv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
			Kind:       infrastructureConfig,
		},
		Networks: gcpv1alpha1.NetworkConfig{
			VPC:      vpc,
			Worker:   shoot.Spec.Cloud.GCP.Networks.Workers[0],
			Internal: shoot.Spec.Cloud.GCP.Networks.Internal,
		},
	}, nil
}

// GardenV1beta1ShootToGCPV1alpha1ControlPlaneConfig converts a garden.sapcloud.io/v1beta1.Shoot to gcpv1alpha1.ControlPlaneConfig.
// This function is only required temporarily for migration purposes and can be removed in the future when we switched to
// core.gardener.cloud/v1alpha1.Shoot.
func GardenV1beta1ShootToGCPV1alpha1ControlPlaneConfig(shoot *gardenv1beta1.Shoot) (*gcpv1alpha1.ControlPlaneConfig, error) {
	if shoot.Spec.Cloud.GCP == nil {
		return nil, fmt.Errorf("shoot is not of type GCP")
	}

	var cloudControllerManager *gcpv1alpha1.CloudControllerManagerConfig
	if shoot.Spec.Kubernetes.CloudControllerManager != nil {
		cloudControllerManager = &gcpv1alpha1.CloudControllerManagerConfig{
			KubernetesConfig: shoot.Spec.Kubernetes.CloudControllerManager.KubernetesConfig,
		}
	}

	return &gcpv1alpha1.ControlPlaneConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
			Kind:       controlPlaneConfig,
		},
		Zone:                   shoot.Spec.Cloud.GCP.Zones[0],
		CloudControllerManager: cloudControllerManager,
	}, nil
}
