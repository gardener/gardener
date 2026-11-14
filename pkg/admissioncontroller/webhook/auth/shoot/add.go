// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"

	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/utils/graph"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
)

const (
	// HandlerName is the name of this authorization webhook handler.
	HandlerName = "shootauthorizer"
	// WebhookPath is the HTTP handler path for this authorization webhook handler.
	WebhookPath = "/webhooks/auth/shoot"
)

// AddToManager adds Handler to the given manager.
func (w *Webhook) AddToManager(ctx context.Context, mgr manager.Manager, seedAuthorizer auth.Authorizer) error {
	if w.Handler == nil {
		g := graph.New(mgr.GetLogger().WithName("shoot-authorizer-graph"), mgr.GetClient(), true)
		if err := g.Setup(ctx, mgr.GetCache()); err != nil {
			return err
		}

		w.Handler = &authorizerwebhook.Handler{
			Logger: w.Logger,
			Authorizer: NewAuthorizer(
				w.Logger,
				mgr.GetClient(),
				mgr.GetAPIReader(),
				g,
				authorizerwebhook.NewWithSelectorsChecker(ctx, w.Logger, w.ClientSet, clock.RealClock{}),
				seedAuthorizer,
			),
		}
	}

	mgr.GetWebhookServer().Register(WebhookPath, w.Handler)

	return nil
}
