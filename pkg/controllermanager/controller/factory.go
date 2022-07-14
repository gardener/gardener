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
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	bastioncontroller "github.com/gardener/gardener/pkg/controllermanager/controller/bastion"
	csrcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/certificatesigningrequest"
	cloudprofilecontroller "github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile"
	controllerdeploymentcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/controllerdeployment"
	controllerregistrationcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration"
	eventcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/event"
	exposureclasscontroller "github.com/gardener/gardener/pkg/controllermanager/controller/exposureclass"
	managedseedsetcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	plantcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/plant"
	projectcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/project"
	quotacontroller "github.com/gardener/gardener/pkg/controllermanager/controller/quota"
	secretbindingcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/secretbinding"
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
	bastionController, err := bastioncontroller.NewBastionController(ctx, log, f.clientMap, f.Config.Controllers.Bastion.MaxLifetime.Duration)
	if err != nil {
		return fmt.Errorf("failed initializing Bastion controller: %w", err)
	}

	cloudProfileController, err := cloudprofilecontroller.NewCloudProfileController(ctx, log, f.clientMap, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing CloudProfile controller: %w", err)
	}

	controllerDeploymentController, err := controllerdeploymentcontroller.New(ctx, log, f.clientMap)
	if err != nil {
		return fmt.Errorf("failed initializing ControllerDeployment controller: %w", err)
	}

	controllerRegistrationController, err := controllerregistrationcontroller.NewController(ctx, log, f.clientMap)
	if err != nil {
		return fmt.Errorf("failed initializing ControllerRegistration controller: %w", err)
	}

	csrController, err := csrcontroller.NewCSRController(ctx, log, f.clientMap)
	if err != nil {
		return fmt.Errorf("failed initializing CSR controller: %w", err)
	}

	exposureClassController, err := exposureclasscontroller.NewExposureClassController(ctx, log, f.clientMap, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing ExposureClass controller: %w", err)
	}

	managedSeedSetController, err := managedseedsetcontroller.NewManagedSeedSetController(ctx, log, f.clientMap, f.Config, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing ManagedSeedSet controller: %w", err)
	}

	plantController, err := plantcontroller.NewController(ctx, log, f.clientMap, f.Config)
	if err != nil {
		return fmt.Errorf("failed initializing Plant controller: %w", err)
	}

	projectController, err := projectcontroller.NewProjectController(ctx, log, clock.RealClock{}, f.clientMap, f.Config, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing Project controller: %w", err)
	}

	quotaController, err := quotacontroller.NewQuotaController(ctx, log, f.clientMap, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing Quota controller: %w", err)
	}

	secretBindingController, err := secretbindingcontroller.NewSecretBindingController(ctx, log, f.clientMap, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing SecretBinding controller: %w", err)
	}

	seedController, err := seedcontroller.NewSeedController(ctx, log, f.clientMap, f.Config)
	if err != nil {
		return fmt.Errorf("failed initializing Seed controller: %w", err)
	}

	shootController, err := shootcontroller.NewShootController(ctx, log, f.clientMap, f.Config, f.recorder)
	if err != nil {
		return fmt.Errorf("failed initializing Shoot controller: %w", err)
	}

	var eventController *eventcontroller.Controller
	if eventControllerConfig := f.Config.Controllers.Event; eventControllerConfig != nil {
		eventController, err = eventcontroller.NewController(ctx, log, f.clientMap, eventControllerConfig)
		if err != nil {
			return fmt.Errorf("failed initializing Event controller: %w", err)
		}
	}

	// run controllers
	go bastionController.Run(ctx, *f.Config.Controllers.Bastion.ConcurrentSyncs)
	go cloudProfileController.Run(ctx, *f.Config.Controllers.CloudProfile.ConcurrentSyncs)
	go controllerDeploymentController.Run(ctx, *f.Config.Controllers.ControllerDeployment.ConcurrentSyncs)
	go controllerRegistrationController.Run(ctx, *f.Config.Controllers.ControllerRegistration.ConcurrentSyncs)
	go csrController.Run(ctx, 1)
	go plantController.Run(ctx, *f.Config.Controllers.Plant.ConcurrentSyncs)
	go projectController.Run(ctx, *f.Config.Controllers.Project.ConcurrentSyncs)
	go quotaController.Run(ctx, *f.Config.Controllers.Quota.ConcurrentSyncs)
	go secretBindingController.Run(ctx, *f.Config.Controllers.SecretBinding.ConcurrentSyncs, *f.Config.Controllers.SecretBindingProvider.ConcurrentSyncs)
	go seedController.Run(ctx, *f.Config.Controllers.Seed.ConcurrentSyncs)
	go shootController.Run(ctx, *f.Config.Controllers.ShootMaintenance.ConcurrentSyncs, *f.Config.Controllers.ShootQuota.ConcurrentSyncs, *f.Config.Controllers.ShootHibernation.ConcurrentSyncs, *f.Config.Controllers.ShootReference.ConcurrentSyncs, *f.Config.Controllers.ShootRetry.ConcurrentSyncs, *f.Config.Controllers.ShootConditions.ConcurrentSyncs, *f.Config.Controllers.ShootStatusLabel.ConcurrentSyncs)
	go exposureClassController.Run(ctx, *f.Config.Controllers.ExposureClass.ConcurrentSyncs)
	go managedSeedSetController.Run(ctx, *f.Config.Controllers.ManagedSeedSet.ConcurrentSyncs)

	if eventController != nil {
		go eventController.Run(ctx)
	}

	// block until shutting down
	<-ctx.Done()
	return nil
}
