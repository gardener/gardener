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

	packetv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-packet/pkg/apis/packet/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GardenV1beta1ShootToPacketV1alpha1InfrastructureConfig converts a garden.sapcloud.io/v1beta1.Shoot to packetv1alpha1.InfrastructureConfig.
// This function is only required temporarily for migration purposes and can be removed in the future when we switched to
// core.gardener.cloud/v1alpha1.Shoot.
func GardenV1beta1ShootToPacketV1alpha1InfrastructureConfig(shoot *gardenv1beta1.Shoot) (*packetv1alpha1.InfrastructureConfig, error) {
	if shoot.Spec.Cloud.Packet == nil {
		return nil, fmt.Errorf("shoot is not of type Packet")
	}

	return &packetv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: packetv1alpha1.SchemeGroupVersion.String(),
			Kind:       infrastructureConfig,
		},
	}, nil
}

// GardenV1beta1ShootToPacketV1alpha1ControlPlaneConfig converts a garden.sapcloud.io/v1beta1.Shoot to packetv1alpha1.ControlPlaneConfig.
// This function is only required temporarily for migration purposes and can be removed in the future when we switched to
// core.gardener.cloud/v1alpha1.Shoot.
func GardenV1beta1ShootToPacketV1alpha1ControlPlaneConfig(shoot *gardenv1beta1.Shoot) (*packetv1alpha1.ControlPlaneConfig, error) {
	if shoot.Spec.Cloud.Packet == nil {
		return nil, fmt.Errorf("shoot is not of type Packet")
	}

	// TODO Possibly extend later
	return &packetv1alpha1.ControlPlaneConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: packetv1alpha1.SchemeGroupVersion.String(),
			Kind:       controlPlaneConfig,
		},
	}, nil
}
