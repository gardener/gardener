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

	"k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/client/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sirupsen/logrus"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// NewPlantTest creates a new shootGardenerTest object, given an already created shoot (created after parsing a shoot YAML)
func NewPlantTest(kubeconfig string, plant *gardencorev1alpha1.Plant, shoot *v1beta1.Shoot, logger *logrus.Logger) (*PlantTest, error) {
	if len(kubeconfig) == 0 {
		return nil, fmt.Errorf("Please specify the kubeconfig path correctly")
	}

	k8sGardenClient, err := kubernetes.NewClientFromFile("", kubeconfig, client.Options{
		Scheme: kubernetes.GardenScheme,
	})
	if err != nil {
		return nil, err
	}

	return &PlantTest{
		GardenClient: k8sGardenClient,
		Plant:        plant,
		Shoot:        shoot,
		Logger:       logger,
	}, nil
}

// CreatePlantSecret creates a new Secret for the Plant
func (s *PlantTest) CreatePlantSecret(ctx context.Context) error {

	if s.Shoot == nil {
		return fmt.Errorf("Shoot to use as Plant cluster is unavailable ")
	}
	// Retrieve Shoot Secret
	secret := &v1.Secret{}

	// Name of the shoot secret in the shoot namespace in the garden cluster
	shootKubeconfigSecretName := s.Shoot.ObjectMeta.Name + ".kubeconfig"
	err := s.GardenClient.Client().Get(ctx, client.ObjectKey{
		Namespace: s.Shoot.ObjectMeta.Namespace,
		Name:      shootKubeconfigSecretName,
	}, secret)

	if err != nil {
		s.Logger.Errorf("Unable to retrieve the secret of the shoot that should be used as a plant secret. Namespace: '%s' , Secret Name: '%s'", s.Shoot.ObjectMeta.Namespace, shootKubeconfigSecretName)
		return err
	}

	kubeconfig, ok := secret.Data["kubeconfig"]
	if !ok {
		s.Logger.Errorf("Shoot secret in namespace: '%s' and name: '%s' that should be used as a plant secret does not contain a valid kubeconfig", s.Shoot.ObjectMeta.Namespace, shootKubeconfigSecretName)
		return err
	}

	plantSecretName := "plant-test-" + secret.Name
	plantSecret := v1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: s.Plant.Namespace, Name: plantSecretName}}
	plantSecret.Data = make(map[string][]byte)
	plantSecret.Data["kubeconfig"] = kubeconfig

	err = s.GardenClient.Client().Create(ctx, &plantSecret)

	if err != nil {
		s.Logger.Errorf("Unable to create the plant secret as a copy of the shoot secret in namespace: '%s' and name: '%s'", s.Shoot.ObjectMeta.Namespace, shootKubeconfigSecretName)
		return err
	}

	s.PlantSecret = &plantSecret
	return nil
}

// DeletePlantSecret deletes the Secret of the Plant
func (s *PlantTest) DeletePlantSecret(ctx context.Context) error {
	err := s.GardenClient.Client().Delete(ctx, s.PlantSecret)

	if err != nil {
		return err
	}
	return nil
}

// UpdatePlantSecret updates the Secret of the Plant
func (s *PlantTest) UpdatePlantSecret(ctx context.Context, updatedPlantSecret *v1.Secret) error {
	err := s.GardenClient.Client().Update(ctx, updatedPlantSecret)

	if err != nil {
		return err
	}

	s.PlantSecret = updatedPlantSecret
	return nil
}

// GetPlantSecret retrieves the Secret of the Plant. Returns the Secret.
func (s *PlantTest) GetPlantSecret(ctx context.Context) (*v1.Secret, error) {
	secret := &v1.Secret{}
	err := s.GardenClient.Client().Get(ctx, client.ObjectKey{
		Namespace: s.Plant.Spec.SecretRef.Namespace,
		Name:      s.Plant.Spec.SecretRef.Name,
	}, secret)

	if err != nil {
		return nil, err
	}
	return secret, err
}

// GetPlant gets the test plant
func (s *PlantTest) GetPlant(ctx context.Context) (*gardencorev1alpha1.Plant, error) {
	plant := &gardencorev1alpha1.Plant{}
	err := s.GardenClient.Client().Get(ctx, client.ObjectKey{
		Namespace: s.Plant.Namespace,
		Name:      s.Plant.Name,
	}, plant)

	if err != nil {
		return nil, err
	}
	return plant, err
}

// CreatePlant Creates a plant from a plant Object
func (s *PlantTest) CreatePlant(ctx context.Context) (*gardencorev1alpha1.Plant, error) {
	_, err := s.GetPlant(ctx)
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	plant := s.Plant

	// Secret has been already created in a previous test step
	plant.Spec.SecretRef.Namespace = s.PlantSecret.Namespace
	plant.Spec.SecretRef.Name = s.PlantSecret.Name

	err = s.GardenClient.Client().Create(ctx, s.Plant)
	if err != nil {
		return nil, err
	}

	err = s.WaitForPlantToBeCreated(ctx)
	if err != nil {
		return nil, err
	}

	s.Logger.Infof("Plant %s was created!", plant.Name)
	return plant, nil
}

// DeletePlant deletes the test plant
func (s *PlantTest) DeletePlant(ctx context.Context) error {

	err := s.GardenClient.Client().Delete(ctx, s.Plant)
	if err != nil {
		return err
	}

	err = s.WaitForPlantToBeDeleted(ctx)
	if err != nil {
		return err
	}

	s.Logger.Infof("Plant %s was deleted successfully!", s.Shoot.ObjectMeta.Name)
	return nil
}

// WaitForPlantToBeCreated waits for the shoot to be created
func (s *PlantTest) WaitForPlantToBeCreated(ctx context.Context) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		plant := &gardencorev1alpha1.Plant{}
		err := s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.Plant.Namespace, Name: s.Plant.Name}, plant)
		if err != nil {
			return false, err
		}

		s.Logger.Infof("Plant %s has been created", s.Plant.Name)
		return true, nil
	}, ctx.Done())
}

// WaitForPlantToBeDeleted waits for the plant to be deleted
func (s *PlantTest) WaitForPlantToBeDeleted(ctx context.Context) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		plant := &gardencorev1alpha1.Plant{}
		err := s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.Shoot.ObjectMeta.Namespace, Name: s.Shoot.ObjectMeta.Name}, plant)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		s.Logger.Infof("Waiting for plant %s to be deleted", s.Shoot.ObjectMeta.Name)
		return false, nil

	}, ctx.Done())
}

// WaitForPlantToBeReconciledSuccessfully waits for the plant to be reconciled with a status indicating success
func (s *PlantTest) WaitForPlantToBeReconciledSuccessfully(ctx context.Context) error {
	return wait.PollImmediateUntil(10*time.Second, func() (bool, error) {
		plant := &gardencorev1alpha1.Plant{}
		err := s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.Plant.Namespace, Name: s.Plant.Name}, plant)
		if err != nil {
			return false, err
		}

		if plantCreationSuccessful(&plant.Status) {
			return true, nil
		}

		s.Logger.Infof("Waiting for Plant %s to be successfully reconciled", s.Plant.Name)
		return false, nil
	}, ctx.Done())
}

// WaitForPlantToBeReconciledWithUnknownStatus waits for the plant to be reconciled, setting the expected status 'unknown'
func (s *PlantTest) WaitForPlantToBeReconciledWithUnknownStatus(ctx context.Context) error {
	return wait.PollImmediateUntil(10*time.Second, func() (bool, error) {
		plant := &gardencorev1alpha1.Plant{}
		err := s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.Plant.Namespace, Name: s.Plant.Name}, plant)
		if err != nil {
			return false, err
		}

		if plantReconciledWithStatusUnknown(&plant.Status) {
			return true, nil
		}

		s.Logger.Infof("Waiting for Plant %s to be reconciled with status : 'unknown'", s.Plant.Name)
		return false, nil
	}, ctx.Done())
}
