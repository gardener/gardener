// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package seed

import (
	"context"
	"net/http"

	"k8s.io/utils/pointer"
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

		if pointer.BoolDeref(enableDebugHandlers, false) {
			h.Logger.Info("Registering debug handlers")
			mgr.GetWebhookServer().Register(seedauthorizergraph.DebugHandlerPath, seedauthorizergraph.NewDebugHandler(graph))
		}
	}

	mgr.GetWebhookServer().Register(WebhookPath, http.HandlerFunc(h.Handle))
	return nil
}
