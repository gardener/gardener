// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (c *Controller) seedAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.seedQueue.Add(key)
}

func (c *Controller) seedUpdate(oldObj, newObj interface{}) {
	var (
		oldSeed       = oldObj.(*gardencorev1beta1.Seed)
		newSeed       = newObj.(*gardencorev1beta1.Seed)
		specChanged   = !apiequality.Semantic.DeepEqual(oldSeed.Spec, newSeed.Spec)
		statusChanged = !apiequality.Semantic.DeepEqual(oldSeed.Status, newSeed.Status)
	)

	if !specChanged && statusChanged {
		return
	}
	c.seedAdd(newObj)
}

func (c *Controller) seedDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.seedQueue.Add(key)
}

func (c *Controller) reconcileSeedKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	seed, err := c.seedLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SEED RECONCILE] %s - skipping because Seed has been deleted", key)

		if err := c.clientMap.InvalidateClient(keys.ForSeedWithName(name)); err != nil {
			return fmt.Errorf("failed to invalidate seed client: %w", err)
		}

		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SEED RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.control.ReconcileSeed(seed, key); err != nil {
		c.seedQueue.AddAfter(key, 15*time.Second)
	} else {
		c.seedQueue.AddAfter(key, c.config.Controllers.Seed.SyncPeriod.Duration)
	}
	return err
}

// ControlInterface implements the control logic for updating Seeds. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	// ReconcileSeed implements the control logic for Seed creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	ReconcileSeed(seed *gardencorev1beta1.Seed, key string) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Seeds. You should use an instance returned from NewDefaultControl() for any
// scenario other than testing.
func NewDefaultControl(
	clientMap clientmap.ClientMap,
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory,
	imageVector imagevector.ImageVector,
	componentImageVectors imagevector.ComponentImageVectors,
	identity *gardencorev1beta1.Gardener,
	recorder record.EventRecorder,
	config *config.GardenletConfiguration,
	seedLister gardencorelisters.SeedLister,
) ControlInterface {
	return &defaultControl{
		clientMap,
		k8sGardenCoreInformers,
		imageVector,
		componentImageVectors,
		identity,
		recorder,
		config,
		seedLister,
	}
}

type defaultControl struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	imageVector            imagevector.ImageVector
	componentImageVectors  imagevector.ComponentImageVectors
	identity               *gardencorev1beta1.Gardener
	recorder               record.EventRecorder
	config                 *config.GardenletConfiguration
	seedLister             gardencorelisters.SeedLister
}

func (c *defaultControl) ReconcileSeed(obj *gardencorev1beta1.Seed, key string) error {
	var (
		ctx        = context.TODO()
		seed       = obj.DeepCopy()
		seedLogger = logger.NewFieldLogger(logger.Logger, "seed", seed.Name)
		err        error
	)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	seedNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: gutil.ComputeGardenNamespace(obj.Name),
		},
	}

	// Check if seed namespace is already available.
	if err := gardenClient.Client().Get(ctx, client.ObjectKeyFromObject(seedNamespace), seedNamespace); err != nil {
		return fmt.Errorf("failed to get seed namespace in garden cluster: %w", err)
	}

	// Initialize capacity and allocatable
	var capacity, allocatable corev1.ResourceList
	if c.config.Resources != nil && len(c.config.Resources.Capacity) > 0 {
		capacity = make(corev1.ResourceList, len(c.config.Resources.Capacity))
		allocatable = make(corev1.ResourceList, len(c.config.Resources.Capacity))
		for resourceName, quantity := range c.config.Resources.Capacity {
			capacity[resourceName] = quantity
			if reservedQuantity, ok := c.config.Resources.Reserved[resourceName]; ok {
				allocatableQuantity := quantity.DeepCopy()
				allocatableQuantity.Sub(reservedQuantity)
				allocatable[resourceName] = allocatableQuantity
			} else {
				allocatable[resourceName] = quantity
			}
		}
	}

	// Initialize conditions based on the current status.
	conditionSeedBootstrapped := gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped)

	seedObj, err := seedpkg.
		NewBuilder().
		WithSeedObject(seed).
		Build()
	if err != nil {
		message := fmt.Sprintf("Failed to create a Seed object (%s).", err.Error())
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionUnknown, gardencorev1beta1.ConditionCheckError, message)
		seedLogger.Error(message)
		_ = c.patchSeedStatus(ctx, gardenClient.Client(), seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped)
		return err
	}

	seedClient, err := c.clientMap.GetClient(ctx, keys.ForSeed(seed))
	if err != nil {
		return fmt.Errorf("failed to get seed client: %w", err)
	}

	// The deletionTimestamp labels a Seed as intended to get deleted. Before deletion,
	// it has to be ensured that no Shoots are depending on the Seed anymore.
	// When this happens the controller will remove the finalizers from the Seed so that it can be garbage collected.
	if seed.DeletionTimestamp != nil {
		if !sets.NewString(seed.Finalizers...).Has(gardencorev1beta1.GardenerName) {
			return nil
		}

		if seed.Spec.Backup != nil {
			if err := deleteBackupBucketInGarden(ctx, gardenClient.Client(), seed); err != nil {
				seedLogger.Error(err.Error())
				return err
			}
		}

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, gardenClient.Client(), seed)
		if err != nil {
			seedLogger.Error(err.Error())
			return err
		}

		associatedBackupBuckets, err := controllerutils.DetermineBackupBucketAssociations(ctx, gardenClient.Client(), seed.Name)
		if err != nil {
			seedLogger.Error(err.Error())
			return err
		}

		if len(associatedShoots) == 0 && len(associatedBackupBuckets) == 0 {
			seedLogger.Info("No Shoots, ControllerInstallations or BackupBuckets are referencing the Seed. Deletion accepted.")

			if err := seedpkg.RunDeleteSeedFlow(ctx, seedClient, gardenClient, seedObj, seedLogger); err != nil {
				message := fmt.Sprintf("Failed to delete Seed Cluster (%s).", err.Error())
				conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "DebootstrapFailed", message)
				seedLogger.Error(message)
				_ = c.patchSeedStatus(ctx, gardenClient.Client(), seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped)
				return err
			}

			if seed.Spec.SecretRef != nil {
				// Remove finalizer from referenced secret
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      seed.Spec.SecretRef.Name,
						Namespace: seed.Spec.SecretRef.Namespace,
					},
				}
				err = gardenClient.Client().Get(ctx, client.ObjectKeyFromObject(secret), secret)
				if err == nil {
					if err2 := controllerutils.PatchRemoveFinalizers(ctx, gardenClient.Client(), secret, gardencorev1beta1.ExternalGardenerName); err2 != nil {
						return fmt.Errorf("failed to remove finalizer from Seed secret '%s/%s': %w", secret.Namespace, secret.Name, err2)
					}
				} else if !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed to get Seed secret '%s/%s': %w", secret.Namespace, secret.Name, err)
				}
			}

			// Remove finalizer from Seed
			if err := controllerutils.PatchRemoveFinalizers(ctx, gardenClient.Client(), seed, gardencorev1beta1.GardenerName); err != nil {
				seedLogger.Error(err.Error())
				return err
			}

			if err := c.clientMap.InvalidateClient(keys.ForSeed(seed)); err != nil {
				return fmt.Errorf("failed to invalidate seed client: %w", err)
			}

			return nil
		}

		parentLogMessage := "Can't delete Seed, because the following objects are still referencing it:"
		if len(associatedShoots) != 0 {
			message := fmt.Sprintf("%s Shoots=%v", parentLogMessage, associatedShoots)
			seedLogger.Info(message)
			c.recorder.Event(seed, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)
		}
		if len(associatedBackupBuckets) != 0 {
			message := fmt.Sprintf("%s BackupBuckets=%v", parentLogMessage, associatedBackupBuckets)
			seedLogger.Info(message)
			c.recorder.Event(seed, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)
		}

		return errors.New("seed still has references")
	}

	seedLogger.Infof("[SEED RECONCILE] %s", key)

	// TODO(timebertt): replace with strategic merge, so we can run without optimistic locking
	// (make use of $deleteFromPrimitiveList directive)
	if !controllerutil.ContainsFinalizer(seed, gardencorev1beta1.GardenerName) {
		if err := controllerutils.PatchAddFinalizers(ctx, gardenClient.Client(), seed, gardencorev1beta1.GardenerName); err != nil {
			err = fmt.Errorf("could not add finalizer to Seed: %s", err.Error())
			seedLogger.Error(err)
			return err
		}
	}

	// Add the Gardener finalizer to the referenced Seed secret to protect it from deletion as long as the Seed resource
	// does exist.
	if seed.Spec.SecretRef != nil {
		secret, err := kutil.GetSecretByReference(ctx, gardenClient.Client(), seed.Spec.SecretRef)
		if err != nil {
			seedLogger.Error(err.Error())
			return err
		}
		if err := controllerutils.PatchAddFinalizers(ctx, gardenClient.Client(), secret, gardencorev1beta1.ExternalGardenerName); err != nil {
			seedLogger.Error(err.Error())
			return err
		}
	}

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	seedKubernetesVersion, err := seedObj.CheckMinimumK8SVersion(seedClient)
	if err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "K8SVersionTooOld", err.Error())
		_ = c.patchSeedStatus(ctx, gardenClient.Client(), seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped)
		seedLogger.Error(err.Error())
		return err
	}

	gardenSecrets, err := garden.ReadGardenSecrets(
		ctx,
		gardenClient.Client(),
		c.seedLister,
		gutil.ComputeGardenNamespace(seed.Name),
	)
	if err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "GardenSecretsError", err.Error())
		_ = c.patchSeedStatus(ctx, gardenClient.Client(), seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped)
		seedLogger.Errorf("Reading Garden secrets failed: %+v", err)
		return err
	}

	// Bootstrap the Seed cluster.
	if err := seedpkg.RunReconcileSeedFlow(ctx, gardenClient, seedClient, seedObj, gardenSecrets, c.imageVector, c.componentImageVectors, c.config.DeepCopy(), seedLogger); err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "BootstrappingFailed", err.Error())
		_ = c.patchSeedStatus(ctx, gardenClient.Client(), seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped)
		seedLogger.Errorf("Seed bootstrapping failed: %+v", err)
		return err
	}

	conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionTrue, "BootstrappingSucceeded", "Seed cluster has been bootstrapped successfully.")
	_ = c.patchSeedStatus(ctx, gardenClient.Client(), seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped)

	if seed.Spec.Backup != nil {
		// This should be post updating the seed is available. Since, scheduler will then mostly use
		// same seed for deploying the backupBucket extension.
		if err := deployBackupBucketInGarden(ctx, gardenClient.Client(), seed); err != nil {
			seedLogger.Error(err.Error())
			return err
		}
	}

	return nil
}

func (c *defaultControl) patchSeedStatus(
	ctx context.Context,
	cl client.Client,
	seed *gardencorev1beta1.Seed,
	k8sVersion string,
	capacity, allocatable corev1.ResourceList,
	updateConditions ...gardencorev1beta1.Condition,
) error {
	patch := client.StrategicMergeFrom(seed.DeepCopy())

	seed.Status.Conditions = gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, updateConditions...)
	seed.Status.ObservedGeneration = seed.Generation
	seed.Status.Gardener = c.identity
	seed.Status.KubernetesVersion = &k8sVersion
	seed.Status.Capacity = capacity
	seed.Status.Allocatable = allocatable

	if err := cl.Status().Patch(ctx, seed, patch); err != nil {
		logger.Logger.Errorf("Could not update the Seed status: %+v", err)
		return err
	}

	return nil
}

func deployBackupBucketInGarden(ctx context.Context, k8sGardenClient client.Client, seed *gardencorev1beta1.Seed) error {
	// By default, we assume the seed.Spec.Backup.Provider matches the seed.Spec.Provider.Type as per the validation logic.
	// However, if the backup region is specified we take it.
	region := seed.Spec.Provider.Region
	if seed.Spec.Backup.Region != nil {
		region = *seed.Spec.Backup.Region
	}

	backupBucket := &gardencorev1beta1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(seed.UID),
		},
	}

	ownerRef := metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))

	_, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, k8sGardenClient, backupBucket, func() error {
		backupBucket.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		backupBucket.Spec = gardencorev1beta1.BackupBucketSpec{
			Provider: gardencorev1beta1.BackupBucketProvider{
				Type:   seed.Spec.Backup.Provider,
				Region: region,
			},
			ProviderConfig: seed.Spec.Backup.ProviderConfig,
			SecretRef: corev1.SecretReference{
				Name:      seed.Spec.Backup.SecretRef.Name,
				Namespace: seed.Spec.Backup.SecretRef.Namespace,
			},
			SeedName: &seed.Name, // In future this will be moved to gardener-scheduler.
		}
		return nil
	})
	return err
}

func deleteBackupBucketInGarden(ctx context.Context, k8sGardenClient client.Client, seed *gardencorev1beta1.Seed) error {
	backupBucket := &gardencorev1beta1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(seed.UID),
		},
	}

	return client.IgnoreNotFound(k8sGardenClient.Delete(ctx, backupBucket))
}
