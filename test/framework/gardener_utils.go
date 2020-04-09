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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSeed returns the seed and its k8s client
func (f *GardenerFramework) GetSeed(ctx context.Context, seedName string) (*gardencorev1beta1.Seed, kubernetes.Interface, error) {
	seed := &gardencorev1beta1.Seed{}
	err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: seedName}, seed)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get Seed from Shoot in Garden cluster")
	}

	seedSecretRef := seed.Spec.SecretRef
	seedClient, err := kubernetes.NewClientFromSecret(f.GardenClient, seedSecretRef.Namespace, seedSecretRef.Name, kubernetes.WithClientOptions(client.Options{
		Scheme: kubernetes.SeedScheme,
	}))
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not construct Seed client")
	}
	return seed, seedClient, nil
}

// GetShoot gets the test shoot
func (f *GardenerFramework) GetShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	return f.GardenClient.Client().Get(ctx, kutil.Key(shoot.Namespace, shoot.Name), shoot)
}

// GetShootProject returns the project of a shoot
func (f *GardenerFramework) GetShootProject(ctx context.Context, shootNamespace string) (*gardencorev1beta1.Project, error) {
	var (
		project = &gardencorev1beta1.Project{}
		ns      = &corev1.Namespace{}
	)
	if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: shootNamespace}, ns); err != nil {
		return nil, errors.Wrap(err, "could not get the Shoot namespace in Garden cluster")
	}

	if ns.Labels == nil {
		return nil, fmt.Errorf("namespace %q does not have any labels", ns.Name)
	}
	projectName, ok := ns.Labels[common.ProjectName]
	if !ok {
		return nil, fmt.Errorf("namespace %q did not contain a project label", ns.Name)
	}

	if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: projectName}, project); err != nil {
		return nil, errors.Wrap(err, "could not get Project in Garden cluster")
	}
	return project, nil
}

// CreateShoot Creates a shoot from a shoot Object and waits until it is successfully reconciled
func (f *GardenerFramework) CreateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	err := f.GetShoot(ctx, shoot)
	if !apierrors.IsNotFound(err) {
		return err
	}

	err = retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.GardenClient.Client().Create(ctx, shoot)
		if apierrors.IsInvalid(err) || apierrors.IsForbidden(err) {
			return retry.SevereError(err)
		}
		if err != nil {
			f.Logger.Debugf("unable to create shoot %s: %s", shoot.Name, err.Error())
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
	if err != nil {
		return err
	}

	// Then we wait for the shoot to be created
	err = f.WaitForShootToBeCreated(ctx, shoot)
	if err != nil {
		return err
	}

	f.Logger.Infof("Shoot %s was created!", shoot.Name)
	return nil
}

// DeleteShootAndWaitForDeletion deletes the test shoot and waits until it cannot be found any more
func (f *GardenerFramework) DeleteShootAndWaitForDeletion(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	err := f.DeleteShoot(ctx, shoot)
	if err != nil {
		return err
	}

	err = f.WaitForShootToBeDeleted(ctx, shoot)
	if err != nil {
		return err
	}

	f.Logger.Infof("Shoot %s was deleted successfully!", shoot.Name)
	return nil
}

// DeleteShoot deletes the test shoot
func (f *GardenerFramework) DeleteShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.RemoveShootAnnotation(shoot, common.ShootIgnore)
		if err != nil {
			return retry.MinorError(err)
		}

		// First we annotate the shoot to be deleted.
		err = f.AnnotateShoot(shoot, map[string]string{
			common.ConfirmationDeletion: "true",
		})
		if err != nil {
			return retry.MinorError(err)
		}

		err = f.GardenClient.Client().Delete(ctx, shoot)
		if err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
	if err != nil {
		return err
	}
	return nil
}

// UpdateShoot Updates a shoot from a shoot Object and waits for its reconciliation
func (f *GardenerFramework) UpdateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot, update func(shoot *gardencorev1beta1.Shoot) error) error {
	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		key, err := client.ObjectKeyFromObject(shoot)
		if err != nil {
			return retry.SevereError(err)
		}

		updatedShoot := &gardencorev1beta1.Shoot{}
		if err := f.GardenClient.Client().Get(ctx, key, updatedShoot); err != nil {
			return retry.MinorError(err)
		}

		if err := update(updatedShoot); err != nil {
			return retry.MinorError(err)
		}

		if err := f.GardenClient.Client().Update(ctx, updatedShoot); err != nil {
			f.Logger.Debugf("unable to update shoot %s: %s", updatedShoot.Name, err.Error())
			return retry.MinorError(err)
		}
		*shoot = *updatedShoot
		return retry.Ok()
	})
	if err != nil {
		return err
	}

	// Then we wait for the shoot to be created
	err = f.WaitForShootToBeReconciled(ctx, shoot)
	if err != nil {
		return err
	}

	f.Logger.Infof("Shoot %s was successfully updated!", shoot.Name)
	return nil
}

// HibernateShoot hibernates the test shoot
func (f *GardenerFramework) HibernateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	// return if the shoot is already hibernated
	if shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled {
		return nil
	}

	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		newShoot := shoot.DeepCopy()
		setHibernation(newShoot, true)
		patchedShoot, err := f.MergePatchShoot(shoot, newShoot)
		if err != nil {
			return retry.MinorError(err)
		}
		*shoot = *patchedShoot

		return retry.Ok()
	})
	if err != nil {
		return err
	}

	err = f.WaitForShootToBeReconciled(ctx, shoot)
	if err != nil {
		return err
	}

	f.Logger.Infof("Shoot %s was hibernated successfully!", shoot.Name)
	return nil
}

// WakeUpShoot wakes up the test shoot from hibernation
func (f *GardenerFramework) WakeUpShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	// return if the shoot is already running
	if shoot.Spec.Hibernation == nil || shoot.Spec.Hibernation.Enabled == nil || !*shoot.Spec.Hibernation.Enabled {
		return nil
	}

	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		newShoot := shoot.DeepCopy()
		setHibernation(newShoot, false)

		patchedShoot, err := f.MergePatchShoot(shoot, newShoot)
		if err != nil {
			return retry.MinorError(err)
		}
		*shoot = *patchedShoot
		return retry.Ok()
	})
	if err != nil {
		return err
	}

	err = f.WaitForShootToBeReconciled(ctx, shoot)
	if err != nil {
		return err
	}

	f.Logger.Infof("Shoot %s has been woken up successfully!", shoot.Name)
	return nil
}

// WaitForShootToBeCreated waits for the shoot to be created
func (f *GardenerFramework) WaitForShootToBeCreated(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	return retry.UntilTimeout(ctx, 30*time.Second, 60*time.Minute, func(ctx context.Context) (done bool, err error) {
		newShoot := &gardencorev1beta1.Shoot{}
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, newShoot)
		if err != nil {
			f.Logger.Infof("Error while waiting for shoot to be created: %s", err.Error())
			return retry.MinorError(err)
		}
		*shoot = *newShoot
		if ShootCreationCompleted(shoot) {
			return retry.Ok()
		}
		f.Logger.Infof("Waiting for shoot %s to be created", shoot.Name)
		if shoot.Status.LastOperation != nil {
			f.Logger.Infof("%d%%: Shoot State: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return retry.MinorError(fmt.Errorf("shoot %q was not successfully reconciled", shoot.Name))
	})
}

// WaitForShootToBeDeleted waits for the shoot to be deleted
func (f *GardenerFramework) WaitForShootToBeDeleted(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	return retry.UntilTimeout(ctx, 30*time.Second, 60*time.Minute, func(ctx context.Context) (done bool, err error) {
		updatedShoot := &gardencorev1beta1.Shoot{}
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, updatedShoot)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			f.Logger.Infof("Error while waiting for shoot to be deleted: %s", err.Error())
			return retry.MinorError(err)
		}
		*shoot = *updatedShoot
		f.Logger.Infof("waiting for shoot %s to be deleted", shoot.Name)
		if shoot.Status.LastOperation != nil {
			f.Logger.Debugf("%d%%: Shoot state: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return retry.MinorError(fmt.Errorf("shoot %q still exists", shoot.Name))
	})
}

// WaitForShootToBeReconciled waits for the shoot to be successfully reconciled
func (f *GardenerFramework) WaitForShootToBeReconciled(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	return retry.UntilTimeout(ctx, 30*time.Second, 60*time.Minute, func(ctx context.Context) (done bool, err error) {
		newShoot := &gardencorev1beta1.Shoot{}
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, newShoot)
		if err != nil {
			f.Logger.Infof("Error while waiting for shoot to be reconciled: %s", err.Error())
			return retry.MinorError(err)
		}
		shoot = newShoot
		if ShootCreationCompleted(shoot) {
			return retry.Ok()
		}
		f.Logger.Infof("Waiting for shoot %s to be reconciled", shoot.Name)
		if newShoot.Status.LastOperation != nil {
			f.Logger.Debugf("%d%%: Shoot State: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return retry.MinorError(fmt.Errorf("shoot %q was not successfully reconciled", shoot.Name))
	})
}

// AnnotateShoot adds shoot annotation(s)
func (f *GardenerFramework) AnnotateShoot(shoot *gardencorev1beta1.Shoot, annotations map[string]string) error {
	shootCopy := shoot.DeepCopy()

	for annotationKey, annotationValue := range annotations {
		metav1.SetMetaDataAnnotation(&shootCopy.ObjectMeta, annotationKey, annotationValue)
	}

	if _, err := f.MergePatchShoot(shoot, shootCopy); err != nil {
		return err
	}

	return nil
}

// RemoveShootAnnotation removes an annotation with key <annotationKey> from a shoot object
func (f *GardenerFramework) RemoveShootAnnotation(shoot *gardencorev1beta1.Shoot, annotationKey string) error {
	shootCopy := shoot.DeepCopy()
	if len(shootCopy.Annotations) == 0 {
		return nil
	}
	if _, ok := shootCopy.Annotations[annotationKey]; !ok {
		return nil
	}

	// start the update process with Kubernetes
	delete(shootCopy.Annotations, annotationKey)

	if _, err := f.MergePatchShoot(shoot, shootCopy); err != nil {
		return err
	}
	return nil
}

// MergePatchShoot performs a two way merge patch operation on a shoot object
func (f *GardenerFramework) MergePatchShoot(oldShoot, newShoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
	patchBytes, err := kutil.CreateTwoWayMergePatch(oldShoot, newShoot)
	if err != nil {
		return nil, fmt.Errorf("failed to patch bytes")
	}

	patchedShoot, err := f.GardenClient.GardenCore().CoreV1beta1().Shoots(oldShoot.GetNamespace()).Patch(oldShoot.GetName(), types.StrategicMergePatchType, patchBytes)
	if err == nil {
		*oldShoot = *patchedShoot
	}
	return patchedShoot, err
}

// GetCloudProfile returns the cloudprofile from gardener with the give name
func (f *GardenerFramework) GetCloudProfile(ctx context.Context, name string) (*gardencorev1beta1.CloudProfile, error) {
	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: name}, cloudProfile); err != nil {
		return nil, errors.Wrap(err, "could not get Seed's CloudProvider in Garden cluster")
	}
	return cloudProfile, nil
}

// DumpState greps all necessary logs and state of the cluster if the test failed
// TODO: dump extension controller namespaces
// TODO: dump logs of gardener extension controllers and other system components
func (f *GardenerFramework) DumpState(ctx context.Context) {
	if f.DisableStateDump {
		return
	}
	if f.GardenClient == nil {
		return
	}

	ctxIdentifier := "[GARDENER]"
	f.Logger.Info(ctxIdentifier)

	if err := f.dumpSeeds(ctx, ctxIdentifier); err != nil {
		f.Logger.Errorf("unable to dump seed status: %s", err.Error())
	}

	// dump all events if no shoot is given
	if err := f.dumpEventsInAllNamespace(ctx, ctxIdentifier, f.GardenClient); err != nil {
		f.Logger.Errorf("unable to dump Events from namespaces gardener: %s", err.Error())
	}
}

// dumpSeeds prints information about all seeds
func (f *GardenerFramework) dumpSeeds(ctx context.Context, ctxIdentifier string) error {
	f.Logger.Infof("%s [SEEDS]", ctxIdentifier)
	seeds := &gardencorev1beta1.SeedList{}
	if err := f.GardenClient.Client().List(ctx, seeds); err != nil {
		return err
	}

	for _, seed := range seeds.Items {
		f.dumpSeed(&seed)
	}
	return nil
}

// dumpSeed prints information about a seed
func (f *GardenerFramework) dumpSeed(seed *gardencorev1beta1.Seed) {
	if err := health.CheckSeed(seed, seed.Status.Gardener); err != nil {
		f.Logger.Printf("Seed %s is %s - Error: %s - Conditions %v", seed.Name, unhealthy, err.Error(), seed.Status.Conditions)
	} else {
		f.Logger.Printf("Seed %s is %s", seed.Name, healthy)
	}
}

func setHibernation(shoot *gardencorev1beta1.Shoot, hibernated bool) {
	if shoot.Spec.Hibernation != nil {
		shoot.Spec.Hibernation.Enabled = &hibernated
	}
	shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
		Enabled: &hibernated,
	}
}
