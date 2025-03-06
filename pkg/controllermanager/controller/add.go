// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	kubernetesclientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/bastion"
	"github.com/gardener/gardener/pkg/controllermanager/controller/certificatesigningrequest"
	"github.com/gardener/gardener/pkg/controllermanager/controller/cloudprofile"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerdeployment"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration"
	"github.com/gardener/gardener/pkg/controllermanager/controller/credentialsbinding"
	"github.com/gardener/gardener/pkg/controllermanager/controller/event"
	"github.com/gardener/gardener/pkg/controllermanager/controller/exposureclass"
	"github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	"github.com/gardener/gardener/pkg/controllermanager/controller/namespacedcloudprofile"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project"
	"github.com/gardener/gardener/pkg/controllermanager/controller/quota"
	"github.com/gardener/gardener/pkg/controllermanager/controller/secretbinding"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shootstate"
)

// AddToManager adds all controller-manager controllers to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager, cfg *controllermanagerconfigv1alpha1.ControllerManagerConfiguration) error {
	kubernetesClient, err := kubernetesclientset.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed creating Kubernetes client: %w", err)
	}

	if err := (&bastion.Reconciler{
		Config: *cfg.Controllers.Bastion,
	}).AddToManager(mgr); err != nil {
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

	if err := (&namespacedcloudprofile.Reconciler{
		Config: *cfg.Controllers.NamespacedCloudProfile,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding NamespacedCloudProfile controller: %w", err)
	}

	if err := (&controllerdeployment.Reconciler{
		Config: *cfg.Controllers.ControllerDeployment,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding ControllerDeployment controller: %w", err)
	}

	if err := controllerregistration.AddToManager(mgr, *cfg); err != nil {
		return fmt.Errorf("failed adding ControllerRegistration controller: %w", err)
	}

	if err := (&credentialsbinding.Reconciler{
		Config: *cfg.Controllers.CredentialsBinding,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding CredentialsBinding controller: %w", err)
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

	if err := project.AddToManager(mgr, *cfg); err != nil {
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

	if err := (&shootstate.Reconciler{
		Config: *cfg.Controllers.ShootState,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding shootState finalizer reconciler: %w", err)
	}

	if err := seed.AddToManager(mgr, *cfg); err != nil {
		return fmt.Errorf("failed adding Seed controller: %w", err)
	}

	if err := shoot.AddToManager(mgr, *cfg); err != nil {
		return fmt.Errorf("failed adding Shoot controller: %w", err)
	}

	return nil
}
