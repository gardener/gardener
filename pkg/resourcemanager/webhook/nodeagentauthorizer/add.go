// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer

import (
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
)

const (
	// HandlerName is the name of this authorization webhook handler.
	HandlerName = "node-agent-authorizer"
	// WebhookPath is the HTTP handler path for this authorization webhook handler.
	WebhookPath = "/webhooks/auth/nodeagent"
)

// AddToManager adds Handler to the given manager.
func (w *Webhook) AddToManager(mgr manager.Manager, sourceClient, targetClient client.Client) error {
	if w.Handler == nil {
		authorizer := NewAuthorizer(w.Logger, sourceClient, targetClient, w.Config.MachineNamespace, ptr.Deref(w.Config.AuthorizeWithSelectors, false))
		w.Handler = &authorizerwebhook.Handler{Logger: w.Logger, Authorizer: authorizer}
	}

	mgr.GetWebhookServer().Register(WebhookPath, w.Handler)
	return nil
}
