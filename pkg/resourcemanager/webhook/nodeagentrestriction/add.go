// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentrestriction

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// HandlerName is the name of this webhook handler.
	HandlerName = "node-agent-restriction"
	// WebhookPath is the HTTP handler path for this webhook handler.
	WebhookPath = "/webhooks/validate-node-agent-restriction"
)

// AddToManager adds Handler to the given manager.
func (h *Handler) AddToManager(mgr manager.Manager) error {
	webhook := &admission.Webhook{
		Handler:      h,
		RecoverPanic: new(true),
	}

	mgr.GetWebhookServer().Register(WebhookPath, webhook)
	return nil
}
