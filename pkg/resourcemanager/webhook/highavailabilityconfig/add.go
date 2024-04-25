// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package highavailabilityconfig

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// HandlerName is the name of the webhook handler.
	HandlerName = "high-availability-config"
	// WebhookPath is the path at which the handler should be registered.
	WebhookPath = "/webhooks/high-availability-config"
)

// AddToManager adds Handler to the given manager.
func (h *Handler) AddToManager(mgr manager.Manager) error {
	webhook := &admission.Webhook{
		Handler:      h,
		RecoverPanic: true,
	}

	mgr.GetWebhookServer().Register(WebhookPath, webhook)
	return nil
}
