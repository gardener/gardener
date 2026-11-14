// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/utils/graph"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
)

const (
	// HandlerName is the name of this authorization webhook handler.
	HandlerName = "seedauthorizer"
	// WebhookPath is the HTTP handler path for this authorization webhook handler.
	WebhookPath = "/webhooks/auth/seed"
)

// AddToManager adds Handler to the given manager.
func (w *Webhook) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if w.Authorizer == nil {
		g := graph.New(mgr.GetLogger().WithName("seed-authorizer-graph"), mgr.GetClient(), false)
		if err := g.Setup(ctx, mgr.GetCache()); err != nil {
			return err
		}

		w.Authorizer = NewAuthorizer(
			w.Logger,
			mgr.GetAPIReader(),
			g,
			authorizerwebhook.NewWithSelectorsChecker(ctx, w.Logger, w.ClientSet, clock.RealClock{}),
		)
	}

	if w.Handler == nil {
		w.Handler = &authorizerwebhook.Handler{
			Logger:     w.Logger,
			Authorizer: w.Authorizer,
		}
	}

	mgr.GetWebhookServer().Register(WebhookPath, w.Handler)
	return nil
}
