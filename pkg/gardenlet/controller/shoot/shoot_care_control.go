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

package shoot

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencore "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (c *Controller) shootCareAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}

	if shoot.Generation == shoot.Status.ObservedGeneration {
		// spread shoot health checks across sync period to avoid checking on all Shoots roughly at the same time
		// after startup of the gardenlet
		c.shootCareQueue.AddAfter(key, utils.RandomDurationWithMetaDuration(c.config.Controllers.ShootCare.SyncPeriod))
		return
	}

	// don't add random duration for enqueueing new Shoots, that have never been health checked
	c.shootCareQueue.Add(key)
}

func (c *Controller) reconcileShootCareKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	shoot, err := c.shootLister.Shoots(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Infof("[SHOOT CARE] Stopping care operations for Shoot %s since it has been deleted", key)
		c.shootCareQueue.Done(key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SHOOT CARE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	// if shoot has not been scheduled, requeue
	if shoot.Spec.SeedName == nil {
		return fmt.Errorf("shoot %s has not yet been scheduled on a Seed", key)
	}

	// if shoot is no longer managed by this gardenlet (e.g., due to migration to another seed) then don't requeue
	if seedName := confighelper.SeedNameFromSeedConfig(c.config.SeedConfig); (len(seedName) > 0 && *shoot.Spec.SeedName != seedName) || (len(seedName) == 0 && !controllerutils.SeedLabelsMatch(c.seedLister, *shoot.Spec.SeedName, c.config.SeedSelector)) {
		return nil
	}

	if err := c.careControl.Care(shoot, key); err != nil {
		return err
	}

	c.shootCareQueue.AddAfter(key, c.config.Controllers.ShootCare.SyncPeriod.Duration)
	return nil
}

// CareControlInterface implements the control logic for caring for Shoots. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type CareControlInterface interface {
	Care(shoot *gardencorev1beta1.Shoot, key string) error
}

// NewDefaultCareControl returns a new instance of the default implementation CareControlInterface that
// implements the documented semantics for caring for Shoots. You should use an instance returned from NewDefaultCareControl()
// for any scenario other than testing.
func NewDefaultCareControl(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.Interface, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, identity *gardencorev1beta1.Gardener, gardenClusterIdentity string, config *config.GardenletConfiguration) CareControlInterface {
	return &defaultCareControl{clientMap, k8sGardenCoreInformers, secrets, imageVector, identity, gardenClusterIdentity, config}
}

type defaultCareControl struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.Interface
	secrets                map[string]*corev1.Secret
	imageVector            imagevector.ImageVector
	identity               *gardencorev1beta1.Gardener
	gardenClusterIdentity  string
	config                 *config.GardenletConfiguration
}

func (c *defaultCareControl) conditionThresholdsToProgressingMapping() map[gardencorev1beta1.ConditionType]time.Duration {
	out := make(map[gardencorev1beta1.ConditionType]time.Duration)
	for _, threshold := range c.config.Controllers.ShootCare.ConditionThresholds {
		out[gardencorev1beta1.ConditionType(threshold.Type)] = threshold.Duration.Duration
	}
	return out
}

func shootClientInitializer(ctx context.Context, b *botanistpkg.Botanist) func() (bool, error) {
	var (
		once             sync.Once
		apiServerRunning bool
		err              error
	)
	return func() (bool, error) {
		once.Do(func() {
			// Don't initialize clients for Shoots, for which the API server is not running
			apiServerRunning, err = b.IsAPIServerRunning(ctx)
			if err != nil || !apiServerRunning {
				return
			}

			err = b.InitializeShootClients(ctx)

			// b.InitializeShootClients might not initialize b.K8sShootClient in case the Shoot is being hibernated
			// and the API server has just been scaled down. So, double-check if b.K8sShootClient is set/initialized,
			// otherwise we cannot execute health and constraint checks and garbage collection
			// This is done to prevent a race between the two calls to b.IsAPIServerRunning which would cause the care
			// controller to use a nil shoot client (panic)
			if b.K8sShootClient == nil {
				apiServerRunning = false
			}
		})
		return apiServerRunning, err
	}
}

func (c *defaultCareControl) Care(shootObj *gardencorev1beta1.Shoot, key string) error {
	var (
		shoot       = shootObj.DeepCopy()
		shootLogger = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace)
	)

	ctx, cancel := context.WithTimeout(context.Background(), c.config.Controllers.ShootCare.SyncPeriod.Duration)
	defer cancel()

	shootLogger.Debugf("[SHOOT CARE] %s", key)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	operation, err := operation.
		NewBuilder().
		WithLogger(shootLogger).
		WithConfig(c.config).
		WithGardenerInfo(c.identity).
		WithGardenClusterIdentity(c.gardenClusterIdentity).
		WithSecrets(c.secrets).
		WithImageVector(c.imageVector).
		WithGardenFrom(c.k8sGardenCoreInformers, shoot.Namespace).
		WithSeedFrom(c.k8sGardenCoreInformers, *shoot.Spec.SeedName).
		WithShootFrom(c.k8sGardenCoreInformers, shoot).
		Build(ctx, c.clientMap)
	if err != nil {
		shootLogger.Errorf("could not initialize a new operation: %s", err.Error())
		return nil // We do not want to run in the exponential backoff for the condition checks.
	}

	// Initialize conditions based on the current status.
	var (
		conditionAPIServerAvailable      = gardencorev1beta1helper.GetOrInitCondition(shoot.Status.Conditions, gardencorev1beta1.ShootAPIServerAvailable)
		conditionControlPlaneHealthy     = gardencorev1beta1helper.GetOrInitCondition(shoot.Status.Conditions, gardencorev1beta1.ShootControlPlaneHealthy)
		conditionEveryNodeReady          = gardencorev1beta1helper.GetOrInitCondition(shoot.Status.Conditions, gardencorev1beta1.ShootEveryNodeReady)
		conditionSystemComponentsHealthy = gardencorev1beta1helper.GetOrInitCondition(shoot.Status.Conditions, gardencorev1beta1.ShootSystemComponentsHealthy)

		seedConditions []gardencorev1beta1.Condition

		constraintHibernationPossible = gardencorev1beta1helper.GetOrInitCondition(shoot.Status.Constraints, gardencorev1beta1.ShootHibernationPossible)
	)

	botanist, err := botanistpkg.New(operation)
	if err != nil {
		message := fmt.Sprintf("Failed to create a botanist object to perform the care operations (%s).", err.Error())
		conditionAPIServerAvailable = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(conditionAPIServerAvailable, message)
		conditionControlPlaneHealthy = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(conditionControlPlaneHealthy, message)
		conditionEveryNodeReady = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(conditionEveryNodeReady, message)
		conditionSystemComponentsHealthy = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(conditionSystemComponentsHealthy, message)

		constraintHibernationPossible = gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(constraintHibernationPossible, message)

		operation.Logger.Error(message)

		_, _ = updateShootStatus(ctx, gardenClient.GardenCore(),
			shoot,
			[]gardencorev1beta1.Condition{
				conditionAPIServerAvailable,
				conditionControlPlaneHealthy,
				conditionEveryNodeReady,
				conditionSystemComponentsHealthy,
			},
			[]gardencorev1beta1.Condition{
				constraintHibernationPossible,
			},
		)

		return nil // We do not want to run in the exponential backoff for the condition checks.
	}

	initializeShootClients := shootClientInitializer(ctx, botanist)

	// Trigger garbage collection
	go garbageCollection(ctx, initializeShootClients, botanist)

	_ = flow.Parallel(
		// Trigger health check
		func(ctx context.Context) error {
			conditionAPIServerAvailable, conditionControlPlaneHealthy, conditionEveryNodeReady, conditionSystemComponentsHealthy = botanist.HealthChecks(
				ctx,
				initializeShootClients,
				c.conditionThresholdsToProgressingMapping(),
				c.config.Controllers.ShootCare.StaleExtensionHealthCheckThreshold,
				conditionAPIServerAvailable,
				conditionControlPlaneHealthy,
				conditionEveryNodeReady,
				conditionSystemComponentsHealthy,
			)
			return nil
		},
		// Fetch seed conditions if shoot is a seed
		func(ctx context.Context) error {
			seedConditions, err = retrieveSeedConditions(ctx, botanist)
			if err != nil {
				botanist.Logger.Errorf("Error retrieving seed conditions: %+v", err)
			}
			return nil
		},
		// Trigger constraint checks
		func(ctx context.Context) error {
			constraintHibernationPossible = botanist.ConstraintsChecks(ctx, initializeShootClients, constraintHibernationPossible)
			return nil
		},
	)(ctx)

	// Update Shoot status
	updatedShoot, err := updateShootStatus(ctx, gardenClient.GardenCore(),
		shoot,
		append(
			[]gardencorev1beta1.Condition{
				conditionAPIServerAvailable,
				conditionControlPlaneHealthy,
				conditionEveryNodeReady,
				conditionSystemComponentsHealthy,
			},
			seedConditions...,
		),
		[]gardencorev1beta1.Condition{
			constraintHibernationPossible,
		},
	)
	if err != nil {
		botanist.Logger.Errorf("Could not update Shoot status: %+v", err)
		return nil // We do not want to run in the exponential backoff for the condition checks.
	}

	// Mark Shoot as healthy/unhealthy
	_, err = kutil.TryUpdateShootLabels(
		ctx,
		gardenClient.GardenCore(),
		retry.DefaultBackoff,
		updatedShoot.ObjectMeta,
		StatusLabelTransform(
			ComputeStatus(
				updatedShoot.Status.LastOperation,
				updatedShoot.Status.LastErrors,
				conditionAPIServerAvailable,
				conditionControlPlaneHealthy,
				conditionEveryNodeReady,
				conditionSystemComponentsHealthy,
			),
		),
	)
	if err != nil {
		botanist.Logger.Errorf("Could not update Shoot health label: %+v", err)
		return nil // We do not want to run in the exponential backoff for the condition checks.
	}

	return nil // We do not want to run in the exponential backoff for the condition checks.
}

func updateShootStatus(ctx context.Context, g gardencore.Interface, shoot *gardencorev1beta1.Shoot, conditions, constraints []gardencorev1beta1.Condition) (*gardencorev1beta1.Shoot, error) {
	newShoot, err := kutil.TryUpdateShootStatus(ctx, g, retry.DefaultBackoff, shoot.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			shoot.Status.Conditions = conditions
			shoot.Status.Constraints = constraints
			return shoot, nil
		})

	return newShoot, err
}

// garbageCollection cleans the Seed and the Shoot cluster from no longer required
// objects. It receives a botanist object <botanist> which stores the Shoot object.
func garbageCollection(ctx context.Context, initShootClients func() (bool, error), botanist *botanistpkg.Botanist) {
	var (
		qualifiedShootName = fmt.Sprintf("%s/%s", botanist.Shoot.Info.Namespace, botanist.Shoot.Info.Name)
		wg                 sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := botanist.PerformGarbageCollectionSeed(ctx); err != nil {
			botanist.Logger.Errorf("Error during seed garbage collection: %+v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if apiServerRunning, err := initShootClients(); err != nil || !apiServerRunning {
			if err != nil {
				botanist.Logger.Errorf("Could not initialize Shoot client for garbage collection of shoot %s: %+v", qualifiedShootName, err)
			}
			return
		}
		if err := botanist.PerformGarbageCollectionShoot(ctx); err != nil {
			botanist.Logger.Errorf("Error during shoot garbage collection: %+v", err)
		}
	}()

	wg.Wait()
	botanist.Logger.Debugf("Successfully performed full garbage collection for Shoot cluster %s", qualifiedShootName)
}

func retrieveSeedConditions(ctx context.Context, botanist *botanistpkg.Botanist) ([]gardencorev1beta1.Condition, error) {
	if botanist.ShootedSeed == nil {
		return nil, nil
	}

	seed := &gardencorev1beta1.Seed{}
	if err := botanist.K8sGardenClient.Client().Get(ctx, kutil.Key(botanist.Shoot.Info.Name), seed); client.IgnoreNotFound(err) != nil {
		return nil, err
	}
	return seed.Status.Conditions, nil
}
