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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles Seed resources and provisions or de-provisions the seed system components.
type Reconciler struct {
	GardenClient                         client.Client
	SeedClientSet                        kubernetes.Interface
	Config                               config.GardenletConfiguration
	Recorder                             record.EventRecorder
	Identity                             *gardencorev1beta1.Gardener
	ImageVector                          imagevector.ImageVector
	ComponentImageVectors                imagevector.ComponentImageVectors
	ClientCertificateExpirationTimestamp *metav1.Time
	GardenNamespaceName                  string
	ChartsPath                           string
}

// Reconcile reconciles Seed resources and provisions or de-provisions the seed system components.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	seed := &gardencorev1beta1.Seed{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// Check if seed namespace is already available.
	if err := r.GardenClient.Get(ctx, client.ObjectKey{Name: gutil.ComputeGardenNamespace(seed.Name)}, &corev1.Namespace{}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed namespace in garden cluster: %w", err)
	}

	// Initialize capacity and allocatable
	var capacity, allocatable corev1.ResourceList
	if r.Config.Resources != nil && len(r.Config.Resources.Capacity) > 0 {
		capacity = make(corev1.ResourceList, len(r.Config.Resources.Capacity))
		allocatable = make(corev1.ResourceList, len(r.Config.Resources.Capacity))

		for resourceName, quantity := range r.Config.Resources.Capacity {
			capacity[resourceName] = quantity
			allocatable[resourceName] = quantity

			if reservedQuantity, ok := r.Config.Resources.Reserved[resourceName]; ok {
				allocatableQuantity := quantity.DeepCopy()
				allocatableQuantity.Sub(reservedQuantity)
				allocatable[resourceName] = allocatableQuantity
			}
		}
	}

	// Initialize conditions based on the current status.
	conditionSeedBootstrapped := gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped)

	seedObj, err := seedpkg.NewBuilder().WithSeedObject(seed).Build(ctx)
	if err != nil {
		log.Error(err, "Failed to create a Seed object")
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionUnknown, gardencorev1beta1.ConditionCheckError, fmt.Sprintf("Failed to create a Seed object (%s).", err.Error()))
		if err := r.patchSeedStatus(ctx, r.GardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after failed creation of Seed object: %w", err)
		}
		return reconcile.Result{}, err
	}

	if seed.DeletionTimestamp != nil {
		return r.delete(ctx, log, seedObj)
	}

	if !controllerutil.ContainsFinalizer(seed, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, seed, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Add the Gardener finalizer to the referenced Seed secret to protect it from deletion as long as the Seed resource
	// does exist.
	if seed.Spec.SecretRef != nil {
		secret, err := kutil.GetSecretByReference(ctx, r.GardenClient, seed.Spec.SecretRef)
		if err != nil {
			return reconcile.Result{}, err
		}

		if !controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
			log.Info("Adding finalizer to referenced secret", "secret", client.ObjectKeyFromObject(secret))
			if err := controllerutils.AddFinalizers(ctx, r.GardenClient, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	seedKubernetesVersion, err := seedObj.CheckMinimumK8SVersion(r.SeedClientSet.Version())
	if err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "K8SVersionTooOld", err.Error())
		if err := r.patchSeedStatus(ctx, r.GardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after check for minimum Kubernetes version failed: %w", err)
		}
		return reconcile.Result{}, err
	}

	gardenSecrets, err := garden.ReadGardenSecrets(ctx, log, r.GardenClient, gutil.ComputeGardenNamespace(seed.Name), true)
	if err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "GardenSecretsError", err.Error())
		if err := r.patchSeedStatus(ctx, r.GardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after reading garden secrets failed: %w", err)
		}
		return reconcile.Result{}, err
	}

	conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionProgressing, "BootstrapProgressing", "Seed cluster is currently being bootstrapped.")
	if err = r.patchSeedStatus(ctx, r.GardenClient, seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status of %s condition to %s: %w", conditionSeedBootstrapped.Type, gardencorev1beta1.ConditionProgressing, err)
	}

	// Bootstrap the Seed cluster.
	if err := seedpkg.RunReconcileSeedFlow(ctx, log, r.GardenClient, r.SeedClientSet, seedObj, gardenSecrets, r.ImageVector, r.ComponentImageVectors, r.Config.DeepCopy(), r.GardenNamespaceName, r.ChartsPath); err != nil {
		conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "BootstrappingFailed", err.Error())
		if err := r.patchSeedStatus(ctx, r.GardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after reconciliation flow failed: %w", err)
		}
		return reconcile.Result{}, err
	}

	// Set the status of SeedSystemComponentsHealthy condition to Progressing so that the Seed does not immediately become ready
	// after being successfully bootstrapped in case the system components got updated. The SeedSystemComponentsHealthy condition
	// will be set to either True, False or Progressing by the seed care reconciler depending on the health of the system components
	// after the necessary checks are completed.
	conditionSeedSystemComponentsHealthy := gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedSystemComponentsHealthy)
	conditionSeedSystemComponentsHealthy = gardencorev1beta1helper.UpdatedCondition(conditionSeedSystemComponentsHealthy, gardencorev1beta1.ConditionProgressing, "SystemComponentsCheckProgressing", "Pending health check of system components after successful bootstrap of seed cluster.")
	conditionSeedBootstrapped = gardencorev1beta1helper.UpdatedCondition(conditionSeedBootstrapped, gardencorev1beta1.ConditionTrue, "BootstrappingSucceeded", "Seed cluster has been bootstrapped successfully.")
	if err = r.patchSeedStatus(ctx, r.GardenClient, seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped, conditionSeedSystemComponentsHealthy); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status of %s condition to %s and %s conditions to %s: %w", conditionSeedBootstrapped.Type, gardencorev1beta1.ConditionTrue, conditionSeedSystemComponentsHealthy.Type, gardencorev1beta1.ConditionProgressing, err)
	}

	if seed.Spec.Backup != nil {
		// This should be post updating the seed is available. Since, scheduler will then mostly use
		// same seed for deploying the backupBucket extension.
		if err := deployBackupBucketInGarden(ctx, r.GardenClient, seed); err != nil {
			return reconcile.Result{}, err
		}
	}

	// TODO (kris94): remove after a couple of releases
	if err := CleanupLegacyPriorityClasses(ctx, r.SeedClientSet.Client()); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.Seed.SyncPeriod.Duration}, nil
}

func (r *Reconciler) patchSeedStatus(
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
	seed.Status.Gardener = r.Identity
	seed.Status.ClientCertificateExpirationTimestamp = r.ClientCertificateExpirationTimestamp
	seed.Status.KubernetesVersion = &seedVersion

	if capacity != nil {
		seed.Status.Capacity = capacity
	}

	if allocatable != nil {
		seed.Status.Allocatable = allocatable
	}

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

// CleanupLegacyPriorityClasses deletes reversed-vpn-auth-server and fluent-bit priority classes.
func CleanupLegacyPriorityClasses(ctx context.Context, seedClient client.Client) error {
	// TODO(ialidzhikov): Clean up the loki PriorityClass as well in a future release.
	for _, name := range []string{"reversed-vpn-auth-server", "fluent-bit"} {
		priorityClass := &schedulingv1.PriorityClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		if err := client.IgnoreNotFound(seedClient.Delete(ctx, priorityClass)); err != nil {
			return err
		}
	}

	return nil
}

// determineClusterIdentity determines the identity of a cluster, in cases where the identity was
// created manually or the Seed was created as Shoot, and later registered as Seed and already has
// an identity, it should not be changed.
func determineClusterIdentity(ctx context.Context, c client.Client) (string, error) {
	clusterIdentity := &corev1.ConfigMap{}
	if err := c.Get(ctx, kutil.Key(metav1.NamespaceSystem, v1beta1constants.ClusterIdentity), clusterIdentity); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		gardenNamespace := &corev1.Namespace{}
		if err := c.Get(ctx, kutil.Key(metav1.NamespaceSystem), gardenNamespace); err != nil {
			return "", err
		}
		return string(gardenNamespace.UID), nil
	}
	return clusterIdentity.Data[v1beta1constants.ClusterIdentity], nil
}

func getDNSProviderSecretData(ctx context.Context, gardenClient client.Client, seed *gardencorev1beta1.Seed) (map[string][]byte, error) {
	if dnsConfig := seed.Spec.DNS; dnsConfig.Provider != nil {
		secret, err := kutil.GetSecretByReference(ctx, gardenClient, &dnsConfig.Provider.SecretRef)
		if err != nil {
			return nil, err
		}
		return secret.Data, nil
	}
	return nil, nil
}

func destroyDNSResources(ctx context.Context, dnsRecord component.DeployMigrateWaiter) error {
	if err := dnsRecord.Destroy(ctx); err != nil {
		return err
	}
	return dnsRecord.WaitCleanup(ctx)
}
