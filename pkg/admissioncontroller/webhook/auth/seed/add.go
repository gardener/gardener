// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	seedauthorizergraph "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed/graph"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
)

const (
	// HandlerName is the name of this authorization webhook handler.
	HandlerName = "seedauthorizer"
	// WebhookPath is the HTTP handler path for this authorization webhook handler.
	WebhookPath = "/webhooks/auth/seed"
)

// AddToManager adds Handler to the given manager.
func (w *Webhook) AddToManager(ctx context.Context, mgr manager.Manager, enableDebugHandlers *bool) error {
	if w.Handler == nil {
		graph := seedauthorizergraph.New(mgr.GetLogger().WithName("seed-authorizer-graph"), mgr.GetClient())
		if err := graph.Setup(ctx, mgr.GetCache()); err != nil {
			return err
		}

		authorizer := NewAuthorizer(w.Logger, graph)
		w.Handler = &authorizerwebhook.Handler{Logger: w.Logger, Authorizer: authorizer}

		if ptr.Deref(enableDebugHandlers, false) {
			w.Logger.Info("Registering debug handlers")
			mgr.GetWebhookServer().Register(seedauthorizergraph.DebugHandlerPath, seedauthorizergraph.NewDebugHandler(graph))
		}
	}

	mgr.GetWebhookServer().Register(WebhookPath, w.Handler)
	return nil
}
