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

package cloudprofile

import (
	"context"
	"errors"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *Controller) cloudProfileAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.cloudProfileQueue.Add(key)
}

func (c *Controller) cloudProfileUpdate(oldObj, newObj interface{}) {
	c.cloudProfileAdd(newObj)
}

func (c *Controller) cloudProfileDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.cloudProfileQueue.Add(key)
}

func (c *Controller) reconcileCloudProfileKey(key string) error {
	_, cloudProfileName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	cloudProfile, err := c.cloudProfileLister.Get(cloudProfileName)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[CLOUDPROFILE RECONCILE] %s - skipping because CloudProfile has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[CLOUDPROFILE RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.control.ReconcileCloudProfile(cloudProfile, key); err != nil {
		c.cloudProfileQueue.AddAfter(key, 15*time.Second)
	}
	return nil
}

// ControlInterface implements the control logic for reconciling CloudProfiles. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	// ReconcileCloudProfile implements the control logic for CloudProfile creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	ReconcileCloudProfile(cloudprofile *gardencorev1beta1.CloudProfile, key string) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for CloudProfiles.
func NewDefaultControl(clientMap clientmap.ClientMap, shootLister gardencorelisters.ShootLister, recorder record.EventRecorder) ControlInterface {
	return &defaultControl{clientMap, shootLister, recorder}
}

type defaultControl struct {
	clientMap   clientmap.ClientMap
	shootLister gardencorelisters.ShootLister
	recorder    record.EventRecorder
}

func (c *defaultControl) ReconcileCloudProfile(obj *gardencorev1beta1.CloudProfile, key string) error {
	_, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}

	var (
		ctx                = context.TODO()
		cloudProfile       = obj.DeepCopy()
		cloudProfileLogger = logger.NewFieldLogger(logger.Logger, "cloudprofile", cloudProfile.Name)
	)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	// The deletionTimestamp labels the CloudProfile as intended to get deleted. Before deletion, it has to be ensured that
	// no Shoots and Seed are assigned to the CloudProfile anymore. If this is the case then the controller will remove
	// the finalizers from the CloudProfile so that it can be garbage collected.
	if cloudProfile.DeletionTimestamp != nil {
		if !sets.NewString(cloudProfile.Finalizers...).Has(gardencorev1beta1.GardenerName) {
			return nil
		}

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(cloudProfile, c.shootLister)
		if err != nil {
			cloudProfileLogger.Error(err.Error())
			return err
		}

		if len(associatedShoots) == 0 {
			cloudProfileLogger.Infof("No Shoots are referencing the CloudProfile. Deletion accepted.")

			if err := controllerutils.PatchRemoveFinalizers(ctx, gardenClient.Client(), cloudProfile, gardencorev1beta1.GardenerName); client.IgnoreNotFound(err) != nil {
				logger.Logger.Errorf("could not remove finalizer from CloudProfile: %s", err.Error())
				return err
			}
			return nil
		}

		message := fmt.Sprintf("Can't delete CloudProfile, because the following Shoots are still referencing it: %+v", associatedShoots)
		cloudProfileLogger.Info(message)
		c.recorder.Event(cloudProfile, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)

		return errors.New("CloudProfile still has references")
	}

	if err := controllerutils.PatchFinalizers(ctx, gardenClient.Client(), cloudProfile, gardencorev1beta1.GardenerName); err != nil {
		logger.Logger.Errorf("could not add finalizer to CloudProfile: %s", err.Error())
		return err
	}

	return nil
}
