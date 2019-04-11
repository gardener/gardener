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

	"github.com/gardener/gardener/pkg/client/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sirupsen/logrus"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// NewShootGardenerTest creates a new shootGardenerTest object, given an already created shoot (created after parsing a shoot YAML)
func NewShootGardenerTest(kubeconfig string, shoot *v1beta1.Shoot, logger *logrus.Logger) (*ShootGardenerTest, error) {
	if len(kubeconfig) == 0 {
		return nil, fmt.Errorf("please specify the kubeconfig path correctly")
	}

	k8sGardenClient, err := kubernetes.NewClientFromFile("", kubeconfig, client.Options{
		Scheme: kubernetes.GardenScheme,
	})
	if err != nil {
		return nil, err
	}

	return &ShootGardenerTest{
		GardenClient: k8sGardenClient,

		Shoot:  shoot,
		Logger: logger,
	}, nil
}

// GetShoot gets the test shoot
func (s *ShootGardenerTest) GetShoot(ctx context.Context) (*v1beta1.Shoot, error) {
	shoot := &v1beta1.Shoot{}
	err := s.GardenClient.Client().Get(ctx, client.ObjectKey{
		Namespace: s.Shoot.Namespace,
		Name:      s.Shoot.Name,
	}, shoot)

	if err != nil {
		return nil, err
	}
	return shoot, err
}

// CreateShoot Creates a shoot from a shoot Object
func (s *ShootGardenerTest) CreateShoot(ctx context.Context) (*v1beta1.Shoot, error) {
	_, err := s.GetShoot(ctx)
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	shoot := s.Shoot
	err = s.GardenClient.Client().Create(ctx, shoot)
	if err != nil {
		return nil, err
	}

	// Then we wait for the shoot to be created
	err = s.WaitForShootToBeCreated(ctx)
	if err != nil {
		return nil, err
	}

	s.Logger.Infof("Shoot %s was created!", shoot.Name)
	return shoot, nil
}

// DeleteShoot deletes the test shoot
func (s *ShootGardenerTest) DeleteShoot(ctx context.Context) error {
	shoot, err := s.GetShoot(ctx)
	if err != nil {
		return err
	}

	s.Shoot = shoot
	err = s.RemoveShootAnnotation(ctx, common.ShootIgnore)
	if err != nil {
		return err
	}

	// First we annotate the shoot to be deleted.
	err = s.AnnotateShoot(ctx, map[string]string{
		common.ConfirmationDeletion: "true",
	})
	if err != nil {
		return err
	}

	err = s.GardenClient.Client().Delete(ctx, s.Shoot)
	if err != nil {
		return err
	}

	err = s.WaitForShootToBeDeleted(ctx)
	if err != nil {
		return err
	}

	s.Logger.Infof("Shoot %s was deleted successfully!", s.Shoot.Name)
	return nil
}

// RemoveShootAnnotation removes an annotation with key <annotationKey> from a shoot object
func (s *ShootGardenerTest) RemoveShootAnnotation(ctx context.Context, annotationKey string) error {
	shootCopy := s.Shoot.DeepCopy()
	if len(shootCopy.Annotations) == 0 {
		return nil
	}
	if _, ok := shootCopy.Annotations[annotationKey]; !ok {
		return nil
	}

	// start the update process with Kubernetes
	s.Logger.Infof("deleting annotation with key: %q in shoot: %s\n", annotationKey, s.Shoot.Name)
	delete(shootCopy.Annotations, annotationKey)

	return s.mergePatch(ctx, s.Shoot, shootCopy)
}

// AnnotateShoot adds shoot annotation(s)
func (s *ShootGardenerTest) AnnotateShoot(ctx context.Context, annotations map[string]string) error {
	shootCopy := s.Shoot.DeepCopy()

	for annotationKey, annotationValue := range annotations {
		metav1.SetMetaDataAnnotation(&shootCopy.ObjectMeta, annotationKey, annotationValue)
	}

	err := s.mergePatch(ctx, s.Shoot, shootCopy)
	if err != nil {
		return err
	}

	return nil
}

// WaitForShootToBeCreated waits for the shoot to be created
func (s *ShootGardenerTest) WaitForShootToBeCreated(ctx context.Context) error {
	return wait.PollImmediateUntil(30*time.Second, func() (bool, error) {
		shoot := &v1beta1.Shoot{}
		err := s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.Shoot.Namespace, Name: s.Shoot.Name}, shoot)
		if err != nil {
			return false, err
		}
		if shootCreationCompleted(&shoot.Status) {
			return true, nil
		}
		s.Logger.Infof("Waiting for shoot %s to be created", s.Shoot.Name)
		if shoot.Status.LastOperation != nil {
			s.Logger.Debugf("%d%%: Shoot State: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return false, nil
	}, ctx.Done())
}

// WaitForShootToBeDeleted waits for the shoot to be deleted
func (s *ShootGardenerTest) WaitForShootToBeDeleted(ctx context.Context) error {
	return wait.PollImmediateUntil(30*time.Second, func() (bool, error) {
		shoot := &v1beta1.Shoot{}
		err := s.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.Shoot.Namespace, Name: s.Shoot.Name}, shoot)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		s.Logger.Infof("waiting for shoot %s to be deleted", s.Shoot.Name)
		if shoot.Status.LastOperation != nil {
			s.Logger.Debugf("%d%%: Shoot state: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return false, nil
	}, ctx.Done())
}
