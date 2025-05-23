// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedrestriction

import (
	"context"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "seedrestriction"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/admission/seedrestriction"
)

// AddToManager adds Handler to the given manager.
func (h *Handler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	// Initialize caches here to ensure the readyz informer check will only succeed once informers required for this
	// handler have synced so that http requests can be served quicker with pre-synchronized caches.
	if _, err := mgr.GetCache().GetInformer(ctx, &gardencorev1beta1.BackupBucket{}); err != nil {
		return err
	}
	if _, err := mgr.GetCache().GetInformer(ctx, &seedmanagementv1alpha1.ManagedSeed{}); err != nil {
		return err
	}
	if _, err := mgr.GetCache().GetInformer(ctx, &gardencorev1beta1.Seed{}); err != nil {
		return err
	}
	if _, err := mgr.GetCache().GetInformer(ctx, &gardencorev1beta1.Shoot{}); err != nil {
		return err
	}

	webhook := &admission.Webhook{
		Handler:      h,
		RecoverPanic: ptr.To(true),
	}

	mgr.GetWebhookServer().Register(WebhookPath, webhook)
	return nil
}
