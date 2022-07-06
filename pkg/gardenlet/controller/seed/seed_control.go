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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const reconcilerName = "seed"

func (c *Controller) seedAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Could not get key", "obj", obj)
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
		c.log.Error(err, "Could not get key", "obj", obj)
		return
	}
	c.seedQueue.Add(key)
}

func newReconciler(
	clientMap clientmap.ClientMap,
	recorder record.EventRecorder,
	imageVector imagevector.ImageVector,
	componentImageVectors imagevector.ComponentImageVectors,
	identity *gardencorev1beta1.Gardener,
	clientCertificateExpirationTimestamp *metav1.Time,
	config *config.GardenletConfiguration,
) reconcile.Reconciler {
	return &reconciler{
		clientMap:                            clientMap,
		recorder:                             recorder,
		imageVector:                          imageVector,
		componentImageVectors:                componentImageVectors,
		identity:                             identity,
		clientCertificateExpirationTimestamp: clientCertificateExpirationTimestamp,
		config:                               config,
	}
}

type reconciler struct {
	clientMap                            clientmap.ClientMap
	recorder                             record.EventRecorder
	imageVector                          imagevector.ImageVector
	componentImageVectors                imagevector.ComponentImageVectors
	identity                             *gardencorev1beta1.Gardener
	clientCertificateExpirationTimestamp *metav1.Time
	config                               *config.GardenletConfiguration
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	seed := &gardencorev1beta1.Seed{}
	if err := gardenClient.Client().Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")

			if err := r.clientMap.InvalidateClient(keys.ForSeedWithName(request.Name)); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to invalidate seed client: %w", err)
			}

			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if err := r.reconcile(ctx, log, gardenClient.Client(), seed); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.config.Controllers.Seed.SyncPeriod.Duration}, nil
}

func (r *reconciler) reconcile(ctx context.Context, log logr.Logger, gardenClient client.Client, seed *gardencorev1beta1.Seed) error {
	seedNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: gutil.ComputeGardenNamespace(seed.Name),
		},
	}

	// Check if seed namespace is already available.
	if err := gardenClient.Get(ctx, client.ObjectKeyFromObject(seedNamespace), seedNamespace); err != nil {
		return fmt.Errorf("failed to get seed namespace in garden cluster: %w", err)
	}

	// Initialize capacity and allocatable
	var capacity, allocatable corev1.ResourceList
	if r.config.Resources != nil && len(r.config.Resources.Capacity) > 0 {
		capacity = make(corev1.ResourceList, len(r.config.Resources.Capacity))
		allocatable = make(corev1.ResourceList, len(r.config.Resources.Capacity))
		for resourceName, quantity := range r.config.Resources.Capacity {
			capacity[resourceName] = quantity
			if reservedQuantity, ok := r.config.Resources.Reserved[resourceName]; ok {
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
		Build(ctx)
	if err != nil {
		log.Error(err, "Failed to create a Seed object")
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionUnknown, gardencorev1beta1.ConditionCheckError, fmt.Sprintf("Failed to create a Seed object (%s).", err.Error()))
		if err := r.patchSeedStatus(ctx, gardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return fmt.Errorf("could not patch seed status after failed creation of Seed object: %w", err)
		}
		return err
	}

	seedClientSet, err := r.clientMap.GetClient(ctx, keys.ForSeed(seed))
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
			if err := deleteBackupBucketInGarden(ctx, gardenClient, seed); err != nil {
				return err
			}
		}

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, gardenClient, seed)
		if err != nil {
			return err
		}

		associatedBackupBuckets, err := controllerutils.DetermineBackupBucketAssociations(ctx, gardenClient, seed.Name)
		if err != nil {
			return err
		}

		if len(associatedShoots) == 0 && len(associatedBackupBuckets) == 0 {
			log.Info("No Shoots, ControllerInstallations or BackupBuckets are referencing the Seed, deletion accepted")

			if err := seedpkg.RunDeleteSeedFlow(ctx, log, gardenClient, seedClientSet, seedObj, r.config.DeepCopy()); err != nil {
				conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "DebootstrapFailed", fmt.Sprintf("Failed to delete Seed Cluster (%s).", err.Error()))
				if err := r.patchSeedStatus(ctx, gardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
					return fmt.Errorf("could not patch seed status after deletion flow failed: %w", err)
				}
				return err
			}

			// Remove finalizer from referenced secret
			if seed.Spec.SecretRef != nil {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      seed.Spec.SecretRef.Name,
						Namespace: seed.Spec.SecretRef.Namespace,
					},
				}
				log.Info("Removing finalizer form referenced secret")
				if err := gardenClient.Get(ctx, client.ObjectKeyFromObject(secret), secret); err == nil {
					if err := controllerutils.PatchRemoveFinalizers(ctx, gardenClient, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
						return fmt.Errorf("failed to remove finalizer from Seed secret '%s/%s': %w", secret.Namespace, secret.Name, err)
					}
				} else if !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed to get Seed secret '%s/%s': %w", secret.Namespace, secret.Name, err)
				}
			}

			// Remove finalizer from Seed
			log.Info("Removing finalizer")
			if err := controllerutils.PatchRemoveFinalizers(ctx, gardenClient, seed, gardencorev1beta1.GardenerName); err != nil {
				return err
			}

			if err := r.clientMap.InvalidateClient(keys.ForSeed(seed)); err != nil {
				return fmt.Errorf("failed to invalidate seed client: %w", err)
			}

			return nil
		}

		parentLogMessage := "Can't delete Seed, because the following objects are still referencing it:"
		if len(associatedShoots) != 0 {
			log.Info("Cannot delete SEed because the following Shoots are still referencing it", "shoots", associatedShoots)
			r.recorder.Event(seed, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, fmt.Sprintf("%s Shoots=%v", parentLogMessage, associatedShoots))
		}
		if len(associatedBackupBuckets) != 0 {
			log.Info("Cannot delete SEed because the following BackupBuckets are still referencing it", "backupBuckets", associatedBackupBuckets)
			r.recorder.Event(seed, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, fmt.Sprintf("%s BackupBuckets=%v", parentLogMessage, associatedBackupBuckets))
		}

		return errors.New("seed still has references")
	}

	if !controllerutil.ContainsFinalizer(seed, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, gardenClient, seed, gardencorev1beta1.GardenerName); err != nil {
			return fmt.Errorf("could not add finalizer to Seed: %s", err.Error())
		}
	}

	// Add the Gardener finalizer to the referenced Seed secret to protect it from deletion as long as the Seed resource
	// does exist.
	if seed.Spec.SecretRef != nil {
		secret, err := kutil.GetSecretByReference(ctx, gardenClient, seed.Spec.SecretRef)
		if err != nil {
			return err
		}

		if !controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
			log.Info("Adding finalizer to referenced secret")
			if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, gardenClient, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
				return err
			}
		}
	}

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	seedKubernetesVersion, err := seedObj.CheckMinimumK8SVersion(seedClientSet.Version())
	if err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "K8SVersionTooOld", err.Error())
		if err := r.patchSeedStatus(ctx, gardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return fmt.Errorf("could not patch seed status after check for minimum Kubernetes version failed: %w", err)
		}
		return err
	}

	gardenSecrets, err := garden.ReadGardenSecrets(ctx, log, gardenClient, gutil.ComputeGardenNamespace(seed.Name), true)
	if err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "GardenSecretsError", err.Error())
		if err := r.patchSeedStatus(ctx, gardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return fmt.Errorf("could not patch seed status after reading garden secrets failed: %w", err)
		}
		return err
	}

	conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionProgressing, "BootstrapProgressing", "Seed cluster is currently being bootstrapped.")
	if err = r.patchSeedStatus(ctx, gardenClient, seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped); err != nil {
		return fmt.Errorf("could not update status of %s condition to %s: %w", conditionSeedBootstrapped.Type, gardencorev1beta1.ConditionProgressing, err)
	}

	// Bootstrap the Seed cluster.
	if err := seedpkg.RunReconcileSeedFlow(ctx, log, gardenClient, seedClientSet, seedObj, gardenSecrets, r.imageVector, r.componentImageVectors, r.config.DeepCopy()); err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "BootstrappingFailed", err.Error())
		if err := r.patchSeedStatus(ctx, gardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return fmt.Errorf("could not patch seed status after reconciliation flow failed: %w", err)
		}
		return err
	}

	// Set the status of SeedSystemComponentsHealthy condition to Progressing so that the Seed does not immediately become ready
	// after being successfully bootstrapped in case the system components got updated. The SeedSystemComponentsHealthy condition
	// will be set to either True, False or Progressing by the seed care reconciler depending on the health of the system components
	// after the necessary checks are completed.
	conditionSeedSystemComponentsHealthy := gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedSystemComponentsHealthy)
	conditionSeedSystemComponentsHealthy = gardencorev1beta1helper.UpdatedCondition(conditionSeedSystemComponentsHealthy, gardencorev1beta1.ConditionProgressing, "SystemComponentsCheckProgressing", "Pending health check of system components after successful bootstrap of seed cluster.")
	conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionTrue, "BootstrappingSucceeded", "Seed cluster has been bootstrapped successfully.")
	if err = r.patchSeedStatus(ctx, gardenClient, seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped, conditionSeedSystemComponentsHealthy); err != nil {
		return fmt.Errorf("could not update status of %s condition to %s and %s conditions to %s: %w", conditionSeedBootstrapped.Type, gardencorev1beta1.ConditionTrue, conditionSeedSystemComponentsHealthy.Type, gardencorev1beta1.ConditionProgressing, err)
	}

	if seed.Spec.Backup != nil {
		// This should be post updating the seed is available. Since, scheduler will then mostly use
		// same seed for deploying the backupBucket extension.
		if err := deployBackupBucketInGarden(ctx, gardenClient, seed); err != nil {
			return err
		}
	}

	return nil
}

func (r *reconciler) patchSeedStatus(
	ctx context.Context,
	c client.Client,
	seed *gardencorev1beta1.Seed,
	seedVersion string,
	capacity, allocatable corev1.ResourceList,
	updateConditions ...gardencorev1beta1.Condition,
) error {
	patch := client.StrategicMergeFrom(seed.DeepCopy())

	seed.Status.Conditions = gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, updateConditions...)
	seed.Status.ObservedGeneration = seed.Generation
	seed.Status.Gardener = r.identity
	seed.Status.ClientCertificateExpirationTimestamp = r.clientCertificateExpirationTimestamp
	seed.Status.KubernetesVersion = &seedVersion
	seed.Status.Capacity = capacity
	seed.Status.Allocatable = allocatable

	return c.Status().Patch(ctx, seed, patch)
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
