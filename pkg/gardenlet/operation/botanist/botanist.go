// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// DefaultInterval is the default interval for retry operations.
const DefaultInterval = 5 * time.Second

// New takes an operation object <o> and creates a new Botanist object.
func New(ctx context.Context, o *operation.Operation) (*Botanist, error) {
	var (
		b   = &Botanist{Operation: o}
		err error

		secretsManagerIdentity = v1beta1constants.SecretManagerIdentityGardenlet
		namespaces             = []string{b.Shoot.ControlPlaneNamespace}
	)

	if o.Shoot.IsSelfHosted() {
		// `gardenadm init` will generate secrets for gardener-resource-manager and etcd-druid in the `garden` namespace
		// with the `self-hosted-shoot` identity. Later, when `gardener-operator` or the seed `gardenlet` will take over
		// management of those shared components, they will generate new secrets with their identity and thereby orphan
		// the existing ones. When the shoot `gardenlet` reconciles the self-hosted shoot, calling the secrets manager
		// `Cleanup` will delete the orphaned secrets from the `garden` namespace.
		secretsManagerIdentity = v1beta1constants.SecretManagerIdentitySelfHostedShoot
		namespaces = append(namespaces, v1beta1constants.GardenNamespace)
	}

	o.SecretsManager, err = secretsmanager.New(
		ctx,
		b.Logger.WithName("secretsmanager"),
		clock.RealClock{},
		b.SeedClientSet.Client(),
		secretsManagerIdentity,
		secretsmanager.WithSecretNamesToTimes(b.lastSecretRotationStartTimes()),
		secretsmanager.WithNamespaces(namespaces...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate secrets manager: %w", err)
	}

	if err := b.instantiateComponents(ctx); err != nil {
		return nil, fmt.Errorf("failed to instantiate components: %w", err)
	}

	return b, nil
}

// IsGardenerResourceManagerReady checks if gardener-resource-manager has ready replicas.
func (b *Botanist) IsGardenerResourceManagerReady(ctx context.Context) (bool, error) {
	resourceManagerDeployment := &appsv1.Deployment{}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Name: v1beta1constants.DeploymentNameGardenerResourceManager, Namespace: b.Shoot.ControlPlaneNamespace}, resourceManagerDeployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}

	return resourceManagerDeployment.Status.ReadyReplicas > 0, nil
}

// RequiredExtensionsReady checks whether all required extensions needed for a shoot operation exist and are ready.
func (b *Botanist) RequiredExtensionsReady(ctx context.Context) error {
	controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
	if err := b.GardenClient.List(ctx, controllerRegistrationList); err != nil {
		return err
	}
	requiredExtensions := gardenerutils.ComputeRequiredExtensionsForShoot(b.Shoot.GetInfo(), b.Seed.GetInfo(), controllerRegistrationList, b.Garden.InternalDomain, b.Shoot.ExternalDomain)

	return gardenerutils.RequiredExtensionsReady(ctx, b.GardenClient, b.Seed.GetInfo().Name, requiredExtensions)
}

// outOfClusterAPIServerFQDN returns the Fully Qualified Domain Name of the apiserver
// with dot "." suffix. It'll prevent extra requests to the DNS in case the record is not
// available.
func (b *Botanist) outOfClusterAPIServerFQDN() string {
	return fmt.Sprintf("%s.", b.Shoot.ComputeOutOfClusterAPIServerAddress(true))
}

// SetInPlaceUpdatePendingWorkers sets the Shoot status with the name of worker pools which are undergoing an in-place update.
func (b *Botanist) SetInPlaceUpdatePendingWorkers(ctx context.Context, worker *extensionsv1alpha1.Worker) error {
	var (
		autoInPlaceUpdatePendingWorkers   []string
		manualInPlaceUpdatePendingWorkers []string
	)

	for _, pool := range b.Shoot.GetInfo().Spec.Provider.Workers {
		if !v1beta1helper.IsUpdateStrategyInPlace(pool.UpdateStrategy) {
			continue
		}

		if worker != nil {
			var oldPool extensionsv1alpha1.WorkerPool
			oldPoolIndex := slices.IndexFunc(worker.Spec.Pools, func(ow extensionsv1alpha1.WorkerPool) bool {
				oldPool = ow
				return ow.Name == pool.Name
			})

			if oldPoolIndex != -1 && worker.Status.InPlaceUpdates != nil && worker.Status.InPlaceUpdates.WorkerPoolToHashMap != nil {
				if oldPoolHash, ok := worker.Status.InPlaceUpdates.WorkerPoolToHashMap[oldPool.Name]; ok {
					var (
						kubernetesVersion    = b.Shoot.GetInfo().Spec.Kubernetes.Version
						kubeletConfiguration = b.Shoot.GetInfo().Spec.Kubernetes.Kubelet
					)

					if pool.Kubernetes != nil {
						if pool.Kubernetes.Version != nil {
							kubernetesVersion = *pool.Kubernetes.Version
						}

						if pool.Kubernetes.Kubelet != nil {
							kubeletConfiguration = pool.Kubernetes.Kubelet
						}
					}

					newPoolHash, err := gardenerutils.CalculateWorkerPoolHashForInPlaceUpdate(
						pool.Name,
						&kubernetesVersion,
						kubeletConfiguration,
						ptr.Deref(pool.Machine.Image.Version, ""),
						b.Shoot.GetInfo().Status.Credentials,
					)
					if err != nil {
						return fmt.Errorf("failed to calculate worker pool %q hash: %w", pool.Name, err)
					}

					if oldPoolHash == newPoolHash {
						continue
					}
				}
			}
		}

		switch ptr.Deref(pool.UpdateStrategy, "") {
		case gardencorev1beta1.AutoInPlaceUpdate:
			autoInPlaceUpdatePendingWorkers = append(autoInPlaceUpdatePendingWorkers, pool.Name)
		case gardencorev1beta1.ManualInPlaceUpdate:
			manualInPlaceUpdatePendingWorkers = append(manualInPlaceUpdatePendingWorkers, pool.Name)
		}
	}

	if len(autoInPlaceUpdatePendingWorkers) == 0 && len(manualInPlaceUpdatePendingWorkers) == 0 {
		return nil
	}

	if b.Shoot.GetInfo().Status.InPlaceUpdates != nil &&
		b.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates != nil &&
		sets.New(autoInPlaceUpdatePendingWorkers...).Equal(sets.New(b.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate...)) &&
		sets.New(manualInPlaceUpdatePendingWorkers...).Equal(sets.New(b.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate...)) {
		return nil
	}

	return b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, false, true, func(shoot *gardencorev1beta1.Shoot) error {
		if shoot.Status.InPlaceUpdates == nil {
			shoot.Status.InPlaceUpdates = &gardencorev1beta1.InPlaceUpdatesStatus{}
		}

		if shoot.Status.InPlaceUpdates.PendingWorkerUpdates == nil {
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates = &gardencorev1beta1.PendingWorkerUpdates{}
		}

		for _, poolName := range autoInPlaceUpdatePendingWorkers {
			if slices.Contains(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate, poolName) {
				continue
			}
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate = append(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate, poolName)
		}

		for _, poolName := range manualInPlaceUpdatePendingWorkers {
			if slices.Contains(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate, poolName) {
				continue
			}
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate = append(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate, poolName)
		}

		return nil
	})
}
