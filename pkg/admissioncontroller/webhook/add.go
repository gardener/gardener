// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/auditpolicy"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/authenticationconfig"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/authorizationconfig"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/internaldomainsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/kubeconfigsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/namespacedeletion"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/providersecretlabels"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/resourcesize"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/seedrestriction"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/shootkubeconfigsecretref"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/shootrestriction"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/updaterestriction"
	seedauthorizer "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed"
	shootauthorizer "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/shoot"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// AddToManager adds all webhook handlers to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	cfg *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration,
) error {
	clientSet, err := kubernetes.NewWithConfig(kubernetes.WithRESTConfig(mgr.GetConfig()))
	if err != nil {
		return fmt.Errorf("failed setting up Kubernetes client: %w", err)
	}

	if err := auditpolicy.AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", auditpolicy.HandlerName, err)
	}

	if err := authenticationconfig.AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", authenticationconfig.HandlerName, err)
	}

	if err := authorizationconfig.AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", authorizationconfig.HandlerName, err)
	}

	if err := (&internaldomainsecret.Handler{
		Logger:    mgr.GetLogger().WithName("webhook").WithName(internaldomainsecret.HandlerName),
		APIReader: mgr.GetAPIReader(),
		Scheme:    mgr.GetScheme(),
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

	if err := (&providersecretlabels.Handler{
		Logger: mgr.GetLogger().WithName("webhook").WithName(providersecretlabels.HandlerName),
		Client: mgr.GetClient(),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", providersecretlabels.HandlerName, err)
	}

	if err := (&resourcesize.Handler{
		Logger:     mgr.GetLogger().WithName("webhook").WithName(resourcesize.HandlerName),
		Config:     cfg.Server.ResourceAdmissionConfiguration,
		APIReader:  mgr.GetAPIReader(),
		RESTMapper: mgr.GetRESTMapper(),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", resourcesize.HandlerName, err)
	}

	if err := (&seedauthorizer.Webhook{
		Logger:    mgr.GetLogger().WithName("webhook").WithName(seedauthorizer.HandlerName),
		ClientSet: clientSet,
	}).AddToManager(ctx, mgr, cfg.Server.EnableDebugHandlers); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", seedauthorizer.HandlerName, err)
	}

	if err := (&seedrestriction.Handler{
		Logger:  mgr.GetLogger().WithName("webhook").WithName(seedrestriction.HandlerName),
		Client:  mgr.GetClient(),
		Decoder: admission.NewDecoder(mgr.GetScheme()),
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", seedrestriction.HandlerName, err)
	}

	if err := (&shootauthorizer.Webhook{
		Logger:    mgr.GetLogger().WithName("webhook").WithName(shootauthorizer.HandlerName),
		ClientSet: clientSet,
	}).AddToManager(ctx, mgr, cfg.Server.EnableDebugHandlers); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", shootauthorizer.HandlerName, err)
	}

	if err := (&shootrestriction.Handler{
		Logger:  mgr.GetLogger().WithName("webhook").WithName(shootrestriction.HandlerName),
		Client:  mgr.GetClient(),
		Decoder: admission.NewDecoder(mgr.GetScheme()),
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", shootrestriction.HandlerName, err)
	}

	if err := (&shootkubeconfigsecretref.Handler{
		Logger: mgr.GetLogger().WithName("webhook").WithName(shootkubeconfigsecretref.HandlerName),
		Client: mgr.GetClient(),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", shootkubeconfigsecretref.HandlerName, err)
	}

	if err := updaterestriction.AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", updaterestriction.HandlerName, err)
	}

	return nil
}
