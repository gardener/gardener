// Copyright 2019 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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
	"github.com/gardener/gardener/pkg/utils/retry"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreatePlantSecret creates a new Secret for the Plant
func (f *GardenerFramework) CreatePlantSecret(ctx context.Context, namespace string, kubeConfigContent []byte) (*corev1.Secret, error) {
	plantSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}}
	plantSecret.ObjectMeta.GenerateName = "test-secret-plant-"

	plantSecret.Data = make(map[string][]byte)
	plantSecret.Data["kubeconfig"] = kubeConfigContent

	err := f.GardenClient.Client().Create(ctx, plantSecret)
	if err != nil {
		return nil, err
	}

	return plantSecret, nil
}

// CreatePlant Creates a plant from a plant Object
func (f *GardenerFramework) CreatePlant(ctx context.Context, plant *gardencorev1beta1.Plant) error {
	log := f.Logger.WithValues("plant", client.ObjectKeyFromObject(plant))

	err := f.GardenClient.Client().Create(ctx, plant)
	if err != nil {
		return err
	}

	err = f.WaitForPlantToBeCreated(ctx, plant)
	if err != nil {
		return err
	}

	log.Info("Plant was created")
	return nil
}

// DeletePlant deletes the test plant
func (f *GardenerFramework) DeletePlant(ctx context.Context, plant *gardencorev1beta1.Plant) error {
	log := f.Logger.WithValues("plant", client.ObjectKeyFromObject(plant))

	err := f.GardenClient.Client().Delete(ctx, plant)
	if err != nil {
		return err
	}

	err = f.WaitForPlantToBeDeleted(ctx, plant)
	if err != nil {
		return err
	}

	log.Info("Plant was deleted successfully")
	return nil
}

// WaitForPlantToBeCreated waits for the plant to be created
func (f *GardenerFramework) WaitForPlantToBeCreated(ctx context.Context, plant *gardencorev1beta1.Plant) error {
	log := f.Logger.WithValues("plant", client.ObjectKeyFromObject(plant))

	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		newPlant := &gardencorev1beta1.Plant{}
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: plant.GetNamespace(), Name: plant.GetName()}, newPlant)
		if err != nil {
			return retry.SevereError(err)
		}
		*plant = *newPlant

		log.Info("Plant has been created")
		return retry.Ok()
	})
}

// WaitForPlantToBeReconciledSuccessfully waits for the plant to be reconciled with a status indicating success
func (f *GardenerFramework) WaitForPlantToBeReconciledSuccessfully(ctx context.Context, plant *gardencorev1beta1.Plant) error {
	log := f.Logger.WithValues("plant", client.ObjectKeyFromObject(plant))

	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		newPlant := &gardencorev1beta1.Plant{}
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: plant.GetNamespace(), Name: plant.GetName()}, newPlant)
		if err != nil {
			return retry.SevereError(err)
		}
		*plant = *newPlant

		if plantCreationSuccessful(&plant.Status) {
			return retry.Ok()
		}

		log.Info("Waiting for Plant to be successfully reconciled")
		return retry.MinorError(fmt.Errorf("plant %s was not successfully reconciled", plant.GetName()))
	})
}

// WaitForPlantToBeDeleted waits for the plant to be deleted
func (f *GardenerFramework) WaitForPlantToBeDeleted(ctx context.Context, plant *gardencorev1beta1.Plant) error {
	log := f.Logger.WithValues("plant", client.ObjectKeyFromObject(plant))

	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		newPlant := &gardencorev1beta1.Plant{}
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: plant.GetNamespace(), Name: plant.GetName()}, newPlant)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}
		*plant = *newPlant

		log.Info("Waiting for plant to be deleted")
		return retry.MinorError(fmt.Errorf("plant %s is still present", plant.GetName()))
	})
}

// WaitForPlantToBeReconciledWithUnknownStatus waits for the plant to be reconciled, setting the expected status 'unknown'
func (f *GardenerFramework) WaitForPlantToBeReconciledWithUnknownStatus(ctx context.Context, plant *gardencorev1beta1.Plant) error {
	log := f.Logger.WithValues("plant", client.ObjectKeyFromObject(plant))

	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		newPlant := &gardencorev1beta1.Plant{}
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: plant.GetNamespace(), Name: plant.GetName()}, newPlant)
		if err != nil {
			return retry.SevereError(err)
		}
		*plant = *newPlant

		if plantReconciledWithStatusUnknown(&plant.Status) {
			return retry.Ok()
		}

		log.Info("Waiting for Plant to be reconciled with status 'unknown'")
		return retry.MinorError(fmt.Errorf("plant %s was not reconciled with status 'unknown'", plant.GetName()))
	})
}

// plantCreationSuccessful determines, based on the plant condition and Cluster Info, if the Plant was reconciled successfully
func plantCreationSuccessful(plantStatus *gardencorev1beta1.PlantStatus) bool {
	if len(plantStatus.Conditions) == 0 {
		return false
	}

	for _, condition := range plantStatus.Conditions {
		if condition.Status != gardencorev1beta1.ConditionTrue {
			return false
		}
	}

	if len(plantStatus.ClusterInfo.Kubernetes.Version) == 0 || len(plantStatus.ClusterInfo.Cloud.Type) == 0 || len(plantStatus.ClusterInfo.Cloud.Region) == 0 {
		return false
	}

	return true
}

// plantReconciledWithStatusUnknown determines, based on the plant status.condition and status.ClusterInfo, if the PlantStatus is 'unknown'
func plantReconciledWithStatusUnknown(plantStatus *gardencorev1beta1.PlantStatus) bool {
	if len(plantStatus.Conditions) == 0 {
		return false
	}

	for _, condition := range plantStatus.Conditions {
		if condition.Status != gardencorev1beta1.ConditionFalse && condition.Status != gardencorev1beta1.ConditionUnknown {
			return false
		}
	}

	if len(plantStatus.ClusterInfo.Kubernetes.Version) != 0 || len(plantStatus.ClusterInfo.Cloud.Type) != 0 && len(plantStatus.ClusterInfo.Cloud.Region) != 0 {
		return false
	}

	return true
}
