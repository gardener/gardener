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
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencore "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	coordinationlister "k8s.io/client-go/listers/coordination/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *Controller) seedAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
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
		logger.Logger.Infof("[SEED LIFECYCLE] Stopping lifecycle operations for Seed %s since it has been deleted", key)
		c.seedQueue.Done(key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SEED LIFECYCLE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	fastRequeue, err := c.control.Reconcile(seed)
	if err != nil {
		return err
	}

	if fastRequeue {
		c.seedQueue.AddAfter(key, 10*time.Second)
	} else {
		c.seedQueue.AddAfter(key, time.Minute)
	}

	return nil
}

// ControlInterface implements the control logic for managing the lifecycle for Seeds. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	Reconcile(seed *gardencorev1beta1.Seed) (bool, error)
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for checking the lifecycle for Seeds. You should use an instance returned from NewDefaultControl()
// for any scenario other than testing.
func NewDefaultControl(clientMap clientmap.ClientMap, leaseLister coordinationlister.LeaseLister, shootLister gardencorelisters.ShootLister, config *config.ControllerManagerConfiguration) ControlInterface {
	return &defaultControl{clientMap, leaseLister, shootLister, config}
}

type defaultControl struct {
	clientMap   clientmap.ClientMap
	leaseLister coordinationlister.LeaseLister
	shootLister gardencorelisters.ShootLister
	config      *config.ControllerManagerConfiguration
}

func (c *defaultControl) Reconcile(seedObj *gardencorev1beta1.Seed) (fastRequeue bool, err error) {
	var (
		ctx        = context.TODO()
		seed       = seedObj.DeepCopy()
		seedLogger = logger.NewFieldLogger(logger.Logger, "seed", seed.Name)
	)

	// New seeds don't have conditions - gardenlet never reported anything yet. Wait for grace period.
	if len(seed.Status.Conditions) == 0 {
		return true, nil
	}

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return false, fmt.Errorf("failed to get garden client: %w", err)
	}

	observedSeedLease, err := c.leaseLister.Leases(gardencorev1beta1.GardenerSeedLeaseNamespace).Get(seed.Name)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}

	if observedSeedLease != nil && observedSeedLease.Spec.RenewTime != nil {
		if observedSeedLease.Spec.RenewTime.UTC().After(time.Now().UTC().Add(-c.config.Controllers.Seed.MonitorPeriod.Duration)) {
			return true, nil
		}

		// Get the latest Lease object in cases which the LeaseLister cache is outdated, to ensure that the lease is really expired
		latestLeaseObject := &coordinationv1.Lease{}
		if err := gardenClient.Client().Get(ctx, kutil.Key(gardencorev1beta1.GardenerSeedLeaseNamespace, seed.Name), latestLeaseObject); err != nil {
			return false, err
		}

		if latestLeaseObject.Spec.RenewTime.UTC().After(time.Now().UTC().Add(-c.config.Controllers.Seed.MonitorPeriod.Duration)) {
			return true, nil
		}
	}

	seedLogger.Debugf("Setting status for seed %q to 'Unknown' as gardenlet stopped reporting seed status.", seed.Name)

	bldr, err := gardencorev1beta1helper.NewConditionBuilder(gardencorev1beta1.SeedGardenletReady)
	if err != nil {
		return false, err
	}

	conditionGardenletReady := gardencorev1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedGardenletReady)
	if conditionGardenletReady != nil {
		bldr.WithOldCondition(*conditionGardenletReady)
	}

	bldr.WithStatus(gardencorev1beta1.ConditionUnknown)
	bldr.WithReason("SeedStatusUnknown")
	bldr.WithMessage("Gardenlet stopped posting seed status.")
	if newCondition, update := bldr.WithNowFunc(metav1.Now).Build(); update {
		seed.Status.Conditions = gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, newCondition)
		if err := gardenClient.Client().Status().Update(ctx, seed); err != nil {
			return false, err
		}
	}

	// If the `GardenletReady` condition is `Unknown` for at least the configured `shootMonitorPeriod` then we will mark the conditions
	// and constraints for all the shoots that belong to this seed as `Unknown`. The reason is that the gardenlet didn't send a heartbeat
	// anymore, hence, it most likely didn't check the shoot status. This means that the current shoot status might not reflect the truth
	// anymore. We are indicating this by marking it as `Unknown`.
	if conditionGardenletReady != nil && !conditionGardenletReady.LastTransitionTime.UTC().Before(time.Now().UTC().Add(-c.config.Controllers.Seed.ShootMonitorPeriod.Duration)) {
		return true, nil
	}

	seedLogger.Debugf("Gardenlet didn't send a heartbeat for at least %s - setting the shoot conditions/constraints to 'unknown' for all shoots on this seed", c.config.Controllers.Seed.ShootMonitorPeriod.Duration)

	shootList, err := c.shootLister.List(labels.Everything())
	if err != nil {
		return false, err
	}

	var fns []flow.TaskFn

	for _, shoot := range shootList {
		if shoot.Spec.SeedName == nil || *shoot.Spec.SeedName != seed.Name {
			continue
		}

		fns = append(fns, func(ctx context.Context) error {
			return setShootStatusToUnknown(ctx, gardenClient.GardenCore(), shoot)
		})
	}

	if err := flow.Parallel(fns...)(ctx); err != nil {
		return false, err
	}

	return false, nil
}

func setShootStatusToUnknown(ctx context.Context, g gardencore.Interface, shoot *gardencorev1beta1.Shoot) error {
	var (
		reason = "StatusUnknown"
		msg    = "Gardenlet stopped sending heartbeats."

		conditions = map[gardencorev1beta1.ConditionType]gardencorev1beta1.Condition{
			gardencorev1beta1.ShootAPIServerAvailable:      {},
			gardencorev1beta1.ShootControlPlaneHealthy:     {},
			gardencorev1beta1.ShootEveryNodeReady:          {},
			gardencorev1beta1.ShootSystemComponentsHealthy: {},
		}

		constraints = map[gardencorev1beta1.ConditionType]gardencorev1beta1.Condition{
			gardencorev1beta1.ShootHibernationPossible: {},
		}
	)

	for conditionType := range conditions {
		c := gardencorev1beta1helper.GetOrInitCondition(shoot.Status.Conditions, conditionType)
		c = gardencorev1beta1helper.UpdatedCondition(c, gardencorev1beta1.ConditionUnknown, reason, msg)
		conditions[conditionType] = c
	}

	for conditionType := range constraints {
		c := gardencorev1beta1helper.GetOrInitCondition(shoot.Status.Constraints, conditionType)
		c = gardencorev1beta1helper.UpdatedCondition(c, gardencorev1beta1.ConditionUnknown, reason, msg)
		constraints[conditionType] = c
	}

	_, err := kutil.TryUpdateShootStatus(ctx, g, retry.DefaultBackoff, shoot.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			shoot.Status.Conditions = gardencorev1beta1helper.MergeConditions(shoot.Status.Conditions, conditionMapToConditions(conditions)...)
			shoot.Status.Constraints = gardencorev1beta1helper.MergeConditions(shoot.Status.Constraints, conditionMapToConditions(constraints)...)
			return shoot, nil
		},
	)
	return err
}

func conditionMapToConditions(m map[gardencorev1beta1.ConditionType]gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	output := make([]gardencorev1beta1.Condition, 0, len(m))

	for _, condition := range m {
		output = append(output, condition)
	}

	return output
}
