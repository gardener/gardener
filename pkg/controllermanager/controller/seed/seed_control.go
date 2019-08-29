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

	"github.com/gardener/gardener/pkg/operation/common"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		oldSeed       = oldObj.(*gardenv1beta1.Seed)
		newSeed       = newObj.(*gardenv1beta1.Seed)
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
	ReconcileSeed(seed *gardenv1beta1.Seed, key string) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Seeds. updater is the UpdaterInterface used
// to update the status of Seeds. You should use an instance returned from NewDefaultControl() for any
// scenario other than testing.
func NewDefaultControl(
	k8sGardenClient kubernetes.Interface,
	k8sGardenInformers gardeninformers.SharedInformerFactory,
	secrets map[string]*corev1.Secret,
	imageVector imagevector.ImageVector,
	identity *gardenv1beta1.Gardener,
	recorder record.EventRecorder,
	updater UpdaterInterface,
	config *config.ControllerManagerConfiguration,
	secretLister kubecorev1listers.SecretLister,
	shootLister gardenlisters.ShootLister,
	backupInfrastructureLister gardenlisters.BackupInfrastructureLister,
) ControlInterface {
	return &defaultControl{k8sGardenClient,
		k8sGardenInformers,
		secrets,
		imageVector,
		identity,
		recorder,
		updater,
		config,
		secretLister,
		shootLister,
		backupInfrastructureLister,
	}
}

type defaultControl struct {
	k8sGardenClient            kubernetes.Interface
	k8sGardenInformers         gardeninformers.SharedInformerFactory
	secrets                    map[string]*corev1.Secret
	imageVector                imagevector.ImageVector
	identity                   *gardenv1beta1.Gardener
	recorder                   record.EventRecorder
	updater                    UpdaterInterface
	config                     *config.ControllerManagerConfiguration
	secretLister               kubecorev1listers.SecretLister
	shootLister                gardenlisters.ShootLister
	backupInfrastructureLister gardenlisters.BackupInfrastructureLister
}

func (c *defaultControl) ReconcileSeed(obj *gardenv1beta1.Seed, key string) error {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}

	var (
		ctx         = context.TODO()
		seed        = obj.DeepCopy()
		seedJSON, _ = json.Marshal(seed)
		seedLogger  = logger.NewFieldLogger(logger.Logger, "seed", seed.Name)
	)

	// The deletionTimestamp labels a Seed as intended to get deleted. Before deletion,
	// it has to be ensured that no Shoots are depending on the Seed anymore.
	// When this happens the controller will remove the finalizers from the Seed so that it can be garbage collected.
	if seed.DeletionTimestamp != nil {
		if !sets.NewString(seed.Finalizers...).Has(gardenv1beta1.GardenerName) {
			return nil
		}

		if seed.Spec.Backup != nil {
			if err := deleteBackupBucketInGarden(ctx, c.k8sGardenClient.Client(), seed); err != nil {
				seedLogger.Error(err.Error())
				return err
			}
		}

		associatedShoots, err := controllerutils.DetermineShootAssociations(seed, c.shootLister)
		if err != nil {
			seedLogger.Error(err.Error())
			return err
		}
		associatedBackupInfrastructures, err := controllerutils.DetermineBackupInfrastructureAssociations(seed, c.backupInfrastructureLister)
		if err != nil {
			seedLogger.Error(err.Error())
			return err
		}

		// As per design, backupBucket's are not tightly coupled with Seed resources. But to reconcile backup bucket on object store, seed
		// provides the worker node for running backup extension controller. Hence, we do check if there is another Seed available for
		// running this backup extension controller for associated backup buckets. Otherwise we block the deletion of current seed.
		// validSeedAvailable, err := validSeedAvailableForBucketRescheduling(ctx, c.k8sGardenClient.Client())
		// if err != nil {
		// 	seedLogger.Error(err.Error())
		// 	return err
		// }
		// associatedBackupBuckets := make([]string, 0)

		//if validSeedAvailable {
		associatedBackupBuckets, err := controllerutils.DetermineBackupBucketAssociations(ctx, c.k8sGardenClient.Client(), seed.Name)
		if err != nil {
			seedLogger.Error(err.Error())
			return err
		}
		//}
		if len(associatedShoots) == 0 && len(associatedBackupInfrastructures) == 0 && len(associatedBackupBuckets) == 0 {
			seedLogger.Info("No Shoots or BackupInfrastructures or BackupBuckets are referencing the Seed. Deletion accepted.")

			// Remove finalizer from referenced secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      seed.Spec.SecretRef.Name,
					Namespace: seed.Spec.SecretRef.Namespace,
				},
			}
			if err := controllerutils.RemoveFinalizer(ctx, c.k8sGardenClient.Client(), secret, gardenv1beta1.ExternalGardenerName); err != nil {
				seedLogger.Error(err.Error())
				return err
			}

			// Remove finalizer from Seed
			if err := controllerutils.RemoveGardenerFinalizer(ctx, c.k8sGardenClient.Client(), seed); err != nil {
				seedLogger.Error(err.Error())
				return err
			}
			return nil
		}

		parentLogMessage := "Can't delete Seed, because the following objects are still referencing it: "
		if len(associatedShoots) != 0 {
			seedLogger.Infof("%s Shoots=%v", parentLogMessage, associatedShoots)
		}
		if len(associatedBackupInfrastructures) != 0 {
			seedLogger.Infof("%s BackupInfrastructures=%v", parentLogMessage, associatedBackupInfrastructures)
		}
		if len(associatedBackupBuckets) != 0 {
			seedLogger.Infof("%s BackupBuckets=%v", parentLogMessage, associatedBackupBuckets)
		}
		return errors.New("seed still has references")
	}

	seedLogger.Infof("[SEED RECONCILE] %s", key)
	seedLogger.Debugf(string(seedJSON))

	// Add the Gardener finalizer to the referenced Seed secret to protect it from deletion as long as the Seed resource
	// does exist.
	secret, err := common.GetSecretFromSecretRef(ctx, c.k8sGardenClient.Client(), &seed.Spec.SecretRef)
	if err != nil {
		seedLogger.Error(err.Error())
		return err
	}

	if err := controllerutils.EnsureFinalizer(ctx, c.k8sGardenClient.Client(), secret, gardenv1beta1.ExternalGardenerName); err != nil {
		seedLogger.Error(err.Error())
		return err
	}

	// Initialize conditions based on the current status.
	conditionSeedAvailable := gardencorev1alpha1helper.GetOrInitCondition(seed.Status.Conditions, gardenv1beta1.SeedAvailable)

	seedObj, err := seedpkg.New(c.k8sGardenClient, c.k8sGardenInformers.Garden().V1beta1(), seed)
	if err != nil {
		message := fmt.Sprintf("Failed to create a Seed object (%s).", err.Error())
		conditionSeedAvailable = gardencorev1alpha1helper.UpdatedCondition(conditionSeedAvailable, gardencorev1alpha1.ConditionUnknown, gardencorev1alpha1.ConditionCheckError, message)
		seedLogger.Error(message)
		c.updateSeedStatus(seed, conditionSeedAvailable)
		return err
	}

	// Fetching associated shoots for the current seed
	associatedShoots, err := controllerutils.DetermineShootAssociations(seed, c.shootLister)
	if err != nil {
		seedLogger.Error(err.Error())
		return err
	}

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	if err := seedObj.CheckMinimumK8SVersion(); err != nil {
		conditionSeedAvailable = gardencorev1alpha1helper.UpdatedCondition(conditionSeedAvailable, gardencorev1alpha1.ConditionFalse, "K8SVersionTooOld", err.Error())
		c.updateSeedStatus(seed, conditionSeedAvailable)
		seedLogger.Error(err.Error())
		return err
	}

	// Bootstrap the Seed cluster.
	if c.config.Controllers.Seed.ReserveExcessCapacity != nil {
		seedObj.MustReserveExcessCapacity(*c.config.Controllers.Seed.ReserveExcessCapacity)
	}
	if err := seedpkg.BootstrapCluster(seedObj, c.config, c.secrets, c.imageVector, len(associatedShoots)); err != nil {
		conditionSeedAvailable = gardencorev1alpha1helper.UpdatedCondition(conditionSeedAvailable, gardencorev1alpha1.ConditionFalse, "BootstrappingFailed", err.Error())
		c.updateSeedStatus(seed, conditionSeedAvailable)
		seedLogger.Error(err.Error())
		return err
	}

	conditionSeedAvailable = gardencorev1alpha1helper.UpdatedCondition(conditionSeedAvailable, gardencorev1alpha1.ConditionTrue, "Passed", "all checks passed")
	c.updateSeedStatus(seed, conditionSeedAvailable)

	if seed.Spec.Backup != nil {
		// This should be post updating the seed is available. Since, scheduler will then mostly use
		// same seed for deploying the backupBucket extension.
		if err := deployBackupBucketInGarden(ctx, c.k8sGardenClient.Client(), seed); err != nil {
			seedLogger.Error(err.Error())
			return err
		}
	}

	return nil
}

func (c *defaultControl) updateSeedStatus(seed *gardenv1beta1.Seed, updateConditions ...gardencorev1alpha1.Condition) error {
	newConditions := gardencorev1alpha1helper.MergeConditions(seed.Status.Conditions, updateConditions...)
	newStatus := gardenv1beta1.SeedStatus{
		Conditions:         newConditions,
		ObservedGeneration: seed.Generation,
		Gardener:           *c.identity,
	}
	if apiequality.Semantic.DeepEqual(seed.Status, newStatus) {
		return nil
	}

	seed.Status = newStatus
	_, err := c.updater.UpdateSeedStatus(seed)
	if err != nil {
		logger.Logger.Errorf("Could not update the Seed status: %+v", err)
	}

	return err
}

func deployBackupBucketInGarden(ctx context.Context, k8sGardenClient client.Client, seed *gardenv1beta1.Seed) error {
	// By default, we assume the seed.Spec.Backup.Provider matches the seed.Spec.Backup.Cloud as per the validation logic.
	// However, if the backup region is specified we take it.
	region := seed.Spec.Cloud.Region
	if seed.Spec.Backup.Region != nil {
		region = *seed.Spec.Backup.Region
	}

	backupBucket := &gardencorev1alpha1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(seed.UID),
		},
	}

	backupSecret, err := common.GetSecretFromSecretRef(ctx, k8sGardenClient, &seed.Spec.Backup.SecretRef)
	if err != nil {
		return err
	}
	checksum := common.ComputeSecretCheckSum(backupSecret.Data)
	ownerRef := metav1.NewControllerRef(seed, gardenv1beta1.SchemeGroupVersion.WithKind("Seed"))

	return kutil.CreateOrUpdate(ctx, k8sGardenClient, backupBucket, func() error {
		backupBucket.ObjectMeta.Annotations = map[string]string{
			common.SecretRefChecksumAnnotation: checksum,
		}
		backupBucket.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		backupBucket.Spec = gardencorev1alpha1.BackupBucketSpec{
			Provider: gardencorev1alpha1.BackupBucketProvider{
				Type:   string(seed.Spec.Backup.Provider),
				Region: region,
			},
			SecretRef: corev1.SecretReference{
				Name:      seed.Spec.Backup.SecretRef.Name,
				Namespace: seed.Spec.Backup.SecretRef.Namespace,
			},
			Seed: &seed.Name, // In future this will be moved to scheduler.
		}

		return nil
	})
}

func deleteBackupBucketInGarden(ctx context.Context, k8sGardenClient client.Client, seed *gardenv1beta1.Seed) error {
	backupBucket := &gardencorev1alpha1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(seed.UID),
		},
	}

	return client.IgnoreNotFound(k8sGardenClient.Delete(ctx, backupBucket))
}

// func validSeedAvailableForBucketRescheduling(ctx context.Context, c client.Client) (bool, error) {
// 	seeds := &gardenv1beta1.SeedList{}
// 	if err := c.List(ctx, seeds); err != nil {
// 		return false, err
// 	}
// 	for _, seed := range seeds.Items {
// 		if seed.DeletionTimestamp == nil {
// 			return true, nil
// 		}
// 	}
// 	return false, nil
// }
