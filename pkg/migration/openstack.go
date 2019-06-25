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

	openstackv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-openstack/pkg/apis/openstack/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GardenV1beta1ShootToOpenStackV1alpha1InfrastructureConfig converts a garden.sapcloud.io/v1beta1.Shoot to openstackv1alpha1.InfrastructureConfig.
// This function is only required temporarily for migration purposes and can be removed in the future when we switched to
// core.gardener.cloud/v1alpha1.Shoot.
func GardenV1beta1ShootToOpenStackV1alpha1InfrastructureConfig(shoot *gardenv1beta1.Shoot) (*openstackv1alpha1.InfrastructureConfig, error) {
	if shoot.Spec.Cloud.OpenStack == nil {
		return nil, fmt.Errorf("shoot is not of type OpenStack")
	}

	if len(shoot.Spec.Cloud.OpenStack.Networks.Workers) != 1 {
		return nil, fmt.Errorf("openstack worker networks must only have exactly one entry")
	}

	var router *openstackv1alpha1.Router
	if shoot.Spec.Cloud.OpenStack.Networks.Router != nil {
		router = &openstackv1alpha1.Router{
			ID: shoot.Spec.Cloud.OpenStack.Networks.Router.ID,
		}
	}

	return &openstackv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
			Kind:       infrastructureConfig,
		},
		FloatingPoolName: shoot.Spec.Cloud.OpenStack.FloatingPoolName,
		Networks: openstackv1alpha1.Networks{
			Router: router,
			Worker: shoot.Spec.Cloud.OpenStack.Networks.Workers[0],
		},
	}, nil
}

// GardenV1beta1ShootToOpenStackV1alpha1ControlPlaneConfig converts a garden.sapcloud.io/v1beta1.Shoot to openstackv1alpha1.ControlPlaneConfig.
// This function is only required temporarily for migration purposes and can be removed in the future when we switched to
// core.gardener.cloud/v1alpha1.Shoot.
func GardenV1beta1ShootToOpenStackV1alpha1ControlPlaneConfig(shoot *gardenv1beta1.Shoot) (*openstackv1alpha1.ControlPlaneConfig, error) {
	if shoot.Spec.Cloud.OpenStack == nil {
		return nil, fmt.Errorf("shoot is not of type OpenStack")
	}

	var cloudControllerManager *openstackv1alpha1.CloudControllerManagerConfig
	if shoot.Spec.Kubernetes.CloudControllerManager != nil {
		cloudControllerManager = &openstackv1alpha1.CloudControllerManagerConfig{
			KubernetesConfig: shoot.Spec.Kubernetes.CloudControllerManager.KubernetesConfig,
		}
	}

	return &openstackv1alpha1.ControlPlaneConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
			Kind:       controlPlaneConfig,
		},
		Zone:                   shoot.Spec.Cloud.OpenStack.Zones[0],
		LoadBalancerProvider:   shoot.Spec.Cloud.OpenStack.LoadBalancerProvider,
		LoadBalancerClasses:    gardenV1beta1OpenStackLoadBalancerClassToOpenStackV1alpha1LoadBalancerClass(shoot.Spec.Cloud.OpenStack.LoadBalancerClasses),
		CloudControllerManager: cloudControllerManager,
	}, nil
}

func gardenV1beta1OpenStackLoadBalancerClassToOpenStackV1alpha1LoadBalancerClass(loadBalancerClasses []gardenv1beta1.OpenStackLoadBalancerClass) []openstackv1alpha1.LoadBalancerClass {
	out := make([]openstackv1alpha1.LoadBalancerClass, 0, len(loadBalancerClasses))
	for _, loadBalancerClass := range loadBalancerClasses {
		out = append(out, openstackv1alpha1.LoadBalancerClass{
			Name:              loadBalancerClass.Name,
			FloatingSubnetID:  loadBalancerClass.FloatingSubnetID,
			FloatingNetworkID: loadBalancerClass.FloatingNetworkID,
			SubnetID:          loadBalancerClass.SubnetID,
		})
	}
	return out
}
