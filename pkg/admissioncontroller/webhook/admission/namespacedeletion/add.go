// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedeletion

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "namespace_validator"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/validate-namespace-deletion"
)

// AddToManager adds Handler to the given manager.
func (h *Handler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	// Initialize caches here to ensure the readyz informer check will only succeed once informers required for this
	// handler have synced so that http requests can be served quicker with pre-synchronized caches.
	if _, err := mgr.GetCache().GetInformer(ctx, &corev1.Namespace{}); err != nil {
		return err
	}
	if _, err := mgr.GetCache().GetInformer(ctx, &gardencorev1beta1.Project{}); err != nil {
		return err
	}

	webhook := admission.
		WithCustomValidator(mgr.GetScheme(), &corev1.Namespace{}, h).
		WithRecoverPanic(true)

	mgr.GetWebhookServer().Register(WebhookPath, webhook)
	return nil
}
