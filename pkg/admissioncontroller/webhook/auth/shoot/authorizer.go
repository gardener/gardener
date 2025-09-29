// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"

	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
	authwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth"
	"github.com/gardener/gardener/pkg/utils/graph"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
)

// NewAuthorizer returns a new authorizer for requests from gardenlets running in autonomous shoots.
func NewAuthorizer(logger logr.Logger, graph graph.Interface, authorizeWithSelectors authorizerwebhook.WithSelectorsChecker) *authorizer {
	return &authorizer{
		logger:                 logger,
		graph:                  graph,
		authorizeWithSelectors: authorizeWithSelectors,
	}
}

type authorizer struct {
	logger                 logr.Logger
	graph                  graph.Interface
	authorizeWithSelectors authorizerwebhook.WithSelectorsChecker
}

var _ = auth.Authorizer(&authorizer{})

func (a *authorizer) Authorize(_ context.Context, attrs auth.Attributes) (auth.Decision, string, error) {
	shootNamespace, shootName, isAutonomousShoot, userType := shootidentity.FromUserInfoInterface(attrs.GetUser())
	if !isAutonomousShoot {
		return auth.DecisionNoOpinion, "", nil
	}

	var (
		log               = a.logger.WithValues("shootNamespace", shootNamespace, "shootName", shootName, "attributes", fmt.Sprintf("%#v", attrs), "userType", userType)
		requestAuthorizer = &authwebhook.RequestAuthorizer{
			Log:                    log,
			Graph:                  a.graph,
			AuthorizeWithSelectors: a.authorizeWithSelectors,
			ToType:                 graph.VertexTypeShoot,
			ToNamespace:            shootNamespace,
			ToName:                 shootName,
		}
		// TODO(rfranzke): Remove this once requestAuthorizer is used.
		_ = requestAuthorizer
	)

	if attrs.IsResourceRequest() {
		requestResource := schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}
		switch requestResource {
		default:
			log.Info(
				"Unhandled resource request",
				"group", attrs.GetAPIGroup(),
				"version", attrs.GetAPIVersion(),
				"resource", attrs.GetResource(),
				"verb", attrs.GetVerb(),
			)
		}
	}

	return auth.DecisionNoOpinion, "", nil
}
