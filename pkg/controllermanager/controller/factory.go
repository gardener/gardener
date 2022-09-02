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

package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	managedseedsetcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	projectcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/project"
	seedcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/seed"
	shootcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
)

// LegacyControllerFactory starts controller-manager's legacy controllers under leader election of the given manager for
// the purpose of gradually migrating to native controller-runtime controllers.
// Deprecated: this will be replaced by adding native controllers directly to the manager.
// New controllers should be implemented as native controller-runtime controllers right away and should be added to
// the manager directly.
type LegacyControllerFactory struct {
	Manager manager.Manager
	Log     logr.Logger
	Config  *config.ControllerManagerConfiguration
}

// Start starts all legacy controllers.
func (f *LegacyControllerFactory) Start(ctx context.Context) error {
	log := f.Log.WithName("controller")

	// create controllers
	managedSeedSetController, err := managedseedsetcontroller.NewManagedSeedSetController(ctx, log, f.Manager, f.Config)
	if err != nil {
		return fmt.Errorf("failed initializing ManagedSeedSet controller: %w", err)
	}

	projectController, err := projectcontroller.NewProjectController(ctx, log, f.Manager, f.Config)
	if err != nil {
		return fmt.Errorf("failed initializing Project controller: %w", err)
	}

	seedController, err := seedcontroller.NewSeedController(ctx, log, f.Manager, f.Config)
	if err != nil {
		return fmt.Errorf("failed initializing Seed controller: %w", err)
	}

	shootController, err := shootcontroller.NewShootController(ctx, log, f.Manager, f.Config)
	if err != nil {
		return fmt.Errorf("failed initializing Shoot controller: %w", err)
	}

	// run controllers
	go projectController.Run(ctx, *f.Config.Controllers.Project.ConcurrentSyncs)
	go seedController.Run(ctx, *f.Config.Controllers.Seed.ConcurrentSyncs, *f.Config.Controllers.SeedBackupBucketsCheck.ConcurrentSyncs, *f.Config.Controllers.SeedExtensionsCheck.ConcurrentSyncs)
	go shootController.Run(ctx, *f.Config.Controllers.ShootReference.ConcurrentSyncs, *f.Config.Controllers.ShootRetry.ConcurrentSyncs, *f.Config.Controllers.ShootStatusLabel.ConcurrentSyncs)
	go managedSeedSetController.Run(ctx, *f.Config.Controllers.ManagedSeedSet.ConcurrentSyncs)

	// block until shutting down
	<-ctx.Done()
	return nil
}
