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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenversionedcoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSeeds returns all registered seeds
func (f *GardenerFramework) GetSeeds(ctx context.Context) ([]gardencorev1beta1.Seed, error) {
	seeds := &gardencorev1beta1.SeedList{}
	err := f.GardenClient.Client().List(ctx, seeds)
	if err != nil {
		return nil, fmt.Errorf("could not get Seeds from Garden cluster: %w", err)
	}

	return seeds.Items, nil
}

// GetSeed returns the seed and its k8s client
func (f *GardenerFramework) GetSeed(ctx context.Context, seedName string) (*gardencorev1beta1.Seed, kubernetes.Interface, error) {
	seed := &gardencorev1beta1.Seed{}
	err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: seedName}, seed)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get Seed from Shoot in Garden cluster: %w", err)
	}

	seedSecretRef := seed.Spec.SecretRef
	if seedSecretRef == nil {
		f.Logger.Info("Seed does not have secretRef set, skip constructing seed client")
		return seed, nil, nil
	}

	seedClient, err := kubernetes.NewClientFromSecret(ctx, f.GardenClient.Client(), seedSecretRef.Namespace, seedSecretRef.Name,
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not construct Seed client: %w", err)
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
		return nil, fmt.Errorf("could not get the Shoot namespace in Garden cluster: %w", err)
	}

	if ns.Labels == nil {
		return nil, fmt.Errorf("namespace %q does not have any labels", ns.Name)
	}
	projectName, ok := ns.Labels[v1beta1constants.ProjectName]
	if !ok {
		return nil, fmt.Errorf("namespace %q did not contain a project label", ns.Name)
	}

	if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: projectName}, project); err != nil {
		return nil, fmt.Errorf("could not get Project in Garden cluster: %w", err)
	}
	return project, nil
}

// createShootResource creates a shoot from a shoot Object
func (f *GardenerFramework) createShootResource(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
	if err := f.GardenClient.Client().Create(ctx, shoot); err != nil {
		return nil, err
	}
	f.Logger.Info("Shoot resource was created", "shoot", client.ObjectKeyFromObject(shoot))
	return shoot, nil
}

// CreateShoot Creates a shoot from a shoot Object and waits until it is successfully reconciled
func (f *GardenerFramework) CreateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		_, err = f.createShootResource(ctx, shoot)
		if apierrors.IsInvalid(err) || apierrors.IsForbidden(err) || apierrors.IsAlreadyExists(err) {
			return retry.SevereError(err)
		}
		if err != nil {
			log.Error(err, "Unable to create shoot")
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

	log.Info("Shoot was created")
	return nil
}

// DeleteShootAndWaitForDeletion deletes the test shoot and waits until it cannot be found any more
func (f *GardenerFramework) DeleteShootAndWaitForDeletion(ctx context.Context, shoot *gardencorev1beta1.Shoot) (rErr error) {
	if f.Config.ExistingShootName != "" {
		f.Logger.Info("Skip deletion of existing shoot", "shoot", client.ObjectKey{Name: f.Config.ExistingShootName, Namespace: f.ProjectNamespace})
		return nil
	}

	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	defer func() {
		if rErr != nil {
			dumpCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			if shootFramework, err := f.NewShootFramework(dumpCtx, shoot); err != nil {
				log.Error(err, "Cannot dump shoot state")
			} else {
				shootFramework.DumpState(dumpCtx)
			}
		}
	}()

	err := f.DeleteShoot(ctx, shoot)
	if err != nil {
		return err
	}

	err = f.WaitForShootToBeDeleted(ctx, shoot)
	if err != nil {
		return err
	}

	log.Info("Shoot was deleted successfully")
	return nil
}

// DeleteShoot deletes the test shoot
func (f *GardenerFramework) DeleteShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.RemoveShootAnnotation(ctx, shoot, v1beta1constants.ShootIgnore)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.MinorError(err)
		}

		// First we annotate the shoot to be deleted.
		err = f.AnnotateShoot(ctx, shoot, map[string]string{
			gutil.ConfirmationDeletion: "true",
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
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		updatedShoot := &gardencorev1beta1.Shoot{}
		if err := f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot); err != nil {
			return retry.MinorError(err)
		}

		if err := update(updatedShoot); err != nil {
			return retry.MinorError(err)
		}

		if err := f.GardenClient.Client().Update(ctx, updatedShoot); err != nil {
			log.Error(err, "Unable to update shoot")
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

	log.Info("Shoot was successfully updated")
	return nil
}

// HibernateShoot hibernates the test shoot
func (f *GardenerFramework) HibernateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	// return if the shoot is already hibernated
	if shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled {
		return nil
	}

	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		patch := client.MergeFrom(shoot.DeepCopy())
		setHibernation(shoot, true)
		if err := f.GardenClient.Client().Patch(ctx, shoot, patch); err != nil {
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
	if err != nil {
		return err
	}

	if err := f.WaitForShootToBeReconciled(ctx, shoot); err != nil {
		return err
	}

	log.Info("Shoot was hibernated successfully")
	return nil
}

// WakeUpShoot wakes up the test shoot from hibernation
func (f *GardenerFramework) WakeUpShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	// return if the shoot is already running
	if shoot.Spec.Hibernation == nil || shoot.Spec.Hibernation.Enabled == nil || !*shoot.Spec.Hibernation.Enabled {
		return nil
	}

	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		patch := client.MergeFrom(shoot.DeepCopy())
		setHibernation(shoot, false)
		if err := f.GardenClient.Client().Patch(ctx, shoot, patch); err != nil {
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
	if err != nil {
		return err
	}

	if err := f.WaitForShootToBeReconciled(ctx, shoot); err != nil {
		return err
	}

	log.Info("Shoot was woken up successfully")
	return nil
}

// ScheduleShoot set the Spec.Cloud.Seed of a shoot to the specified seed.
// This is the request the Gardener Scheduler executes after a scheduling decision.
func (f *GardenerFramework) ScheduleShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed) error {
	shoot.Spec.SeedName = &seed.Name
	return f.GardenClient.Client().Update(ctx, shoot)
}

// WaitForShootToBeCreated waits for the shoot to be created
func (f *GardenerFramework) WaitForShootToBeCreated(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	return retry.UntilTimeout(ctx, 30*time.Second, 60*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, shoot)
		if err != nil {
			log.Error(err, "Error while waiting for shoot to be created")
			return retry.MinorError(err)
		}
		completed, msg := ShootReconciliationSuccessful(shoot)
		if completed {
			return retry.Ok()
		}
		log.Info("Shoot not yet created", "reason", msg)
		if shoot.Status.LastOperation != nil {
			log.Info("Last Operation", "lastOperation", shoot.Status.LastOperation)
		}
		return retry.MinorError(fmt.Errorf("shoot %q was not successfully reconciled", shoot.Name))
	})
}

// WaitForShootToBeDeleted waits for the shoot to be deleted
func (f *GardenerFramework) WaitForShootToBeDeleted(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	return retry.UntilTimeout(ctx, 30*time.Second, 60*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, shoot)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			log.Error(err, "Error while waiting for shoot to be deleted")
			return retry.MinorError(err)
		}
		log.Info("Shoot is not yet deleted")
		if shoot.Status.LastOperation != nil {
			log.Info("Last Operation", "lastOperation", shoot.Status.LastOperation)
		}
		return retry.MinorError(fmt.Errorf("shoot %q still exists", shoot.Name))
	})
}

// WaitForShootToBeReconciled waits for the shoot to be successfully reconciled
func (f *GardenerFramework) WaitForShootToBeReconciled(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	return retry.UntilTimeout(ctx, 30*time.Second, 60*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, shoot)
		if err != nil {
			log.Error(err, "Error while waiting for shoot to be reconciled")
			return retry.MinorError(err)
		}
		completed, msg := ShootReconciliationSuccessful(shoot)
		if completed {
			return retry.Ok()
		}
		log.Info("Shoot is not yet reconciled", "reason", msg)
		if shoot.Status.LastOperation != nil {
			log.Info("Last Operation", "lastOperation", shoot.Status.LastOperation)
		}
		return retry.MinorError(fmt.Errorf("shoot %q was not successfully reconciled", shoot.Name))
	})
}

// AnnotateShoot adds shoot annotation(s)
func (f *GardenerFramework) AnnotateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot, annotations map[string]string) error {
	patch := client.MergeFrom(shoot.DeepCopy())

	for annotationKey, annotationValue := range annotations {
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, annotationKey, annotationValue)
	}

	return f.GardenClient.Client().Patch(ctx, shoot, patch)
}

// RemoveShootAnnotation removes an annotation with key <annotationKey> from a shoot object
func (f *GardenerFramework) RemoveShootAnnotation(ctx context.Context, shoot *gardencorev1beta1.Shoot, annotationKey string) error {
	if len(shoot.Annotations) == 0 {
		return nil
	}
	if _, ok := shoot.Annotations[annotationKey]; !ok {
		return nil
	}

	patch := client.MergeFrom(shoot.DeepCopy())
	delete(shoot.Annotations, annotationKey)

	return f.GardenClient.Client().Patch(ctx, shoot, patch)
}

// MigrateShoot changes the spec.Seed.Name of a shoot and waits for it to be migrated
func (f *GardenerFramework) MigrateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed, prerequisites func(shoot *gardencorev1beta1.Shoot) error) error {
	if prerequisites != nil {
		if err := f.UpdateShoot(ctx, shoot, func(shoot *gardencorev1beta1.Shoot) error {
			return prerequisites(shoot)
		}); err != nil {
			return err
		}
	}

	if err := f.GetShoot(ctx, shoot); err != nil {
		return err
	}

	if _, _, err := f.GetSeed(ctx, seed.Name); err != nil {
		return err
	}

	restConfig := f.GardenClient.RESTConfig()
	versionedCoreClient, err := gardenversionedcoreclientset.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed create versioned core client: %w", err)
	}

	shoot.Spec.SeedName = &seed.Name
	if _, err = versionedCoreClient.CoreV1beta1().Shoots(shoot.GetNamespace()).UpdateBinding(ctx, shoot.GetName(), shoot, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed updating binding for shoot %q: %w", client.ObjectKeyFromObject(shoot), err)
	}

	return f.WaitForShootToBeCreated(ctx, shoot)
}

// GetCloudProfile returns the cloudprofile from gardener with the give name
func (f *GardenerFramework) GetCloudProfile(ctx context.Context, name string) (*gardencorev1beta1.CloudProfile, error) {
	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: name}, cloudProfile); err != nil {
		return nil, fmt.Errorf("could not get CloudProfile '%s' in Garden cluster: %w", name, err)
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

	if err := f.dumpSeeds(ctx); err != nil {
		f.Logger.Error(err, "Unable to dump seed status")
	}

	// dump events if project namespace set
	if f.ProjectNamespace != "" {
		if err := f.dumpEventsInNamespace(ctx, f.Logger, f.GardenClient, f.ProjectNamespace); err != nil {
			f.Logger.Error(err, "Unable to dump gardener events from project namespace", "namespace", f.ProjectNamespace)
		}
	}
}

// dumpSeeds prints information about all seeds
func (f *GardenerFramework) dumpSeeds(ctx context.Context) error {
	f.Logger.Info("Dumping core.gardener.cloud/v1beta1.Seed resources")

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
	log := f.Logger.WithValues("seedName", seed.Name)

	if err := health.CheckSeed(seed, seed.Status.Gardener); err != nil {
		log.Info("Found unhealthy Seed", "reason", err.Error(), "conditions", seed.Status.Conditions)
	} else {
		log.Info("Found healthy Seed")
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
