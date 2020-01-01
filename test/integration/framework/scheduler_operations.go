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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"

	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const configurationFileName = "schedulerconfiguration.yaml"

// NewGardenSchedulerTest creates a new SchedulerGardenerTest by retrieving the ConfigMap containing the Scheduler Configuration & parsing the Scheduler Configuration
func NewGardenSchedulerTest(ctx context.Context, shootGardenTest *ShootGardenerTest, hostKubeconfigPath string) (*SchedulerGardenerTest, error) {
	k8sHostClient, err := kubernetes.NewClientFromFile("", hostKubeconfigPath, kubernetes.WithClientOptions(
		client.Options{
			Scheme: kubernetes.ShootScheme,
		}),
	)
	if err != nil {
		return nil, err
	}

	schedulerConfigurationConfigMap := &corev1.ConfigMap{}
	if err := k8sHostClient.Client().Get(ctx, client.ObjectKey{Namespace: schedulerconfigv1alpha1.SchedulerDefaultConfigurationConfigMapNamespace, Name: schedulerconfigv1alpha1.SchedulerDefaultConfigurationConfigMapName}, schedulerConfigurationConfigMap); err != nil {
		return nil, err
	}

	schedulerConfiguration, err := parseSchedulerConfiguration(schedulerConfigurationConfigMap)
	if err != nil {
		return nil, err
	}

	cloudProfileForShoot := &gardencorev1beta1.CloudProfile{}
	if err := shootGardenTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: "garden", Name: shootGardenTest.Shoot.Spec.CloudProfileName}, cloudProfileForShoot); err != nil {
		return nil, err
	}

	allSeeds := &gardencorev1beta1.SeedList{}
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
func (s *SchedulerGardenerTest) CreateShoot(ctx context.Context) (*gardencorev1beta1.Shoot, error) {
	_, err := s.ShootGardenerTest.GetShoot(ctx)
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	return s.ShootGardenerTest.CreateShootResource(ctx, s.ShootGardenerTest.Shoot)
}

// ScheduleShoot set the Spec.Cloud.Seed of a shoot to the specified seed.
// This is the request the Gardener Scheduler executes after a scheduling decision.
func (s *SchedulerGardenerTest) ScheduleShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed) error {
	executeSchedulingRequest := func(ctx context.Context, shootToUpdate *gardencorev1beta1.Shoot) error {
		if _, err := s.ShootGardenerTest.GardenClient.GardenCore().CoreV1beta1().Shoots(shootToUpdate.Namespace).Update(shootToUpdate); err != nil {
			return err
		}
		return nil
	}
	shoot.Spec.SeedName = &seed.Name
	return executeSchedulingRequest(ctx, shoot)
}

// ChooseRegionAndZoneWithNoSeed determines all available Zones from the Shoot and the CloudProfile and then delegates the choosing of a region were no seed is deployed
func (s *SchedulerGardenerTest) ChooseRegionAndZoneWithNoSeed() (*gardencorev1beta1.Region, error) {
	return ChooseRegionAndZoneWithNoSeed(s.CloudProfile.Spec.Regions, s.Seeds)
}

// ChooseRegionAndZoneWithNoSeed chooses a region within the cloud provider of the shoot were no seed is deployed and that is allowed by the cloud profile
func ChooseRegionAndZoneWithNoSeed(regions []gardencorev1beta1.Region, seeds []gardencorev1beta1.Seed) (*gardencorev1beta1.Region, error) {
	if len(regions) == 0 {
		return nil, fmt.Errorf("no regions configured in CloudProfile")
	}

	// Find Region where no seed is deployed -
	for _, region := range regions {
		foundRegionInSeed := false
		for _, seed := range seeds {
			if seed.Spec.Provider.Region == region.Name {
				foundRegionInSeed = true
				break
			}
		}
		if foundRegionInSeed == false {
			return &region, nil
		}
	}
	return nil, fmt.Errorf("could not determine a region where no seed is deployed already")
}

// ChooseSeedWhereTestShootIsNotDeployed determines a Seed different from the shoot's seed using the same Provider(e.g aws)
// If none can be found, it returns an error
func (s *SchedulerGardenerTest) ChooseSeedWhereTestShootIsNotDeployed(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Seed, error) {
	for _, seed := range s.Seeds {
		if seed.Name != *shoot.Spec.SeedName && seed.Spec.Provider.Type == shoot.Spec.Provider.Type {
			return &seed, nil
		}
	}

	return nil, fmt.Errorf("could not find another seed that is not in use by the test shoot")
}

// WaitForShootToBeUnschedulable waits for the shoot to be unschedulable. This is indicated by Events created by the scheduler on the shoot
func (s *SchedulerGardenerTest) WaitForShootToBeUnschedulable(ctx context.Context) error {
	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (bool, error) {
		shoot := &gardencorev1beta1.Shoot{}
		err := s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.ShootGardenerTest.Shoot.Namespace, Name: s.ShootGardenerTest.Shoot.Name}, shoot)
		if err != nil {
			return false, err
		}
		s.ShootGardenerTest.Logger.Infof("waiting for shoot %s to be unschedulable", s.ShootGardenerTest.Shoot.Name)

		uid := fmt.Sprintf("%s", s.ShootGardenerTest.Shoot.UID)
		kind := "Shoot"
		fieldSelector := s.ShootGardenerTest.GardenClient.Kubernetes().CoreV1().Events(s.ShootGardenerTest.Shoot.Namespace).GetFieldSelector(&s.ShootGardenerTest.Shoot.Name, &s.ShootGardenerTest.Shoot.Namespace, &kind, &uid)
		eventList, err := s.ShootGardenerTest.GardenClient.Kubernetes().CoreV1().Events(s.ShootGardenerTest.Shoot.Namespace).List(metav1.ListOptions{FieldSelector: fieldSelector.String()})
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
	})
}

// WaitForShootToBeScheduled waits for the shoot to be scheduled successfully
func (s *SchedulerGardenerTest) WaitForShootToBeScheduled(ctx context.Context) error {
	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (bool, error) {
		shoot := &gardencorev1beta1.Shoot{}
		err := s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.ShootGardenerTest.Shoot.Namespace, Name: s.ShootGardenerTest.Shoot.Name}, shoot)
		if err != nil {
			return retry.SevereError(err)
		}
		if shootIsScheduledSuccessfully(&shoot.Spec) {
			return retry.Ok()
		}
		s.ShootGardenerTest.Logger.Infof("waiting for shoot %s to be scheduled", s.ShootGardenerTest.Shoot.Name)
		if shoot.Status.LastOperation != nil {
			s.ShootGardenerTest.Logger.Debugf("%d%%: Shoot State: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return retry.MinorError(fmt.Errorf("shoot %s is not yet scheduled", s.ShootGardenerTest.Shoot.Name))
	})
}

// GenerateInvalidShoot generates a shoot with an invalid dummy name
func (s *SchedulerGardenerTest) GenerateInvalidShoot() (*gardencorev1beta1.Shoot, error) {
	shoot := s.ShootGardenerTest.Shoot.DeepCopy()
	randomString, err := utils.GenerateRandomString(10)
	if err != nil {
		return nil, err
	}
	shoot.ObjectMeta.Name = "dummy-" + randomString
	return shoot, nil
}

// GenerateInvalidSeed generates a seed with an invalid dummy name
func (s *SchedulerGardenerTest) GenerateInvalidSeed() (*gardencorev1beta1.Seed, error) {
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
