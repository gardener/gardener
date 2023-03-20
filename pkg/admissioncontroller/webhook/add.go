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

package webhook

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/auditpolicy"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/internaldomainsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/kubeconfigsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/namespacedeletion"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/resourcesize"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/seedrestriction"
	seedauthorizer "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed"
)

// AddToManager adds all webhook handlers to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	cfg *config.AdmissionControllerConfiguration,
) error {
	if err := (&auditpolicy.Handler{
		Logger:    mgr.GetLogger().WithName("webhook").WithName(auditpolicy.HandlerName),
		APIReader: mgr.GetAPIReader(),
		Client:    mgr.GetClient(),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", auditpolicy.HandlerName, err)
	}

	if err := (&internaldomainsecret.Handler{
		Logger:    mgr.GetLogger().WithName("webhook").WithName(internaldomainsecret.HandlerName),
		APIReader: mgr.GetAPIReader(),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", internaldomainsecret.HandlerName, err)
	}

	if err := (&kubeconfigsecret.Handler{
		Logger: mgr.GetLogger().WithName("webhook").WithName(kubeconfigsecret.HandlerName),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", kubeconfigsecret.HandlerName, err)
	}

	if err := (&namespacedeletion.Handler{
		Logger:    mgr.GetLogger().WithName("webhook").WithName(namespacedeletion.HandlerName),
		APIReader: mgr.GetAPIReader(),
		Client:    mgr.GetClient(),
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", namespacedeletion.HandlerName, err)
	}

	if err := (&resourcesize.Handler{
		Logger: mgr.GetLogger().WithName("webhook").WithName(resourcesize.HandlerName),
		Config: cfg.Server.ResourceAdmissionConfiguration,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", resourcesize.HandlerName, err)
	}

	if err := (&seedauthorizer.Handler{
		Logger: mgr.GetLogger().WithName("webhook").WithName(seedauthorizer.HandlerName),
	}).AddToManager(ctx, mgr, cfg.Server.EnableDebugHandlers); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", seedauthorizer.HandlerName, err)
	}

	if err := (&seedrestriction.Handler{
		Logger: mgr.GetLogger().WithName("webhook").WithName(seedrestriction.HandlerName),
		Client: mgr.GetClient(),
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", seedrestriction.HandlerName, err)
	}

	return nil
}
