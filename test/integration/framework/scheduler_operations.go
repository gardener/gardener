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
// limitations under the License.

package framework

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/scheduler/controller"
	"github.com/gardener/gardener/pkg/utils"
)

const configurationFileName = "schedulerconfiguration.yaml"

// NewGardenSchedulerTest creates a new SchedulerGardenerTest by retrieving the ConfigMap containing the Scheduler Configuration & parsing the Scheduler Configuration
func NewGardenSchedulerTest(ctx context.Context, shootGardenTest *ShootGardenerTest) (*SchedulerGardenerTest, error) {
	schedulerConfigurationConfigMap := &corev1.ConfigMap{}
	if err := shootGardenTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: schedulerconfigv1alpha1.SchedulerDefaultConfigurationConfigMapNamespace, Name: schedulerconfigv1alpha1.SchedulerDefaultConfigurationConfigMapName}, schedulerConfigurationConfigMap); err != nil {
		return nil, err
	}

	schedulerConfiguration, err := parseSchedulerConfiguration(schedulerConfigurationConfigMap)
	if err != nil {
		return nil, err
	}

	cloudProfileForShoot := &gardenv1beta1.CloudProfile{}
	if err := shootGardenTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: "garden", Name: shootGardenTest.Shoot.Spec.Cloud.Profile}, cloudProfileForShoot); err != nil {
		return nil, err
	}

	allSeeds := &gardenv1beta1.SeedList{}
	if err := shootGardenTest.GardenClient.Client().List(ctx, allSeeds); err != nil {
		return nil, err
	}

	return &SchedulerGardenerTest{
		ShootGardenerTest:      shootGardenTest,
		SchedulerConfiguration: schedulerConfiguration,
		CloudProfile:           cloudProfileForShoot,
		Seeds:                  allSeeds.Items,
	}, nil
}

func parseSchedulerConfiguration(configuration *corev1.ConfigMap) (*config.SchedulerConfiguration, error) {
	if configuration == nil {
		return nil, fmt.Errorf("scheduler Configuration could not be extracted from ConfigMap. The gardener setup with the helm chart creates this config map")
	}

	rawConfig := configuration.Data[configurationFileName]
	byteConfig := []byte(rawConfig)
	scheme := runtime.NewScheme()
	if err := config.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := schedulerconfigv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	codecs := serializer.NewCodecFactory(scheme)
	configObj, gvk, err := codecs.UniversalDecoder().Decode(byteConfig, nil, nil)
	if err != nil {
		return nil, err
	}
	config, ok := configObj.(*config.SchedulerConfiguration)
	if !ok {
		return nil, fmt.Errorf("got unexpected config type: %v", gvk)
	}
	return config, nil
}

// CreateShoot Creates a shoot from a shoot Object
func (s *SchedulerGardenerTest) CreateShoot(ctx context.Context) (*gardenv1beta1.Shoot, error) {
	_, err := s.ShootGardenerTest.GetShoot(ctx)
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	shoot := s.ShootGardenerTest.Shoot
	if err := s.ShootGardenerTest.GardenClient.Client().Create(ctx, shoot); err != nil {
		return nil, err
	}

	s.ShootGardenerTest.Logger.Infof("Shoot %s was created!", shoot.Name)
	return shoot, nil
}

// ScheduleShoot set the Spec.Cloud.Seed of a shoot to the specified seed.
// This is the request the Gardener Scheduler executes after a scheduling decision.
func (s *SchedulerGardenerTest) ScheduleShoot(ctx context.Context, shoot *gardenv1beta1.Shoot, seed *gardenv1beta1.Seed) error {
	executeSchedulingRequest := func(ctx context.Context, shootToUpdate *gardenv1beta1.Shoot) error {
		if _, err := s.ShootGardenerTest.GardenClient.Garden().GardenV1beta1().Shoots(shootToUpdate.Namespace).Update(shootToUpdate); err != nil {
			return err
		}
		return nil
	}
	return controller.UpdateShootToBeScheduledOntoSeed(ctx, shoot, seed, executeSchedulingRequest)
}

// ChooseRegionAndZoneWithNoSeed determines all available Zones from the Shoot and the CloudProfile and then delegates the choosing of a region were no seed is deployed
func (s *SchedulerGardenerTest) ChooseRegionAndZoneWithNoSeed() (*gardenv1beta1.CloudProvider, *string, []string, error) {
	// use helper to get all available Zones
	cloudProvider, allAvailableZones, err := helper.GetZones(*s.ShootGardenerTest.Shoot, s.CloudProfile)
	if err != nil {
		return nil, nil, nil, err
	}
	foundRegion, zones, err := ChooseRegionAndZoneWithNoSeed(cloudProvider, allAvailableZones, s.CloudProfile, s.Seeds)
	return &cloudProvider, foundRegion, zones, err
}

// ChooseRegionAndZoneWithNoSeed chooses a region within the cloud provider of the shoot were no seed is deployed and that is allowed by the cloud profile
func ChooseRegionAndZoneWithNoSeed(cloudProvider gardenv1beta1.CloudProvider, allAvailableZones []gardenv1beta1.Zone, cloudProfile *gardenv1beta1.CloudProfile, seeds []gardenv1beta1.Seed) (*string, []string, error) {
	allAvailableRegions := []string{}

	if cloudProvider == gardenv1beta1.CloudProviderAzure {
		allAvailableRegions = append(allAvailableRegions, GetAllAzureRegions(cloudProfile.Spec.Azure.CountUpdateDomains)...)
		allAvailableRegions = append(allAvailableRegions, GetAllAzureRegions(cloudProfile.Spec.Azure.CountFaultDomains)...)
	} else {
		for _, zone := range allAvailableZones {
			allAvailableRegions = append(allAvailableRegions, zone.Region)
		}
	}

	if len(allAvailableRegions) == 0 {
		return nil, nil, fmt.Errorf("could not find any region in Cloud Profile %s", cloudProfile.Name)
	}

	var (
		foundRegion string
		zones       []string
	)

	// Find Region where no seed is deployed -
	for index, region := range allAvailableRegions {
		foundRegionInSeed := false
		for _, seed := range seeds {
			if seed.Spec.Cloud.Region == region {
				foundRegionInSeed = true
				break
			}
		}
		if foundRegionInSeed == false {
			foundRegion = region
			// Azure has no regions, so for all other cloud providers we get a zoneName from the CloudProvider for the determined region
			if len(allAvailableZones) >= index {
				// we only pick one zone in the test. The shoot must specify as many workers networks as zones.
				zones = []string{allAvailableZones[index].Names[0]}
			}
			break
		}
	}

	if len(foundRegion) == 0 {
		return nil, nil, fmt.Errorf("could not determine a region where no seed is deployed already")
	}

	return &foundRegion, zones, nil
}

// ChooseSeedWhereTestShootIsNotDeployed determines a Seed different from the shoot's seed respecting the CloudProvider
// If none can be found, it returns an error
func (s *SchedulerGardenerTest) ChooseSeedWhereTestShootIsNotDeployed(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Seed, error) {
	for _, seed := range s.Seeds {
		if seed.Name != *shoot.Spec.Cloud.Seed && seed.Spec.Cloud.Profile == shoot.Spec.Cloud.Profile {
			return &seed, nil
		}
	}

	return nil, fmt.Errorf("Could not find another seed that is not in use by the test shoot")
}

// GetAllAzureRegions extract the regions as a string slice from AzureDomainCounts
func GetAllAzureRegions(domainCounts []gardenv1beta1.AzureDomainCount) []string {
	regions := []string{}
	for _, domainCount := range domainCounts {
		regions = append(regions, domainCount.Region)
	}
	return regions
}

// WaitForShootToBeUnschedulable waits for the shoot to be unschedulable. This is indicated by Events created by the scheduler on the shoot
func (s *SchedulerGardenerTest) WaitForShootToBeUnschedulable(ctx context.Context) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		shoot := &gardenv1beta1.Shoot{}
		err := s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.ShootGardenerTest.Shoot.Namespace, Name: s.ShootGardenerTest.Shoot.Name}, shoot)
		if err != nil {
			return false, err
		}
		s.ShootGardenerTest.Logger.Infof("waiting for shoot %s to be unschedulable", s.ShootGardenerTest.Shoot.Name)

		uid := fmt.Sprintf("%s", s.ShootGardenerTest.Shoot.UID)
		kind := "Shoot"
		fieldSelector := s.ShootGardenerTest.GardenClient.Kubernetes().CoreV1().Events(s.ShootGardenerTest.Shoot.Namespace).GetFieldSelector(&s.ShootGardenerTest.Shoot.Name, &s.ShootGardenerTest.Shoot.Namespace, &kind, &uid)
		eventList, err := s.ShootGardenerTest.GardenClient.Kubernetes().CoreV1().Events(s.ShootGardenerTest.Shoot.Namespace).List(v1.ListOptions{FieldSelector: fieldSelector.String()})
		if err != nil {
			return false, err
		}
		if shootIsUnschedulable(eventList.Items) {
			return true, nil
		}

		if shoot.Status.LastOperation != nil {
			s.ShootGardenerTest.Logger.Debugf("%d%%: Shoot State: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return false, nil
	}, ctx.Done())
}

// WaitForShootToBeScheduled waits for the shoot to be scheduled successfully
func (s *SchedulerGardenerTest) WaitForShootToBeScheduled(ctx context.Context) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		shoot := &gardenv1beta1.Shoot{}
		err := s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.ShootGardenerTest.Shoot.Namespace, Name: s.ShootGardenerTest.Shoot.Name}, shoot)
		if err != nil {
			return false, err
		}
		if shootIsScheduledSuccessfully(&shoot.Spec) {
			return true, nil
		}
		s.ShootGardenerTest.Logger.Infof("waiting for shoot %s to be scheduled", s.ShootGardenerTest.Shoot.Name)
		if shoot.Status.LastOperation != nil {
			s.ShootGardenerTest.Logger.Debugf("%d%%: Shoot State: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return false, nil
	}, ctx.Done())
}

// GenerateInvalidShoot generates a shoot with an invalid dummy name
func (s *SchedulerGardenerTest) GenerateInvalidShoot() (*gardenv1beta1.Shoot, error) {
	shoot := s.ShootGardenerTest.Shoot.DeepCopy()
	randomString, err := utils.GenerateRandomString(10)
	if err != nil {
		return nil, err
	}
	shoot.ObjectMeta.Name = "dummy-" + randomString
	return shoot, nil
}

// GenerateInvalidSeed generates a seed with an invalid dummy name
func (s *SchedulerGardenerTest) GenerateInvalidSeed() (*gardenv1beta1.Seed, error) {
	validSeed := s.Seeds[0]
	if len(validSeed.Name) == 0 {
		return nil, fmt.Errorf("failed to retrieve a valid seed from the current cluster")
	}
	invalidSeed := validSeed.DeepCopy()
	randomString, err := utils.GenerateRandomString(10)
	if err != nil {
		return nil, fmt.Errorf("failed to generate a random string for the name of the seed cluster")
	}
	invalidSeed.ObjectMeta.Name = "dummy-" + randomString
	return invalidSeed, nil
}
