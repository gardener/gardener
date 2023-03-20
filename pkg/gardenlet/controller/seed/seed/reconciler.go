// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles Seed resources and provisions or de-provisions the seed system components.
type Reconciler struct {
	GardenClient                         client.Client
	SeedClientSet                        kubernetes.Interface
	Config                               config.GardenletConfiguration
	Clock                                clock.Clock
	Recorder                             record.EventRecorder
	Identity                             *gardencorev1beta1.Gardener
	ImageVector                          imagevector.ImageVector
	ComponentImageVectors                imagevector.ComponentImageVectors
	ClientCertificateExpirationTimestamp *metav1.Time
	GardenNamespace                      string
	ChartsPath                           string
}

// Reconcile reconciles Seed resources and provisions or de-provisions the seed system components.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	gardenCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.Controllers.Seed.SyncPeriod.Duration)
	defer cancel()
	seedCtx, cancel := controllerutils.GetChildReconciliationContext(ctx, r.Config.Controllers.Seed.SyncPeriod.Duration)
	defer cancel()

	seed := &gardencorev1beta1.Seed{}
	if err := r.GardenClient.Get(gardenCtx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// Check if seed namespace is already available.
	if err := r.GardenClient.Get(gardenCtx, client.ObjectKey{Name: gardenerutils.ComputeGardenNamespace(seed.Name)}, &corev1.Namespace{}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed namespace in garden cluster: %w", err)
	}

	seedObj, err := seedpkg.NewBuilder().WithSeedObject(seed).Build(gardenCtx)
	if err != nil {
		log.Error(err, "Failed to create a Seed object")
		conditionSeedBootstrapped := v1beta1helper.GetOrInitConditionWithClock(r.Clock, seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped)
		conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionUnknown, gardencorev1beta1.ConditionCheckError, fmt.Sprintf("Failed to create a Seed object (%s).", err.Error()))
		if err := r.patchSeedStatus(gardenCtx, r.GardenClient, seed, "<unknown>", nil, nil, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after failed creation of Seed object: %w", err)
		}
		return reconcile.Result{}, err
	}

	if seed.Status.ClusterIdentity == nil {
		seedClusterIdentity, err := determineClusterIdentity(seedCtx, r.SeedClientSet.Client())
		if err != nil {
			return reconcile.Result{}, err
		}

		log.Info("Setting cluster identity", "identity", seedClusterIdentity)
		seed.Status.ClusterIdentity = &seedClusterIdentity
		if err := r.GardenClient.Status().Update(seedCtx, seed); err != nil {
			return reconcile.Result{}, err
		}
	}

	seedIsGarden, err := kubernetesutils.ResourcesExist(seedCtx, r.SeedClientSet.Client(), operatorv1alpha1.SchemeGroupVersion.WithKind("GardenList"))
	if err != nil {
		if !meta.IsNoMatchError(err) {
			return reconcile.Result{}, err
		}
		seedIsGarden = false
	}

	if seed.DeletionTimestamp != nil {
		return r.delete(gardenCtx, seedCtx, log, seedObj, seedIsGarden)
	}

	return r.reconcile(gardenCtx, seedCtx, log, seedObj, seedIsGarden)
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

	seed.Status.Conditions = v1beta1helper.MergeConditions(seed.Status.Conditions, updateConditions...)
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

// determineClusterIdentity determines the identity of a cluster, in cases where the identity was
// created manually or the Seed was created as Shoot, and later registered as Seed and already has
// an identity, it should not be changed.
func determineClusterIdentity(ctx context.Context, c client.Client) (string, error) {
	clusterIdentity := &corev1.ConfigMap{}
	if err := c.Get(ctx, kubernetesutils.Key(metav1.NamespaceSystem, v1beta1constants.ClusterIdentity), clusterIdentity); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		gardenNamespace := &corev1.Namespace{}
		if err := c.Get(ctx, kubernetesutils.Key(metav1.NamespaceSystem), gardenNamespace); err != nil {
			return "", err
		}
		return string(gardenNamespace.UID), nil
	}
	return clusterIdentity.Data[v1beta1constants.ClusterIdentity], nil
}

func getDNSProviderSecretData(ctx context.Context, gardenClient client.Client, seed *gardencorev1beta1.Seed) (map[string][]byte, error) {
	if dnsConfig := seed.Spec.DNS; dnsConfig.Provider != nil {
		secret, err := kubernetesutils.GetSecretByReference(ctx, gardenClient, &dnsConfig.Provider.SecretRef)
		if err != nil {
			return nil, err
		}
		return secret.Data, nil
	}
	return nil, nil
}

func deployDNSResources(ctx context.Context, dnsRecord component.DeployMigrateWaiter) error {
	if err := dnsRecord.Deploy(ctx); err != nil {
		return err
	}
	return dnsRecord.Wait(ctx)
}

func destroyDNSResources(ctx context.Context, dnsRecord component.DeployMigrateWaiter) error {
	if err := dnsRecord.Destroy(ctx); err != nil {
		return err
	}
	return dnsRecord.WaitCleanup(ctx)
}
