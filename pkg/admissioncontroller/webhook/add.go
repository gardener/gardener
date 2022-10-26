// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package webhook

import (
	"context"

	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/auditpolicy"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/internaldomainsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/kubeconfigsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/namespacedeletion"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/resourcesize"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/seedrestriction"
	seedauthorizer "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed"
	seedauthorizergraph "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed/graph"
)

// AddToManager adds all webhook handlers to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	cfg *config.AdmissionControllerConfiguration,
) error {
	var (
		log         = mgr.GetLogger().WithName("webhook")
		logSeedAuth = log.WithName(seedauthorizer.AuthorizerName)
		server      = mgr.GetWebhookServer()
	)

	graph := seedauthorizergraph.New(mgr.GetLogger().WithName("seed-authorizer-graph"), mgr.GetClient())
	if err := graph.Setup(ctx, mgr.GetCache()); err != nil {
		return err
	}

	namespaceValidationHandler, err := namespacedeletion.New(ctx, log.WithName(namespacedeletion.HandlerName), mgr.GetCache())
	if err != nil {
		return err
	}

	seedRestrictionHandler, err := seedrestriction.New(ctx, log.WithName(seedrestriction.HandlerName), mgr.GetCache())
	if err != nil {
		return err
	}

	server.Register(seedauthorizer.WebhookPath, seedauthorizer.NewHandler(logSeedAuth, seedauthorizer.NewAuthorizer(logSeedAuth, graph)))
	server.Register(seedrestriction.WebhookPath, &webhook.Admission{Handler: seedRestrictionHandler, RecoverPanic: true})
	server.Register(namespacedeletion.WebhookPath, &webhook.Admission{Handler: namespaceValidationHandler, RecoverPanic: true})
	server.Register(kubeconfigsecret.WebhookPath, &webhook.Admission{Handler: kubeconfigsecret.New(log.WithName(kubeconfigsecret.HandlerName)), RecoverPanic: true})
	server.Register(resourcesize.WebhookPath, &webhook.Admission{Handler: resourcesize.New(log.WithName(resourcesize.HandlerName), cfg.Server.ResourceAdmissionConfiguration), RecoverPanic: true})
	server.Register(auditpolicy.WebhookPath, &webhook.Admission{Handler: auditpolicy.New(log.WithName(auditpolicy.HandlerName)), RecoverPanic: true})
	server.Register(internaldomainsecret.WebhookPath, &webhook.Admission{Handler: internaldomainsecret.New(log.WithName(internaldomainsecret.HandlerName)), RecoverPanic: true})

	if pointer.BoolDeref(cfg.Server.EnableDebugHandlers, false) {
		log.Info("Registering debug handlers")
		server.Register(seedauthorizergraph.DebugHandlerPath, seedauthorizergraph.NewDebugHandler(graph))
	}

	return nil
}
