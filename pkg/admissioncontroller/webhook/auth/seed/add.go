// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"net/http"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	seedauthorizergraph "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed/graph"
)

const (
	// HandlerName is the name of this authorization webhook handler.
	HandlerName = "seedauthorizer"
	// WebhookPath is the HTTP handler path for this authorization webhook handler.
	WebhookPath = "/webhooks/auth/seed"
)

// AddToManager adds Handler to the given manager.
func (h *Handler) AddToManager(ctx context.Context, mgr manager.Manager, enableDebugHandlers *bool) error {
	if h.Authorizer == nil {
		graph := seedauthorizergraph.New(mgr.GetLogger().WithName("seed-authorizer-graph"), mgr.GetClient())
		if err := graph.Setup(ctx, mgr.GetCache()); err != nil {
			return err
		}

		h.Authorizer = NewAuthorizer(h.Logger, graph)

		if ptr.Deref(enableDebugHandlers, false) {
			h.Logger.Info("Registering debug handlers")
			mgr.GetWebhookServer().Register(seedauthorizergraph.DebugHandlerPath, seedauthorizergraph.NewDebugHandler(graph))
		}
	}

	mgr.GetWebhookServer().Register(WebhookPath, http.HandlerFunc(h.Handle))
	return nil
}
