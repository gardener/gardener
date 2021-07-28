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

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
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
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// GardenControllerFactory contains information relevant to controllers for the Garden API group.
type GardenControllerFactory struct {
	cfg       *config.ControllerManagerConfiguration
	clientMap clientmap.ClientMap
	recorder  record.EventRecorder
	logger    logr.Logger
}

// NewGardenControllerFactory creates a new factory for controllers for the Garden API group.
func NewGardenControllerFactory(clientMap clientmap.ClientMap, cfg *config.ControllerManagerConfiguration, recorder record.EventRecorder, logger logr.Logger) *GardenControllerFactory {
	return &GardenControllerFactory{
		cfg:       cfg,
		clientMap: clientMap,
		recorder:  recorder,
		logger:    logger,
	}
}

// AddControllers adds all the controllers for the Garden API group. It also performs bootstrapping tasks.
func (f *GardenControllerFactory) AddControllers(ctx context.Context, mgr manager.Manager, k8sClient kubernetes.Interface) error {
	if err := addAllFieldIndexes(ctx, mgr.GetFieldIndexer()); err != nil {
		return fmt.Errorf("failed to setup field indexes: %w", err)
	}

	if err := f.clientMap.Start(ctx.Done()); err != nil {
		return fmt.Errorf("failed to start ClientMap: %w", err)
	}

	k8sGardenClient, err := f.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	runtime.Must(garden.BootstrapCluster(ctx, k8sGardenClient))
	f.logger.Info("Successfully bootstrapped the Garden cluster.")

	// Setup controllers
	if err := bastioncontroller.AddToManager(ctx, mgr, f.cfg.Controllers.Bastion); err != nil {
		return fmt.Errorf("failed to setup bastion controller: %w", err)
	}

	if err := csrcontroller.AddToManager(mgr, k8sClient); err != nil {
		return fmt.Errorf("failed to setup CSR controller: %w", err)
	}

	if err := cloudprofilecontroller.AddToManager(mgr, f.cfg.Controllers.CloudProfile); err != nil {
		return fmt.Errorf("failed to setup cloudprofile controller: %w", err)
	}

	if err := controllerdeploymentcontroller.AddToManager(mgr, f.cfg.Controllers.ControllerDeployment); err != nil {
		return fmt.Errorf("failed to setup controllerdeployment controller: %w", err)
	}

	if err := controllerregistrationcontroller.AddToManager(ctx, mgr, f.cfg.Controllers.ControllerRegistration); err != nil {
		return fmt.Errorf("failed to setup controllerregistration controller: %w", err)
	}

	if err := exposureclasscontroller.AddToManager(mgr, f.cfg.Controllers.ExposureClass); err != nil {
		return fmt.Errorf("failed to setup exposureclass controller: %w", err)
	}

	if eventControllerConfig := f.cfg.Controllers.Event; eventControllerConfig != nil {
		if err := eventcontroller.AddToManager(mgr, eventControllerConfig); err != nil {
			return fmt.Errorf("failed to setup event controller: %w", err)
		}
	}

	if err := managedseedsetcontroller.AddToManager(ctx, mgr, f.cfg.Controllers.ManagedSeedSet); err != nil {
		return fmt.Errorf("failed to setup managedseedset controller: %w", err)
	}

	if err := plantcontroller.AddToManager(ctx, mgr, f.clientMap, f.cfg.Controllers.Plant); err != nil {
		return fmt.Errorf("failed to setup plant controller: %w", err)
	}

	if err := projectcontroller.AddToManager(ctx, mgr, f.cfg.Controllers.Project); err != nil {
		return fmt.Errorf("failed to setup project controller: %w", err)
	}

	if err := quotacontroller.AddToManager(mgr, f.cfg.Controllers.Quota); err != nil {
		return fmt.Errorf("failed to setup quota controller: %w", err)
	}

	if err := secretbindingcontroller.AddToManager(mgr, f.cfg.Controllers.SecretBinding); err != nil {
		return fmt.Errorf("failed to setup secretbinding controller: %w", err)
	}

	if err := seedcontroller.AddToManager(ctx, mgr, f.cfg.Controllers.Seed); err != nil {
		return fmt.Errorf("failed to setup seed controller: %w", err)
	}

	if err := shootcontroller.AddToManager(mgr, &f.cfg.Controllers); err != nil {
		return fmt.Errorf("failed to setup shoot controller: %w", err)
	}

	// Done :)
	f.logger.WithValues("version", version.Get().GitVersion).Info("Gardener controller manager initialized.")

	return nil
}

// addAllFieldIndexes adds all field indexes used by gardener-controller-manager to the given FieldIndexer (i.e. cache).
// field indexes have to be added before the cache is started (i.e. before the clientmap is started)
func addAllFieldIndexes(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &gardencorev1beta1.Project{}, gardencore.ProjectNamespace, func(obj client.Object) []string {
		project, ok := obj.(*gardencorev1beta1.Project)
		if !ok {
			return []string{""}
		}
		if project.Spec.Namespace == nil {
			return []string{""}
		}
		return []string{*project.Spec.Namespace}
	}); err != nil {
		return fmt.Errorf("failed to add indexer to Project Informer: %w", err)
	}

	if err := indexer.IndexField(ctx, &gardencorev1beta1.Shoot{}, gardencore.ShootSeedName, func(obj client.Object) []string {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return []string{""}
		}
		if shoot.Spec.SeedName == nil {
			return []string{""}
		}
		return []string{*shoot.Spec.SeedName}
	}); err != nil {
		return fmt.Errorf("failed to add indexer to Shoot Informer: %w", err)
	}

	return nil
}
