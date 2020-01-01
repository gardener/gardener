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
	retryutils "github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewPlantTest creates a new plantGardenerTest object, given an already created plant (created after parsing a plant YAML) and a path to a kubeconfig of an external cluster
func NewPlantTest(kubeconfig string, kubeconfigPathExternalCluster string, plant *gardencorev1beta1.Plant, logger *logrus.Logger) (*PlantTest, error) {
	if len(kubeconfig) == 0 {
		return nil, fmt.Errorf("Please specify the kubeconfig path correctly")
	}

	if len(kubeconfigPathExternalCluster) == 0 {
		return nil, fmt.Errorf("Please specify the kubeconfig path for the external cluster correctly")
	}

	k8sGardenClient, err := kubernetes.NewClientFromFile("", kubeconfig, kubernetes.WithClientOptions(
		client.Options{
			Scheme: kubernetes.GardenScheme,
		}),
	)
	if err != nil {
		return nil, err
	}

	return &PlantTest{
		GardenClient:                  k8sGardenClient,
		Plant:                         plant,
		kubeconfigPathExternalCluster: kubeconfigPathExternalCluster,
		Logger:                        logger,
	}, nil
}

// CreatePlantSecret creates a new Secret for the Plant
func (s *PlantTest) CreatePlantSecret(ctx context.Context, kubeConfigContent []byte) (*corev1.Secret, error) {
	if len(s.kubeconfigPathExternalCluster) == 0 {
		return nil, fmt.Errorf("Path to kubeconfig of external cluster not set")
	}

	plantSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: s.Plant.Namespace}}
	plantSecret.ObjectMeta.GenerateName = "test-secret-plant-"

	plantSecret.Data = make(map[string][]byte)
	plantSecret.Data["kubeconfig"] = kubeConfigContent

	err := s.GardenClient.Client().Create(ctx, plantSecret)
	if err != nil {
		return nil, err
	}

	return plantSecret, nil
}

// UpdatePlantSecret updates the Secret of the Plant
func (s *PlantTest) UpdatePlantSecret(ctx context.Context, updatedPlantSecret *corev1.Secret) error {
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		existingSecret := &corev1.Secret{}
		if err = s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: updatedPlantSecret.Namespace, Name: updatedPlantSecret.Name}, existingSecret); err != nil {
			return err
		}
		existingSecret.Data = updatedPlantSecret.Data
		return s.GardenClient.Client().Update(ctx, existingSecret)
	}); err != nil {
		return err
	}
	return nil
}

// GetPlantSecret retrieves the Secret of the Plant. Returns the Secret.
func (s *PlantTest) GetPlantSecret(ctx context.Context) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := s.GardenClient.Client().Get(ctx, client.ObjectKey{
		Namespace: s.Plant.Namespace,
		Name:      s.Plant.Spec.SecretRef.Name,
	}, secret)

	if err != nil {
		return nil, err
	}
	return secret, err
}

// GetPlant gets the test plant
func (s *PlantTest) GetPlant(ctx context.Context) (*gardencorev1beta1.Plant, error) {
	plant := &gardencorev1beta1.Plant{}
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
func (s *PlantTest) CreatePlant(ctx context.Context, secret *corev1.Secret) error {
	fmt.Println(secret.Name)
	plantToBeCreated := s.Plant.DeepCopy()
	plantToBeCreated.Name = ""
	plantToBeCreated.ObjectMeta.GenerateName = "test-plant-"
	plantToBeCreated.Spec.SecretRef.Name = secret.Name
	err := s.GardenClient.Client().Create(ctx, plantToBeCreated)
	if err != nil {
		return err
	}

	s.Plant.Name = plantToBeCreated.Name

	err = s.WaitForPlantToBeCreated(ctx)
	if err != nil {
		return err
	}

	s.Logger.Infof("Plant %s was created!", s.Plant.Name)
	return nil
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

	s.Logger.Infof("Plant %s was deleted successfully!", s.Plant.ObjectMeta.Name)
	return nil
}

// WaitForPlantToBeCreated waits for the plant to be created
func (s *PlantTest) WaitForPlantToBeCreated(ctx context.Context) error {
	return retryutils.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		plant := &gardencorev1beta1.Plant{}
		err = s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.Plant.Namespace, Name: s.Plant.Name}, plant)
		if err != nil {
			return retryutils.SevereError(err)
		}

		s.Logger.Infof("Plant %s has been created", s.Plant.Name)
		return retryutils.Ok()
	})
}

// WaitForPlantToBeDeleted waits for the plant to be deleted
func (s *PlantTest) WaitForPlantToBeDeleted(ctx context.Context) error {
	return retryutils.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		plant := &gardencorev1beta1.Plant{}
		err = s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.Plant.ObjectMeta.Namespace, Name: s.Plant.ObjectMeta.Name}, plant)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return retryutils.Ok()
			}
			return retryutils.SevereError(err)
		}
		s.Logger.Infof("Waiting for plant %s to be deleted", s.Plant.ObjectMeta.Name)
		return retryutils.MinorError(fmt.Errorf("plant %q is still present", s.Plant.Name))

	})
}

// WaitForPlantToBeReconciledSuccessfully waits for the plant to be reconciled with a status indicating success
func (s *PlantTest) WaitForPlantToBeReconciledSuccessfully(ctx context.Context) error {
	return retryutils.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		plant := &gardencorev1beta1.Plant{}
		err = s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.Plant.Namespace, Name: s.Plant.Name}, plant)
		if err != nil {
			return retryutils.SevereError(err)
		}

		if plantCreationSuccessful(&plant.Status) {
			return retryutils.Ok()
		}

		s.Logger.Infof("Waiting for Plant %s to be successfully reconciled", s.Plant.Name)
		return retryutils.MinorError(fmt.Errorf("plant %q was not successfully reconciled", s.Plant.Name))
	})
}

// WaitForPlantToBeReconciledWithUnknownStatus waits for the plant to be reconciled, setting the expected status 'unknown'
func (s *PlantTest) WaitForPlantToBeReconciledWithUnknownStatus(ctx context.Context) error {
	return retryutils.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		plant := &gardencorev1beta1.Plant{}
		err = s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.Plant.Namespace, Name: s.Plant.Name}, plant)
		if err != nil {
			return retryutils.SevereError(err)
		}

		if plantReconciledWithStatusUnknown(&plant.Status) {
			return retryutils.Ok()
		}

		s.Logger.Infof("Waiting for Plant %s to be reconciled with status : 'unknown'", s.Plant.Name)
		return retryutils.MinorError(fmt.Errorf("plant %q was not reconciled with status 'unknown'", s.Plant.Name))
	})
}
