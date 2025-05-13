// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/utils/access"
	shootoperation "github.com/gardener/gardener/test/utils/shoots/operation"
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

	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: seed.Name}, managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			f.Logger.Info("Seed is not a ManagedSeed, checking seed secret")

			// For tests, we expect the seed kubeconfig secret to be present in the garden namespace
			seedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seed-" + seedName,
					Namespace: "garden",
				},
			}

			if err := f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(seedSecret), seedSecret); err != nil {
				return seed, nil, fmt.Errorf("seed is not a ManagedSeed also no seed kubeconfig secret present in the garden namespace, %s: %w", client.ObjectKeyFromObject(seed), err)
			}

			seedClient, err := kubernetes.NewClientFromSecret(ctx, f.GardenClient.Client(), seedSecret.Namespace, seedSecret.Name,
				kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
				kubernetes.WithDisabledCachedClient(),
			)
			if err != nil {
				return nil, nil, fmt.Errorf("could not construct Seed client: %w", err)
			}

			return seed, seedClient, nil
		}

		return seed, nil, fmt.Errorf("failed to get ManagedSeed for Seed, %s: %w", client.ObjectKeyFromObject(seed), err)
	}

	if managedSeed.Spec.Shoot == nil {
		return seed, nil, fmt.Errorf("shoot for ManagedSeed, %s is nil", client.ObjectKeyFromObject(managedSeed))
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: managedSeed.Spec.Shoot.Name}, shoot); err != nil {
		return seed, nil, fmt.Errorf("failed to get Shoot %s for ManagedSeed, %s: %w", managedSeed.Spec.Shoot.Name, client.ObjectKeyFromObject(managedSeed), err)
	}

	const expirationSeconds int64 = 6 * 3600 // 6h
	kubeconfig, err := access.RequestAdminKubeconfigForShoot(ctx, f.GardenClient, shoot, ptr.To(expirationSeconds))
	if err != nil {
		return seed, nil, fmt.Errorf("failed to request AdminKubeConfig for Shoot %s: %w", client.ObjectKeyFromObject(shoot), err)
	}

	seedClient, err := kubernetes.NewClientFromBytes(kubeconfig,
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
	return f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, shoot)
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
func (f *GardenerFramework) CreateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot, waitForCreation bool) error {
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

	if waitForCreation {
		err = f.WaitForShootToBeCreated(ctx, shoot)
		if err != nil {
			return err
		}
	}

	log.Info("Shoot was created")
	return nil
}

// DeleteShootAndWaitForDeletion deletes the test shoot and waits until it cannot be found anymore.
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

// ForceDeleteShootAndWaitForDeletion forcefully deletes the test shoot and waits until it cannot be found anymore.
func (f *GardenerFramework) ForceDeleteShootAndWaitForDeletion(ctx context.Context, shoot *gardencorev1beta1.Shoot) (rErr error) {
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	if err := f.AnnotateShoot(ctx, shoot, map[string]string{v1beta1constants.ShootIgnore: "true"}); err != nil {
		return fmt.Errorf("failed patching Shoot to be ignored")
	}

	if err := f.DeleteShoot(ctx, shoot); err != nil {
		return err
	}

	patch := client.MergeFrom(shoot.DeepCopy())
	shoot.Status.LastErrors = []gardencorev1beta1.LastError{{
		Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraDependencies},
	}}
	if err := f.GardenClient.Client().Status().Patch(ctx, shoot, patch); err != nil {
		return err
	}

	if err := f.AnnotateShoot(ctx, shoot, map[string]string{
		v1beta1constants.AnnotationConfirmationForceDeletion: "true",
		v1beta1constants.ShootIgnore:                         "false",
	}); err != nil {
		return fmt.Errorf("failed annotating Shoot with force-delete and to not be ignored")
	}

	if err := f.WaitForShootToBeDeleted(ctx, shoot); err != nil {
		return fmt.Errorf("failed waiting for Shoot to be deleted")
	}

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

	log.Info("Shoot was force-deleted successfully")
	return nil
}

// DeleteShoot deletes the test shoot
func (f *GardenerFramework) DeleteShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		// First we annotate the shoot to be deleted.
		err = f.AnnotateShoot(ctx, shoot, map[string]string{
			v1beta1constants.ConfirmationDeletion: "true",
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

	if err := f.UpdateShootSpec(ctx, shoot, update); err != nil {
		return err
	}

	if err := f.WaitForShootToBeReconciled(ctx, shoot); err != nil {
		return err
	}

	log.Info("Shoot was successfully updated")
	return nil
}

// UpdateShootSpec updates a shoot from a shoot Object
func (f *GardenerFramework) UpdateShootSpec(ctx context.Context, shoot *gardencorev1beta1.Shoot, update func(shoot *gardencorev1beta1.Shoot) error) error {
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

	log.Info("Shoot spec was successfully updated")
	return nil
}

func (f *GardenerFramework) updateBinding(ctx context.Context, shoot *gardencorev1beta1.Shoot, seedName string) error {
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		updatedShoot := &gardencorev1beta1.Shoot{}
		if err := f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(shoot), updatedShoot); err != nil {
			return retry.MinorError(err)
		}

		updatedShoot.Spec.SeedName = ptr.To(seedName)
		if err := f.GardenClient.Client().SubResource("binding").Update(ctx, updatedShoot); err != nil {
			log.Error(err, "Unable to update binding")
			return retry.MinorError(err)
		}

		*shoot = *updatedShoot
		return retry.Ok()
	})
	if err != nil {
		return fmt.Errorf("failed updating binding for shoot %q: %w", client.ObjectKeyFromObject(shoot), err)
	}

	log.Info("Shoot binding was successfully updated")
	return nil
}

// HibernateShoot hibernates the test shoot
func (f *GardenerFramework) HibernateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	// return if the shoot is already hibernated
	if shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled {
		return nil
	}

	if err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		patch := client.MergeFrom(shoot.DeepCopy())
		setHibernation(shoot, true)
		if err := f.GardenClient.Client().Patch(ctx, shoot, patch); err != nil {
			return retry.MinorError(err)
		}
		return retry.Ok()
	}); err != nil {
		return err
	}

	if err := f.WaitForShootToBeReconciled(ctx, shoot); err != nil {
		return err
	}

	if err := retry.UntilTimeout(ctx, 10*time.Second, 2*time.Minute, func(ctx context.Context) (done bool, err error) {
		// Verify no running pods after hibernation
		if err := f.VerifyNoRunningPods(ctx, shoot); err != nil {
			return retry.MinorError(fmt.Errorf("failed to verify no running pods after hibernation: %v", err))
		}
		return retry.Ok()
	}); err != nil {
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
		completed, msg := shootoperation.ReconciliationSuccessful(shoot)
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
		completed, msg := shootoperation.ReconciliationSuccessful(shoot)
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

	if err := f.updateBinding(ctx, shoot, seed.Name); err != nil {
		return err
	}

	return f.WaitForShootToBeReconciled(ctx, shoot)
}

// GetCloudProfile returns the cloudprofile from gardener using the cloudprofile reference, alternatively by cloudProfileName.
func (f *GardenerFramework) GetCloudProfile(ctx context.Context, cloudProfileRef *gardencorev1beta1.CloudProfileReference, namespace string, name *string) (*gardencorev1beta1.CloudProfile, error) {
	// The cloudProfile reference will become the only option once cloudProfileName is deprecated and removed.
	if cloudProfileRef != nil {
		switch cloudProfileRef.Kind {
		case v1beta1constants.CloudProfileReferenceKindCloudProfile:
			cloudProfile := &gardencorev1beta1.CloudProfile{}
			if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: cloudProfileRef.Name}, cloudProfile); err != nil {
				return nil, fmt.Errorf("could not get CloudProfile '%s' in Garden cluster: %w", cloudProfileRef.Name, err)
			}
			return cloudProfile, nil
		case v1beta1constants.CloudProfileReferenceKindNamespacedCloudProfile:
			namespacedCloudProfile := &gardencorev1beta1.NamespacedCloudProfile{}
			if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: cloudProfileRef.Name, Namespace: namespace}, namespacedCloudProfile); err != nil {
				return nil, fmt.Errorf("could not get NamespacedCloudProfile '%s' in Garden cluster: %w", cloudProfileRef.Name, err)
			}
			return &gardencorev1beta1.CloudProfile{Spec: namespacedCloudProfile.Status.CloudProfileSpec}, nil
		}
	} else if name != nil {
		// Until cloudProfileName is deprecated, use it as a fallback.
		cloudProfile := &gardencorev1beta1.CloudProfile{}
		if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: *name}, cloudProfile); err != nil {
			return nil, fmt.Errorf("could not get CloudProfile '%s' in Garden cluster: %w", *name, err)
		}
		return cloudProfile, nil
	}
	return nil, errors.New("cloudprofile is required to be set in shoot spec")
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

// VerifyNoRunningPods verifies that no control plane pods are running for a given shoot.
// If any control plane pods are found to be running, returns an error with their names. Otherwise, returns nil.
func (f *GardenerFramework) VerifyNoRunningPods(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	_, seedClient, err := f.GetSeed(ctx, *shoot.Spec.SeedName)
	if err != nil {
		return err
	}

	controlPlaneNamespace := shoot.Status.TechnicalID
	podList := &metav1.PartialObjectMetadataList{}
	podList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("PodList"))
	if err := seedClient.Client().List(ctx, podList, client.InNamespace(controlPlaneNamespace)); err != nil {
		return err
	}

	if len(podList.Items) > 0 {
		runningPodNames := []string{}
		for _, pod := range podList.Items {
			runningPodNames = append(runningPodNames, pod.Name)
		}
		return fmt.Errorf("found pods in namespace %s: %v", controlPlaneNamespace, runningPodNames)
	}

	return nil
}
