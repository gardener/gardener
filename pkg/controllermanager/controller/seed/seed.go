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
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "seed-controller"

	backupBucketQueue  = "backupbucket"
	seedLifecycleQueue = "seed-lifecycle"
	seedQueue          = "seed"
)

// AddToManager adds a new seed controller to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	config *config.SeedControllerConfiguration,
) error {
	logger := mgr.GetLogger()
	gardenClient := mgr.GetClient()

	reconciler := controllerutils.NewMultiplexReconciler(map[string]reconcile.Reconciler{
		seedQueue:          NewDefaultControl(logger, gardenClient),
		seedLifecycleQueue: NewLifecycleDefaultControl(logger, gardenClient, config),
		backupBucketQueue:  NewDefaultBackupBucketControl(logger, gardenClient),
	})

	ctrlOptions := controller.Options{
		Reconciler:              reconciler,
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(ControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	backupBucket := &gardencorev1beta1.BackupBucket{}
	if err := c.Watch(&source.Kind{Type: backupBucket}, newBackupBucketEventHandler(reconciler)); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", backupBucket, err)
	}

	secret := &corev1.Secret{}
	if err := c.Watch(&source.Kind{Type: secret}, newSecretEventHandler(ctx, gardenClient, logger, reconciler)); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", secret, err)
	}

	seed := &gardencorev1beta1.Seed{}
	if err := c.Watch(&source.Kind{Type: seed}, newSeedEventHandler(reconciler)); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", seed, err)
	}

	return nil
}

func reconcileAfter(d time.Duration) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: d}, nil
}
