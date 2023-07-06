// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	kubernetesclientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/bastion"
	"github.com/gardener/gardener/pkg/controllermanager/controller/certificatesigningrequest"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerdeployment"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration"
	"github.com/gardener/gardener/pkg/controllermanager/controller/event"
	"github.com/gardener/gardener/pkg/controllermanager/controller/exposureclass"
	"github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project"
	"github.com/gardener/gardener/pkg/controllermanager/controller/quota"
	"github.com/gardener/gardener/pkg/controllermanager/controller/secretbinding"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
)

// AddToManager adds all controller-manager controllers to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager, cfg *config.ControllerManagerConfiguration) error {
	kubernetesClient, err := kubernetesclientset.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed creating Kubernetes client: %w", err)
	}

	if err := (&bastion.Reconciler{
		Config: *cfg.Controllers.Bastion,
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding Bastion controller: %w", err)
	}

	if err := (&certificatesigningrequest.Reconciler{
		CertificatesClient: kubernetesClient.CertificatesV1().CertificateSigningRequests(),
		Config:             *cfg.Controllers.CertificateSigningRequest,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding CertificateSigningRequest controller: %w", err)
	}

	if err := (&cloudprofile.Reconciler{
		Config: *cfg.Controllers.CloudProfile,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding CloudProfile controller: %w", err)
	}

	if err := (&controllerdeployment.Reconciler{
		Config: *cfg.Controllers.ControllerDeployment,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding ControllerDeployment controller: %w", err)
	}

	if err := controllerregistration.AddToManager(ctx, mgr, *cfg); err != nil {
		return fmt.Errorf("failed adding ControllerRegistration controller: %w", err)
	}

	if config := cfg.Controllers.Event; config != nil {
		if err := (&event.Reconciler{
			Config: *config,
		}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding Event controller: %w", err)
		}
	}

	if err := (&exposureclass.Reconciler{
		Config: *cfg.Controllers.ExposureClass,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding ExposureClass controller: %w", err)
	}

	if err := (&managedseedset.Reconciler{
		Config: *cfg.Controllers.ManagedSeedSet,
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding ManagedSeedSet controller: %w", err)
	}

	if err := project.AddToManager(ctx, mgr, *cfg); err != nil {
		return fmt.Errorf("failed adding Project controller: %w", err)
	}

	if err := (&quota.Reconciler{
		Config: *cfg.Controllers.Quota,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Quota controller: %w", err)
	}

	if err := (&secretbinding.Reconciler{
		Config: *cfg.Controllers.SecretBinding,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding SecretBinding controller: %w", err)
	}

	if err := seed.AddToManager(ctx, mgr, *cfg); err != nil {
		return fmt.Errorf("failed adding Seed controller: %w", err)
	}

	if err := shoot.AddToManager(ctx, mgr, *cfg); err != nil {
		return fmt.Errorf("failed adding Shoot controller: %w", err)
	}

	return nil
}
