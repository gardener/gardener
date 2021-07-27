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

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AddToManager adds new seed controllers to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	config *config.SeedControllerConfiguration,
) error {
	if err := addDefaultBackupBucketController(mgr, config); err != nil {
		return fmt.Errorf("failed to add default-backupbucket controller: %w", err)
	}

	if err := addSeedController(ctx, mgr, config); err != nil {
		return fmt.Errorf("failed to add seed controller: %w", err)
	}

	if err := addSeedLifecycleController(mgr, config); err != nil {
		return fmt.Errorf("failed to add seed-lifecycle controller: %w", err)
	}

	return nil
}

func reconcileAfter(d time.Duration) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: d}, nil
}
