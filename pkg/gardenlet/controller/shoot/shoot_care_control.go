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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

func (c *Controller) shootCareUpdate(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		return
	}

	oldShoot, ok := oldObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}
	newShoot, ok := newObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}

	// re-evaluate shoot health status right after a reconciliation operation has finished
	if shootReconciliationFinishedSuccessful(oldShoot, newShoot) {
		c.shootCareQueue.Add(key)
	}
}

func shootReconciliationFinishedSuccessful(oldShoot, newShoot *gardencorev1beta1.Shoot) bool {
	return oldShoot.Status.LastOperation != nil &&
		oldShoot.Status.LastOperation.Type != gardencorev1beta1.LastOperationTypeDelete &&
		oldShoot.Status.LastOperation.State == gardencorev1beta1.LastOperationStateProcessing &&
		newShoot.Status.LastOperation != nil &&
		newShoot.Status.LastOperation.Type != gardencorev1beta1.LastOperationTypeDelete &&
		newShoot.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded
}

// NewCareReconciler returns an implementation of reconcile.Reconciler which is dedicated to execute care operations
// on shoots, e.g., health checks or garbage collection.
func NewCareReconciler(
	clientMap clientmap.ClientMap,
	l logrus.FieldLogger,
	imageVector imagevector.ImageVector,
	identity *gardencorev1beta1.Gardener,
	gardenClusterIdentity string,
	config *config.GardenletConfiguration,
) reconcile.Reconciler {
	return &careReconciler{
		clientMap:             clientMap,
		logger:                l,
		imageVector:           imageVector,
		identity:              identity,
		gardenClusterIdentity: gardenClusterIdentity,
		config:                config,
	}
}

type careReconciler struct {
	clientMap             clientmap.ClientMap
	logger                logrus.FieldLogger
	imageVector           imagevector.ImageVector
	identity              *gardencorev1beta1.Gardener
	gardenClusterIdentity string
	config                *config.GardenletConfiguration

	gardenSecrets map[string]*corev1.Secret
}

func (r *careReconciler) conditionThresholdsToProgressingMapping() map[gardencorev1beta1.ConditionType]time.Duration {
	out := make(map[gardencorev1beta1.ConditionType]time.Duration)
	for _, threshold := range r.config.Controllers.ShootCare.ConditionThresholds {
		out[gardencorev1beta1.ConditionType(threshold.Type)] = threshold.Duration.Duration
	}
	return out
}

func shootClientInitializer(ctx context.Context, o *operation.Operation) func() (kubernetes.Interface, bool, error) {
	var (
		once             sync.Once
		apiServerRunning bool
		err              error
	)
	return func() (kubernetes.Interface, bool, error) {
		once.Do(func() {
			// Don't initialize clients for Shoots, for which the API server is not running
			apiServerRunning, err = o.IsAPIServerRunning(ctx)
			if err != nil || !apiServerRunning {
				return
			}

			err = o.InitializeShootClients(ctx)

			// b.InitializeShootClients might not initialize b.K8sShootClient in case the Shoot is being hibernated
			// and the API server has just been scaled down. So, double-check if b.K8sShootClient is set/initialized,
			// otherwise we cannot execute health and constraint checks and garbage collection
			// This is done to prevent a race between the two calls to b.IsAPIServerRunning which would cause the care
			// controller to use a nil shoot client (panic)
			if o.K8sShootClient == nil {
				apiServerRunning = false
			}
		})
		return o.K8sShootClient, apiServerRunning, err
	}
}

func careSetupFailure(ctx context.Context, gardenClient client.Client, shoot *gardencorev1beta1.Shoot, message string, conditions, constraints []gardencorev1beta1.Condition) error {
	updatedConditions := make([]gardencorev1beta1.Condition, 0, len(conditions))
	for _, cond := range conditions {
		updatedConditions = append(updatedConditions, gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(cond, message))
	}

	updatedConstraints := make([]gardencorev1beta1.Condition, 0, len(constraints))
	for _, constr := range constraints {
		updatedConstraints = append(updatedConstraints, gardencorev1beta1helper.UpdatedConditionUnknownErrorMessage(constr, message))
	}

	if !gardencorev1beta1helper.ConditionsNeedUpdate(conditions, updatedConditions) &&
		!gardencorev1beta1helper.ConditionsNeedUpdate(constraints, updatedConstraints) {
		return nil
	}

	return patchShootStatus(ctx, gardenClient, shoot, updatedConditions, updatedConstraints)
}

func (r *careReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.logger.WithField("shoot", req.String())

	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := gardenClient.Client().Get(ctx, req.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.Infof("[SHOOT CARE] Stopping care operations for Shoot since it has been deleted")
			return reconcile.Result{}, nil
		}
		log.Infof("[SHOOT CARE] unable to retrieve object from store: %+v", err)
		return reconcile.Result{}, err
	}

	// if shoot has not been scheduled, requeue
	if shoot.Spec.SeedName == nil {
		return reconcile.Result{}, fmt.Errorf("shoot %s/%s has not yet been scheduled on a Seed", req.Namespace, req.Name)
	}

	// if shoot is no longer managed by this gardenlet (e.g., due to migration to another seed) then don't requeue
	if !controllerutils.ShootIsManagedByThisGardenlet(shoot, r.config) {
		return reconcile.Result{}, nil
	}

	if err := r.care(ctx, gardenClient, shoot, log); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.config.Controllers.ShootCare.SyncPeriod.Duration}, nil
}

var (
	// NewOperation is used to create a new `operation.Operation` instance.
	NewOperation = defaultNewOperationFunc
	// NewHealthCheck is used to create a new Health check instance.
	NewHealthCheck = defaultNewHealthCheck
	// NewConstraintCheck is used to create a new Constraint check instance.
	NewConstraintCheck = defaultNewConstraintCheck
	// NewGarbageCollector is used to create a new Constraint check instance.
	NewGarbageCollector = defaultNewGarbageCollector
)

func (r *careReconciler) care(ctx context.Context, gardenClientSet kubernetes.Interface, shoot *gardencorev1beta1.Shoot, log logrus.FieldLogger) error {
	careCtx, cancel := context.WithTimeout(ctx, r.config.Controllers.ShootCare.SyncPeriod.Duration)
	defer cancel()

	gardenClient := gardenClientSet.Client()
	log.Debugf("[SHOOT CARE]")

	// Initialize conditions based on the current status.
	var conditions []gardencorev1beta1.Condition
	for _, cond := range []gardencorev1beta1.ConditionType{
		gardencorev1beta1.ShootAPIServerAvailable,
		gardencorev1beta1.ShootControlPlaneHealthy,
		gardencorev1beta1.ShootEveryNodeReady,
		gardencorev1beta1.ShootSystemComponentsHealthy,
	} {
		conditions = append(conditions, gardencorev1beta1helper.GetOrInitCondition(shoot.Status.Conditions, cond))
	}

	// Initialize constraints
	var constraints []gardencorev1beta1.Condition
	for _, constr := range []gardencorev1beta1.ConditionType{
		gardencorev1beta1.ShootHibernationPossible,
		gardencorev1beta1.ShootMaintenancePreconditionsSatisfied,
	} {
		constraints = append(constraints, gardencorev1beta1helper.GetOrInitCondition(shoot.Status.Constraints, constr))
	}

	seedClient, err := r.clientMap.GetClient(careCtx, keys.ForSeedWithName(*shoot.Spec.SeedName))
	if err != nil {
		log.Errorf("seedClient cannot be constructed: %s", err.Error())

		if err := careSetupFailure(ctx, gardenClient, shoot, "Precondition failed: seed client cannot be constructed", conditions, constraints); err != nil {
			log.Error(err)
		}
		return nil
	}

	// Only read Garden secrets once because we don't rely on up-to-date secrets for health checks.
	if r.gardenSecrets == nil {
		secrets, err := garden.ReadGardenSecrets(careCtx, gardenClient, gutil.ComputeGardenNamespace(*shoot.Spec.SeedName), log)
		if err != nil {
			return fmt.Errorf("error reading Garden secrets: %w", err)
		}
		r.gardenSecrets = secrets
	}

	operation, err := NewOperation(
		careCtx,
		gardenClientSet,
		seedClient,
		r.config,
		r.identity,
		r.gardenClusterIdentity,
		r.gardenSecrets,
		r.imageVector,
		r.clientMap,
		shoot,
		log,
	)
	if err != nil {
		log.Errorf("could not initialize a new operation: %s", err.Error())

		if err := careSetupFailure(ctx, gardenClient, shoot, "Precondition failed: operation could not be initialized", conditions, constraints); err != nil {
			log.Error(err)
		}
		return nil // We do not want to run in the exponential backoff for the condition checks.
	}

	if err := operation.InitializeSeedClients(careCtx); err != nil {
		log.Errorf("Health checks cannot be performed: %s", err.Error())

		if err := careSetupFailure(ctx, gardenClient, shoot, "Precondition failed: seed client cannot be constructed", conditions, constraints); err != nil {
			log.Error(err)
		}
		return nil
	}

	staleExtensionHealthCheckThreshold := confighelper.StaleExtensionHealthChecksThreshold(r.config.Controllers.ShootCare.StaleExtensionHealthChecks)
	initializeShootClients := shootClientInitializer(careCtx, operation)

	var updatedConditions, updatedConstraints, seedConditions []gardencorev1beta1.Condition

	_ = flow.Parallel(
		// Trigger health check
		func(ctx context.Context) error {
			shootHealth := NewHealthCheck(operation, initializeShootClients)
			updatedConditions = shootHealth.Check(
				ctx,
				r.conditionThresholdsToProgressingMapping(),
				staleExtensionHealthCheckThreshold,
				conditions,
			)
			return nil
		},
		// Fetch seed conditions if shoot is a seed
		// TODO This logic could be moved to the managed seed controller.
		// It should watch Seed objects and enqueue them if they belong to a ManagedSeed and the conditions have changed.
		// Then it should update the conditions on the Shoot object.
		func(ctx context.Context) error {
			seedConditions, err = retrieveSeedConditions(ctx, operation)
			if err != nil {
				operation.Logger.Errorf("Error retrieving seed conditions: %+v", err)
			}
			return nil
		},
		// Trigger constraint checks
		func(ctx context.Context) error {
			constraint := NewConstraintCheck(operation, initializeShootClients)
			updatedConstraints = constraint.Check(
				ctx,
				constraints,
			)
			return nil
		},
		// Trigger garbage collection
		func(ctx context.Context) error {
			garbageCollector := NewGarbageCollector(operation, initializeShootClients)
			garbageCollector.Collect(ctx)
			// errors during garbage collection are only being logged and do not cause the care operation to fail
			return nil
		},
	)(careCtx)

	updatedConditions = append(updatedConditions, seedConditions...)

	// Update Shoot status if necessary
	if gardencorev1beta1helper.ConditionsNeedUpdate(conditions, updatedConditions) ||
		gardencorev1beta1helper.ConditionsNeedUpdate(constraints, updatedConstraints) {
		if err := patchShootStatus(ctx, gardenClient, shoot, updatedConditions, updatedConstraints); err != nil {
			operation.Logger.Errorf("Could not update Shoot status: %+v", err)
			return nil // We do not want to run in the exponential backoff for the condition checks.
		}
	}

	// Mark Shoot as healthy/unhealthy
	metaPatch := client.MergeFrom(shoot.DeepCopy())
	kutil.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.ShootStatus, string(shootpkg.ComputeStatus(
		shoot.Status.LastOperation,
		shoot.Status.LastErrors,
		updatedConditions...,
	)))
	if err := gardenClient.Patch(ctx, shoot, metaPatch); err != nil {
		operation.Logger.Errorf("Could not update Shoot health label: %+v", err)
		return nil // We do not want to run in the exponential backoff for the condition checks.
	}

	return nil
}

func patchShootStatus(ctx context.Context, c client.StatusClient, shoot *gardencorev1beta1.Shoot, conditions, constraints []gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(shoot.DeepCopy())
	shoot.Status.Conditions = conditions
	shoot.Status.Constraints = constraints
	return c.Status().Patch(ctx, shoot, patch)
}

func retrieveSeedConditions(ctx context.Context, operation *operation.Operation) ([]gardencorev1beta1.Condition, error) {
	if operation.ManagedSeed == nil {
		return nil, nil
	}

	seed := &gardencorev1beta1.Seed{}
	if err := operation.K8sGardenClient.Client().Get(ctx, kutil.Key(operation.Shoot.GetInfo().Name), seed); client.IgnoreNotFound(err) != nil {
		return nil, err
	}
	return seed.Status.Conditions, nil
}
