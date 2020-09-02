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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencore "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
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
	secrets map[string]*corev1.Secret,
	imageVector imagevector.ImageVector,
	componentImageVectors imagevector.ComponentImageVectors,
	identity *gardencorev1beta1.Gardener,
	recorder record.EventRecorder,
	config *config.GardenletConfiguration,
	secretLister kubecorev1listers.SecretLister,
	shootLister gardencorelisters.ShootLister,
) ControlInterface {
	return &defaultControl{
		clientMap,
		k8sGardenCoreInformers,
		secrets,
		imageVector,
		componentImageVectors,
		identity,
		recorder,
		config,
		secretLister,
		shootLister,
	}
}

type defaultControl struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	secrets                map[string]*corev1.Secret
	imageVector            imagevector.ImageVector
	componentImageVectors  imagevector.ComponentImageVectors
	identity               *gardencorev1beta1.Gardener
	recorder               record.EventRecorder
	config                 *config.GardenletConfiguration
	secretLister           kubecorev1listers.SecretLister
	shootLister            gardencorelisters.ShootLister
}

func (c *defaultControl) ReconcileSeed(obj *gardencorev1beta1.Seed, key string) error {
	var (
		ctx         = context.TODO()
		seed        = obj.DeepCopy()
		seedJSON, _ = json.Marshal(seed)
		seedLogger  = logger.NewFieldLogger(logger.Logger, "seed", seed.Name)
		err         error
	)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
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

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(seed, c.shootLister)
		if err != nil {
			seedLogger.Error(err.Error())
			return err
		}
		// As per design, backupBucket's are not tightly coupled with Seed resources. But to reconcile backup bucket on object store, seed
		// provides the worker node for running backup extension controller. Hence, we do check if there is another Seed available for
		// running this backup extension controller for associated backup buckets. Otherwise we block the deletion of current seed.
		// validSeedBootstrapped, err := validSeedBootstrappedForBucketRescheduling(ctx, gardenClient.Client())
		// if err != nil {
		// 	seedLogger.Error(err.Error())
		// 	return err
		// }
		// associatedBackupBuckets := make([]string, 0)

		//if validSeedBootstrapped {
		associatedBackupBuckets, err := controllerutils.DetermineBackupBucketAssociations(ctx, gardenClient.Client(), seed.Name)
		if err != nil {
			seedLogger.Error(err.Error())
			return err
		}
		//}
		if len(associatedShoots) == 0 && len(associatedBackupBuckets) == 0 {
			seedLogger.Info("No Shoots or BackupBuckets are referencing the Seed. Deletion accepted.")

			if seed.Spec.SecretRef != nil {
				// Remove finalizer from referenced secret
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      seed.Spec.SecretRef.Name,
						Namespace: seed.Spec.SecretRef.Namespace,
					},
				}
				if err := controllerutils.RemoveFinalizer(ctx, gardenClient.DirectClient(), secret, gardencorev1beta1.ExternalGardenerName); err != nil {
					seedLogger.Error(err.Error())
					return err
				}
			}

			// Remove finalizer from Seed
			if err := controllerutils.RemoveGardenerFinalizer(ctx, gardenClient.DirectClient(), seed); err != nil {
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
	seedLogger.Debugf(string(seedJSON))

	// need retry logic, because controllerregistration controller is acting on it at the same time and cached object might not be up to date
	seed, err = kutil.TryUpdateSeed(ctx, gardenClient.GardenCore(), retry.DefaultBackoff, seed.ObjectMeta, func(curSeed *gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error) {
		finalizers := sets.NewString(curSeed.Finalizers...)
		if finalizers.Has(gardencorev1beta1.GardenerName) {
			return curSeed, nil
		}

		finalizers.Insert(gardencorev1beta1.GardenerName)
		curSeed.Finalizers = finalizers.UnsortedList()

		return curSeed, nil
	})

	if err != nil {
		err = fmt.Errorf("could not add finalizer to Seed: %s", err.Error())
		seedLogger.Error(err)
		return err
	}

	// Add the Gardener finalizer to the referenced Seed secret to protect it from deletion as long as the Seed resource
	// does exist.
	if seed.Spec.SecretRef != nil {
		secret, err := common.GetSecretFromSecretRef(ctx, gardenClient.Client(), seed.Spec.SecretRef)
		if err != nil {
			seedLogger.Error(err.Error())
			return err
		}
		if err := controllerutils.EnsureFinalizer(ctx, gardenClient.Client(), secret, gardencorev1beta1.ExternalGardenerName); err != nil {
			seedLogger.Error(err.Error())
			return err
		}
	}

	// Initialize conditions based on the current status.
	conditionSeedBootstrapped := gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped)

	seedObj, err := seedpkg.
		NewBuilder().
		WithSeedObject(seed).
		WithSeedSecretFromClient(ctx, gardenClient.Client()).
		Build()
	if err != nil {
		message := fmt.Sprintf("Failed to create a Seed object (%s).", err.Error())
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionUnknown, gardencorev1beta1.ConditionCheckError, message)
		seedLogger.Error(message)
		_ = c.updateSeedStatus(ctx, gardenClient.GardenCore(), seed, "<unknown>", conditionSeedBootstrapped)
		return err
	}

	seedClient, err := c.clientMap.GetClient(ctx, keys.ForSeed(seed))
	if err != nil {
		return fmt.Errorf("failed to get seed client: %w", err)
	}

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	seedKubernetesVersion, err := seedObj.CheckMinimumK8SVersion(seedClient)
	if err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "K8SVersionTooOld", err.Error())
		_ = c.updateSeedStatus(ctx, gardenClient.GardenCore(), seed, seedKubernetesVersion, conditionSeedBootstrapped)
		seedLogger.Error(err.Error())
		return err
	}

	// Bootstrap the Seed cluster.
	if err := seedpkg.BootstrapCluster(ctx, gardenClient, seedClient, seedObj, c.secrets, c.imageVector, c.componentImageVectors); err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "BootstrappingFailed", err.Error())
		_ = c.updateSeedStatus(ctx, gardenClient.GardenCore(), seed, seedKubernetesVersion, conditionSeedBootstrapped)
		seedLogger.Errorf("Seed bootstrapping failed: %+v", err)
		return err
	}

	conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionTrue, "BootstrappingSucceeded", "Seed cluster has been bootstrapped successfully.")
	_ = c.updateSeedStatus(ctx, gardenClient.GardenCore(), seed, seedKubernetesVersion, conditionSeedBootstrapped)

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

func (c *defaultControl) updateSeedStatus(ctx context.Context, g gardencore.Interface, seed *gardencorev1beta1.Seed, k8sVersion string, updateConditions ...gardencorev1beta1.Condition) error {
	if _, err := kutil.TryUpdateSeedStatus(ctx, g, retry.DefaultBackoff, seed.ObjectMeta,
		func(seed *gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error) {
			// remove "available condition"
			for i, c := range seed.Status.Conditions {
				if c.Type == "Available" {
					seed.Status.Conditions = append(seed.Status.Conditions[:i], seed.Status.Conditions[i+1:]...)
					break
				}
			}

			seed.Status.Conditions = gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, updateConditions...)
			seed.Status.ObservedGeneration = seed.Generation
			seed.Status.Gardener = c.identity
			seed.Status.KubernetesVersion = &k8sVersion
			return seed, nil
		},
	); err != nil {
		logger.Logger.Errorf("Could not update the Seed status: %+v", err)
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

	_, err := controllerutil.CreateOrUpdate(ctx, k8sGardenClient, backupBucket, func() error {
		backupBucket.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		backupBucket.Spec = gardencorev1beta1.BackupBucketSpec{
			Provider: gardencorev1beta1.BackupBucketProvider{
				Type:   string(seed.Spec.Backup.Provider),
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
