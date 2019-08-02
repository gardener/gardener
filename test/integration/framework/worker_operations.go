// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
)

// NewWorkerGardenerTest creates a new NewWorkerGardenerTest
func NewWorkerGardenerTest(ctx context.Context, shootGardenTest *ShootGardenerTest, k8sShootClient kubernetes.Interface) (*WorkerGardenerTest, error) {
	cloudProfileForShoot := &gardenv1beta1.CloudProfile{}
	if err := shootGardenTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: "garden", Name: shootGardenTest.Shoot.Spec.Cloud.Profile}, cloudProfileForShoot); err != nil {
		return nil, err
	}

	return &WorkerGardenerTest{
		ShootGardenerTest: shootGardenTest,
		CloudProfile:      cloudProfileForShoot,
		ShootClient:       k8sShootClient,
	}, nil
}

// GetMachineImagesFromShoot returns the MachineImages specified in the Shoot spec
func (s *WorkerGardenerTest) GetMachineImagesFromShoot() ([]gardenv1beta1.ShootMachineImage, error) {
	cloudProvider, err := s.GetCloudProvider()
	if err != nil {
		s.ShootGardenerTest.Logger.Infof("Can not get CloudProvider")
		return nil, err
	}

	machineImages := []gardenv1beta1.ShootMachineImage{}

	switch cloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		for _, worker := range s.ShootGardenerTest.Shoot.Spec.Cloud.AWS.Workers {
			if worker.MachineImage != nil {
				machineImages = append(machineImages, *worker.MachineImage)
			}
		}
	case gardenv1beta1.CloudProviderAzure:
		for _, worker := range s.ShootGardenerTest.Shoot.Spec.Cloud.Azure.Workers {
			if worker.MachineImage != nil {
				machineImages = append(machineImages, *worker.MachineImage)
			}
		}
	case gardenv1beta1.CloudProviderGCP:
		for _, worker := range s.ShootGardenerTest.Shoot.Spec.Cloud.GCP.Workers {
			if worker.MachineImage != nil {
				machineImages = append(machineImages, *worker.MachineImage)
			}
		}
	case gardenv1beta1.CloudProviderAlicloud:
		for _, worker := range s.ShootGardenerTest.Shoot.Spec.Cloud.Alicloud.Workers {
			if worker.MachineImage != nil {
				machineImages = append(machineImages, *worker.MachineImage)
			}
		}
	case gardenv1beta1.CloudProviderOpenStack:
		for _, worker := range s.ShootGardenerTest.Shoot.Spec.Cloud.OpenStack.Workers {
			if worker.MachineImage != nil {
				machineImages = append(machineImages, *worker.MachineImage)
			}
		}
	case gardenv1beta1.CloudProviderPacket:
		for _, worker := range s.ShootGardenerTest.Shoot.Spec.Cloud.Packet.Workers {
			if worker.MachineImage != nil {
				machineImages = append(machineImages, *worker.MachineImage)
			}
		}
	}

	return machineImages, nil
}

//GetCloudProvider returns the CloudProvider of the shoot
func (s *WorkerGardenerTest) GetCloudProvider() (gardenv1beta1.CloudProvider, error) {
	return helper.DetermineCloudProviderInProfile(s.CloudProfile.Spec)
}
